// internal/db/payments_db.go
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"shaman-ai.kz/internal/models"
	"time"
)

// GetPaymentByID находит платеж по его ID (вашему order_id)
func GetPaymentByID(paymentID string) (*models.Payment, error) {
	if DB == nil {
		return nil, errors.New("БД не инициализирована")
	}
	query := `SELECT id, user_id, subscription_id, payment_gateway_transaction_id, amount, currency, status, payment_date, created_at, updated_at FROM payments WHERE id = ?`
	row := DB.QueryRow(query, paymentID)

	var p models.Payment
	var subID, gatewayTxID sql.NullString
	var paymentDate, createdAt, updatedAt sql.NullTime

	err := row.Scan(
		&p.ID, &p.UserID, &subID, &gatewayTxID,
		&p.Amount, &p.Currency, &p.Status, &paymentDate, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Платеж не найден, это не всегда ошибка
		}
		slog.Error("Ошибка получения платежа по ID", "paymentID", paymentID, "error", err)
		return nil, fmt.Errorf("ошибка получения платежа: %w", err)
	}

	p.SubscriptionID = subID.String
	p.PaymentGatewayTransactionID = gatewayTxID.String
	p.PaymentDate = paymentDate.Time
	p.CreatedAt = createdAt.Time
	p.UpdatedAt = updatedAt.Time

	return &p, nil
}

// UpdateGatewayInfo обновляет информацию о платеже от шлюза
func (pdb *PaymentsDB) UpdateGatewayInfo(ctx context.Context, internalPaymentID int64, gatewayOrderID, status string) error {
	query := `UPDATE payments SET gateway_order_id = $1, status = $2, updated_at = NOW() WHERE id = $3`
	_, err := pdb.db.ExecContext(ctx, query, gatewayOrderID, status, internalPaymentID)
	return err
}

// UpdateStatusByGatewayID обновляет статус платежа по ID заказа в шлюзе
func (pdb *PaymentsDB) UpdateStatusByGatewayID(ctx context.Context, gatewayOrderID, status string) error {
	query := `UPDATE payments SET status = $1, updated_at = NOW() WHERE gateway_order_id = $2`
	_, err := pdb.db.ExecContext(ctx, query, status, gatewayOrderID)
	return err
}

// GetPaymentByGatewayID находит платеж по ID заказа в шлюзе
func (pdb *PaymentsDB) GetPaymentByGatewayID(ctx context.Context, gatewayOrderID string) (*models.Payment, error) {
	query := `SELECT id, user_id, subscription_id, amount, currency, status, created_at, updated_at FROM payments WHERE gateway_order_id = $1`
	row := pdb.db.QueryRowContext(ctx, query, gatewayOrderID)
	
	var p models.Payment
	err := row.Scan(&p.ID, &p.UserID, &p.SubscriptionID, &p.Amount, &p.Currency, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Платеж не найден
		}
		return nil, err
	}
	return &p, nil
}


// UpdatePaymentStatus обновляет статус и ID транзакции для существующего платежа
func UpdatePaymentStatus(paymentID string, newStatus models.PaymentStatus, gatewayTxID string) error {
	if DB == nil {
		return errors.New("БД не инициализирована")
	}
	query := `UPDATE payments SET status = ?, payment_gateway_transaction_id = ?, updated_at = ? WHERE id = ?`
	_, err := DB.Exec(query, newStatus, gatewayTxID, time.Now(), paymentID)
	if err != nil {
		slog.Error("Ошибка обновления статуса платежа", "paymentID", paymentID, "newStatus", newStatus, "error", err)
		return fmt.Errorf("не удалось обновить статус платежа: %w", err)
	}
	return nil
}