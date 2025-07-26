// internal/utils/load_prompt.go
package utils

import (
	"fmt"
	"os"
)

func LoadSystemPrompt(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения файла системного промпта '%s': %w", filePath, err)
	}
	if len(content) == 0 {
		return "", fmt.Errorf("файл системного промпта '%s' пуст", filePath)
	}
	return string(content), nil
}