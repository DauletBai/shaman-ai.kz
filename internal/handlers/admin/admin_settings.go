// internal/handlers/admin/admin_settings.go
package adminhandlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/handlers"
	// "shaman-ai.kz/internal/config"
)

// type AdminSettingsPageData struct {
// 	handlers.PageData 
// 	// SiteNameFromConfig string
// 	// LLMModelFromConfig string
// 	// GlobalMaintenanceMode bool
// }

func AdminSettingsPageHandler(app *handlers.AppHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageData := app.NewPageData(r) // Используем существующую PageData
		pageData.AdminPageTitle = "Настройки Сервиса"

		appSettings, err := db.GetAllAppSettings()
		if err != nil {
			slog.Error("AdminSettingsPageHandler: не удалось загрузить настройки приложения", "error", err)
			app.SessionManager.Put(r.Context(), "flash_error", "Ошибка загрузки текущих настроек.")
			// Продолжаем с пустыми настройками или значениями из config.yaml как fallback
			appSettings = make(map[string]string) 
		}
		pageData.AppSettings = appSettings // Передаем карту настроек

		// Для отображения текущих путей к промптам из config.yaml, если в БД они не переопределены (пусты)
		if pageData.AppSettings["shaman_system_prompt_content"] == "" {
			pageData.AppSettings["shaman_system_prompt_path_from_config"] = app.Config.RemoteLLM.ShamanSystemPromptPath
		}
		if pageData.AppSettings["general_system_prompt_content"] == "" {
			pageData.AppSettings["general_system_prompt_path_from_config"] = app.Config.RemoteLLM.GeneralSystemPromptPath
		}


		app.RenderAdminPage(w, r, "settings_page.html", pageData)
	}
}

func AdminUpdateSettingsHandler(app *handlers.AppHandlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			slog.Error("AdminUpdateSettingsHandler: ошибка парсинга формы", "error", err)
			app.SessionManager.Put(r.Context(), "flash_error", "Ошибка обработки данных формы.")
			http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
			return
		}

		// Получаем значения из формы
		siteName := r.PostForm.Get("site_name")
		siteDescription := r.PostForm.Get("site_description")
		maintenanceMode := r.PostForm.Get("maintenance_mode") // Будет "on" или ""
		trialDialogueEnabled := r.PostForm.Get("trial_dialogue_enabled") // Будет "on" или ""
		defaultLLMModel := r.PostForm.Get("default_llm_model")
		
		shamanPromptContent := r.PostForm.Get("shaman_system_prompt_content")
		generalPromptContent := r.PostForm.Get("general_system_prompt_content")


		// Обновляем настройки в БД
		settingsToUpdate := map[string]string{
			"site_name":              siteName,
			"site_description":       siteDescription,
			"maintenance_mode":       boolToString(maintenanceMode == "on"),
			"trial_dialogue_enabled": boolToString(trialDialogueEnabled == "on"),
			"default_llm_model":      defaultLLMModel,
			"shaman_system_prompt_content": shamanPromptContent,
			"general_system_prompt_content": generalPromptContent,
		}

		var updateErrors []string
		for key, value := range settingsToUpdate {
			// Получаем описание для существующей настройки, чтобы не затереть его
			currentSetting, _ := db.GetSetting(key)
			var currentDesc string
			if currentSetting != nil {
				currentDesc = currentSetting.Description
			}

			errDb := db.UpdateSetting(key, value, currentDesc)
			if errDb != nil {
				slog.Error("AdminUpdateSettingsHandler: не удалось обновить настройку", "key", key, "error", errDb)
				updateErrors = append(updateErrors, fmt.Sprintf("Ошибка сохранения '%s'", key))
			}
		}

		if len(updateErrors) > 0 {
			app.SessionManager.Put(r.Context(), "flash_error", "Некоторые настройки не удалось сохранить: "+strings.Join(updateErrors, ", "))
		} else {
			app.SessionManager.Put(r.Context(), "flash_success", "Настройки успешно обновлены.")
		}
		
		// Важно: Изменение этих настроек в БД не означает, что приложение сразу их подхватит.
		// Для `site_name` и `site_description` из `config.Config` это потребует перезапуска или механизма горячей перезагрузки конфига.
		// `maintenance_mode` должен проверяться в middleware на каждом запросе.
		// Системные промпты, если хранятся в БД, должны загружаться перед каждым запросом к LLM или кешироваться.

		http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}