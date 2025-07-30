package bcc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client для взаимодействия с API Банка ЦентрКредит
type Client struct {
	httpClient *http.Client
	baseURL    string
	login      string
	password   string
}

// NewClient создает новый экземпляр клиента
func NewClient(baseURL, login, password string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    baseURL,
		login:      login,
		password:   password,
	}
}

// CreateOrderResponse содержит URL для оплаты и ID заказа в шлюзе
type CreateOrderResult struct {
	PaymentURL    string
	GatewayOrderID string
}

// CreateOrder создает заказ и возвращает URL для оплаты
func (c *Client) CreateOrder(ctx context.Context, reqData CreateOrderRequest) (*CreateOrderResult, error) {
	endpoint := "/orders/create"

	bodyBytes, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("bcc: failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("bcc: failed to create request: %w", err)
	}

	req.SetBasicAuth(c.login, c.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcc: failed to perform request: %w", err)
	}
	defer resp.Body.Close()
	
	// Согласно документации, успешный ответ имеет код 201
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bcc: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// URL для оплаты находится в заголовке Location
	paymentURL := resp.Header.Get("Location")
	if paymentURL == "" {
		return nil, fmt.Errorf("bcc: 'Location' header not found in response")
	}
	
	// Получаем ID заказа из тела ответа
	var orderResp OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, fmt.Errorf("bcc: failed to decode successful response: %w", err)
	}
	
	if len(orderResp.Orders) == 0 {
		return nil, fmt.Errorf("bcc: order details not found in response body")
	}

	result := &CreateOrderResult{
		PaymentURL:    paymentURL,
		GatewayOrderID: orderResp.Orders[0].ID,
	}

	return result, nil
}

// GetOrderStatus запрашивает статус заказа
func (c *Client) GetOrderStatus(ctx context.Context, gatewayOrderID string) (*OrderResponse, error) {
	endpoint := fmt.Sprintf("/orders/%s", gatewayOrderID)

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("bcc: failed to create status request: %w", err)
	}

	req.SetBasicAuth(c.login, c.password)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcc: failed to perform status request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bcc: unexpected status code on status check: %d, body: %s", resp.StatusCode, string(body))
	}

	var orderResp OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, fmt.Errorf("bcc: failed to decode status response: %w", err)
	}

	return &orderResp, nil
}