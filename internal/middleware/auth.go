package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"shaman-ai.kz/internal/db" // Для загрузки пользователя
	"shaman-ai.kz/internal/models"

	"github.com/alexedwards/scs/v2"
)

type contextKey string

const UserIDContextKey contextKey = "userID"
const IsAuthenticatedContextKey contextKey = "isAuthenticated"
const LoggedInUserIDContextKey contextKey = "loggedInUserID" // Можно удалить, если UserContextKey будет использоваться везде
const UserContextKey contextKey = "user"                     // Новый ключ для хранения всего объекта User

func RequireAuthentication(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := sessionManager.GetInt64(r.Context(), string(UserIDContextKey))
			if userID == 0 {
				slog.Warn("Access denied: user not authenticated", "path", r.URL.Path)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			// Загружаем пользователя здесь, чтобы он был доступен во всех защищенных хендлерах
			user, err := db.GetUserByID(userID)
			if err != nil || user == nil {
				slog.Error("RequireAuthentication: User not found in DB or error", "userID", userID, "error", err)
				// Можно сбросить сессию или просто запретить доступ
				sessionManager.Remove(r.Context(), string(UserIDContextKey))
				http.Redirect(w, r, "/login?err=session_invalid", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
			ctx = context.WithValue(ctx, UserContextKey, user) // Кладем всего пользователя в контекст
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func InjectUserData(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			isAuthenticated := false
			var loggedInUserID int64
			var currentUser *models.User

			// Проверяем, был ли пользователь уже загружен RequireAuthentication
			userFromAuth, ok := ctx.Value(UserContextKey).(*models.User)
			if ok && userFromAuth != nil {
				currentUser = userFromAuth
				loggedInUserID = currentUser.ID
				isAuthenticated = true
			} else {
				// Если нет, проверяем сессию (для публичных страниц)
				sessionUserID := sessionManager.GetInt64(ctx, string(UserIDContextKey))
				if sessionUserID != 0 {
					// Загружаем пользователя, если он есть в сессии, но не в контексте еще
					// (например, для страниц, которые не требуют RequireAuthentication, но показывают имя пользователя)
					userFromDB, err := db.GetUserByID(sessionUserID)
					if err == nil && userFromDB != nil {
						currentUser = userFromDB
						loggedInUserID = currentUser.ID
						isAuthenticated = true
						ctx = context.WithValue(ctx, UserContextKey, currentUser) // Добавляем в контекст
					} else if err != nil {
						slog.Warn("InjectUserData: error fetching user from session ID", "userID", sessionUserID, "error", err)
					}
				}
			}

			ctx = context.WithValue(ctx, IsAuthenticatedContextKey, isAuthenticated)
			if isAuthenticated {
				ctx = context.WithValue(ctx, LoggedInUserIDContextKey, loggedInUserID) // Старый ключ для обратной совместимости, если нужен
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}