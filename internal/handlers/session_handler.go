// internal/handlers/session_handler.go
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/middleware"
	"github.com/google/uuid" 
)

// ListUserChatSessionsHandler возвращает список сессий пользователя
func ListUserChatSessionsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(middleware.UserIDContextKey).(int64)
		if !ok || userID == 0 { /* ... ошибка аутентификации ... */ return }

		sessions, err := db.GetUserChatSessions(userID, 50) // Лимит на 50 сессий
		if err != nil { /* ... ошибка сервера ... */ return }

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	}
}

// CreateUserChatSessionHandler создает новую сессию и возвращает ее UUID
func CreateUserChatSessionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(middleware.UserIDContextKey).(int64)
		if !ok || userID == 0 { http.Error(w, "Ошибка аутентификации", http.StatusUnauthorized); return }

		newUUID := uuid.NewString()
		// Заголовок можно взять из первого сообщения или оставить пустым
		initialTitle := "Новый диалог" // Или пусто

		err := db.CreateChatSession(userID, newUUID, initialTitle)
		if err != nil {
			http.Error(w, "Не удалось создать новую сессию", http.StatusInternalServerError)
			return
		}

		slog.Info("Создана новая сессия чата", "user_id", userID, "session_uuid", newUUID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"session_uuid": newUUID, "title": initialTitle})
	}
}