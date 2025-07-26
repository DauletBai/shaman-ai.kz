// internal/handlers/user_settings_handlers.go
package handlers

import (
	"log/slog"
	"net/http"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/models"

	"github.com/alexedwards/scs/v2"
)

// UserSettingsHandlers содержит зависимости для действий на странице настроек пользователя.
type UserSettingsHandlers struct {
	SessionManager *scs.SessionManager
	// RenderPage и NewPageData здесь не нужны, т.к. рендеринг страницы идет через pages.go
}

func NewUserSettingsHandlers(sm *scs.SessionManager) *UserSettingsHandlers {
	return &UserSettingsHandlers{
		SessionManager: sm,
	}
}

// UpdateUserSettingsHandler обрабатывает POST-запросы для обновления настроек пользователя.
func (ush *UserSettingsHandlers) UpdateUserSettingsHandler(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok || currentUser == nil {
		http.Error(w, "Пользователь не аутентифицирован", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		slog.Error("UpdateUserSettingsHandler: Ошибка парсинга формы", "userID", currentUser.ID, "error", err)
		ush.SessionManager.Put(r.Context(), "flash_error", "Произошла ошибка при обработке данных.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	ttsEnabled := r.PostForm.Get("tts_enabled_default") == "on"

	err := db.UpdateUserTTSEnabledDefault(currentUser.ID, ttsEnabled)
	if err != nil {
		slog.Error("UpdateUserSettingsHandler: Ошибка обновления настроек TTS в БД", "userID", currentUser.ID, "error", err)
		ush.SessionManager.Put(r.Context(), "flash_error", "Не удалось сохранить настройки. Попробуйте позже.")
	} else {
		slog.Info("Настройки пользователя успешно обновлены", "userID", currentUser.ID, "tts_enabled", ttsEnabled)
		ush.SessionManager.Put(r.Context(), "flash_success", "Настройки успешно сохранены!")
		
		// Обновляем значение в объекте пользователя в сессии
		currentUser.TTSEnabledDefault = &ttsEnabled
		ush.SessionManager.Put(r.Context(), string(middleware.UserContextKey), currentUser)
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}