package models

import "time"

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusSuccess PaymentStatus = "success"
	PaymentStatusFailed  PaymentStatus = "failed"
)

type Payment struct {
	ID                          string
	UserID                      int64
	SubscriptionID              string
	PaymentGatewayTransactionID string
	Amount                      int64
	Currency                    string
	Status                      PaymentStatus
	PaymentDate                 time.Time
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}
