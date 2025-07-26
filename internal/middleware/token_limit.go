// internal/middleware/token_limit.go
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/models"
	"strings"
	"time"
)

// TokenLimitExceededContextKey - ключ для передачи в контекст информации о превышении лимита
const TokenLimitExceededContextKey contextKey = "isTokenLimitExceeded"

// CheckTokenLimit - это middleware, проверяющий, не превысил ли пользователь месячный лимит токенов.
func CheckTokenLimit(appConfig *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем текущего пользователя из контекста (должен быть добавлен предыдущим middleware)
			user, ok := r.Context().Value(UserContextKey).(*models.User)
			if !ok || user == nil {
				// Это не должно произойти, если middleware вызывается после InjectUserData/RequireAuthentication
				slog.Warn("CheckTokenLimit: пользователь не найден в контексте. Пропускаем проверку.")
				next.ServeHTTP(w, r)
				return
			}

			// Проверка не нужна для администраторов
			if user.RoleName != nil && *user.RoleName == models.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}
			
			// Проверка на истечение расчетного периода
			// Если с даты последней оплаты прошел месяц, а подписка все еще активна,
			// это может означать проблему с вебхуком об оплате. В этом случае мы сбрасываем счетчик,
			// чтобы не блокировать пользователя. Основная логика сброса будет в обработчике вебхука.
			if user.BillingCycleAnchorDate != nil && time.Since(*user.BillingCycleAnchorDate) > (31*24*time.Hour) {
				slog.Info("Прошел месяц с последней точки оплаты, счетчик токенов сброшен для пользователя", "userID", user.ID)
				db.IncrementTokenUsage(user.ID, -user.TokensUsedInputThisPeriod, -user.TokensUsedOutputThisPeriod) // Сброс в 0
				user.TokensUsedInputThisPeriod = 0
				user.TokensUsedOutputThisPeriod = 0
			}

			// Рассчитываем стоимость использованных токенов
			costInputUSD := (float64(user.TokensUsedInputThisPeriod) / 1000000.0) * appConfig.RemoteLLM.TokenCostInputPerMillion
			costOutputUSD := (float64(user.TokensUsedOutputThisPeriod) / 1000000.0) * appConfig.RemoteLLM.TokenCostOutputPerMillion
			totalCostUSD := costInputUSD + costOutputUSD
			totalCostKZT := totalCostUSD * appConfig.Billing.USDToKZTRate

			// Сравниваем с лимитом
			if totalCostKZT >= appConfig.TokenMonthlyLimitKZT {
				slog.Warn("Пользователь превысил месячный лимит токенов", "userID", user.ID, "spent_kzt", totalCostKZT, "limit_kzt", appConfig.TokenMonthlyLimitKZT)

				// Для API-запросов возвращаем ошибку
				if strings.HasPrefix(r.URL.Path, "/api/") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden) // 403 Forbidden
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Вы превысили месячный лимит использования. Услуга будет возобновлена после следующего списания абонентской платы.",
					})
					return
				}

				// Для обычных страниц просто добавляем флаг в контекст
				ctx := context.WithValue(r.Context(), TokenLimitExceededContextKey, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Если лимит не превышен, пропускаем запрос дальше
			next.ServeHTTP(w, r)
		})
	}
}