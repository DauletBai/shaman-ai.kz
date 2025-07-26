// internal/handlers/legal.go
package handlers

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	//"path/filepath"
	"time"
	"shaman-ai.kz/internal/utils" 
)

type LegalDocResponse struct {
	Title   string        `json:"Title"`
	Content template.HTML `json:"Content"`
	UpdateDate string        `json:"UpdateDate"`
}

func GetLegalDocHandler(docType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var filePath string
		var title string
		//basePath := "templates/legal"

		switch docType {
		case "terms":
			filePath = "templates/legal/terms_of_use.html"
			title = "Условия использования Sham'an AI"
		case "privacy":
			filePath = "templates/legal/privacy_policy.html"
			title = "Политика конфиденциальности Sham'an AI"
		default:
			http.Error(w, "Запрошен неизвестный документ", http.StatusNotFound)
			return
		}

		content, err := utils.LoadHTMLContentFromFile(filePath)
		if err != nil {
			slog.Error("Не удалось загрузить юридический документ", "type", docType, "path", filePath, "error", err)
			http.Error(w, "Не удалось загрузить документ", http.StatusInternalServerError)
			return
		}

		updateDate := time.Now().Format("02.01.2006") 

		respData := LegalDocResponse{
			Title:   title,
			Content: content,
			UpdateDate: updateDate,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(respData); err != nil {
		    slog.Error("Ошибка кодирования JSON для юридического документа", "error", err)
		}
	}
}