// internal/handlers/auth.go
package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"shaman-ai.kz/internal/auth"
	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/email" 
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/models"
	"shaman-ai.kz/internal/sms"
	"shaman-ai.kz/internal/validation"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
)

type AuthHandlers struct {
	SessionManager *scs.SessionManager
	Render         func(w http.ResponseWriter, r *http.Request, pageName string, data *PageData)
	NewPageData    func(r *http.Request) *PageData
	AppConfig      *config.Config
}

func NewAuthHandlers(sm *scs.SessionManager, renderFunc func(http.ResponseWriter, *http.Request, string, *PageData), newPageDataFunc func(*http.Request) *PageData, cfg *config.Config) *AuthHandlers {
	return &AuthHandlers{
		SessionManager: sm,
		Render:         renderFunc,
		NewPageData:    newPageDataFunc,
		AppConfig:      cfg,
	}
}

func (h *AuthHandlers) RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
	data := h.NewPageData(r)
	data.PageTitle = "Register Account"
	data.PageDescription = "Create your account to start using Shaman AI."
	data.RobotsContent = "noindex, follow"
	data.Form = models.RegistrationForm{}
	h.Render(w, r, "register.html", data)
}



// Заменяем старую заглушку SendVerificationEmail
func SendUserVerificationEmail(appCfg *config.Config, toEmail, verificationLink string) error {
	subject := "Подтвердите ваш email на " + appCfg.SiteName

	// Данные для HTML шаблона
	templateData := struct {
		SiteName         string
		VerificationLink string
		UserEmail        string
	}{
		SiteName:         appCfg.SiteName,
		VerificationLink: verificationLink,
		UserEmail:        toEmail,
	}
	// Предполагаем, что у вас есть шаблон verification_email.html
	// Вместо передачи пустого bodyContent, мы будем полагаться на шаблон
	return email.SendEmail(appCfg, toEmail, subject, "", true, "verification_email.html", templateData)
}

func SendSMSVerificationCode(cfg *config.Config, phoneNumber, code string) error {
    message := fmt.Sprintf("Ваш код подтверждения для Shaman AI: %s", code)
    return sms.SendSMS(cfg, phoneNumber, message)
}

func (h *AuthHandlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		slog.Error("Ошибка парсинга формы регистрации", "error", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	form := models.RegistrationForm{
		Email:       r.PostForm.Get("email"),
		Phone:       r.PostForm.Get("phone"),
		Password:    r.PostForm.Get("password"),
		ConfirmPass: r.PostForm.Get("confirm_password"),
		FirstName:   r.PostForm.Get("first_name"),
		LastName:    r.PostForm.Get("last_name"),
		Gender:      r.PostForm.Get("gender"),
		Birthday:    r.PostForm.Get("birthday"),
		AgreeTerms:  r.PostForm.Get("agree_terms"),
		Honeypot:    r.PostForm.Get("website"),
	}

	if form.Honeypot != "" {
		http.Error(w, "Обнаружена подозрительная активность", http.StatusBadRequest)
		return
	}

	validationErrors := validation.ValidateStruct(form)
	if validationErrors == nil {
		validationErrors = url.Values{}
	}

	if form.AgreeTerms != "on" {
		validationErrors.Add("agree_terms", "Необходимо согласиться с условиями.")
	}

	if len(validationErrors) > 0 {
		slog.Warn("Ошибки валидации при регистрации", "errors", validationErrors)
		form.Password = ""
		form.ConfirmPass = ""
		data := h.NewPageData(r)
		data.PageTitle = "Register Account - Error"
		data.PageDescription = "Please correct the errors below."
		data.RobotsContent = "noindex, follow"
		data.Form = form
		data.Errors = validationErrors
		w.WriteHeader(http.StatusBadRequest)
		h.Render(w, r, "register.html", data)
		return
	}

	hashedPassword, err := auth.HashPassword(form.Password)
	if err != nil {
		slog.Error("Ошибка хеширования пароля", "error", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	user := &models.User{
		Email:        strings.ToLower(form.Email),
		PasswordHash: hashedPassword,
		FirstName:    auth.SanitizeName(form.FirstName),
		LastName:     auth.SanitizeName(form.LastName),
		Gender:       form.Gender,
		Birthday:     form.Birthday,
	}
	phoneSanitized := form.Phone 
	if phoneSanitized != "" {
		user.Phone = &phoneSanitized
	}

	userID, err := db.CreateUser(user, models.RoleUser)
	if err != nil {
		slog.Error("Ошибка создания пользователя в БД", "error", err, "email", user.Email)
		data := h.NewPageData(r)
		data.PageTitle = "Register Account - Error"
		data.PageDescription = "Error during account creation."
		data.RobotsContent = "noindex, follow"
		data.Form = form 
		data.Errors = url.Values{}

		if strings.Contains(err.Error(), "уже существует") {
			w.WriteHeader(http.StatusBadRequest) 
			if strings.Contains(err.Error(), "email") {
				data.Errors.Add("email", "Пользователь с таким email уже существует.")
			} else if strings.Contains(err.Error(), "телефоном") {
				data.Errors.Add("phone", "Пользователь с таким телефоном уже существует.")
			} else {
				data.Errors.Add("general", "Ошибка уникальности данных. Попробуйте другие значения.")
			}
		} else if strings.Contains(err.Error(), "роль по умолчанию") {
			w.WriteHeader(http.StatusInternalServerError) 
			data.Errors.Add("general", "Критическая ошибка сервера при регистрации. Пожалуйста, свяжитесь с поддержкой.")
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			data.Errors.Add("general", "Ошибка сервера при регистрации. Пожалуйста, попробуйте позже.")
		}
		h.Render(w, r, "register.html", data)
		return
	}
	user.ID = userID // Присваиваем ID в модель для дальнейшего использования

	// 1. Отправка письма для верификации email (происходит в фоне)
	go func() {
		rawToken, errToken := db.GenerateSecureToken(32)
		if errToken != nil {
			slog.Error("Ошибка генерации токена верификации email", "userID", userID, "error", errToken)
			return
		}
		if errSetToken := db.SetEmailVerificationToken(userID, rawToken); errSetToken != nil {
			slog.Error("Ошибка сохранения токена верификации email в БД", "userID", userID, "error", errSetToken)
			return
		}
		verificationLink := fmt.Sprintf("%s/verify-email?token=%s", h.AppConfig.BaseURL, rawToken)
		if errSendMail := SendUserVerificationEmail(h.AppConfig, user.Email, verificationLink); errSendMail != nil {
			slog.Error("Ошибка отправки письма для верификации email", "userID", userID, "email", user.Email, "error", errSendMail)
		} else {
			slog.Info("Письмо для верификации email успешно отправлено (или поставлено в очередь)", "userID", userID, "email", user.Email)
		}
	}()

	// 2. Отправка СМС для верификации телефона (блокирует дальнейшие действия до отправки)
	if user.Phone != nil && *user.Phone != "" {
		code := strconv.Itoa(100000 + rand.Intn(900000)) // 6-значный код
		if err := db.SetPhoneVerificationCode(userID, code); err != nil {
			slog.Error("Не удалось сохранить код верификации телефона", "userID", userID, "error", err)
			http.Error(w, "Произошла внутренняя ошибка, регистрация не может быть завершена.", http.StatusInternalServerError)
			return
		}
		if err := sms.SendSMS(h.AppConfig, *user.Phone, "Ваш код подтверждения для Shaman AI: "+code); err != nil {
			slog.Error("Не удалось отправить СМС", "userID", userID, "error", err)
			http.Error(w, "Произошла ошибка при отправке СМС, регистрация не может быть завершена.", http.StatusInternalServerError)
			return
		}
	}
    
    // 3. Автоматический логин пользователя и редирект на страницу верификации телефона
    // Это улучшает UX: пользователю не нужно снова вводить логин/пароль.
    err = h.SessionManager.RenewToken(r.Context())
	if err != nil {
		slog.Error("Ошибка обновления токена сессии после регистрации", "error", err)
		// Не критично, просто отправим на логин
        h.SessionManager.Put(r.Context(), "flash_success", "Регистрация почти завершена! Пожалуйста, подтвердите ваш номер телефона, а также проверьте почту для подтверждения email.")
	    http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.SessionManager.Put(r.Context(), string(middleware.UserIDContextKey), user.ID)

	slog.Info("Пользователь успешно зарегистрирован и залогинен, ожидает верификации телефона", "userID", user.ID)
	h.SessionManager.Put(r.Context(), "flash_success", "Регистрация почти завершена! Мы отправили код на ваш номер телефона для подтверждения.")
	http.Redirect(w, r, "/verify-phone", http.StatusSeeOther)
}


// The following block was outside any function and caused a compile error.
// If you want to enable local development subscription logic, move this code
// into the appropriate handler function after user registration, for example
// after successful registration in RegisterHandler.

// Example: (uncomment and place inside RegisterHandler if needed)
/*
isLocalDevelopment := h.AppConfig.AppEnv == "development"

if isLocalDevelopment && userID != 0 {
	slog.Info("Локальная разработка: автоматическое создание тестовой подписки для нового пользователя", "userID", userID, "email", user.Email)

	priceIDForDev := h.AppConfig.Billing.PriceID
	customerIDForDev := "dev_cust_" + fmt.Sprintf("%d", userID)
	subscriptionIDForDev := "dev_sub_" + fmt.Sprintf("%d", userID)
	status := models.SubscriptionStatusActive
	startDate := time.Now()
	currentPeriodEnd := time.Now().AddDate(1, 0, 0) // Подписка на 1 год для разработки
	var subscriptionEndDate time.Time // Для бессрочной подписки оставляем Zero Time

	errUpdateSub := db.UpdateUserSubscriptionDetails(
		userID, subscriptionIDForDev, customerIDForDev, status,
		startDate, subscriptionEndDate, currentPeriodEnd,
	)
	if errUpdateSub != nil {
		slog.Error("Локальная разработка: не удалось обновить данные тестовой подписки в таблице users", "userID", userID, "error", errUpdateSub)
	}

	devSub := models.Subscription{
		ID:                           subscriptionIDForDev,
		UserID:                       userID,
		PaymentGatewaySubscriptionID: subscriptionIDForDev,
		PlanID:                       priceIDForDev,
		Status:                       status,
		StartDate:                    startDate,
		EndDate:                      subscriptionEndDate, // Оставляем Zero Time для бессрочной
		CurrentPeriodStart:           startDate,
		CurrentPeriodEnd:             currentPeriodEnd,
		CancelAtPeriodEnd:            false,
		CreatedAt:                    time.Now(),
		UpdatedAt:                    time.Now(),
	}
	if !subscriptionEndDate.IsZero() { // Если бы EndDate была не нулевая
		devSub.EndDate = subscriptionEndDate
	}
	errCreateSub := db.CreateOrUpdateSubscription(&devSub)
	if errCreateSub != nil {
		slog.Error("Локальная разработка: не удалось создать/обновить тестовую подписку в таблице subscriptions", "userID", userID, "error", errCreateSub)
	} else {
		slog.Info("Локальная разработка: тестовая подписка успешно настроена.", "userID", userID, "subID", subscriptionIDForDev)
	}
}
*/

// Removed erroneous code block that was outside any function.

func (h *AuthHandlers) LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	flashSuccess := h.SessionManager.PopString(r.Context(), "flash_success") // Используем flash_success
	flashError := h.SessionManager.PopString(r.Context(), "flash_error")

	data := h.NewPageData(r)
	data.PageTitle = "Login to Sham'an AI"
	data.PageDescription = "Access your Shaman AI dashboard."
	data.RobotsContent = "noindex, follow"
	data.Form = models.LoginForm{}
	data.FlashSuccess = flashSuccess // Передаем в соответствующие поля PageData
	data.FlashError = flashError

	h.Render(w, r, "login.html", data)
}

func (h *AuthHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	err := r.ParseForm()
	if err != nil {
		slog.Error("Ошибка парсинга формы входа", "error", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	form := models.LoginForm{
		Email:    r.PostForm.Get("email"),
		Password: r.PostForm.Get("password"),
	}
	validationErrors := validation.ValidateStruct(form)
	if len(validationErrors) > 0 {
		data := h.NewPageData(r)
		data.PageTitle = "Login - Error"
		data.RobotsContent = "noindex, follow"
		data.Form = form
		data.Errors = validationErrors
		w.WriteHeader(http.StatusBadRequest)
		h.Render(w, r, "login.html", data)
		return
	}
	user, err := db.GetUserByEmail(strings.ToLower(form.Email))
	passwordMatch := false
	if user != nil && err == nil {
		passwordMatch = auth.CheckPasswordHash(form.Password, user.PasswordHash)
	}

	if err != nil || !passwordMatch {
		data := h.NewPageData(r)
		data.PageTitle = "Login - Error"
		data.RobotsContent = "noindex, follow"
		data.Form = form
		data.Errors = url.Values{}
		if errors.Is(err, sql.ErrNoRows) || !passwordMatch {
			data.Errors.Add("general", "Неверный email или пароль.")
			w.WriteHeader(http.StatusUnauthorized) // 401 для неверных кредов
		} else {
			slog.Error("Ошибка поиска пользователя по email при входе", "email", form.Email, "error", err)
			data.Errors.Add("general", "Ошибка сервера при входе.")
			w.WriteHeader(http.StatusInternalServerError)
		}
		h.Render(w, r, "login.html", data)
		return
	}

	// Проверка верификации email перед входом
	if !user.IsEmailVerified {
    slog.Warn("Попытка входа пользователя с не верифицированным email", "userID", user.ID, "email", user.Email)
    
    // Вместо flash-сообщения, которое исчезает, передадим ошибку и флаг напрямую в PageData
    data := h.NewPageData(r)
    data.PageTitle = "Требуется подтверждение Email"
    data.RobotsContent = "noindex, follow"
    data.Form = form
    data.Errors = url.Values{}
    data.Errors.Add("general", "Ваш email не подтвержден. Пожалуйста, проверьте вашу почту и перейдите по ссылке для верификации.")
    data.ShowResendVerificationLink = true // Устанавливаем флаг для показа кнопки/ссылки
    
    w.WriteHeader(http.StatusUnauthorized)
    h.Render(w, r, "login.html", data)
    return
}

	err = h.SessionManager.RenewToken(r.Context())
	if err != nil {
		slog.Error("Ошибка обновления токена сессии", "error", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	h.SessionManager.Put(r.Context(), string(middleware.UserIDContextKey), user.ID)
	// Важно: после этого middleware.InjectUserData должен подхватить UserID и положить всего пользователя в контекст

	// Сохраняем всего пользователя в сессию для доступа через middleware.UserContextKey
    // Это делается в middleware.InjectUserData, но здесь мы можем обновить для немедленного доступа
    // Или, если InjectUserData всегда вызывается после, можно не дублировать.
    // h.SessionManager.Put(r.Context(), string(middleware.UserContextKey), user) // Убедитесь, что это не конфликтует с InjectUserData

	slog.Info("Пользователь успешно вошел", "user_id", user.ID, "email", user.Email, "role", user.RoleName)

	redirectURL := h.SessionManager.PopString(r.Context(), "redirectAfterLogin")
    if redirectURL == "" {
        if user.RoleName != nil && *user.RoleName == models.RoleAdmin {
            redirectURL = "/admin/dashboard"
        } else {
            redirectURL = "/dashboard"
        }
    }
    http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *AuthHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	userID := h.SessionManager.GetInt64(r.Context(), string(middleware.UserIDContextKey))
	userRole := ""
	if ctxUser, ok := r.Context().Value(middleware.UserContextKey).(*models.User); ok && ctxUser != nil && ctxUser.RoleName != nil {
		userRole = *ctxUser.RoleName
	}

	err := h.SessionManager.Destroy(r.Context())
	if err != nil {
		slog.Error("Ошибка удаления сессии при выходе", "error", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	slog.Info("Пользователь вышел", "user_id", userID, "role", userRole)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ResendVerificationEmailHandler обрабатывает запрос на повторную отправку письма верификации
func (h *AuthHandlers) ResendVerificationEmailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	emailAddr := r.PostFormValue("email")
	if emailAddr == "" {
		h.SessionManager.Put(r.Context(), "flash_error", "Email адрес не был предоставлен.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := db.GetUserByEmail(emailAddr)
	// Если пользователь не найден или уже верифицирован, просто показываем общее сообщение об успехе,
	// чтобы не раскрывать информацию о том, зарегистрирован ли email.
	if err != nil || user == nil || user.IsEmailVerified {
		slog.Warn("Запрос на повторную отправку верификации для несуществующего или уже верифицированного email", "email", emailAddr)
		h.SessionManager.Put(r.Context(), "flash_success", "Если такой аккаунт существует и требует верификации, письмо было отправлено повторно.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Генерируем новый токен
	rawToken, errToken := db.GenerateSecureToken(32)
	if errToken != nil {
		slog.Error("Ошибка генерации токена при повторной отправке", "userID", user.ID, "error", errToken)
		h.SessionManager.Put(r.Context(), "flash_error", "Произошла внутренняя ошибка. Пожалуйста, попробуйте позже.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Устанавливаем новый токен в БД
	if err := db.SetEmailVerificationToken(user.ID, rawToken); err != nil {
		slog.Error("Ошибка сохранения токена при повторной отправке", "userID", user.ID, "error", err)
		h.SessionManager.Put(r.Context(), "flash_error", "Произошла внутренняя ошибка. Пожалуйста, попробуйте позже.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Отправляем письмо
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", h.AppConfig.BaseURL, rawToken)
	// Используем локальную функцию для отправки email
	errSendMail := SendUserVerificationEmail(h.AppConfig, user.Email, verificationLink)

	if errSendMail != nil {
		slog.Error("Ошибка повторной отправки письма для верификации email", "userID", user.ID, "email", user.Email, "error", errSendMail)
		// Не показываем ошибку пользователю, но логируем ее
	} else {
		slog.Info("Письмо для верификации email успешно отправлено повторно", "userID", user.ID, "email", user.Email)
	}
	
	h.SessionManager.Put(r.Context(), "flash_success", "Письмо с подтверждением было отправлено повторно. Пожалуйста, проверьте вашу почту.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
