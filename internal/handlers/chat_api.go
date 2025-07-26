// internal/handlers/chat_api.go
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/llm"
	"shaman-ai.kz/internal/middleware"

	"github.com/google/uuid"
)

const maxUploadSize = 10 * 1024 * 1024 // 10 MB

func DialogueWithFileHandler(appConfig *config.Config, shamanSystemPrompt string, generalSystemPrompt string) http.HandlerFunc {
	if appConfig.UploadPath == "" {
		slog.Error("Критическая ошибка: путь для загрузки файлов (UploadPath) не сконфигурирован!")
	} else {
		if err := os.MkdirAll(appConfig.UploadPath, os.ModePerm); err != nil {
			slog.Error("Не удалось создать папку для загрузок при инициализации DialogueWithFileHandler", "path", appConfig.UploadPath, "error", err)
		} else {
			slog.Info("Директория для загрузки файлов проверена/создана", "path", appConfig.UploadPath)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := r.Context().Value(middleware.UserIDContextKey).(int64)
		if !ok || userID == 0 {
			http.Error(w, "Ошибка аутентификации", http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024*1024)

		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			slog.Error("Ошибка парсинга multipart формы", "userID", userID, "error", err)
			if strings.Contains(err.Error(), "request body too large") {
				http.Error(w, fmt.Sprintf("Файл слишком большой. Максимальный размер: %dMB", maxUploadSize/(1024*1024)), http.StatusBadRequest)
			} else {
				http.Error(w, "Ошибка обработки формы", http.StatusBadRequest)
			}
			return
		}

		userPrompt := r.FormValue("prompt")
		chatSessionUUID := r.FormValue("chat_session_uuid")

		if chatSessionUUID == "" {
			http.Error(w, "ChatSessionUUID обязателен", http.StatusBadRequest)
			return
		}
		sessionMeta, err := db.GetChatSessionMeta(chatSessionUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "Сессия чата не найдена", http.StatusNotFound)
				return
			}
			slog.Error("Ошибка получения метаданных сессии при загрузке файла", "uuid", chatSessionUUID, "userID", userID, "error", err)
			http.Error(w, "Ошибка сервера при получении данных сессии", http.StatusInternalServerError)
			return
		}
		if sessionMeta.UserID != userID {
			slog.Warn("Попытка загрузки файла в чужую сессию чата", "user_id", userID, "session_owner_id", sessionMeta.UserID, "session_uuid", chatSessionUUID)
			http.Error(w, "Доступ запрещен к данной сессии чата", http.StatusForbidden)
			return
		}

		slog.Info("Получен запрос (возможно с файлом) от пользователя", "user_id", userID, "chat_uuid", chatSessionUUID, "prompt_length", len(userPrompt))

		var uploadedFile multipart.File
		var originalFilename string
		var savedFilePath string
		var fileType string

		file, header, errFile := r.FormFile("file")
		if errFile == nil {
			defer file.Close()
			uploadedFile = file
			originalFilename = header.Filename

			slog.Info("Получен файл", "filename", originalFilename, "size", header.Size, "mime_header", header.Header.Get("Content-Type"))

			uploadDir := appConfig.UploadPath
			if uploadDir == "" {
				slog.Error("UploadPath не установлен в конфигурации, используется временный fallback './uploads_emergency'")
				uploadDir = "./uploads_emergency"
				_ = os.MkdirAll(uploadDir, os.ModePerm)
			}

			fileExtension := filepath.Ext(originalFilename)
			newFilename := fmt.Sprintf("%d_%s%s", userID, uuid.NewString(), fileExtension)
			savedFilePath = filepath.Join(uploadDir, newFilename)

			dst, errCreate := os.Create(savedFilePath)
			if errCreate != nil {
				slog.Error("Не удалось создать файл на сервере", "path", savedFilePath, "error", errCreate)
				http.Error(w, "Ошибка сервера при сохранении файла", http.StatusInternalServerError)
				return
			}
			defer dst.Close()

			if _, errCopy := io.Copy(dst, uploadedFile); errCopy != nil {
				slog.Error("Не удалось скопировать содержимое файла", "path", savedFilePath, "error", errCopy)
				http.Error(w, "Ошибка сервера при сохранении файла", http.StatusInternalServerError)
				return
			}
			slog.Info("Файл успешно сохранен", "path", savedFilePath)

			contentType := header.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "image/") {
				fileType = "image"
			} else if strings.Contains(contentType, "pdf") || strings.Contains(contentType, "document") || strings.Contains(contentType, "text") {
				fileType = "document"
			}

		} else if !errors.Is(errFile, http.ErrMissingFile) {
			slog.Error("Ошибка при получении файла из формы", "userID", userID, "error", errFile)
			http.Error(w, "Ошибка при обработке файла", http.StatusBadRequest)
			return
		}

		llmPrompt := userPrompt
		if savedFilePath != "" {
			if fileType == "image" {
				llmPrompt += fmt.Sprintf("\n\n[Прикреплено изображение: %s. Опиши его или ответь на вопрос с его учетом.]", originalFilename)
				slog.Warn("Обработка изображений для LLM не реализована в текущем API клиенте. Передан только текст.")
			}
			if fileType == "document" {
				extractedText := ""
				if strings.HasSuffix(strings.ToLower(originalFilename), ".txt") {
					contentBytes, errRead := os.ReadFile(savedFilePath)
					if errRead == nil {
						extractedText = string(contentBytes)
					} else {
						slog.Error("Ошибка чтения текстового файла", "path", savedFilePath, "error", errRead)
					}
				} else {
					slog.Warn("Извлечение текста из не-TXT документов не реализовано.", "filename", originalFilename)
				}

				if extractedText != "" {
					maxTextLength := 4000
					if len(extractedText) > maxTextLength {
						extractedText = extractedText[:maxTextLength] + "...\n[текст документа был сокращен]"
					}
					llmPrompt += fmt.Sprintf("\n\n[Извлеченный текст из документа '%s']:\n%s\n[/конец текста из документа]", originalFilename, extractedText)
				} else if savedFilePath != "" {
					llmPrompt += fmt.Sprintf("\n\n[Прикреплен документ: %s. Извлечение текста не удалось или не поддерживается для этого типа.]", originalFilename)
				}
			}
		}

		currentSystemPrompt := generalSystemPrompt
		if isShamanRequest(llmPrompt) {
			currentSystemPrompt = shamanSystemPrompt
			slog.Info("Активирован режим 'Шаман' для запроса (с файлом).", "userID", userID, "chat_uuid", chatSessionUUID)
		} else {
			slog.Info("Активирован общий режим для запроса (с файлом).", "userID", userID, "chat_uuid", chatSessionUUID)
		}

		const historyLimit = 10
		history, errHist := db.GetMessagesForChatSession(chatSessionUUID, historyLimit)
		if errHist != nil {
			slog.Error("Ошибка получения истории для сессии (с файлом)", "chat_uuid", chatSessionUUID, "userID", userID, "error", errHist)
			history = []db.Message{}
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(appConfig.RemoteLLM.RequestTimeoutSeconds+20)*time.Second)
		defer cancel()

		aiResponse, usage, errAI := llm.GenerateRemoteResponse(ctx, appConfig.RemoteLLM, currentSystemPrompt, history, llmPrompt)
		if errAI != nil {
			slog.Error("Ошибка при генерации ответа Remote LLM (с файлом)", "user_id", userID, "chat_uuid", chatSessionUUID, "error", errAI)
			http.Error(w, fmt.Sprintf("Ошибка взаимодействия с ИИ: %v", errAI), http.StatusInternalServerError)
			return
		}
		slog.Info("Ответ от Remote LLM получен (с файлом)", "user_id", userID, "chat_uuid", chatSessionUUID, "response_length", len(aiResponse))

		promptToSave := userPrompt
		if originalFilename != "" {
			promptToSave += fmt.Sprintf(" (Прикреплен файл: %s)", originalFilename)
		}
		errSave := db.SaveChatMessage(userID, chatSessionUUID, promptToSave, aiResponse)
		if errSave != nil {
			slog.Error("Не удалось сохранить сообщение в БД (с файлом)", "user_id", userID, "chat_uuid", chatSessionUUID, "error", errSave)
		}

		// Инкрементируем счетчик токенов пользователя
		if usage != nil {
			errToken := db.IncrementTokenUsage(userID, usage.PromptTokens, usage.CompletionTokens)
			if errToken != nil {
				// Это не критичная ошибка для пользователя, но важная для нас, поэтому логируем
				slog.Error("Не удалось обновить счетчик токенов для пользователя", "user_id", userID, "error", errToken)
			} else {
				slog.Info("Счетчик токенов успешно обновлен", "user_id", userID, "input_tokens", usage.PromptTokens, "output_tokens", usage.CompletionTokens)
			}
		}

		resp := DialogueResponse{
			Response: aiResponse,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Ошибка кодирования/отправки JSON-ответа (с файлом)", "user_id", userID, "error", err)
		}
	}
}

type DialogueRequest struct {
	Prompt          string `json:"prompt"`
	ChatSessionUUID string `json:"chat_session_uuid"`
}

type DialogueResponse struct {
	Response string `json:"response"`
}

var shamanKeywords = []string{
	"здоровье", "симптом", "симптомы", "болит", "больно", "болезнь", "недомогание", "недуг",
	"лечение", "лечить", "вылечить", "избавиться от", "помоги с", "проблема с",
	"диагноз", "диагностика", "анализы", "обследование", "врач", "доктор", "медик", "медицина",
	"клиника", "больница", "специалист", "рецепт", "таблетки", "лекарство",
	"психосоматика", "гнм", "германская новая медицина", "хамер", "dhs", "сбп",
	"биологический конфликт", "эмоциональная причина", "психолог", "психотерапевт", "психиатр",
	"душа", "эмоции", "чувства", "переживания",
	"аллергия", "астма", "давление", "мигрень", "бессонница", "депрессия", "апатия",
	"тревога", "стресс", "паническая атака", "страх", "фобия",
	"голова", "сердце", "желудок", "кишечник", "печень", "почки", "легкие", "бронхи",
	"кожа", "сыпь", "зуд", "экзема", "псориаз",
	"спина", "поясница", "шея", "сустав", "мышцы", "кость",
	"ухо", "глаз", "нос", "горло", "зубы",
	"свист", "шум", "звон", "онемение", "покалывание", "головокружение", "тошнота", "рвота",
	"слабость", "усталость", "температура", "жар", "озноб", "кашель", "насморк",
	"отек", "опухоль", "воспаление", "инфекция", "вирус", "бактерия",
	"что со мной", "почему я так себя чувствую", "что это может быть", "из-за чего это",
	"как мне быть", "что делать если",
}

var personalIndicators = []string{
	"у меня", "меня беспокоит", "я чувствую", "мои ", "мой ", "моя ", "мне ", "со мной",
	"я страдаю", "я болею",
}

func isShamanRequest(prompt string) bool {
	lowerPrompt := strings.ToLower(prompt)
	for _, keyword := range shamanKeywords {
		if strings.Contains(lowerPrompt, strings.ToLower(keyword)) {
			return true
		}
	}
	hasPersonalIndicator := false
	for _, indicator := range personalIndicators {
		if strings.Contains(lowerPrompt, strings.ToLower(indicator)) {
			hasPersonalIndicator = true
			break
		}
	}
	if hasPersonalIndicator {
		if strings.Contains(lowerPrompt, "проблем") || strings.Contains(lowerPrompt, "вопрос о здоровь") || strings.Contains(lowerPrompt, "самочувстви") {
			return true
		}
	}
	if strings.Contains(lowerPrompt, "помоги") && (strings.Contains(lowerPrompt, "здоровь") || strings.Contains(lowerPrompt, "симптом") || strings.Contains(lowerPrompt, "недуг")) {
		return true
	}
	return false
}

func ListChatSessionsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := r.Context().Value(middleware.UserIDContextKey).(int64)
		if !ok || userID == 0 {
			http.Error(w, "Не авторизован", http.StatusUnauthorized)
			return
		}

		const sessionListLimit = 50
		sessions, err := db.GetUserChatSessions(userID, sessionListLimit)
		if err != nil {
			slog.Error("Ошибка получения списка сессий пользователя", "user_id", userID, "error", err)
			http.Error(w, "Ошибка сервера при получении списка сессий", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	}
}

func GetChatSessionMessagesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := r.Context().Value(middleware.UserIDContextKey).(int64)
		if !ok || userID == 0 {
			http.Error(w, "Не авторизован", http.StatusUnauthorized)
			return
		}

		sessionUUID := r.URL.Query().Get("uuid")
		if sessionUUID == "" {
			http.Error(w, "Параметр 'uuid' сессии чата обязателен", http.StatusBadRequest)
			return
		}

		const messagesLimit = 200
		messages, err := db.GetMessagesForChatSession(sessionUUID, messagesLimit)
		if err != nil {
			slog.Error("Ошибка получения сообщений сессии", "uuid", sessionUUID, "user_id", userID, "error", err)
			http.Error(w, "Ошибка сервера при получении сообщений сессии", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	}
}

func CreateNewChatSessionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := r.Context().Value(middleware.UserIDContextKey).(int64)
		if !ok || userID == 0 {
			http.Error(w, "Не авторизован", http.StatusUnauthorized)
			return
		}

		newUUID := uuid.NewString()
		initialTitle := "Новый диалог от " + time.Now().Format("02.01.06 15:04")

		err := db.CreateChatSession(userID, newUUID, initialTitle)
		if err != nil {
			slog.Error("Ошибка создания новой сессии в БД", "user_id", userID, "error", err)
			http.Error(w, "Не удалось создать новую сессию", http.StatusInternalServerError)
			return
		}

		slog.Info("Создана новая сессия чата", "user_id", userID, "session_uuid", newUUID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"uuid":       newUUID,
			"title":      initialTitle,
			"created_at": time.Now(),
			"updated_at": time.Now(),
		})
	}
}