// cmd/server/main.go
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"shaman-ai.kz/internal/config"
	"shaman-ai.kz/internal/db"
	"shaman-ai.kz/internal/handlers"
	adminhandlers "shaman-ai.kz/internal/handlers/admin"
	"shaman-ai.kz/internal/middleware"
	"shaman-ai.kz/internal/models"
	"shaman-ai.kz/internal/utils"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/v2"

	_ "github.com/go-sql-driver/mysql"
)

var sessionManager *scs.SessionManager
var shamanSystemPrompt string
var generalSystemPrompt string

func main() {
	configPath := "configs/config.yaml"
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Критическая ошибка: не удалось загрузить конфигурацию: %v\n", err)
		os.Exit(1)
	}

	config.InitLogger(cfg.AppEnv)
	slog.Info("Запуск сервера Shaman...", "app_env", cfg.AppEnv)

	shamanSystemPrompt, err = utils.LoadSystemPrompt(cfg.RemoteLLM.ShamanSystemPromptPath)
	if err != nil {
		slog.Error("Критическая ошибка: не удалось загрузить системный промпт Shaman", "path", cfg.RemoteLLM.ShamanSystemPromptPath, "error", err)
		os.Exit(1)
	}
	slog.Info("Системный промпт Shaman успешно загружен")

	if cfg.RemoteLLM.GeneralSystemPromptPath != "" {
		generalSystemPrompt, err = utils.LoadSystemPrompt(cfg.RemoteLLM.GeneralSystemPromptPath)
		if err != nil {
			slog.Error("Ошибка: не удалось загрузить общий системный промпт, используется дефолтный", "path", cfg.RemoteLLM.GeneralSystemPromptPath, "error", err)
			generalSystemPrompt = "Ты — полезный AI-ассистент."
		} else {
			slog.Info("Общий системный промпт успешно загружен")
		}
	} else {
		generalSystemPrompt = "Ты — полезный AI-ассистент."
		slog.Info("Общий системный промпт не указан в конфиге, используется дефолтный.")
	}

	// Загрузка или установка smokingSystemPrompt (если необходимо)
	// if cfg.RemoteLLM.QuitSmokingSystemPromptPath != "" { ... }

	err = db.InitDB(cfg)
	if err != nil {
		slog.Error("Критическая ошибка: не удалось инициализировать базу данных", "error", err)
		os.Exit(1)
	}
	if db.DB != nil {
		defer db.DB.Close()
	} else {
		slog.Error("Критическая ошибка: подключение к БД равно nil после InitDB")
		os.Exit(1) // Критично, если db.DB == nil
	}
	slog.Info("База данных успешно инициализирована и миграции применены.")

	// Запуск планировщика очистки токенов (например, раз в 24 часа)
	db.StartTokenCleanupScheduler(24 * time.Hour)

	firstAdminEmail := os.Getenv("FIRST_ADMIN_EMAIL")
	if firstAdminEmail != "" {
		adminUser, errDbGetUserByEmail := db.GetUserByEmail(firstAdminEmail)
		if errDbGetUserByEmail == nil && adminUser != nil {
			if adminUser.RoleName == nil || *adminUser.RoleName != models.RoleAdmin {
				adminRole, errRole := db.GetRoleByName(models.RoleAdmin)
				if errRole == nil && adminRole != nil {
					errSetRole := db.SetUserRole(adminUser.ID, adminRole.ID)
					if errSetRole != nil {
						slog.Error("Не удалось установить роль администратора", "email", firstAdminEmail, "error", errSetRole)
					} else {
						slog.Info("Роль администратора успешно установлена", "email", firstAdminEmail)
					}
				} else {
					slog.Error("Не удалось найти роль 'admin' в БД", "error", errRole)
				}
			} else if adminUser.RoleName != nil && *adminUser.RoleName == models.RoleAdmin {
				slog.Info("Пользователь уже является администратором", "email", firstAdminEmail)
			}
		} else {
			slog.Warn("Пользователь для назначения администратором не найден или ошибка", "email", firstAdminEmail, "error", errDbGetUserByEmail)
		}
	} else {
		slog.Info("Переменная окружения FIRST_ADMIN_EMAIL не установлена, первый администратор не назначается автоматически.")
	}

	sessionManager = scs.New()
	sessionManager.Store = mysqlstore.New(db.DB)
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Name = "shaman_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Persist = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = cfg.AppEnv == "production"
	sessionManager.Cookie.Path = "/"

	slog.Info("Менеджер сессий инициализирован", "store", "mysqlstore", "lifetime", sessionManager.Lifetime, "secure_cookie", sessionManager.Cookie.Secure)

	appHandlers, err := handlers.NewAppHandlers(cfg, sessionManager)
	if err != nil {
		slog.Error("Критическая ошибка: не удалось инициализировать обработчики страниц", "error", err)
		os.Exit(1)
	}
	authHandlers := handlers.NewAuthHandlers(sessionManager, appHandlers.RenderPage, appHandlers.NewPageData, cfg)
	billingHandlers := handlers.NewBillingHandlers(sessionManager, cfg, appHandlers)
	userProfileHandlers := handlers.NewUserProfileHandlers(sessionManager)
	userSettingsHandlers := handlers.NewUserSettingsHandlers(sessionManager)

	mainMux := http.NewServeMux()
	fs := http.FileServer(http.Dir("./static"))
	mainMux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Middleware
	injectUserMiddleware := middleware.InjectUserData(sessionManager)
	requireAuthMiddleware := middleware.RequireAuthentication(sessionManager)
	requireSubscriptionMiddleware := middleware.RequireActiveSubscription(sessionManager)
	requireAdminRoleMiddleware := middleware.RequireRole(models.RoleAdmin)

	// Public Routes
	mainMux.Handle("/", injectUserMiddleware(http.HandlerFunc(appHandlers.WelcomePageHandler)))
	mainMux.Handle("/documentation", injectUserMiddleware(http.HandlerFunc(appHandlers.DocumentationPageHandler)))
	mainMux.Handle("/public-offer", injectUserMiddleware(http.HandlerFunc(appHandlers.PublicOfferPageHandler)))

	// Auth Routes
	mainMux.Handle("/register", injectUserMiddleware(http.HandlerFunc(authHandlers.RegisterPageHandler)))
	mainMux.HandleFunc("/api/register", authHandlers.RegisterHandler)
	mainMux.HandleFunc("/verify-email", authHandlers.VerifyEmailHandler)
	mainMux.HandleFunc("/resend-verification-email", authHandlers.ResendVerificationEmailHandler)
	mainMux.Handle("/login", injectUserMiddleware(http.HandlerFunc(authHandlers.LoginPageHandler)))
	mainMux.HandleFunc("/api/login", authHandlers.LoginHandler)
	mainMux.HandleFunc("/api/logout", authHandlers.LogoutHandler)
	
	// Password Reset
	mainMux.Handle("/forgot-password", injectUserMiddleware(http.HandlerFunc(authHandlers.ForgotPasswordPageHandler)))
	mainMux.HandleFunc("/request-password-reset", authHandlers.RequestPasswordResetHandler)
	mainMux.Handle("/reset-password", injectUserMiddleware(http.HandlerFunc(authHandlers.ResetPasswordPageHandler)))
	mainMux.HandleFunc("/process-password-reset", authHandlers.ProcessPasswordResetHandler)

	// Auth Phone Routes
	mainMux.Handle("/verify-phone", requireAuthMiddleware(injectUserMiddleware(http.HandlerFunc(authHandlers.VerifyPhonePageHandler))))
	mainMux.Handle("/process-phone-verification", requireAuthMiddleware(http.HandlerFunc(authHandlers.ProcessPhoneVerificationHandler)))
	mainMux.Handle("/resend-phone-verification", requireAuthMiddleware(http.HandlerFunc(authHandlers.ResendPhoneVerificationHandler)))

	// Subscription Routes
	mainMux.Handle("/subscribe", requireAuthMiddleware(injectUserMiddleware(http.HandlerFunc(appHandlers.SubscribePageHandler))))
	mainMux.Handle("/billing/create-payment-link", requireAuthMiddleware(http.HandlerFunc(billingHandlers.CreatePaymentLinkHandler)))
	mainMux.HandleFunc("/billing/success", billingHandlers.PaymentSuccessPageHandler)
	mainMux.HandleFunc("/billing/failure", billingHandlers.PaymentFailurePageHandler)
	mainMux.HandleFunc("/api/billing/webhook", billingHandlers.PaymentWebhookHandler)
	mainMux.Handle("/api/billing/cancel-subscription", requireAuthMiddleware(http.HandlerFunc(billingHandlers.CancelSubscriptionHandler)))

	// Authenticated User Routes
	mainMux.Handle("/dashboard", requireAuthMiddleware(requireSubscriptionMiddleware(injectUserMiddleware(http.HandlerFunc(appHandlers.DashboardPageHandler)))))
	mainMux.Handle("/profile", requireAuthMiddleware(injectUserMiddleware(http.HandlerFunc(appHandlers.ProfilePageHandler))))
	mainMux.Handle("/settings", requireAuthMiddleware(injectUserMiddleware(http.HandlerFunc(appHandlers.SettingsPageHandler))))

	// Authenticated User API Routes
	mainMux.Handle("/api/profile/update", requireAuthMiddleware(http.HandlerFunc(userProfileHandlers.UpdateProfileHandler)))
	mainMux.Handle("/api/profile/change-password", requireAuthMiddleware(http.HandlerFunc(userProfileHandlers.ChangePasswordHandler)))
	mainMux.Handle("/api/settings/update", requireAuthMiddleware(http.HandlerFunc(userSettingsHandlers.UpdateUserSettingsHandler)))

	// Dialogue API (защищенные)
	dialogueWithFileHandler := handlers.DialogueWithFileHandler(cfg, shamanSystemPrompt, generalSystemPrompt)
	mainMux.Handle("/api/dialogue_with_file", requireAuthMiddleware(requireSubscriptionMiddleware(dialogueWithFileHandler)))

	mainMux.Handle("/api/chat_sessions", requireAuthMiddleware(requireSubscriptionMiddleware(handlers.ListChatSessionsHandler())))
	mainMux.Handle("/api/chat_session_messages", requireAuthMiddleware(requireSubscriptionMiddleware(handlers.GetChatSessionMessagesHandler())))
	mainMux.Handle("/api/chat_session_create", requireAuthMiddleware(requireSubscriptionMiddleware(handlers.CreateNewChatSessionHandler())))

	// Legal Docs API (публичные)
	mainMux.Handle("/api/legal/terms", handlers.GetLegalDocHandler("terms"))
	mainMux.Handle("/api/legal/privacy", handlers.GetLegalDocHandler("privacy"))

	// Оборачиваем основной MUX в CSRF защиту
	csrfProtectedRoutes := middleware.NoSurfMiddleware(mainMux, cfg.AppEnv == "production")

	// --- Admin Routes ---
	adminRouter := http.NewServeMux()
	adminDashboardHandlerFunc := adminhandlers.AdminDashboardPageHandler(appHandlers)
	adminUsersListHandlerFunc := adminhandlers.AdminUsersListPageHandler(appHandlers)
	adminEditUserPageHandlerFunc := adminhandlers.AdminEditUserPageHandler(appHandlers)
	adminUpdateUserHandlerFunc := adminhandlers.AdminUpdateUserHandler(appHandlers)
	adminReportsHandlerFunc := adminhandlers.AdminReportsPageHandler(appHandlers)
	adminSettingsHandlerFunc := adminhandlers.AdminSettingsPageHandler(appHandlers)
	adminUpdateSettingsHandlerFunc := adminhandlers.AdminUpdateSettingsHandler(appHandlers)

	adminRouter.HandleFunc("/dashboard", adminDashboardHandlerFunc)
	adminRouter.HandleFunc("/users", adminUsersListHandlerFunc)
	adminRouter.HandleFunc("/users/edit", adminEditUserPageHandlerFunc)
	adminRouter.HandleFunc("/users/update", adminUpdateUserHandlerFunc)
	adminRouter.HandleFunc("/reports", adminReportsHandlerFunc)
	adminRouter.HandleFunc("/settings", adminSettingsHandlerFunc)
	adminRouter.HandleFunc("/settings/update", adminUpdateSettingsHandlerFunc)

	adminProtectedHandler := injectUserMiddleware(
		requireAuthMiddleware(
			requireAdminRoleMiddleware(
				middleware.NoSurfMiddleware(adminRouter, cfg.AppEnv == "production"),
			),
		),
	)
	// --- End Admin Routes ---

	// Top Level Mux
	topLevelMux := http.NewServeMux()
	topLevelMux.HandleFunc("/api/trial-dialogue", handlers.TrialDialogueHandler(cfg, generalSystemPrompt))
	topLevelMux.Handle("/admin/", http.StripPrefix("/admin", adminProtectedHandler))
	topLevelMux.Handle("/", csrfProtectedRoutes)

	// Обертываем topLevelMux в менеджер сессий
	finalHandler := sessionManager.LoadAndSave(topLevelMux)

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("Сервер Shaman запущен и слушает", "address", fmt.Sprintf("http://localhost%s", addr))

	server := &http.Server{
		Addr:         addr,
		Handler:      finalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  240 * time.Second,
	}

	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Критическая ошибка: не удалось запустить HTTP-сервер", "address", addr, "error", err)
		os.Exit(1)
	}
}