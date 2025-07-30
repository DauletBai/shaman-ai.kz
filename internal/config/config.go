// internal/config/config.go
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type BCCGatewayConfig struct {
	BaseURL   string `yaml:"base_url"`
	Login     string `yaml:"login"`
	Password  string `yaml:"password"`
	ReturnURL string `yaml:"return_url"`
	Currency  string `yaml:"currency"`
}
type RemoteLLMConfig struct {
	APIKey                    string  `yaml:"api_key"`
	APIUrl                    string  `yaml:"api_url"`
	ModelName                 string  `yaml:"model_name"`
	ShamanSystemPromptPath    string  `yaml:"shaman_system_prompt_path"`
	GeneralSystemPromptPath   string  `yaml:"general_system_prompt_path"`
	RequestTimeoutSeconds     int     `yaml:"request_timeout_seconds"`
	TokenCostInputPerMillion  float64 `yaml:"token_cost_input_per_million"`
	TokenCostOutputPerMillion float64 `yaml:"token_cost_output_per_million"`
}

type DatabaseConfig struct {
	Path     string `yaml:"path"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

type BillingConfig struct {
	PriceID                      string  `yaml:"price_id"`
	PaymentGatewayPublishableKey string  `yaml:"payment_gateway_publishable_key"`
	PaymentGatewaySecretKey      string  `yaml:"payment_gateway_secret_key"`
	WebhookSecret                string  `yaml:"webhook_secret"`
	Currency                     string  `yaml:"currency"`
	MonthlyAmount                int64   `yaml:"monthly_amount"`
	USDToKZTRate                 float64 `yaml:"usd_to_kzt_rate"` // Новое поле
}

type EmailConfig struct {
	SMTPhost     string `yaml:"smtp_host"`
	SMTPport     int    `yaml:"smtp_port"`
	SMTPuser     string `yaml:"smtp_user"`
	SMTPpassword string `yaml:"smtp_password"`
	Sender       string `yaml:"sender"`
}

type SMSConfig struct {
	APIURL   string `yaml:"api_url" env:"SMS_GATEWAY_API_URL"`
	APIKey   string `env:"SMS_GATEWAY_API_KEY"`
	SenderID string `yaml:"sender_id" env:"SMS_GATEWAY_SENDER_ID"`
}

type Config struct {
	SiteName             string          `yaml:"site_name"`
	SiteDescription      string          `yaml:"site_description"`
	CurrentYear          int             `yaml:"current_year"`
	BaseURL              string          `yaml:"base_url"`
	Port                 int             `yaml:"port"`
	AppEnv               string          `yaml:"app_env"`
	RemoteLLM            RemoteLLMConfig `yaml:"remote_llm"`
	Database             DatabaseConfig  `yaml:"database"`
	Billing              BillingConfig   `yaml:"billing"`
	CSRFAuthKey          string
	UploadPath           string      `yaml:"upload_path"`
	Email                EmailConfig `yaml:"email"`
	SMS                  SMSConfig   `yaml:"sms"`
	TokenMonthlyLimitKZT float64
	BCCGateway           BCCGatewayConfig `yaml:"bcc_gateway"`
}

// ... функции getStringEnvOrDefault и getIntEnvOrDefault без изменений ...
func getStringEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
func getIntEnvOrDefault(key string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
		slog.Warn("Не удалось преобразовать переменную окружения в число, используется значение по умолчанию", "key", key, "value", valueStr)
	}
	return defaultValue
}

func LoadConfig(filename string) (*Config, error) {
	// ... (начало функции без изменений) ...
	appEnvFromSystem := os.Getenv("APP_ENV")
	if appEnvFromSystem != "production" {
		if err := godotenv.Load("configs/.env"); err != nil {
			slog.Info("configs/.env не найден или ошибка загрузки, это ожидаемо для production или если переменные установлены системно.", "error", err)
		} else {
			slog.Info("Переменные окружения загружены из configs/.env")
		}
	}

	file, err := os.Open(filename)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("файл конфигурации не найден: %s", filename)
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла конфигурации '%s': %w", filename, err)
	}
	defer file.Close()

	var cfg Config
	if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("ошибка декодирования YAML из файла '%s': %w", filename, err)
	}

	// ... (середина функции без изменений) ...
	cfg.AppEnv = getStringEnvOrDefault("APP_ENV", cfg.AppEnv)
	isProduction := cfg.AppEnv == "production"

	cfg.BaseURL = getStringEnvOrDefault("BASE_URL", cfg.BaseURL)
	cfg.Port = getIntEnvOrDefault("PORT", cfg.Port)

	cfg.RemoteLLM.APIKey = getStringEnvOrDefault("REMOTE_LLM_API_KEY", cfg.RemoteLLM.APIKey)
	if isProduction && cfg.RemoteLLM.APIKey == "" {
		slog.Error("КРИТИЧЕСКАЯ ОШИБКА: REMOTE_LLM_API_KEY должен быть установлен в переменных окружения для production")
		return nil, fmt.Errorf("REMOTE_LLM_API_KEY должен быть установлен в переменных окружения для production")
	}

	cfg.Database.Password = getStringEnvOrDefault("DB_PASSWORD", "")
	if isProduction && cfg.Database.Host != "" && cfg.Database.Password == "" && !strings.Contains(getStringEnvOrDefault("DATABASE_DSN", ""), cfg.Database.User) {
		slog.Error("КРИТИЧЕСКАЯ ОШИБКА: DB_PASSWORD должен быть установлен в переменных окружения для production, если не используется DSN с полными кредами")
		return nil, fmt.Errorf("DB_PASSWORD должен быть установлен в переменных окружения для production, если не используется DSN с полными кредами")
	}

	cfg.Billing.PaymentGatewaySecretKey = getStringEnvOrDefault("PAYMENT_GATEWAY_SECRET_KEY", cfg.Billing.PaymentGatewaySecretKey)
	if isProduction && cfg.Billing.PaymentGatewaySecretKey == "" {
		slog.Error("КРИТИЧЕСКАЯ ОШИБКА: PAYMENT_GATEWAY_SECRET_KEY должен быть установлен в переменных окружения для production")
		return nil, fmt.Errorf("PAYMENT_GATEWAY_SECRET_KEY должен быть установлен в переменных окружения для production")
	}

	cfg.Billing.WebhookSecret = getStringEnvOrDefault("WEBHOOK_SECRET", cfg.Billing.WebhookSecret)
	if isProduction && cfg.Billing.WebhookSecret == "" {
		slog.Error("КРИТИЧЕСКАЯ ОШИБКА: WEBHOOK_SECRET должен быть установлен в переменных окружения для production")
		return nil, fmt.Errorf("WEBHOOK_SECRET должен быть установлен в переменных окружения для production")
	}

	cfg.CSRFAuthKey = getStringEnvOrDefault("CSRF_AUTH_KEY", "")
	if isProduction && cfg.CSRFAuthKey == "" {
		slog.Error("КРИТИЧЕСКАЯ ОШИБКА: CSRF_AUTH_KEY должен быть установлен в переменных окружения для production (рекомендуется 32-байтный случайный ключ)")
		return nil, fmt.Errorf("CSRF_AUTH_KEY должен быть установлен в переменных окружения для production (рекомендуется 32-байтный случайный ключ)")
	}
	if !isProduction && cfg.CSRFAuthKey == "" {
		slog.Warn("CSRF_AUTH_KEY не установлен! Используется небезопасный ключ по умолчанию (ТОЛЬКО ДЛЯ РАЗРАБОТКИ).")
	}

	cfg.RemoteLLM.APIUrl = getStringEnvOrDefault("REMOTE_LLM_API_URL", cfg.RemoteLLM.APIUrl)
	cfg.RemoteLLM.ModelName = getStringEnvOrDefault("REMOTE_LLM_MODEL_NAME", cfg.RemoteLLM.ModelName)

	if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
		cfg.Database.Path = dsn
		cfg.Database.Host = ""
		cfg.Database.Port = 0
		cfg.Database.User = ""
		cfg.Database.DBName = ""
		cfg.Database.SSLMode = ""
	} else {
		cfg.Database.Host = getStringEnvOrDefault("DB_HOST", cfg.Database.Host)
		cfg.Database.Port = getIntEnvOrDefault("DB_PORT", cfg.Database.Port)
		cfg.Database.User = getStringEnvOrDefault("DB_USER", cfg.Database.User)
		cfg.Database.DBName = getStringEnvOrDefault("DB_NAME", cfg.Database.DBName)
		cfg.Database.SSLMode = getStringEnvOrDefault("DB_SSLMODE", cfg.Database.SSLMode)
		cfg.Database.Path = ""
	}

	cfg.Billing.PaymentGatewayPublishableKey = getStringEnvOrDefault("PAYMENT_GATEWAY_PUBLISHABLE_KEY", cfg.Billing.PaymentGatewayPublishableKey)
	if priceIDFromEnv := os.Getenv("PRICE_ID"); priceIDFromEnv != "" {
		cfg.Billing.PriceID = priceIDFromEnv
	}

	// Загрузка конфигурации Email
	cfg.Email.SMTPhost = getStringEnvOrDefault("SMTP_HOST", cfg.Email.SMTPhost)
	cfg.Email.SMTPport = getIntEnvOrDefault("SMTP_PORT", cfg.Email.SMTPport)
	cfg.Email.SMTPuser = getStringEnvOrDefault("SMTP_USER", cfg.Email.SMTPuser)
	cfg.Email.SMTPpassword = getStringEnvOrDefault("SMTP_PASSWORD", "") // Пароль SMTP - только из ENV
	cfg.Email.Sender = getStringEnvOrDefault("EMAIL_SENDER", cfg.Email.Sender)
	cfg.SMS.APIKey = os.Getenv("SMS_GATEWAY_API_KEY")

	if isProduction && (cfg.Email.SMTPhost == "" || cfg.Email.Sender == "") {
		slog.Warn("Параметры SMTP (SMTP_HOST, EMAIL_SENDER) не полностью настроены для production. Отправка email может не работать.")
	}

	cfg.UploadPath = getStringEnvOrDefault("UPLOAD_PATH", cfg.UploadPath)
	if cfg.UploadPath == "" {
		cfg.UploadPath = "./uploads"
	}

	if cfg.CurrentYear == 0 {
		cfg.CurrentYear = time.Now().Year()
	}
	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")

	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = "development"
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("BASE_URL не задан")
	}
	if isProduction && !strings.HasPrefix(cfg.BaseURL, "https://") {
		return nil, fmt.Errorf("в production окружении BASE_URL должен начинаться с https://")
	}
	if cfg.RemoteLLM.APIUrl == "" {
		return nil, fmt.Errorf("remote_llm.api_url (REMOTE_LLM_API_URL) не задан")
	}
	if cfg.RemoteLLM.ModelName == "" {
		return nil, fmt.Errorf("remote_llm.model_name (REMOTE_LLM_MODEL_NAME) не задан")
	}
	if cfg.RemoteLLM.ShamanSystemPromptPath == "" {
		return nil, fmt.Errorf("remote_llm.shaman_system_prompt_path не задан")
	}
	if cfg.RemoteLLM.RequestTimeoutSeconds <= 0 {
		cfg.RemoteLLM.RequestTimeoutSeconds = 90
	}
	if cfg.Database.Path == "" && cfg.Database.Host == "" {
		return nil, fmt.Errorf("параметры подключения к БД (DATABASE_DSN или DB_HOST и др.) не заданы")
	}
	if cfg.Database.Host != "" {
		if cfg.Database.User == "" {
			return nil, fmt.Errorf("DB_USER не задан для подключения к БД")
		}
		if cfg.Database.DBName == "" {
			return nil, fmt.Errorf("DB_NAME не задан для подключения к БД")
		}
	}
	if cfg.Billing.PriceID == "" {
		return nil, fmt.Errorf("billing.price_id (PRICE_ID) не задан")
	}
	if cfg.Billing.PaymentGatewayPublishableKey == "" && isProduction {
		slog.Warn("PAYMENT_GATEWAY_PUBLISHABLE_KEY не установлен для production")
	}
	if cfg.Billing.Currency == "" {
		cfg.Billing.Currency = "KZT"
	}
	if cfg.Billing.MonthlyAmount == 0 {
		return nil, fmt.Errorf("billing.monthly_amount не задан или равен 0")
	}
	if cfg.Billing.USDToKZTRate <= 0 {
		return nil, fmt.Errorf("billing.usd_to_kzt_rate не задан или равен 0")
	}

	cfg.TokenMonthlyLimitKZT = float64(cfg.Billing.MonthlyAmount) / 100.0

	slog.Info("Конфигурация загружена", "app_env", cfg.AppEnv, "base_url", cfg.BaseURL, "port", cfg.Port, "token_limit_kzt", cfg.TokenMonthlyLimitKZT)
	return &cfg, nil
}

func InitLogger(appEnv string) {
	var logger *slog.Logger
	logLevel := slog.LevelInfo

	if appEnv == "development" {
		logLevel = slog.LevelDebug
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: true,
		}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: false,
		}))
	}
	slog.SetDefault(logger)
}
