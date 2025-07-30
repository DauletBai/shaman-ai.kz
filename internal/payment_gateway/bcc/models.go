package bcc

// Структуры для запросов и ответов API Банка ЦентрКредит

// CreateOrderRequest описывает тело запроса для создания заказа
type CreateOrderRequest struct {
	Amount          float64    `json:"amount"`
	MerchantOrderID string     `json:"merchant_order_id"`
	Currency        string     `json:"currency"`
	Description     string     `json:"description"`
	Client          ClientInfo `json:"client"`
	Options         Options    `json:"options"`
}

// ClientInfo содержит информацию о клиенте
type ClientInfo struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// Options содержит дополнительные параметры заказа
type Options struct {
	ReturnURL string `json:"return_url"`
}

// OrderResponse описывает структуру успешного ответа от API
// (включая ответ на запрос статуса)
type OrderResponse struct {
	Orders []struct {
		ID              string  `json:"id"`
		Status          string  `json:"status"`
		Amount          string  `json:"amount"`
		AmountCharged   string  `json:"amount_charged"`
		AmountRefunded  string  `json:"amount_refunded"`
		MerchantOrderID string  `json:"merchant_order_id"`
		Currency        string  `json:"currency"`
		Description     string  `json:"description"`
	} `json:"orders"`
}

// ErrorResponse описывает структуру ответа с ошибкой
type ErrorResponse struct {
	FailureType    string `json:"failure_type"`
	FailureMessage string `json:"failure_message"`
	OrderID        string `json:"order_id"`
}