// internal/utils/file_reader.go
package utils

import (
	"html/template"
	"os"
	"path/filepath"
	"fmt"
)

func LoadHTMLContentFromFile(filePath string) (template.HTML, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("ошибка получения абсолютного пути для '%s': %w", filePath, err)
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения файла '%s': %w", absPath, err)
	}
	return template.HTML(content), nil
}