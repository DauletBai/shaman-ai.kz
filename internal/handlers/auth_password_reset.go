// internal/handlers/auth_password_reset.go
package handlers

import (
	"database/sql"
	"net/http"
	"shaman-ai.kz/internal/auth" 
	"shaman-ai.kz/internal/db"
	"strings"
	"errors"
	"log/slog"
	"time"
    "net/url" 
)

// Предположим, у вас есть функция для отправки email
func SendPasswordResetEmail(toEmail, resetLink string) error {
 slog.Info("ПСЕВДО-ОТПРАВКА: Письмо для сброса пароля отправлено", "to", toEmail, "link", resetLink)
 // Здесь будет реальная интеграция с email-сервисом
 return nil
}

// ForgotPasswordPageHandler отображает страницу запроса сброса пароля.
func (h *AuthHandlers) ForgotPasswordPageHandler(w http.ResponseWriter, r *http.Request) {
	data := h.NewPageData(r)
	data.PageTitle = "Восстановление пароля"
	data.Form = struct{ Email string }{}
	h.Render(w, r, "forgot_password.html", data)
}

// RequestPasswordResetHandler обрабатывает запрос на сброс пароля.
func (h *AuthHandlers) RequestPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	email := strings.ToLower(r.PostForm.Get("email"))
	data := h.NewPageData(r)
	data.PageTitle = "Восстановление пароля"
	data.Form = struct{ Email string }{Email: email}

	if email == "" {
		data.Errors = url.Values{"email": {"Email обязателен."}}
		w.WriteHeader(http.StatusBadRequest)
		h.Render(w, r, "forgot_password.html", data)
		return
	}

	user, err := db.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Не сообщаем пользователю, что email не найден, из соображений безопасности
			slog.Info("Запрос на сброс пароля для несуществующего email", "email", email)
			h.SessionManager.Put(r.Context(), "flash_success", "Если такой email зарегистрирован, на него будет отправлена инструкция по сбросу пароля.")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		slog.Error("Ошибка поиска пользователя при запросе сброса пароля", "email", email, "error", err)
		h.SessionManager.Put(r.Context(), "flash_error", "Произошла ошибка. Попробуйте позже.")
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}

	rawToken, errToken := db.GenerateSecureToken(32)
	if errToken != nil {
		slog.Error("Ошибка генерации токена сброса пароля", "userID", user.ID, "error", errToken)
		h.SessionManager.Put(r.Context(), "flash_error", "Произошла ошибка. Попробуйте позже.")
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}

	if err := db.SetPasswordResetToken(user.ID, rawToken); err != nil {
		h.SessionManager.Put(r.Context(), "flash_error", "Произошла ошибка при установке токена. Попробуйте позже.")
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}
    
    // TODO: Заменить на реальную отправку email
	// resetLink := fmt.Sprintf("%s/reset-password?token=%s", h.AppConfig.BaseURL, rawToken)
	// if err := SendPasswordResetEmail(user.Email, resetLink); err != nil {
	//  slog.Error("Ошибка отправки email для сброса пароля", "userID", user.ID, "error", err)
    //      // Можно показать ошибку или продолжить с сообщением об успехе, чтобы не раскрывать статус email
	// }

	h.SessionManager.Put(r.Context(), "flash_success", "Если такой email зарегистрирован, на него будет отправлена инструкция по сбросу пароля.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ResetPasswordPageHandler отображает страницу установки нового пароля.
func (h *AuthHandlers) ResetPasswordPageHandler(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")
	data := h.NewPageData(r)
	data.PageTitle = "Установка нового пароля"
	data.Form = struct { Token string; Password string; ConfirmPassword string } {Token: rawToken}

	if rawToken == "" {
		h.SessionManager.Put(r.Context(), "flash_error", "Неверная или отсутствующая ссылка для сброса пароля.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := db.GetUserByPasswordResetToken(rawToken)
	if err != nil || user.PasswordResetTokenExpiresAt == nil || time.Now().After(*user.PasswordResetTokenExpiresAt) {
		errMsg := "Ссылка для сброса пароля недействительна или истек ее срок действия."
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Ошибка проверки токена сброса пароля", "error", err)
			errMsg = "Ошибка при проверке токена. Попробуйте запросить сброс пароля еще раз."
		}
		h.SessionManager.Put(r.Context(), "flash_error", errMsg)
		db.ClearPasswordResetToken(user.ID) // Очищаем невалидный токен
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}
	// Передаем токен в шаблон для формы
	h.Render(w, r, "reset_password.html", data)
}

// ProcessPasswordResetHandler обрабатывает установку нового пароля.
func (h *AuthHandlers) ProcessPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	rawToken := r.PostForm.Get("token")
	password := r.PostForm.Get("password")
	confirmPassword := r.PostForm.Get("confirm_password")

	data := h.NewPageData(r)
	data.PageTitle = "Установка нового пароля"
    data.Form = struct { Token string; Password string; ConfirmPassword string } {Token: rawToken}


	if rawToken == "" { // Дополнительная проверка
		h.SessionManager.Put(r.Context(), "flash_error", "Ошибка: токен отсутствует.")
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}
    
    validationErrors := url.Values{}
    if password == "" { validationErrors.Add("password", "Пароль не может быть пустым.") }
    if confirmPassword == "" { validationErrors.Add("confirm_password", "Подтверждение пароля не может быть пустым.") }
    if password != confirmPassword { validationErrors.Add("confirm_password", "Пароли не совпадают.") }
    if !auth.IsPasswordComplex(password) { validationErrors.Add("password", "Пароль должен содержать буквы, цифры и символы, и быть не менее 8 символов.")}


	user, err := db.GetUserByPasswordResetToken(rawToken)
	if err != nil || user.PasswordResetTokenExpiresAt == nil || time.Now().After(*user.PasswordResetTokenExpiresAt) {
        errMsg := "Ссылка для сброса пароля недействительна или истек ее срок действия. Пожалуйста, запросите сброс пароля снова."
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Ошибка проверки токена при установке нового пароля", "error", err)
            errMsg = "Ошибка сервера при проверке токена."
		}
        if user != nil { // Если пользователь найден, но токен истек/невалиден
		    db.ClearPasswordResetToken(user.ID)
        }
		h.SessionManager.Put(r.Context(), "flash_error", errMsg)
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}

    if len(validationErrors) > 0 {
        data.Errors = validationErrors
        w.WriteHeader(http.StatusBadRequest)
        h.Render(w, r, "reset_password.html", data)
        return
    }

	newHashedPassword, errHash := auth.HashPassword(password)
	if errHash != nil {
		slog.Error("Ошибка хеширования нового пароля", "userID", user.ID, "error", errHash)
        data.Errors = url.Values{"general": {"Произошла ошибка при установке нового пароля."}}
        w.WriteHeader(http.StatusInternalServerError)
        h.Render(w, r, "reset_password.html", data)
		return
	}

	if err := db.UpdateUserPassword(user.ID, newHashedPassword); err != nil {
        data.Errors = url.Values{"general": {"Не удалось обновить пароль. Попробуйте позже."}}
        w.WriteHeader(http.StatusInternalServerError)
        h.Render(w, r, "reset_password.html", data)
		return
	}

	db.ClearPasswordResetToken(user.ID) // Важно очистить токен после успешной смены

	h.SessionManager.Put(r.Context(), "flash_success", "Пароль успешно изменен. Теперь вы можете войти с новым паролем.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}