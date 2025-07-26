// internal/middleware/admin_auth.go
package middleware

import (
	"log/slog"
	"net/http"
	"shaman-ai.kz/internal/db"
	//"shaman-ai.kz/internal/models" 
)

// RequireRole проверяет, имеет ли аутентифицированный пользователь одну из разрешенных ролей.
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDContextKey).(int64)
			if !ok || userID == 0 {
				// Это не должно произойти, если RequireAuthentication отработал раньше
				slog.Error("RequireRole: UserID не найден в контексте, хотя ожидался.")
				http.Error(w, "Доступ запрещен: пользователь не аутентифицирован.", http.StatusUnauthorized)
				return
			}

			user, err := db.GetUserByID(userID) // Получаем пользователя с его ролью
			if err != nil {
				slog.Error("RequireRole: Ошибка получения пользователя из БД", "userID", userID, "error", err)
				http.Error(w, "Ошибка сервера при проверке прав доступа.", http.StatusInternalServerError)
				return
			}
			if user == nil || user.RoleName == nil {
				slog.Warn("RequireRole: Пользователь не найден или не имеет роли", "userID", userID)
				http.Error(w, "Доступ запрещен: не удалось определить роль пользователя.", http.StatusForbidden)
				return
			}

			userRole := *user.RoleName
			isAllowed := false
			for _, allowedRole := range allowedRoles {
				if userRole == allowedRole {
					isAllowed = true
					break
				}
			}

			if !isAllowed {
				slog.Warn("Доступ запрещен: недостаточная роль", "userID", userID, "userRole", userRole, "requiredRoles", allowedRoles, "path", r.URL.Path)
				http.Error(w, "Доступ запрещен: у вас нет необходимых прав для доступа к этому ресурсу.", http.StatusForbidden)
				return
			}

			// slog.Debug("Доступ разрешен", "userID", userID, "userRole", userRole, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}