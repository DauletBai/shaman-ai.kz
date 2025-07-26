// internal/llm/client.go
package llm

import (
	"context"
	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
)

type Client interface {
	GenerateRemoteResponse(ctx context.Context, llmCfg config.RemoteLLMConfig, systemPrompt string, history []db.Message, userPrompt string) (string, *Usage, error)
}

// Ваша существующая структура
type APIClient struct {
	// поля если есть
}

// Реализация интерфейса вашим существующим клиентом
func (c *APIClient) GenerateRemoteResponse(ctx context.Context, llmCfg config.RemoteLLMConfig, systemPrompt string, history []db.Message, userPrompt string) (string, *Usage, error) {
	return GenerateRemoteResponse(ctx, llmCfg, systemPrompt, history, userPrompt) // Вызов вашей текущей функции
}

// Ваша существующая функция, возможно, станет методом или будет вызываться из метода
// func GenerateRemoteResponse(...) (string, error) { ... }