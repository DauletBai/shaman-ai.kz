// internal/llm/api_client.go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
)

// Структура для сообщений в запросе к API
type APIRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Структура тела запроса к API
type APIRequestBody struct {
	Model       string              `json:"model"`
	Messages    []APIRequestMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream"`
}

// Usage содержит информацию о количестве использованных токенов
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Структура для разбора ответа API (упрощенная, совместимая с OpenAI-подобными)
type APIResponseBody struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage     `json:"usage"` // Поле для токенов
	Error *APIError `json:"error,omitempty"` // Поле для ошибок API
}

// Структура для ошибок API
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// GenerateRemoteResponse отправляет запрос к удаленному LLM API и возвращает Usage
func GenerateRemoteResponse(ctx context.Context, llmConfig config.RemoteLLMConfig, systemPrompt string, history []db.Message, userPrompt string) (string, *Usage, error) {

	// Формируем историю сообщений для API
	messages := []APIRequestMessage{}
	if systemPrompt != "" {
		messages = append(messages, APIRequestMessage{Role: "system", Content: systemPrompt})
	}

	// Добавляем историю (если есть) - Убедитесь, что db.Message имеет поля Role и Content
	for _, msg := range history {
		apiRole := "user"
		if msg.Role == "assistant" {
			apiRole = "assistant"
		}
		// Пропускаем системные сообщения из истории, т.к. оно уже добавлено
		if msg.Role != "system" {
			messages = append(messages, APIRequestMessage{Role: apiRole, Content: msg.Content})
		}
	}

	// Добавляем текущий промпт пользователя
	messages = append(messages, APIRequestMessage{Role: "user", Content: userPrompt})

	// Формируем тело запроса
	requestBody := APIRequestBody{
		Model:       llmConfig.ModelName,
		Messages:    messages,
		Stream:      false,
		MaxTokens:   2048,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", nil, fmt.Errorf("ошибка кодирования запроса для LLM API: %w", err)
	}

	// Создаем HTTP клиент с таймаутом из конфига
	client := &http.Client{
		Timeout: time.Duration(llmConfig.RequestTimeoutSeconds) * time.Second,
	}

	// Создаем HTTP запрос с контекстом
	req, err := http.NewRequestWithContext(ctx, "POST", llmConfig.APIUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, fmt.Errorf("ошибка создания запроса к LLM API: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+llmConfig.APIKey)

	slog.Info("Отправка запроса к Remote LLM API", "url", llmConfig.APIUrl, "model", llmConfig.ModelName)

	// Отправляем запрос
	resp, err := client.Do(req)
	if err != nil {
		// Проверяем ошибку таймаута или отмены контекста
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			slog.Error("Тайм-аут или отмена при запросе к LLM API", "url", llmConfig.APIUrl, "error", err)
			return "", nil, fmt.Errorf("LLM API не ответил вовремя или запрос был отменен (%w)", err)
		}
		// Другие сетевые ошибки
		return "", nil, fmt.Errorf("ошибка отправки запроса к LLM API (%s): %w", llmConfig.APIUrl, err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("ошибка чтения ответа от LLM API: %w", err)
	}

	// Парсим JSON ответ
	var apiResp APIResponseBody
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		slog.Error("Не удалось декодировать JSON от LLM API", "status_code", resp.StatusCode, "response_body", string(bodyBytes), "error", err)
		// Если статус не ОК, пытаемся вернуть текст ошибки
		if resp.StatusCode >= 400 {
			return "", nil, fmt.Errorf("ошибка от LLM API: статус %d, тело: %s", resp.StatusCode, string(bodyBytes))
		}
		return "", nil, fmt.Errorf("ошибка декодирования JSON ответа от LLM API: %w", err)
	}

	// Проверяем на ошибки внутри JSON ответа API
	if apiResp.Error != nil {
		slog.Error("LLM API вернул ошибку в JSON", "type", apiResp.Error.Type, "message", apiResp.Error.Message, "code", apiResp.Error.Code)
		return "", nil, fmt.Errorf("ошибка LLM API: %s (%s)", apiResp.Error.Message, apiResp.Error.Type)
	}

	// Проверяем статус HTTP ответа
	if resp.StatusCode >= 400 {
		slog.Error("LLM API вернул ошибку HTTP", "status_code", resp.StatusCode, "response_body", string(bodyBytes))
		// Используем ошибку из JSON, если она есть, иначе общую
		errMsg := fmt.Sprintf("ошибка LLM API: статус %d", resp.StatusCode)
		if apiResp.Error != nil {
			errMsg = fmt.Sprintf("ошибка LLM API: %s (%s)", apiResp.Error.Message, apiResp.Error.Type)
		}
		return "", nil, errors.New(errMsg)
	}

	// Проверяем наличие ответа
	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		slog.Warn("Ответ от LLM API не содержит текста", "response_body", string(bodyBytes))
		return "", nil, errors.New("получен пустой ответ от LLM API")
	}

	// Получаем ответ
	aiResponse := apiResp.Choices[0].Message.Content
	slog.Info("Сгенерирован ответ Remote LLM", "response_length", len(aiResponse), "usage", apiResp.Usage)

	return aiResponse, &apiResp.Usage, nil
}