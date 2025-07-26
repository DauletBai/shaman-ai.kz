// internal/middleware/csrf.go
package middleware

import (
	"log/slog"
	"net/http"
	"os" 

	"github.com/justinas/nosurf"
)

// NoSurfMiddleware обеспечивает CSRF-защиту.
// isProduction: true для production окружения (Secure cookie).
func NoSurfMiddleware(next http.Handler, isProduction bool) http.Handler {
	csrfHandler := nosurf.New(next)

	// Проверяем наличие CSRF_AUTH_KEY, но напрямую в nosurf его не передаем,
	// так как стандартная библиотека сама управляет ключом для генерации токенов.
	// Наличие ключа в ENV важно для общей безопасности и если он используется где-то еще.
	csrfAuthKey := os.Getenv("CSRF_AUTH_KEY")
	if csrfAuthKey == "" {
		if isProduction {
			// В production отсутствие ключа должно быть критической ошибкой,
			// даже если nosurf сам сгенерирует временный.
			// Это больше для осведомленности и для политик безопасности.
			slog.Error("КРИТИЧЕСКАЯ ОШИБКА: CSRF_AUTH_KEY не установлена в переменных окружения для production! Это серьезная уязвимость.")
			// Можно даже паниковать здесь или возвращать обработчик, который всегда выдает ошибку.
			// panic("CSRF_AUTH_KEY must be set in production environment")
		} else {
			slog.Warn("CSRF_AUTH_KEY не установлена! Используется ключ, генерируемый nosurf по умолчанию (НЕБЕЗОПАСНО для production, если токены должны быть консистентны между перезапусками).")
		}
	}

	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   isProduction, // true для HTTPS в production
		SameSite: http.SameSiteLaxMode,
		// MaxAge: 0, // Сессионная cookie по умолчанию
	})

	csrfHandler.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Warn("Неудачная проверка CSRF токена", "path", r.URL.Path, "method", r.Method, "reason", nosurf.Reason(r))
		http.Error(w, "Ошибка безопасности: Неверный или отсутствующий CSRF токен.", http.StatusForbidden)
	}))

	return csrfHandler
}