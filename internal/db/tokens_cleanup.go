// internal/db/tokens_cleanup.go (новый файл)
package db

import (
	"log/slog"
	"time"
)

func CleanupExpiredTokens() {
	if DB == nil {
		slog.Error("CleanupExpiredTokens: База данных не инициализирована")
		return
	}

	// Очистка токенов сброса пароля
	queryReset := `UPDATE users SET password_reset_token = NULL, password_reset_token_expires_at = NULL, updated_at = NOW()
	               WHERE password_reset_token_expires_at IS NOT NULL AND password_reset_token_expires_at < NOW()`
	resReset, errReset := DB.Exec(queryReset)
	if errReset != nil {
		slog.Error("Ошибка очистки просроченных токенов сброса пароля", "error", errReset)
	} else {
		affectedReset, _ := resReset.RowsAffected()
		if affectedReset > 0 {
			slog.Info("Очищены просроченные токены сброса пароля", "count", affectedReset)
		}
	}

	// Очистка токенов верификации email
	queryVerify := `UPDATE users SET email_verification_token = NULL, email_verification_token_expires_at = NULL, updated_at = NOW()
	                WHERE is_email_verified = FALSE AND email_verification_token_expires_at IS NOT NULL AND email_verification_token_expires_at < NOW()`
	resVerify, errVerify := DB.Exec(queryVerify)
	if errVerify != nil {
		slog.Error("Ошибка очистки просроченных токенов верификации email", "error", errVerify)
	} else {
		affectedVerify, _ := resVerify.RowsAffected()
		if affectedVerify > 0 {
			slog.Info("Очищены просроченные токены верификации email", "count", affectedVerify)
		}
	}
}

// StartTokenCleanupScheduler запускает периодическую очистку токенов
func StartTokenCleanupScheduler(interval time.Duration) {
	slog.Info("Планировщик очистки токенов запущен", "interval", interval.String())
	ticker := time.NewTicker(interval)
	go func() {
		for {
			<-ticker.C
			slog.Info("Запуск плановой очистки просроченных токенов...")
			CleanupExpiredTokens()
		}
	}()
}