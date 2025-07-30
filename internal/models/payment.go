package models

import "time"

type Payment struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	SubscriptionID int64     `json:"subscription_id"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"` // e.g., pending, success, failed
	GatewayOrderID string    `json:"-"` // ID заказа в шлюзе
	GatewayName    string    `json:"-"` // Название шлюза (bcc)
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// type PaymentStatus string

// const (
// 	PaymentStatusPending PaymentStatus = "pending"
// 	PaymentStatusSuccess PaymentStatus = "success"
// 	PaymentStatusFailed  PaymentStatus = "failed"
// )

// type Payment struct {
// 	ID                          string
// 	UserID                      int64
// 	SubscriptionID              string
// 	PaymentGatewayTransactionID string
// 	Amount                      int64
// 	Currency                    string
// 	Status                      PaymentStatus
// 	PaymentDate                 time.Time
// 	CreatedAt                   time.Time
// 	UpdatedAt                   time.Time
// }
