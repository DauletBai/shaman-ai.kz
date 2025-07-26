// internal/handlers/user_profile_handlers.go
package handlers

import (
	"log/slog"
	"net/http"
	"net/url"
	"shaman-ai.kz/internal/auth"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/models"
	"shaman-ai.kz/internal/validation"
	"strings"
	// "time" // Не используется напрямую здесь, но может понадобиться для других функций

	"github.com/alexedwards/scs/v2"
)

// UserProfileHandlers содержит зависимости для действий, связанных с профилем пользователя.
type UserProfileHandlers struct {
	SessionManager *scs.SessionManager
	// RenderPage не нужен здесь, так как рендеринг страницы профиля остается в pages.go
	// NewPageData также не нужен здесь напрямую
}

func NewUserProfileHandlers(sm *scs.SessionManager) *UserProfileHandlers {
	return &UserProfileHandlers{
		SessionManager: sm,
	}
}

// ProfileUpdateForm определяет структуру для формы обновления профиля.
type ProfileUpdateForm struct {
	FirstName string `form:"first_name" validate:"required,alpha_space"`
	LastName  string `form:"last_name" validate:"required,alpha_space"`
	Phone     string `form:"phone" validate:"omitempty,valid_phone"` // Телефон может быть опциональным
}

// PasswordChangeForm определяет структуру для формы смены пароля.
type PasswordChangeForm struct {
	CurrentPassword    string `form:"current_password" validate:"required"`
	NewPassword        string `form:"new_password" validate:"required,min=8,complex_password"`
	ConfirmNewPassword string `form:"confirm_new_password" validate:"required,eqfield=NewPassword"`
}

// UpdateProfileHandler обрабатывает POST-запросы для обновления информации профиля пользователя.
func (uph *UserProfileHandlers) UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok || currentUser == nil {
		http.Error(w, "Пользователь не аутентифицирован", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		slog.Error("UpdateProfileHandler: Ошибка парсинга формы", "userID", currentUser.ID, "error", err)
		uph.SessionManager.Put(r.Context(), "flash_error", "Произошла ошибка при обработке данных.")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	form := ProfileUpdateForm{
		FirstName: strings.TrimSpace(r.PostForm.Get("first_name")),
		LastName:  strings.TrimSpace(r.PostForm.Get("last_name")),
		Phone:     strings.TrimSpace(r.PostForm.Get("phone")),
	}

	validationErrors := validation.ValidateStruct(form)
	if len(validationErrors) > 0 {
		slog.Warn("UpdateProfileHandler: Ошибки валидации", "userID", currentUser.ID, "errors", validationErrors)
		uph.SessionManager.Put(r.Context(), "profile_update_errors", validationErrors)
		uph.SessionManager.Put(r.Context(), "profile_update_form_values", r.PostForm) // Сохраняем все значения формы
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	var phonePtr *string
	if form.Phone != "" {
		phonePtr = &form.Phone
	}

	err := db.UpdateUserProfile(currentUser.ID, auth.SanitizeName(form.FirstName), auth.SanitizeName(form.LastName), phonePtr)
	if err != nil {
		slog.Error("UpdateProfileHandler: Ошибка обновления профиля в БД", "userID", currentUser.ID, "error", err)
		uph.SessionManager.Put(r.Context(), "flash_error", "Не удалось обновить профиль. Попробуйте позже.")
	} else {
		slog.Info("Профиль пользователя успешно обновлен", "userID", currentUser.ID)
		uph.SessionManager.Put(r.Context(), "flash_success", "Профиль успешно обновлен!")
		// Обновляем данные пользователя в сессии
		currentUser.FirstName = auth.SanitizeName(form.FirstName)
		currentUser.LastName = auth.SanitizeName(form.LastName)
		currentUser.Phone = phonePtr
		uph.SessionManager.Put(r.Context(), string(middleware.UserContextKey), currentUser)
	}
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

// ChangePasswordHandler обрабатывает POST-запросы для смены пароля пользователя.
func (uph *UserProfileHandlers) ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok || currentUser == nil {
		http.Error(w, "Пользователь не аутентифицирован", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		slog.Error("ChangePasswordHandler: Ошибка парсинга формы", "userID", currentUser.ID, "error", err)
		uph.SessionManager.Put(r.Context(), "flash_error_pw", "Произошла ошибка при обработке данных.")
		http.Redirect(w, r, "/profile#change-password-section", http.StatusSeeOther)
		return
	}

	form := PasswordChangeForm{
		CurrentPassword:    r.PostForm.Get("current_password"),
		NewPassword:        r.PostForm.Get("new_password"),
		ConfirmNewPassword: r.PostForm.Get("confirm_new_password"),
	}
	
	validationErrors := validation.ValidateStruct(form)
	if validationErrors == nil {
		validationErrors = url.Values{} 
	}

	userFromDB, err := db.GetUserByID(currentUser.ID) 
	if err != nil || userFromDB == nil {
		slog.Error("ChangePasswordHandler: Не удалось получить пользователя из БД", "userID", currentUser.ID, "error", err)
		uph.SessionManager.Put(r.Context(), "flash_error_pw", "Ошибка сервера. Попробуйте позже.")
		http.Redirect(w, r, "/profile#change-password-section", http.StatusSeeOther)
		return
	}

	if !auth.CheckPasswordHash(form.CurrentPassword, userFromDB.PasswordHash) {
		validationErrors.Add("current_password", "Текущий пароль неверен.")
	}

	if len(validationErrors) > 0 {
		slog.Warn("ChangePasswordHandler: Ошибки валидации или неверный текущий пароль", "userID", currentUser.ID, "errors", validationErrors)
		uph.SessionManager.Put(r.Context(), "password_change_errors", validationErrors)
		http.Redirect(w, r, "/profile#change-password-section", http.StatusSeeOther)
		return
	}

	newHashedPassword, err := auth.HashPassword(form.NewPassword)
	if err != nil {
		slog.Error("ChangePasswordHandler: Ошибка хеширования нового пароля", "userID", currentUser.ID, "error", err)
		uph.SessionManager.Put(r.Context(), "flash_error_pw", "Ошибка при смене пароля. Попробуйте позже.")
		http.Redirect(w, r, "/profile#change-password-section", http.StatusSeeOther)
		return
	}

	err = db.UpdateUserPassword(currentUser.ID, newHashedPassword)
	if err != nil {
		slog.Error("ChangePasswordHandler: Ошибка обновления пароля в БД", "userID", currentUser.ID, "error", err)
		uph.SessionManager.Put(r.Context(), "flash_error_pw", "Не удалось сменить пароль. Попробуйте позже.")
	} else {
		slog.Info("Пароль пользователя успешно изменен", "userID", currentUser.ID)
		uph.SessionManager.Put(r.Context(), "flash_success", "Пароль успешно изменен!")
	}
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}