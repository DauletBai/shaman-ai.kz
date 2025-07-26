// internal/db/settings_db.go
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// AppSetting определяет структуру для настройки приложения.
type AppSetting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetSetting извлекает одну настройку по ключу.
func GetSetting(key string) (*AppSetting, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	query := "SELECT setting_key, setting_value, description, updated_at FROM app_settings WHERE setting_key = ?"
	row := DB.QueryRow(query, key)
	setting := &AppSetting{}
	var value sql.NullString
	var description sql.NullString
	err := row.Scan(&setting.Key, &value, &description, &setting.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Возвращаем nil и nil, если настройка не найдена, чтобы ее можно было создать
			return nil, nil
		}
		slog.Error("Ошибка получения настройки по ключу", "key", key, "error", err)
		return nil, fmt.Errorf("ошибка получения настройки '%s': %w", key, err)
	}
	if value.Valid {
		setting.Value = value.String
	}
	if description.Valid {
		setting.Description = description.String
	}
	return setting, nil
}

// GetAllAppSettings извлекает все настройки приложения в виде карты.
func GetAllAppSettings() (map[string]string, error) {
	if DB == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	query := "SELECT setting_key, setting_value FROM app_settings"
	rows, err := DB.Query(query)
	if err != nil {
		slog.Error("Ошибка получения всех настроек приложения", "error", err)
		return nil, fmt.Errorf("ошибка получения всех настроек: %w", err)
	}
	defer rows.Close()

	settingsMap := make(map[string]string)
	for rows.Next() {
		var key string
		var value sql.NullString // Используем sql.NullString для обработки возможных NULL значений
		if err := rows.Scan(&key, &value); err != nil {
			slog.Error("Ошибка сканирования строки настройки", "error", err)
			continue // Пропускаем ошибочную строку
		}
		if value.Valid {
			settingsMap[key] = value.String
		} else {
			settingsMap[key] = "" // Или другое значение по умолчанию для NULL
		}
	}
	if err = rows.Err(); err != nil {
		slog.Error("Ошибка итерации по строкам настроек", "error", err)
		return nil, fmt.Errorf("ошибка итерации настроек: %w", err)
	}
	return settingsMap, nil
}

// UpdateSetting обновляет или создает настройку.
func UpdateSetting(key string, value string, description ...string) error {
	if DB == nil {
		return errors.New("база данных не инициализирована")
	}

	desc := ""
	if len(description) > 0 {
		desc = description[0]
	}

	query := `
		INSERT INTO app_settings (setting_key, setting_value, description, updated_at) 
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE 
		setting_value = VALUES(setting_value), 
		description = IF(VALUES(description) = '' AND description IS NOT NULL, description, VALUES(description)),
		updated_at = VALUES(updated_at)
	`
	// При обновлении, если новое описание пустое, оставляем старое (если оно было)
	
	_, err := DB.Exec(query, key, value, desc, time.Now())
	if err != nil {
		slog.Error("Ошибка обновления/вставки настройки", "key", key, "error", err)
		return fmt.Errorf("не удалось обновить/вставить настройку '%s': %w", key, err)
	}
	slog.Info("Настройка приложения обновлена/вставлена", "key", key, "value", value)
	return nil
}

// SeedInitialSettings гарантирует наличие базовых настроек в БД.
// Вызывается после применения миграций.
func SeedInitialSettings() {
	slog.Info("Инициализация/проверка настроек приложения по умолчанию...")
	
	defaultSettings := []struct {
		Key         string
		Value       string
		Description string
	}{
		{"site_name", "Sham'an AI KZ", "Глобальное имя сайта, отображаемое в заголовках и брендинге."},
		{"site_description", "Ваш универсальный семейный AI", "Мета-описание сайта по умолчанию."},
		{"maintenance_mode", "false", "Установите 'true' для включения режима обслуживания (сайт недоступен для не-администраторов)."},
		{"trial_dialogue_enabled", "true", "Установите 'true' для включения пробного диалога на главной странице."},
		{"default_llm_model", "accounts/fireworks/models/llama4-maverick-instruct-basic", "Имя LLM модели по умолчанию для чата."},
		{"shaman_system_prompt_path_db", "configs/prompt_shaman.txt", "Путь к файлу системного промпта Shaman (если не переопределен в БД)."},
		{"general_system_prompt_path_db", "configs/prompt_general.txt", "Путь к файлу общего системного промпта (если не переопределен в БД)."},
		{"shaman_system_prompt_content", "", "Содержимое системного промпта Shaman (хранится в БД, приоритетнее файла)."},
		{"general_system_prompt_content", "", "Содержимое общего системного промпта (хранится в БД, приоритетнее файла)."},
	}

	for _, s := range defaultSettings {
		existingSetting, err := GetSetting(s.Key)
		if err != nil && !strings.Contains(err.Error(), "not found") { // Проверяем ошибку, отличную от "не найдено"
			slog.Error("Ошибка проверки существующей настройки при инициализации", "key", s.Key, "error", err)
			continue
		}
		if existingSetting == nil { // Настройка не найдена, нужно создать
			err := UpdateSetting(s.Key, s.Value, s.Description)
			if err != nil {
				slog.Error("Не удалось инициализировать настройку по умолчанию", "key", s.Key, "error", err)
			} else {
				slog.Info("Настройка по умолчанию установлена", "key", s.Key, "value", s.Value)
			}
		}
	}
}