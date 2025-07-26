// internal/models/user.go
package models

import "time"

type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusInactive  SubscriptionStatus = "inactive"
	SubscriptionStatusPending   SubscriptionStatus = "pending"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusCanceled  SubscriptionStatus = "canceled"
	SubscriptionStatusTrial     SubscriptionStatus = "trial"
	SubscriptionStatusCompleted SubscriptionStatus = "completed"
)

type User struct {
	ID                               int64      `json:"id"`
	Email                            string     `json:"email"`
	Phone                            *string    `json:"phone"`
	PasswordHash                     string     `json:"-"`
	PasswordResetToken               *string    `json:"-"`
	PasswordResetTokenExpiresAt      *time.Time `json:"-"`
	FirstName                        string     `json:"first_name"`
	LastName                         string     `json:"last_name"`
	Gender                           string     `json:"gender"`
	Birthday                         string     `json:"birthday"`
	CreatedAt                        time.Time  `json:"created_at"`
	UpdatedAt                        time.Time  `json:"updated_at"`
	RoleID                           *int64     `json:"-"`
	RoleName                         *string    `json:"role_name,omitempty"`
	SubscriptionID                   *string    `json:"-"`
	CustomerID                       *string    `json:"-"`
	SubscriptionStatus               SubscriptionStatus `json:"subscription_status"`
	SubscriptionStartDate            *time.Time         `json:"-"`
	SubscriptionEndDate              *time.Time         `json:"-"`
	CurrentPeriodEnd                 *time.Time         `json:"-"`
	TTSEnabledDefault                *bool      `json:"tts_enabled_default,omitempty"`
	EmailVerificationToken           *string    `json:"-"`
	EmailVerificationTokenExpiresAt  *time.Time `json:"-"`
	IsEmailVerified                  bool       `json:"is_email_verified"`
	EmailVerifiedAt                  *time.Time `json:"-"`
	TokensUsedInputThisPeriod        int        `json:"-"` // Не отдаем на клиент
	TokensUsedOutputThisPeriod       int        `json:"-"` // Не отдаем на клиент
	BillingCycleAnchorDate           *time.Time `json:"-"` 
	IsPhoneVerified                     bool
	PhoneVerificationCode               *string
	PhoneVerificationCodeExpiresAt      *time.Time
}

type RegistrationForm struct {
	Email       string `form:"email" validate:"required,email"`
	Phone       string `form:"phone" validate:"required,valid_phone"`
	Password    string `form:"password" validate:"required,min=8,complex_password"`
	ConfirmPass string `form:"confirm_password" validate:"required,eqfield=Password"`
	FirstName   string `form:"first_name" validate:"required,alpha_space"`
	LastName    string `form:"last_name" validate:"required,alpha_space"`
	Gender      string `form:"gender" validate:"required,oneof=male female"`
	Birthday    string `form:"birthday" validate:"required,adult_birthday"`
	AgreeTerms  string `form:"agree_terms" validate:"required"`
	Honeypot    string `form:"website"`
}

type LoginForm struct {
	Email    string `form:"email" validate:"required,email"`
	Password string `form:"password" validate:"required"`
}

type Subscription struct {
	ID                           string             `json:"id"`
	UserID                       int64              `json:"user_id"`
	PaymentGatewaySubscriptionID string             `json:"payment_gateway_subscription_id"`
	PlanID                       string             `json:"plan_id"`
	Status                       SubscriptionStatus `json:"status"`
	StartDate                    time.Time          `json:"start_date"`
	EndDate                      time.Time          `json:"end_date"`
	CurrentPeriodStart           time.Time          `json:"current_period_start"`
	CurrentPeriodEnd             time.Time          `json:"current_period_end"`
	CancelAtPeriodEnd            bool               `json:"cancel_at_period_end"`
	CreatedAt                    time.Time          `json:"created_at"`
	UpdatedAt                    time.Time          `json:"updated_at"`
}
