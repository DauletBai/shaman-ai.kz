// internal/handlers/trial_handler.go)
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/llm"
	"time"
)

// --- Новый обработчик для пробного диалога ---
type TrialDialogueRequest struct {
	Prompt string `json:"prompt"`
}

// Использует только generalSystemPrompt
func TrialDialogueHandler(appConfig *config.Config, generalSystemPrompt string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": "Метод не поддерживается"}`, http.StatusMethodNotAllowed)
			return
		}

		var req TrialDialogueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("TrialDialogueHandler: Ошибка декодирования JSON", "error", err)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": "Некорректный формат запроса"}`, http.StatusBadRequest)
			return
		}

		if req.Prompt == "" {
			slog.Warn("TrialDialogueHandler: Получен пустой промпт")
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": "Промпт не может быть пустым"}`, http.StatusBadRequest)
			return
		}

		slog.Info("TrialDialogueHandler: Получен пробный запрос", "prompt_length", len(req.Prompt))

		// Используем только generalSystemPrompt, без истории
		history := []db.Message{} // Пустая история

		// Уменьшенный таймаут для пробных запросов, если нужно
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RemoteLLM.RequestTimeoutSeconds/2)*time.Second)
		defer cancel()

		// ИСПРАВЛЕНИЕ: Используем _ для игнорирования данных о токенах
		aiResponse, _, err := llm.GenerateRemoteResponse(ctx, appConfig.RemoteLLM, generalSystemPrompt, history, req.Prompt)
		if err != nil {
			slog.Error("TrialDialogueHandler: Ошибка при генерации ответа LLM", "error", err)
			w.Header().Set("Content-Type", "application/json")
			// Не выводим детальную ошибку LLM пользователю триала
			http.Error(w, `{"error": "Не удалось получить ответ от AI. Попробуйте позже."}`, http.StatusInternalServerError)
			return
		}
		slog.Info("TrialDialogueHandler: Ответ от LLM получен", "response_length", len(aiResponse))

		resp := DialogueResponse{Response: aiResponse} // Используем ту же структуру ответа
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("TrialDialogueHandler: Ошибка кодирования JSON-ответа", "error", err)
		}
	}
}