// internal/middleware/subscription.go
package middleware

import (
	"log/slog"
	"net/http"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/models"
	"strings" 
	"time"

	"github.com/alexedwards/scs/v2"
)

// RequireActiveSubscription проверяет, есть ли у пользователя активная подписка.
// Если нет, перенаправляет на страницу подписки или возвращает ошибку.
func RequireActiveSubscription(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDContextKey).(int64)
			if !ok || userID == 0 {
				slog.Error("RequireActiveSubscription: UserID не найден в контексте, хотя ожидался.")
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			status, currentPeriodEnd, err := db.GetUserSubscriptionStatus(userID)
			if err != nil {
				slog.Error("RequireActiveSubscription: Ошибка получения статуса подписки", "userID", userID, "error", err)
				http.Error(w, "Ошибка сервера при проверке вашей подписки. Пожалуйста, попробуйте позже.", http.StatusInternalServerError)
				return
			}

			isActive := false
			if status == models.SubscriptionStatusActive {
				if currentPeriodEnd == nil || currentPeriodEnd.After(time.Now()) {
					isActive = true
				} else {
					slog.Info("Подписка пользователя истекла (currentPeriodEnd в прошлом)", "userID", userID, "status", status, "periodEnd", currentPeriodEnd)
				}
			}

			if !isActive {
				slog.Warn("Доступ запрещен: неактивная подписка", "userID", userID, "status", status, "currentPeriodEnd", currentPeriodEnd)
				
				sessionManager.Put(r.Context(), "redirectAfterSubscription", r.URL.RequestURI())

				if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("Accept") == "application/json" {
					http.Error(w, "Для доступа к этому ресурсу требуется активная подписка.", http.StatusForbidden)
				} else {
					http.Redirect(w, r, "/subscribe", http.StatusSeeOther)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}