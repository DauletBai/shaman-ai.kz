// internal/handlers/admin/admin_reports.go
package adminhandlers

import (
	"log/slog" // <--- ДОБАВЛЕН slog
	"net/http"
	"shaman-ai.kz/internal/db" // <--- ДОБАВЛЕН db
	"shaman-ai.kz/internal/handlers"
)

func AdminReportsPageHandler(app *handlers.AppHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := app.NewPageData(r)
		data.AdminPageTitle = "Отчеты и Статистика"

		stats, err := db.GetDashboardStats()
		if err != nil {
			slog.Error("AdminReportsPageHandler: не удалось получить статистику для отчетов", "error", err)
			// Можно показать ошибку на странице или пустые значения
			// Для простоты, если есть ошибка, stats будет nil, и шаблон это обработает
		}
		data.Stats = stats 

		app.RenderAdminPage(w, r, "reports_page.html", data)
	}
}