// internal/handlers/admin/admin_dashboard.go
package adminhandlers

import (
	"net/http"
	"shaman-ai.kz/internal/handlers" 
)

func AdminDashboardPageHandler(app *handlers.AppHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := app.NewPageData(r)
		data.AdminPageTitle = "Панель администратора - Обзор"
		// Здесь можно будет добавить логику для сбора статистики для дашборда

		app.RenderAdminPage(w, r, "dashboard.html", data)
	}
}