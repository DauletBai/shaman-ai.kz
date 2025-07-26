// internal/handlers/pages.go
package handlers

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime" 
	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/models"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
)

type PageData struct {
	SiteName                 string
	SiteDescription          string
	CurrentYear              int
	BaseURL                  string
	CurrentPath              string
	CSRFToken                string
	IsAuthenticated          bool
	LoggedInUserID           int64
	User                     *models.User
	Flash                    string
	Errors                   url.Values
	Form                     interface{}
	PageTitle                string
	PageDescription          string
	UserName                 string
	CanonicalURL             string
	RobotsContent            string
	AdminPageTitle           string
	Users                    []*models.User
	AllRoles                 []models.Role
	TotalUsers               int
	CurrentPage              int
	TotalPages               int
	Limit                    int
	FormAction               string
	EditingUser              *models.User
	SessionManager           *scs.SessionManager
	Request                  *http.Request
	Stats                    *db.ReportStats
	AppSettings              map[string]string
	AppConfig                *config.Config
	ProfileUpdateFormValues  url.Values
	ProfileUpdateErrors      url.Values
	PasswordChangeErrors     url.Values
	FlashSuccess             string
	FlashError               string
	FlashErrorPW             string
	FormValues               url.Values
	IsComingSoonMode           bool 
	LaunchDate                 string 
	TokenUsageWarning          string
	ShowResendVerificationLink bool
}

type AppHandlers struct {
	Config              *config.Config
	BaseTmpl            *template.Template
	AdminBaseTmpl       *template.Template
	PagesPath           string
	AdminPagesPath      string
	SessionManager      *scs.SessionManager
	RenderPageFunc      func(w http.ResponseWriter, r *http.Request, pageName string, data *PageData)
	RenderAdminPageFunc func(w http.ResponseWriter, r *http.Request, pageName string, data *PageData)
}

func parseBaseTemplates(baseDir string, baseFilename string, appBaseURL string) (*template.Template, error) {
	baseFile := filepath.Join(baseDir, baseFilename)
	if _, err := os.Stat(baseFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("базовый шаблон '%s' не найден в '%s'", baseFilename, baseDir)
	}

	// Определение пути к директории parts относительно текущего файла (pages.go)
	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("не удалось получить путь к текущему файлу для определения пути к parts")
	}
	projectRootForParts := filepath.Join(filepath.Dir(currentFilePath), "..", "..") // Два ".." чтобы выйти в корень shaman-ai.kz
	partsDir := filepath.Join(projectRootForParts, "templates", "parts")


	partFiles, err := filepath.Glob(filepath.Join(partsDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска частичных шаблонов в '%s': %w", partsDir, err)
	}

	funcMap := template.FuncMap{
		"eq":         func(a, b interface{}) bool { return a == b },
		"safeHTML":   func(s string) template.HTML { return template.HTML(s) },
		"add":        func(a, b int) int { return a + b },
		"hasPrefix":  strings.HasPrefix,
		"base_url":   func() string { return strings.TrimSuffix(appBaseURL, "/") },
		"trimSuffix": strings.TrimSuffix,
		"div":        func(a, b int) int { if b == 0 { return 0 }; return a / b }, // Для деления в шаблоне (например, цены)
		"seq": func(start, end int) []int {
			var s []int
			if start > end {
				return s
			}
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
	}

	tmpl, err := template.New(filepath.Base(baseFile)).Funcs(funcMap).ParseFiles(baseFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга базового шаблона '%s': %w", baseFile, err)
	}

	if len(partFiles) > 0 {
		tmpl, err = tmpl.ParseFiles(partFiles...)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга частичных шаблонов из '%s': %w", partsDir, err)
		}
	}
	slog.Info("Базовый шаблон и частичные шаблоны успешно загружены", "base_template", baseFile, "parts_dir", partsDir)
	return tmpl, nil
}


func NewAppHandlers(cfg *config.Config, sm *scs.SessionManager) (*AppHandlers, error) {
    baseTmpl, err := parseBaseTemplates("templates", "base.html", cfg.BaseURL)
    if err != nil {
        return nil, fmt.Errorf("failed to parse base templates: %w", err)
    }
    adminBaseTmpl, err := parseBaseTemplates("templates/admin", "base_admin.html", cfg.BaseURL)
    if err != nil {
        slog.Warn("Не удалось загрузить базовый шаблон админки. Админка может не работать.", "error", err)
        // Не возвращаем ошибку, чтобы основное приложение могло работать
    }
    if cfg.CurrentYear == 0 {
        cfg.CurrentYear = time.Now().Year()
    }

    appH := &AppHandlers{
        Config:         cfg,
        BaseTmpl:       baseTmpl,
        AdminBaseTmpl:  adminBaseTmpl,
        PagesPath:      filepath.Join("templates", "pages"),
        AdminPagesPath: filepath.Join("templates", "admin", "pages"),
        SessionManager: sm,
    }
    appH.RenderPageFunc = appH.renderPageInternal
    appH.RenderAdminPageFunc = appH.renderAdminPageInternal
    return appH, nil
}

func (h *AppHandlers) renderPageInternal(w http.ResponseWriter, r *http.Request, pageName string, data *PageData) {
    h.render(w, r, h.BaseTmpl, h.PagesPath, "base.html", pageName, data)
}

func (h *AppHandlers) renderAdminPageInternal(w http.ResponseWriter, r *http.Request, pageName string, data *PageData) {
    if data == nil { data = h.NewPageData(r) }
    if data.AdminPageTitle == "" && data.PageTitle != "" { data.AdminPageTitle = data.PageTitle
    } else if data.AdminPageTitle == "" { data.AdminPageTitle = "Панель администратора" }
    // Для страниц админки PageTitle может быть более специфичным
    // data.PageTitle = fmt.Sprintf("%s | %s Admin", data.AdminPageTitle, h.Config.SiteName)
    h.render(w, r, h.AdminBaseTmpl, h.AdminPagesPath, "base_admin.html", pageName, data)
}

func (h *AppHandlers) RenderPage(w http.ResponseWriter, r *http.Request, pageName string, data *PageData) {
    h.RenderPageFunc(w, r, pageName, data)
}

func (h *AppHandlers) RenderAdminPage(w http.ResponseWriter, r *http.Request, pageName string, data *PageData) {
    h.RenderAdminPageFunc(w, r, pageName, data)
}

func (h *AppHandlers) NewPageData(r *http.Request) *PageData {
	isAuthenticatedVal, _ := r.Context().Value(middleware.IsAuthenticatedContextKey).(bool)
	currentUser, _ := r.Context().Value(middleware.UserContextKey).(*models.User)

	canonicalURL := strings.TrimSuffix(h.Config.BaseURL, "/") + r.URL.Path
	var userName string
	var loggedInUserIDVal int64

	if isAuthenticatedVal && currentUser != nil {
		userName = currentUser.FirstName
		if userName == "" {
			userName = currentUser.Email
		}
		loggedInUserIDVal = currentUser.ID
	} else {
		userName = "Гость"
	}

	flashSuccess := h.SessionManager.PopString(r.Context(), "flash_success")
	flashError := h.SessionManager.PopString(r.Context(), "flash_error")
	flashErrorPW := h.SessionManager.PopString(r.Context(), "flash_error_pw")

	profileUpdateErrors, _ := h.SessionManager.Pop(r.Context(), "profile_update_errors").(url.Values)
	if profileUpdateErrors == nil {
		profileUpdateErrors = url.Values{}
	}
	profileUpdateFormValues, _ := h.SessionManager.Pop(r.Context(), "profile_update_form_values").(url.Values)
	if profileUpdateFormValues == nil {
		profileUpdateFormValues = url.Values{}
	}

	passwordChangeErrors, _ := h.SessionManager.Pop(r.Context(), "password_change_errors").(url.Values)
	if passwordChangeErrors == nil {
		passwordChangeErrors = url.Values{}
	}

    // Установка даты для таймера "Скоро открытие" (например, через 7 дней от сейчас)
    // Формат для JS: "YYYY/MM/DD HH:MM:SS"
    launchTime := time.Now().AddDate(0,0,7) // Через 7 дней

	return &PageData{
		SiteName:                 h.Config.SiteName,
		SiteDescription:          h.Config.SiteDescription,
		CurrentYear:              h.Config.CurrentYear,
		BaseURL:                  strings.TrimSuffix(h.Config.BaseURL, "/"),
		CurrentPath:              r.URL.Path,
		CSRFToken:                nosurf.Token(r),
		IsAuthenticated:          isAuthenticatedVal,
		LoggedInUserID:           loggedInUserIDVal,
		User:                     currentUser,
		UserName:                 userName,
		CanonicalURL:             canonicalURL,
		RobotsContent:            "index, follow",
		SessionManager:           h.SessionManager,
		Request:                  r,
		AppConfig:                h.Config,
		Errors:                   url.Values{},
		FlashSuccess:             flashSuccess,
		FlashError:               flashError,
		FlashErrorPW:             flashErrorPW,
		ProfileUpdateErrors:      profileUpdateErrors,
		ProfileUpdateFormValues:  profileUpdateFormValues,
		PasswordChangeErrors:     passwordChangeErrors,
		IsComingSoonMode:         true, // Установите true для активации режима "Скоро открытие"
        LaunchDate:               launchTime.Format("2006/01/02 15:04:05"),
	}
}

func (h *AppHandlers) render(w http.ResponseWriter, r *http.Request, baseTmpl *template.Template, pagesDir, baseFile, pageName string, data *PageData) {
	if data == nil {
		data = h.NewPageData(r)
	} else {
		if data.SessionManager == nil {
			data.SessionManager = h.SessionManager
		}
		if data.Request == nil {
			data.Request = r
		}
        // Убедимся, что AppConfig всегда есть в PageData
        if data.AppConfig == nil {
            data.AppConfig = h.Config
        }
	}

	if baseTmpl == nil {
		slog.Error("Базовый шаблон не инициализирован для рендера", "base_file_expected", baseFile)
		http.Error(w, "Внутренняя ошибка сервера (шаблон)", http.StatusInternalServerError)
		return
	}

	if data.PageTitle == "" {
		data.PageTitle = h.Config.SiteName
	}
	if data.PageDescription == "" && baseFile == "base.html" { // Только для основного сайта, не админки
		data.PageDescription = h.Config.SiteDescription
	}
    if data.IsComingSoonMode && pageName != "coming_soon.html" && data.CurrentPath == "/" {
        // Если режим "скоро открытие" и это главная, но не сама страница заглушки
        // (это условие больше для WelcomePageHandler)
    }

	slog.Debug("Данные для рендера страницы",
		"page", pageName, "base_file", baseFile, "URL_Path", r.URL.Path,
		"data.IsAuthenticated", data.IsAuthenticated, "data.IsComingSoonMode", data.IsComingSoonMode)

	pagePath := filepath.Join(pagesDir, pageName)
	if _, err := os.Stat(pagePath); os.IsNotExist(err) {
		slog.Error("Файл шаблона страницы не найден", "page", pageName, "path", pagePath)
		http.Error(w, "Внутренняя ошибка сервера (шаблон страницы)", http.StatusInternalServerError)
		return
	}

	tmplToExecute, err := baseTmpl.Clone()
	if err != nil {
		slog.Error("Не удалось клонировать базовый шаблон", "base_file", baseFile, "error", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	tmplToExecute, err = tmplToExecute.ParseFiles(pagePath)
	if err != nil {
		slog.Error("Не удалось загрузить шаблон страницы", "page", pageName, "path", pagePath, "error", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	slog.Debug("Определение блоков в шаблоне", "base_file", baseFile, "page_name", pageName)
	if tmplToExecute.Lookup(baseFile) != nil {
		slog.Debug("Найден базовый шаблон в tmplToExecute", "name", tmplToExecute.Lookup(baseFile).Name())
		if tmplToExecute.Lookup(baseFile).Lookup("admin_content") != nil && strings.HasPrefix(baseFile, "base_admin") {
			slog.Debug("Найден блок 'admin_content' в базовом шаблоне админки")
		} else if tmplToExecute.Lookup(baseFile).Lookup("content") != nil && baseFile == "base.html" {
			slog.Debug("Найден блок 'content' в основном базовом шаблоне")
		} else {
			slog.Warn("Блок контента ('admin_content' или 'content') НЕ найден в соответствующем базовом шаблоне tmplToExecute", "baseFile", baseFile)
		}
	} else {
		slog.Error("Базовый шаблон НЕ найден в tmplToExecute перед рендерингом", "baseFile", baseFile)
	}
	if tmplToExecute.Lookup(pageName) != nil {
		slog.Debug("Найден шаблон страницы в tmplToExecute", "name", tmplToExecute.Lookup(pageName).Name())
	} else {
		slog.Warn("Шаблон страницы НЕ найден в tmplToExecute", "page_name", pageName)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	err = tmplToExecute.ExecuteTemplate(w, baseFile, data)
	if err != nil {
		slog.Error("Ошибка выполнения шаблона", "template", baseFile, "page", pageName, "error", err)
	}
}

func (h *AppHandlers) WelcomePageHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

    data := h.NewPageData(r) // NewPageData теперь устанавливает IsComingSoonMode и LaunchDate

    // ПРОВЕРКА: Если IsComingSoonMode = true (или другой ваш флаг из cfg или AppSettings)
    // И мы еще не на странице "Скоро открытие" (хотя для WelcomePageHandler это всегда так)
    // То показываем страницу "Скоро открытие"
    // Для простоты, будем всегда показывать coming_soon.html, если IsComingSoonMode=true
    // В production вы можете управлять этим флагом через ENV или app_settings в БД.

    // Для этого примера, мы будем считать, что isComingSoonMode всегда true на данном этапе
    // и WelcomePageHandler всегда показывает coming_soon.html.
    // В реальном приложении, вы бы читали этот флаг из cfg.SomeComingSoonFlag

    // if data.IsComingSoonMode { // Если вы хотите иметь возможность переключать этот режим
    // data.PageTitle = "Скоро открытие Sham'an AI"
    // data.PageDescription = "Уникальная модель ИИ, разработанная в Казахстане. Следите за обновлениями!"
    // data.RobotsContent = "noindex, nofollow" // Пока не открылись, лучше не индексировать
    // h.RenderPage(w, r, "coming_soon.html", data)
    // return
    // }

    // Код ниже будет выполняться, если IsComingSoonMode = false
    isAuthenticated, _ := r.Context().Value(middleware.IsAuthenticatedContextKey).(bool)
	if isAuthenticated {
		currentUser, _ := r.Context().Value(middleware.UserContextKey).(*models.User)
		if currentUser != nil && currentUser.RoleName != nil && *currentUser.RoleName == models.RoleAdmin {
			http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		}
		return
	}

	data.PageTitle = "Welcome to Sham'an AI"
	data.PageDescription = "Discover Shaman AI, your digital family doctor for psychological support and self-healing."
	h.RenderPage(w, r, "welcome.html", data)
}
// ... остальные хендлеры без изменений ...
func (h *AppHandlers) DashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	data := h.NewPageData(r)
	data.PageTitle = "Ваша панель управления"
	data.PageDescription = "Управляйте сессиями и взаимодействуйте с Sham'an AI."
	data.RobotsContent = "noindex, nofollow"

	// Проверяем лимит токенов для текущего пользователя
	if data.User != nil {
		costInputUSD := (float64(data.User.TokensUsedInputThisPeriod) / 1000000.0) * h.Config.RemoteLLM.TokenCostInputPerMillion
		costOutputUSD := (float64(data.User.TokensUsedOutputThisPeriod) / 1000000.0) * h.Config.RemoteLLM.TokenCostOutputPerMillion
		totalCostUSD := costInputUSD + costOutputUSD
		totalCostKZT := totalCostUSD * h.Config.Billing.USDToKZTRate

		if totalCostKZT >= h.Config.TokenMonthlyLimitKZT {
			nextBillingDate := "следующего платежа"
			if data.User.CurrentPeriodEnd != nil {
				nextBillingDate = "даты " + data.User.CurrentPeriodEnd.Format("02.01.2006")
			}
			data.TokenUsageWarning = fmt.Sprintf("Вы превысили месячный лимит использования. Доступ к AI будет возобновлен после %s.", nextBillingDate)
			slog.Info("Пользователю будет показано предупреждение о превышении лимита", "userID", data.User.ID)
		}
	}

	h.RenderPage(w, r, "dashboard.html", data)
}

func (h *AppHandlers) SubscribePageHandler(w http.ResponseWriter, r *http.Request) {
	pageDataForCheck := h.NewPageData(r)
	isAuthenticated := pageDataForCheck.IsAuthenticated

	if isAuthenticated && pageDataForCheck.User != nil {
		userFromDB, err := db.GetUserByID(pageDataForCheck.User.ID)
		if err == nil && userFromDB != nil {
			if userFromDB.SubscriptionStatus == models.SubscriptionStatusActive && (userFromDB.CurrentPeriodEnd == nil || userFromDB.CurrentPeriodEnd.After(time.Now())) {
				slog.Info("Пользователь уже имеет активную подписку, перенаправление на дашборд", "userID", userFromDB.ID)
				http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
				return
			}
		} else if err != nil {
			slog.Error("SubscribePageHandler: не удалось получить пользователя из БД для проверки подписки", "userID", pageDataForCheck.User.ID, "error", err)
		}
	}

	if !isAuthenticated {
		h.SessionManager.Put(r.Context(), "flash_error", "Пожалуйста, сначала зарегистрируйтесь или войдите.")
		http.Redirect(w, r, "/login?redirect=/subscribe", http.StatusSeeOther)
		return
	}

	data := h.NewPageData(r)
	data.PageTitle = "Оформление подписки"
	data.PageDescription = "Оформите подписку, чтобы получить доступ к Sham'an AI."
	data.RobotsContent = "noindex, nofollow"

	userEmail := ""
	if data.User != nil {
		userEmail = data.User.Email
	}

	data.Form = map[string]string{
		"PaymentGatewayPublishableKey": h.Config.Billing.PaymentGatewayPublishableKey,
		"PriceID":                      h.Config.Billing.PriceID,
		"UserEmail":                    userEmail,
		"Currency":                     h.Config.Billing.Currency,
		// "Amount" теперь берется из h.Config.Billing.MonthlyAmount в шаблоне
		// Для передачи в JS, если нужно, можно сделать так:
		// "AmountValue": fmt.Sprintf("%d", h.Config.Billing.MonthlyAmount / 100),
	}

	h.RenderPage(w, r, "subscribe.html", data)
}

func (h *AppHandlers) DocumentationPageHandler(w http.ResponseWriter, r *http.Request) {
	data := h.NewPageData(r)
	data.PageTitle = "Документация Sham'an AI"
	data.PageDescription = "Подробное описание работы и возможностей приложения Sham'an AI."
	h.RenderPage(w, r, "documentation.html", data)
}

func (h *AppHandlers) ProfilePageHandler(w http.ResponseWriter, r *http.Request) {
	data := h.NewPageData(r)
	data.PageTitle = "Профиль пользователя"
	data.PageDescription = "Управление вашим профилем в Sham'an AI."
	data.RobotsContent = "noindex, nofollow"
	h.RenderPage(w, r, "profile.html", data)
}

func (h *AppHandlers) SettingsPageHandler(w http.ResponseWriter, r *http.Request) {
	data := h.NewPageData(r)
	data.PageTitle = "Настройки приложения"
	data.PageDescription = "Настройки вашего приложения Sham'an AI."
	data.RobotsContent = "noindex, nofollow"
	h.RenderPage(w, r, "settings.html", data)
}

func (h *AppHandlers) PublicOfferPageHandler(w http.ResponseWriter, r *http.Request) {
	data := h.NewPageData(r)
	data.PageTitle = "Публичный договор-оферта"
	data.PageDescription = "Условия предоставления услуг сервиса Sham'an AI."
	data.RobotsContent = "index, follow" // Оставляем для индексации
	h.RenderPage(w, r, "public_offer_agreement.html", data)
}