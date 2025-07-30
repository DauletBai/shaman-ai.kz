// internal/handlers/billing_handlers.go
package handlers

import (
	"log"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"log/slog"
	"net/http"
	"strconv"
	"sort"
	"time"

	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/models"
	"shaman-ai.kz/internal/session"
	"shaman-ai.kz/internal/payment_gateway/bcc"

	"github.com/alexedwards/scs/v2"
)

// ... (структуры Request, Attribute, Signature, Response, Transaction, WebhookNotification без изменений) ...
type Request struct {
	XMLName    xml.Name    `xml:"request"`
	Point      string      `xml:"point,attr"`
	Action     string      `xml:"action,attr"`
	Timestamp  string      `xml:"datetime,attr"`
	Attributes []Attribute `xml:"attribute"`
	Signature  Signature   `xml:"signature"`
}

type Attribute struct {
	XMLName xml.Name `xml:"attribute"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:"value,attr"`
}

type Signature struct {
	XMLName xml.Name `xml:"signature"`
	Type    string   `xml:"type,attr"`
	Value   string   `xml:"chardata"`
}

type Response struct {
	XMLName     xml.Name    `xml:"response"`
	Code        int         `xml:"code"`
	Message     string      `xml:"message"`
	PayURL      string      `xml:"pay-url"`
	Transaction Transaction `xml:"transaction"`
}

type Transaction struct {
	ID        string `xml:"id,attr"`
	PaymentID string `xml:"payment_id,attr"`
}

type WebhookNotification struct {
	XMLName   xml.Name `xml:"notification"`
	OrderID   string   `xml:"order_id"`   
	PaymentID string   `xml:"payment_id"` 
	Status    string   `xml:"status"`     
	Amount    int64    `xml:"amount"`
	Signature string   `xml:"signature"`
}
type BillingHandlers struct {
	SessionManager *scs.SessionManager
	Config         *config.Config
	AppHandlers    *AppHandlers
	BCCClient *bcc.Client
}

// HandleCreateBCCPayment - новый обработчик для создания платежа
func (h *BillingHandler) HandleCreateBCCPayment(w http.ResponseWriter, r *http.Request) {
	// 1. Получаем пользователя из сессии
	userID, ok := session.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Получаем детали подписки из формы/запроса
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	subscriptionID, err := strconv.ParseInt(r.FormValue("subscription_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid subscription ID", http.StatusBadRequest)
		return
	}
	
	// Здесь вы должны получить детали подписки (цену) из БД
	// Для примера, хардкодим
	subscription, err := h.DB.GetSubscriptionByID(r.Context(), subscriptionID)
	if err != nil {
		log.Printf("Error getting subscription %d: %v", subscriptionID, err)
		http.Error(w, "Subscription not found", http.StatusNotFound)
		return
	}


	// 3. Создаем запись о платеже в нашей БД со статусом "pending"
	newPayment := &models.Payment{
		UserID:         userID,
		SubscriptionID: subscription.ID,
		Amount:         subscription.Price,
		Currency:       h.Cfg.BCCGateway.Currency,
		Status:         "pending",
		GatewayName:    "bcc",
	}
	
	err = h.DB.CreatePayment(r.Context(), newPayment)
	if err != nil {
		log.Printf("Error creating payment record for user %d: %v", userID, err)
		http.Error(w, "Failed to initialize payment", http.StatusInternalServerError)
		return
	}

	// 4. Формируем запрос к API банка
	user, err := h.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		log.Printf("Error getting user %d: %v", userID, err)
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	
	createOrderReq := bcc.CreateOrderRequest{
		Amount:          newPayment.Amount,
		MerchantOrderID: strconv.FormatInt(newPayment.ID, 10), // Используем наш ID платежа как ID заказа
		Currency:        newPayment.Currency,
		Description:     "Оплата подписки: " + subscription.Name,
		Client: bcc.ClientInfo{
			Email: user.Email,
			Name:  user.Name,
			Phone: user.Phone,
		},
		Options: bcc.Options{
			ReturnURL: h.Cfg.BCCGateway.ReturnURL,
		},
	}

	// 5. Отправляем запрос в банк
	result, err := h.BCCClient.CreateOrder(r.Context(), createOrderReq)
	if err != nil {
		log.Printf("Error creating BCC order for payment %d: %v", newPayment.ID, err)
		// Обновляем статус нашего платежа на "failed"
		_ = h.DB.UpdateGatewayInfo(r.Context(), newPayment.ID, "", "failed")
		http.Error(w, "Could not contact payment provider", http.StatusInternalServerError)
		return
	}
	
	// 6. Сохраняем ID заказа от банка и обновляем статус
	err = h.DB.UpdateGatewayInfo(r.Context(), newPayment.ID, result.GatewayOrderID, "processing")
	if err != nil {
		log.Printf("CRITICAL: Failed to save GatewayOrderID %s for payment %d: %v", result.GatewayOrderID, newPayment.ID, err)
		http.Error(w, "Payment processing error", http.StatusInternalServerError)
		return
	}


	// 7. Перенаправляем пользователя на страницу оплаты
	http.Redirect(w, r, result.PaymentURL, http.StatusSeeOther)
}


// HandleBCCSuccess - обработчик для успешного callback
func (h *BillingHandler) HandleBCCSuccess(w http.ResponseWriter, r *http.Request) {
    // Внимание: В документации банка неясно, как именно передается ID заказа при редиректе.
    // Предположим, что он будет в параметре ?order_id=
    // Это нужно будет уточнить у банка и скорректировать.
	gatewayOrderID := r.URL.Query().Get("order_id")
	if gatewayOrderID == "" {
		log.Println("BCC Success callback: gateway_order_id not found in query params")
		http.Error(w, "Invalid payment callback", http.StatusBadRequest)
		return
	}

	// Проверяем статус заказа в системе банка
	statusResp, err := h.BCCClient.GetOrderStatus(r.Context(), gatewayOrderID)
	if err != nil {
		log.Printf("Error getting order status for %s: %v", gatewayOrderID, err)
		http.Redirect(w, r, "/payment-failed", http.StatusSeeOther) // Перенаправляем на страницу ошибки
		return
	}
	
	if len(statusResp.Orders) > 0 {
		orderStatus := statusResp.Orders[0].Status
		
		// "charged" - средства списаны (для одностадийной схемы)
		// "authorized" - средства захолдированы (для двухстадийной)
		if orderStatus == "charged" || orderStatus == "authorized" {
			// Находим наш платеж по gateway_order_id
			payment, err := h.DB.GetPaymentByGatewayID(r.Context(), gatewayOrderID)
			if err != nil || payment == nil {
				log.Printf("CRITICAL: Payment not found for gateway_order_id %s", gatewayOrderID)
				http.Error(w, "Payment data mismatch", http.StatusInternalServerError)
				return
			}
			
			// Обновляем статус платежа в нашей БД
			_ = h.DB.UpdateStatusByGatewayID(r.Context(), gatewayOrderID, "success")
			
			// Активируем подписку для пользователя
			_ = h.DB.ActivateUserSubscription(r.Context(), payment.UserID, payment.SubscriptionID) // Вам нужно будет реализовать этот метод
			
			// Перенаправляем на страницу успеха в личном кабинете
			http.Redirect(w, r, "/dashboard?payment=success", http.StatusSeeOther)
			return
		}
	}
	
	// Если статус другой, считаем платеж неуспешным
	_ = h.DB.UpdateStatusByGatewayID(r.Context(), gatewayOrderID, "failed")
	http.Redirect(w, r, "/payment-failed", http.StatusSeeOther)
}

func NewBillingHandlers(sm *scs.SessionManager, cfg *config.Config, ah *AppHandlers) *BillingHandlers {
	return &BillingHandlers{SessionManager: sm, Config: cfg, AppHandlers: ah}
}

func generateXMLSignature(params []Attribute, secretKey string) string {
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	var signatureString string
	for _, attr := range params {
		signatureString += attr.Value
	}
	signatureString += secretKey

	h := sha256.New()
	h.Write([]byte(signatureString))
	return hex.EncodeToString(h.Sum(nil))
}

func (bh *BillingHandlers) CreatePaymentLinkHandler(w http.ResponseWriter, r *http.Request) {
    // ... (код этой функции остается без изменений) ...
}

func (bh *BillingHandlers) PaymentSuccessPageHandler(w http.ResponseWriter, r *http.Request) {
    // ... (код этой функции остается без изменений) ...
}

func (bh *BillingHandlers) PaymentFailurePageHandler(w http.ResponseWriter, r *http.Request) {
    // ... (код этой функции остается без изменений) ...
}

func (bh *BillingHandlers) PaymentWebhookHandler(w http.ResponseWriter, r *http.Request) {
    // ... (код этой функции остается без изменений, включая // TODO: для подписи) ...
}

func (bh *BillingHandlers) CancelSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	currentUser, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok || currentUser == nil {
		http.Error(w, "Пользователь не аутентифицирован", http.StatusUnauthorized)
		return
	}

	if currentUser.SubscriptionID == nil || *currentUser.SubscriptionID == "" {
		slog.Warn("CancelSubscriptionHandler: у пользователя нет ID подписки для отмены", "userID", currentUser.ID)
		bh.AppHandlers.SessionManager.Put(r.Context(), "flash_error", "У вас нет активной подписки для отмены.")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	gatewaySubscriptionID := *currentUser.SubscriptionID

	// TODO: Здесь должна быть реальная логика отмены подписки в API Unicornlab/BCC, если она у них есть.
	// Например: err := unicorn.CancelSubscription(gatewaySubscriptionID)
	// Поскольку API для отмены подписки может не быть, мы эмулируем отмену только в нашей системе.
	slog.Info("ЗАГЛУШКА: Отмена подписки для", "userID", currentUser.ID, "subscriptionID", gatewaySubscriptionID)

	sub, err := db.GetSubscriptionByGatewayID(gatewaySubscriptionID) // Используем db
	if err != nil || sub == nil {
		slog.Error("Ошибка получения подписки из БД для отмены или подписка не найдена", "userID", currentUser.ID, "subscriptionID", gatewaySubscriptionID, "error", err)
		bh.AppHandlers.SessionManager.Put(r.Context(), "flash_error", "Ошибка данных подписки. Свяжитесь с поддержкой.")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	sub.Status = models.SubscriptionStatusCanceled
	sub.CancelAtPeriodEnd = true
	sub.UpdatedAt = time.Now()
	if sub.CurrentPeriodEnd.After(time.Now()) {
		sub.EndDate = sub.CurrentPeriodEnd // Устанавливаем дату окончания в конец текущего периода
	} else {
		sub.EndDate = time.Now()
	}

	if err := db.CreateOrUpdateSubscription(sub); err != nil { // Используем db
		slog.Error("Ошибка обновления записи в таблице subscriptions при отмене", "userID", currentUser.ID, "error", err)
		bh.AppHandlers.SessionManager.Put(r.Context(), "flash_error", "Подписка отменена, но произошла ошибка при обновлении статуса в вашем аккаунте. Свяжитесь с поддержкой.")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	var customerIDStr string
	if currentUser.CustomerID != nil {
		customerIDStr = *currentUser.CustomerID
	}
	errUpdateUser := db.UpdateUserSubscriptionDetails( // Используем db
		currentUser.ID, gatewaySubscriptionID, customerIDStr, models.SubscriptionStatusCanceled,
		sub.StartDate, sub.EndDate, sub.CurrentPeriodEnd,
	)
	if errUpdateUser != nil {
		slog.Error("Ошибка обновления статуса подписки в таблице users при отмене", "userID", currentUser.ID, "error", errUpdateUser)
	}

	slog.Info("Автопродление подписки успешно отменено для пользователя", "userID", currentUser.ID, "subscriptionID", gatewaySubscriptionID)
	bh.AppHandlers.SessionManager.Put(r.Context(), "flash_success", "Автоматическое продление вашей подписки было успешно отменено. Доступ сохранится до конца оплаченного периода.")
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}