// internal/models/role.go
package models

import "time"

type Role struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Константы для имен ролей (для удобства и избежания опечаток в коде)
const (
	RoleUser      string = "user"
	RoleAdmin     string = "admin"
	RoleModerator string = "moderator"
	RoleSupport   string = "support"
)