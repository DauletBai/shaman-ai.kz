// internal/middleware/ratelimit.go
package middleware

import (
	"log/slog"
	"net/http"
	"strings" 
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ClientLimiter хранит информацию о лимитере для каждого IP
type ClientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	clients = make(map[string]*ClientLimiter)
	mu      sync.Mutex
)

// init запускает горутину для периодической очистки старых записей клиентов.
func init() {
	go cleanupClients()
}

func cleanupClients() {
	for {
		// Пауза перед следующей очисткой
		time.Sleep(10 * time.Minute) // Например, каждые 10 минут

		mu.Lock()
		for ip, client := range clients {
			// Удаляем запись, если клиент не появлялся более 15 минут (или другого интервала)
			if time.Since(client.lastSeen) > 15*time.Minute {
				delete(clients, ip)
				slog.Debug("Удален лимитер для неактивного IP", "ip", ip)
			}
		}
		mu.Unlock()
	}
}

// RateLimitMiddleware ограничивает количество запросов с одного IP.
// rps - это количество разрешенных запросов в секунду.
// burst - это максимальное количество запросов, которые могут быть обработаны в "пачке" (burst).
func RateLimitMiddleware(next http.Handler, rps float64, burst int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем IP-адрес клиента.
		// r.RemoteAddr может содержать порт, поэтому мы его отсекаем.
		// В реальных условиях за прокси IP может быть в заголовках X-Forwarded-For или X-Real-IP.
		var clientIP string
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			clientIP = strings.Split(xff, ",")[0] // Берем первый IP, если их несколько
		} else {
			clientIP = r.Header.Get("X-Real-IP")
		}

		if clientIP == "" {
			clientIP = strings.Split(r.RemoteAddr, ":")[0]
		}

		mu.Lock()
		// Проверяем, есть ли уже лимитер для этого IP.
		clientData, found := clients[clientIP]
		if !found {
			// Создаем новый лимитер для нового IP.
			clientData = &ClientLimiter{
				limiter: rate.NewLimiter(rate.Limit(rps), burst),
			}
			clients[clientIP] = clientData
			slog.Debug("Создан новый лимитер", "ip", clientIP, "rps", rps, "burst", burst)
		}
		// Обновляем время последнего обращения.
		clientData.lastSeen = time.Now()
		limiterInstance := clientData.limiter
		mu.Unlock()

		// Проверяем, разрешен ли запрос текущим лимитером.
		if !limiterInstance.Allow() {
			slog.Warn("Превышен лимит запросов (Rate Limit)", "ip", clientIP, "path", r.URL.Path)
			http.Error(w, "Слишком много запросов. Пожалуйста, попробуйте позже.", http.StatusTooManyRequests)
			return
		}

		// Если запрос разрешен, передаем его следующему обработчику.
		next.ServeHTTP(w, r)
	})
}