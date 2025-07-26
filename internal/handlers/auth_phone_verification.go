// internal/handlers/auth_phone_verification.go
package handlers

import (
	"log/slog"
	"math/rand"
	"net/http"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/models"
	"shaman-ai.kz/internal/sms"
	"strconv"
)

// VerifyPhonePageHandler отображает страницу для ввода кода из СМС.
func (h *AuthHandlers) VerifyPhonePageHandler(w http.ResponseWriter, r *http.Request) {
	data := h.NewPageData(r)
	data.PageTitle = "Подтверждение телефона"
	// ИСПРАВЛЕНИЕ ЗДЕСЬ: используем h.Render, как определено в структуре AuthHandlers
	h.Render(w, r, "verify_phone.html", data)
}

// ProcessPhoneVerificationHandler обрабатывает код из СМС.
func (h *AuthHandlers) ProcessPhoneVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	currentUser, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok || currentUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	code := r.PostFormValue("verification_code")
	if code == "" {
		h.SessionManager.Put(r.Context(), "flash_error", "Пожалуйста, введите код подтверждения.")
		http.Redirect(w, r, "/verify-phone", http.StatusSeeOther)
		return
	}

	err := db.VerifyUserPhone(currentUser.ID, code)
	if err != nil {
		slog.Warn("Ошибка верификации номера телефона", "userID", currentUser.ID, "error", err)
		h.SessionManager.Put(r.Context(), "flash_error", "Не удалось подтвердить номер: "+err.Error())
		http.Redirect(w, r, "/verify-phone", http.StatusSeeOther)
		return
	}

	// После верификации телефона, проверяем, верифицирован ли email.
	updatedUser, err := db.GetUserByID(currentUser.ID)
	if err != nil || updatedUser == nil {
		slog.Error("Не удалось получить данные пользователя после верификации телефона", "userID", currentUser.ID, "error", err)
		h.SessionManager.Put(r.Context(), "flash_error", "Произошла ошибка, пожалуйста, войдите снова.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if !updatedUser.IsEmailVerified {
		h.SessionManager.Put(r.Context(), "flash_success", "Ваш номер телефона подтвержден! Теперь, пожалуйста, подтвердите ваш email, перейдя по ссылке в письме.")
		http.Redirect(w, r, "/login", http.StatusSeeOther) // Отправляем на логин с сообщением о необходимости проверить почту
		return
	}

	// Если и телефон, и email подтверждены - поздравляем и отправляем на дашборд.
	h.SessionManager.Put(r.Context(), "flash_success", "Ваш аккаунт полностью активирован!")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// ResendPhoneVerificationHandler обрабатывает запрос на повторную отправку СМС.
func (h *AuthHandlers) ResendPhoneVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	currentUser, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok || currentUser == nil || currentUser.Phone == nil {
		h.SessionManager.Put(r.Context(), "flash_error", "Не удалось определить пользователя или номер телефона.")
		http.Redirect(w, r, "/verify-phone", http.StatusSeeOther)
		return
	}

	// Генерируем, сохраняем и отправляем новый код
	code := strconv.Itoa(100000 + rand.Intn(900000)) // 6-значный код

	if err := db.SetPhoneVerificationCode(currentUser.ID, code); err != nil {
		h.SessionManager.Put(r.Context(), "flash_error", "Не удалось сгенерировать новый код. Попробуйте позже.")
		http.Redirect(w, r, "/verify-phone", http.StatusSeeOther)
		return
	}

	if err := sms.SendSMS(h.AppConfig, *currentUser.Phone, "Ваш код подтверждения для Shaman AI: "+code); err != nil {
		h.SessionManager.Put(r.Context(), "flash_error", "Не удалось отправить СМС. Попробуйте позже.")
		http.Redirect(w, r, "/verify-phone", http.StatusSeeOther)
		return
	}

	h.SessionManager.Put(r.Context(), "flash_success", "Новый код подтверждения отправлен на ваш номер.")
	http.Redirect(w, r, "/verify-phone", http.StatusSeeOther)
}