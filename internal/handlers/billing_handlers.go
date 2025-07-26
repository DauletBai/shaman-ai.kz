// internal/handlers/billing_handlers.go
package handlers

import (
	//"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	//"fmt"
	//"io"
	"log/slog"
	"net/http"
	"sort"
	//"strings"
	"time"

	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/models"

	"github.com/alexedwards/scs/v2"
	//"github.com/google/uuid"
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