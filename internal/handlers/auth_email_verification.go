// internal/handlers/auth_email_verification.go
package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"shaman-ai.kz/internal/db"
)

// безопасныйПрефикс возвращает начало строки для безопасного логирования.
func безопасныйПрефикс(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// VerifyEmailHandler обрабатывает переход по ссылке верификации.
func (h *AuthHandlers) VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")
	if rawToken == "" {
		h.SessionManager.Put(r.Context(), "flash_error", "Неверная или отсутствующая ссылка для верификации email.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID, err := db.VerifyUserEmail(rawToken) // Используем db
	if err != nil {
		slog.Warn("Ошибка верификации email", "token_prefix", безопасныйПрефикс(rawToken, 10), "error", err.Error())
		flashMsg := "Не удалось подтвердить email. " + err.Error()
		if strings.Contains(err.Error(), "уже подтвержден") {
			flashMsg = "Ваш email уже был подтвержден."
			h.SessionManager.Put(r.Context(), "flash_success", flashMsg)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if strings.Contains(err.Error(), "истек срок") || strings.Contains(err.Error(), "неверная или истекшая ссылка") {
			flashMsg = "Ссылка для подтверждения email недействительна или истек ее срок действия. Пожалуйста, запросите новую ссылку или обратитесь в поддержку."
		}
		h.SessionManager.Put(r.Context(), "flash_error", flashMsg)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	slog.Info("Email успешно подтвержден для пользователя", "userID", userID)
	h.SessionManager.Put(r.Context(), "flash_success", "Ваш email успешно подтвержден! Теперь вы можете войти.")

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}