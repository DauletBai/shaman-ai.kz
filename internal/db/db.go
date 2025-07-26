// internal/db/db.go
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/models"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/google/uuid"
)

var DB *sql.DB

func RunMigrations(dbConn *sql.DB, dbName string) error {
	driverInstance, err := mysql.WithInstance(dbConn, &mysql.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return fmt.Errorf("не удалось создать драйвер миграций mysql: %w", err)
	}

	// Get the directory of the current file (db.go) to build a reliable path to project root
	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("не удалось получить путь к текущему файлу для определения пути миграций")
	}
	// Expects db.go to be in internal/db/, so two ".." to get to project root
	projectRoot := filepath.Join(filepath.Dir(currentFilePath), "..", "..")
	migrationsPath := filepath.Join(projectRoot, "migrations")

	migrationsURL := "file://" + migrationsPath
	slog.Info("Calculated migrations path for migrate", "path", migrationsPath, "url", migrationsURL)

	m, err := migrate.NewWithDatabaseInstance(migrationsURL, "mysql", driverInstance)
	if err != nil {
		slog.Error("Ошибка создания экземпляра migrate", "url", migrationsURL, "dbName", dbName, "error", err)
		return fmt.Errorf("ошибка создания экземпляра migrate (проверьте путь '%s' и совместимость миграций с MariaDB): %w", migrationsURL, err)
	}

	slog.Info("Применение миграций MariaDB...", "path", migrationsURL)
	err = m.Up()

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		version, dirty, verr := m.Version()
		if verr != nil {
			slog.Error("Ошибка получения статуса миграции после неудачного Up", "migration_error", err, "status_error", verr)
		} else {
			slog.Error("Ошибка применения миграций. Проверьте логи и файлы миграций.", "current_version", version, "dirty_state", dirty, "error_up", err)
		}
		return fmt.Errorf("ошибка применения миграций MariaDB: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("Миграции MariaDB: нет изменений.")
	} else {
		slog.Info("Миграции MariaDB успешно применены.")
	}

	return nil
}

func InitDB(appConfig *config.Config) error {
	var err error
	var dsn string

	dbCfg := appConfig.Database

	slog.Debug("InitDB: Начало инициализации БД")
	slog.Debug("InitDB: dbCfg.Path (потенциальный DSN из env)", "value", dbCfg.Path)
	slog.Debug("InitDB: dbCfg.Host", "value", dbCfg.Host)
	slog.Debug("InitDB: dbCfg.User", "value", dbCfg.User)
	slog.Debug("InitDB: dbCfg.DBName", "value", dbCfg.DBName)
	slog.Debug("InitDB: dbCfg.Port", "value", dbCfg.Port)

	if dbCfg.Path != "" && strings.Contains(dbCfg.Path, "://") {
		dsn = dbCfg.Path
		// Убедимся, что multiStatements=true есть в DSN из env
		if !strings.Contains(dsn, "multiStatements=true") {
			if strings.Contains(dsn, "?") {
				dsn += "&multiStatements=true"
			} else {
				dsn += "?multiStatements=true"
			}
		}
		slog.Info("Используется DATABASE_DSN для подключения к MariaDB.", "dsn_preview", strings.Split(dsn, "@")[0]+"@...")
	} else if dbCfg.Host != "" && dbCfg.User != "" && dbCfg.DBName != "" {
		// DSN уже включает multiStatements=true из предыдущих исправлений
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&multiStatements=true",
			dbCfg.User,
			dbCfg.Password, 
			dbCfg.Host,
			dbCfg.Port,
			dbCfg.DBName,
		)
		slog.Info("Формируется DSN из компонентов для MariaDB.")
	} else {
		slog.Error("InitDB: Не удалось сформировать DSN - недостаточно параметров.",
			"dbCfg.Path", dbCfg.Path,
			"dbCfg.Host", dbCfg.Host,
			"dbCfg.User", dbCfg.User,
			"dbCfg.DBName", dbCfg.DBName,
		)
		return fmt.Errorf("недостаточно параметров для подключения к MariaDB: DSN или Host+User+DBName должны быть заданы")
	}

	safeDSN := dsn
	if dbCfg.Password != "" {
		if strings.Contains(dsn, dbCfg.Password) {
			safeDSN = strings.Replace(dsn, dbCfg.Password, "****", 1)
		}
	}
	slog.Info("Подключение к MariaDB", "dsn_for_connection", safeDSN)

	DB, err = sql.Open("mysql", dsn) 
	if err != nil {
		return fmt.Errorf("ошибка открытия соединения с MariaDB: %w", err)
	}

	DB.SetConnMaxLifetime(time.Minute * 3)
	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(10)

	if err = DB.Ping(); err != nil {
		openedDB := DB
		if openedDB != nil {
			_ = openedDB.Close() 
		}
		return fmt.Errorf("ошибка подключения к MariaDB (ping failed): %w. DSN: %s", err, safeDSN)
	}
	slog.Info("Успешное подключение к MariaDB.")

	if err = RunMigrations(DB, dbCfg.DBName); err != nil {
		// Закрываем соединение с БД, если миграции не прошли, т.к. приложение не сможет корректно работать
		if DB != nil {
			_ = DB.Close()
		}
		return fmt.Errorf("ошибка выполнения миграций MariaDB: %w", err)
	}

	// Разделим на две команды для большей надежности, хотя с multiStatements=true в DSN и одна должна работать
	createTableSQL := `CREATE TABLE IF NOT EXISTS sessions (
		token CHAR(43) PRIMARY KEY,
		data BLOB NOT NULL,
		expiry TIMESTAMP(6) NOT NULL
	);`
	createIndexSQL := `CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions (expiry);`

	if _, errTable := DB.Exec(createTableSQL); errTable != nil {
		slog.Error("Не удалось создать таблицу 'sessions' для MariaDB.", "error", errTable)
		// Можно вернуть ошибку, если создание таблицы сессий критично для запуска
		// return fmt.Errorf("не удалось создать таблицу sessions: %w", errTable)
	} else {
		slog.Info("Таблица 'sessions' проверена/создана.")
		if _, errIndex := DB.Exec(createIndexSQL); errIndex != nil {
			slog.Warn("Не удалось создать индекс 'sessions_expiry_idx' для таблицы 'sessions'.", "error", errIndex)
		} else {
			slog.Info("Индекс 'sessions_expiry_idx' для таблицы 'sessions' проверен/создан.")
		}
	}

	defaultRoles := []models.Role{
		{Name: models.RoleUser, Description: "Default user role"},
		{Name: models.RoleAdmin, Description: "Administrator with full access"},
		{Name: models.RoleModerator, Description: "Moderator with content management access"},
		{Name: models.RoleSupport, Description: "Support agent with access to user issues"},
	}
	for _, r := range defaultRoles {
		_, err_role := CreateRoleIfNotExists(&r)
		if err_role != nil {
			slog.Warn("Не удалось создать/проверить роль по умолчанию", "role_name", r.Name, "error", err_role)
		}
	}

	SeedInitialSettings() 

	slog.Info("База данных MariaDB успешно инициализирована (включая миграции и начальные данные).")
	return nil
}

// Функции для Chat Sessions
type ChatSessionMeta struct {
	UUID      string    `json:"uuid"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func CreateChatSession(userID int64, sessionUUID, title string) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}
	query := `INSERT INTO chat_sessions (uuid, user_id, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	now := time.Now()
	_, err := DB.Exec(query, sessionUUID, userID, title, now, now)
	if err != nil {
		slog.Error("Ошибка создания сессии чата", "userID", userID, "uuid", sessionUUID, "error", err)
		return fmt.Errorf("не удалось создать сессию чата: %w", err)
	}
	return nil
}

func UpdateChatSessionTimestamp(sessionUUID string) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}
	query := `UPDATE chat_sessions SET updated_at = ? WHERE uuid = ?`
	_, err := DB.Exec(query, time.Now(), sessionUUID)
	if err != nil {
		slog.Error("Ошибка обновления updated_at сессии", "uuid", sessionUUID, "error", err)
		return fmt.Errorf("не удалось обновить сессию: %w", err)
	}
	return nil
}

func GetUserChatSessions(userID int64, limit int) ([]ChatSessionMeta, error) {
	if DB == nil {
		return nil, errors.New("БД не инициализирована")
	}
	query := `SELECT uuid, user_id, title, created_at, updated_at FROM chat_sessions WHERE user_id = ? ORDER BY updated_at DESC LIMIT ?`
	rows, err := DB.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения сессий пользователя: %w", err)
	}
	defer rows.Close()

	var sessions []ChatSessionMeta
	for rows.Next() {
		var s ChatSessionMeta
		var title sql.NullString
		if err := rows.Scan(&s.UUID, &s.UserID, &title, &s.CreatedAt, &s.UpdatedAt); err != nil {
			slog.Error("Ошибка сканирования сессии", "error", err)
			continue
		}
		if title.Valid && strings.TrimSpace(title.String) != "" { 
			s.Title = title.String
		} else {
			s.Title = "Диалог от " + s.CreatedAt.Format("02.01.06 15:04")
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации при получении сессий: %w", err)
	}
	return sessions, nil
}

// Функции для Сообщений Диалога
type Message struct {
	Role    string `json:"Role"`
	Content string `json:"Content"`
}

func SaveChatMessage(userID int64, chatSessionUUID string, userPrompt, aiResponse string) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}
	query := `INSERT INTO dialogues (user_id, chat_session_uuid, user_prompt, ai_response, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := DB.Exec(query, userID, chatSessionUUID, userPrompt, aiResponse, time.Now())
	if err != nil {
		slog.Error("Ошибка сохранения сообщения", "userID", userID, "chatUUID", chatSessionUUID, "error", err)
		return fmt.Errorf("ошибка сохранения сообщения: %w", err)
	}
	go UpdateChatSessionTimestamp(chatSessionUUID) // Обновляем время последнего сообщения в сессии
	return nil
}

func GetMessagesForChatSession(chatSessionUUID string, limit int) ([]Message, error) {
	if DB == nil {
		return nil, errors.New("БД не инициализирована")
	}
	query := `SELECT user_prompt, ai_response FROM dialogues WHERE chat_session_uuid = ? ORDER BY created_at ASC LIMIT ?`
	rows, err := DB.Query(query, chatSessionUUID, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения сообщений сессии: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var userPrompt string
		var aiResponse sql.NullString 
		if err := rows.Scan(&userPrompt, &aiResponse); err != nil {
			slog.Error("Ошибка сканирования сообщения сессии", "chatUUID", chatSessionUUID, "error", err)
			continue
		}
		messages = append(messages, Message{Role: "user", Content: userPrompt})
		if aiResponse.Valid && aiResponse.String != "" { // Добавляем ответ ассистента только если он не пустой
			messages = append(messages, Message{Role: "assistant", Content: aiResponse.String})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации при получении сообщений сессии: %w", err)
	}
	return messages, nil
}

func GetChatSessionMeta(sessionUUID string) (*ChatSessionMeta, error) {
	if DB == nil {
		return nil, errors.New("БД не инициализирована")
	}
	query := `SELECT uuid, user_id, title, created_at, updated_at FROM chat_sessions WHERE uuid = ?`
	row := DB.QueryRow(query, sessionUUID)

	var s ChatSessionMeta
	var dbTitle sql.NullString

	err := row.Scan(&s.UUID, &s.UserID, &dbTitle, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("сессия чата с UUID %s не найдена: %w", sessionUUID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("ошибка сканирования метаданных сессии чата: %w", err)
	}

	if dbTitle.Valid && strings.TrimSpace(dbTitle.String) != "" { // Проверка на пустую строку после TrimSpace
		s.Title = dbTitle.String
	} else {
		s.Title = "Диалог от " + s.CreatedAt.Format("02.01.06 15:04")
	}
	return &s, nil
}

// Функции для подписок и платежей
func CreateOrUpdateSubscription(sub *models.Subscription) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}
	query := `
	INSERT INTO subscriptions (id, user_id, payment_gateway_subscription_id, plan_id, status,
	                           start_date, end_date, current_period_start, current_period_end,
	                           cancel_at_period_end, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		status = VALUES(status),
		start_date = COALESCE(VALUES(start_date), start_date),
		end_date = COALESCE(VALUES(end_date), end_date),
		current_period_start = COALESCE(VALUES(current_period_start), current_period_start),
		current_period_end = COALESCE(VALUES(current_period_end), current_period_end),
		cancel_at_period_end = VALUES(cancel_at_period_end),
		updated_at = VALUES(updated_at);
	`
	now := time.Now()
	var createdAt, updatedAt time.Time
	if sub.CreatedAt.IsZero() {
		createdAt = now
	} else {
		createdAt = sub.CreatedAt
	}
	updatedAt = now

	_, err := DB.Exec(query,
		sub.ID,
		sub.UserID,
		sub.PaymentGatewaySubscriptionID,
		sub.PlanID,
		sub.Status,
		sql.NullTime{Time: sub.StartDate, Valid: !sub.StartDate.IsZero()},
		sql.NullTime{Time: sub.EndDate, Valid: !sub.EndDate.IsZero()},
		sql.NullTime{Time: sub.CurrentPeriodStart, Valid: !sub.CurrentPeriodStart.IsZero()},
		sql.NullTime{Time: sub.CurrentPeriodEnd, Valid: !sub.CurrentPeriodEnd.IsZero()},
		sub.CancelAtPeriodEnd,
		createdAt,
		updatedAt,
	)
	if err != nil {
		slog.Error("Ошибка создания/обновления подписки в БД (MariaDB)", "subscriptionID", sub.ID, "userID", sub.UserID, "error", err)
		return fmt.Errorf("не удалось сохранить подписку: %w", err)
	}
	return nil
}

func CreatePayment(payment *models.Payment) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}
	if payment.ID == "" {
		payment.ID = "pay_" + uuid.NewString()[:12]
	}
	query := `
	INSERT INTO payments (id, user_id, subscription_id, payment_gateway_transaction_id, amount, currency, status, payment_date, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`
	now := time.Now()
	var createdAt, paymentDate, updatedAt time.Time

	if payment.CreatedAt.IsZero() {
		createdAt = now
	} else {
		createdAt = payment.CreatedAt
	}
	if payment.PaymentDate.IsZero() {
		paymentDate = now
	} else {
		paymentDate = payment.PaymentDate
	}
	updatedAt = now 

	_, err := DB.Exec(query,
		payment.ID,
		payment.UserID,
		sql.NullString{String: payment.SubscriptionID, Valid: payment.SubscriptionID != ""},
		payment.PaymentGatewayTransactionID,
		payment.Amount,
		payment.Currency,
		payment.Status,
		paymentDate,
		createdAt,
		updatedAt, 
	)
	if err != nil {
		slog.Error("Ошибка создания записи о платеже в БД", "transactionID", payment.PaymentGatewayTransactionID, "userID", payment.UserID, "error", err)
		return fmt.Errorf("не удалось сохранить платеж: %w", err)
	}
	return nil
}

func GetSubscriptionByGatewayID(gatewaySubscriptionID string) (*models.Subscription, error) {
	if DB == nil {
		return nil, errors.New("БД не инициализирована")
	}
	query := `SELECT id, user_id, payment_gateway_subscription_id, plan_id, status,
	                 start_date, end_date, current_period_start, current_period_end,
	                 cancel_at_period_end, created_at, updated_at
	          FROM subscriptions WHERE payment_gateway_subscription_id = ?`
	row := DB.QueryRow(query, gatewaySubscriptionID)
	var sub models.Subscription
	var startDate, endDate, currentPeriodStart, currentPeriodEnd sql.NullTime
	var createdAt, updatedAt sql.NullTime

	err := row.Scan(
		&sub.ID, &sub.UserID, &sub.PaymentGatewaySubscriptionID, &sub.PlanID, &sub.Status,
		&startDate, &endDate, &currentPeriodStart, &currentPeriodEnd,
		&sub.CancelAtPeriodEnd, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		slog.Error("Ошибка получения подписки по gateway_id", "gatewayID", gatewaySubscriptionID, "error", err)
		return nil, fmt.Errorf("ошибка получения подписки по gateway_id: %w", err)
	}
	if startDate.Valid {
		sub.StartDate = startDate.Time
	}
	if endDate.Valid {
		sub.EndDate = endDate.Time
	}
	if currentPeriodStart.Valid {
		sub.CurrentPeriodStart = currentPeriodStart.Time
	}
	if currentPeriodEnd.Valid {
		sub.CurrentPeriodEnd = currentPeriodEnd.Time
	}
	if createdAt.Valid {
		sub.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		sub.UpdatedAt = updatedAt.Time
	}

	return &sub, nil
}