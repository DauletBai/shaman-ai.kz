// internal/db/reports_db.go
package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// ReportStats содержит агрегированные данные для отчетов.
type ReportStats struct {
	TotalUsers              int
	NewUsersToday           int
	NewUsersLast7Days       int
	NewUsersLast30Days      int
	ActiveSubscriptions     int
	TotalChatSessions       int
	TotalDialogueMessages   int
	TotalTokensUsedInput    int 
	TotalTokensUsedOutput   int
}

// GetDashboardStats извлекает основную статистику для панели администратора.
func GetDashboardStats() (*ReportStats, error) {
	if DB == nil {
		return nil, fmt.Errorf("база данных не инициализирована")
	}

	stats := &ReportStats{}
	var err error

	// Общее количество пользователей
	err = DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
	if err != nil {
		slog.Error("Ошибка получения общего количества пользователей для статистики", "error", err)
		// Можно вернуть ошибку, либо продолжить сбор остальной статистики
	}

	// Новые пользователи за сегодня
	todayStart := time.Now().Truncate(24 * time.Hour)
	err = DB.QueryRow("SELECT COUNT(*) FROM users WHERE created_at >= ?", todayStart).Scan(&stats.NewUsersToday)
	if err != nil {
		slog.Error("Ошибка получения новых пользователей за сегодня", "error", err)
	}

	// Новые пользователи за последние 7 дней
	sevenDaysAgo := time.Now().AddDate(0, 0, -7).Truncate(24 * time.Hour)
	err = DB.QueryRow("SELECT COUNT(*) FROM users WHERE created_at >= ?", sevenDaysAgo).Scan(&stats.NewUsersLast7Days)
	if err != nil {
		slog.Error("Ошибка получения новых пользователей за последние 7 дней", "error", err)
	}

	// Новые пользователи за последние 30 дней
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Truncate(24 * time.Hour)
	err = DB.QueryRow("SELECT COUNT(*) FROM users WHERE created_at >= ?", thirtyDaysAgo).Scan(&stats.NewUsersLast30Days)
	if err != nil {
		slog.Error("Ошибка получения новых пользователей за последние 30 дней", "error", err)
	}
	
	// Активные подписки (пример, используя таблицу users. Более точно - через таблицу subscriptions)
	// Условие current_period_end > NOW() гарантирует, что подписка не истекла.
	err = DB.QueryRow("SELECT COUNT(*) FROM subscriptions WHERE status = 'active' AND (current_period_end IS NULL OR current_period_end > NOW())").Scan(&stats.ActiveSubscriptions)
	if err != nil {
		slog.Error("Ошибка получения количества активных подписок", "error", err)
	}

	// Общее количество сессий чата
	err = DB.QueryRow("SELECT COUNT(*) FROM chat_sessions").Scan(&stats.TotalChatSessions)
	if err != nil {
		slog.Error("Ошибка получения общего количества сессий чата", "error", err)
	}

	// Общее количество сообщений в диалогах (user_prompt + ai_response)
	// Подсчитываем каждую пару как одно "сообщение обмена" или можно считать user_prompt и ai_response отдельно
	err = DB.QueryRow("SELECT COUNT(*) FROM dialogues").Scan(&stats.TotalDialogueMessages) // Считает каждую запись (промпт+ответ)
	if err != nil {
		slog.Error("Ошибка получения общего количества сообщений в диалогах", "error", err)
	}

	// Новые запросы для токенов
	var totalInput, totalOutput sql.NullInt64 // Используем NullInt64 на случай если таблица пуста
	err = DB.QueryRow("SELECT SUM(tokens_used_input_this_period) FROM users").Scan(&totalInput)
	if err != nil {
		slog.Error("Ошибка получения суммы использованных входных токенов", "error", err)
	}
	if totalInput.Valid {
		stats.TotalTokensUsedInput = int(totalInput.Int64)
	}

	err = DB.QueryRow("SELECT SUM(tokens_used_output_this_period) FROM users").Scan(&totalOutput)
	if err != nil {
		slog.Error("Ошибка получения суммы использованных выходных токенов", "error", err)
	}
	if totalOutput.Valid {
		stats.TotalTokensUsedOutput = int(totalOutput.Int64)
	}

	return stats, nil // Возвращаем собранную статистику, даже если некоторые запросы вернули ошибку (они залогированы)
}