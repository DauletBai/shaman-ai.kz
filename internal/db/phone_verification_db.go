// internal/db/phone_verification_db.go
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// SetPhoneVerificationCode сохраняет код верификации телефона для пользователя.
func SetPhoneVerificationCode(userID int64, code string) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}
	expiresAt := time.Now().Add(10 * time.Minute) // Код действителен 10 минут
	query := `UPDATE users SET phone_verification_code = ?, phone_verification_code_expires_at = ? WHERE id = ?`
	_, err := DB.Exec(query, code, expiresAt, userID)
	if err != nil {
		slog.Error("Ошибка установки кода верификации телефона", "userID", userID, "error", err)
		return fmt.Errorf("ошибка БД при установке кода: %w", err)
	}
	return nil
}

// VerifyUserPhone проверяет код и верифицирует номер телефона пользователя.
func VerifyUserPhone(userID int64, code string) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}

	var storedCode sql.NullString
	var expiresAt sql.NullTime

	query := `SELECT phone_verification_code, phone_verification_code_expires_at FROM users WHERE id = ? AND is_phone_verified = FALSE`
	err := DB.QueryRow(query, userID).Scan(&storedCode, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("пользователь не найден или уже верифицирован")
		}
		slog.Error("Ошибка получения пользователя для верификации телефона", "userID", userID, "error", err)
		return fmt.Errorf("ошибка БД: %w", err)
	}

	if !storedCode.Valid || storedCode.String == "" {
		return errors.New("код верификации не был запрошен")
	}
	if storedCode.String != code {
		return errors.New("неверный код подтверждения")
	}
	if !expiresAt.Valid || time.Now().After(expiresAt.Time) {
		return errors.New("срок действия кода истек, пожалуйста, запросите новый")
	}

	// Код верный, верифицируем пользователя и очищаем поля
	updateQuery := `UPDATE users SET is_phone_verified = TRUE, phone_verified_at = NOW(), phone_verification_code = NULL, phone_verification_code_expires_at = NULL WHERE id = ?`
	_, err = DB.Exec(updateQuery, userID)
	if err != nil {
		slog.Error("Ошибка обновления статуса верификации телефона", "userID", userID, "error", err)
		return fmt.Errorf("ошибка БД при обновлении статуса: %w", err)
	}

	slog.Info("Номер телефона успешно подтвержден", "userID", userID)
	return nil
}