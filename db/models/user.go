package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	AdminRole           Role = "admin"
	SuperUserRole       Role = "super_user"
	PropertyManagerRole Role = "property_manager"
)

type AuthMethod string

const (
	AuthMethodMagicLink     AuthMethod = "magic_link"
	AuthMethodPassword      AuthMethod = "password"
	AuthMethodAuthenticator AuthMethod = "authenticator"
)

type User struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	FirstName   string         `json:"first_name"`
	LastName    string         `json:"last_name"`
	Email       string         `gorm:"unique" json:"email"`
	Password    string         `json:"password"`
	AuthMethod  AuthMethod     `json:"auth_method" gorm:"default:'magic_link'"`
	TOTPSecret  string         `json:"-" gorm:"column:totp_secret"`
	Phone       string         `gorm:"unique" json:"phone"`
	Role        Role           `json:"role"`
	Active      *bool          `json:"active"`
	IsSuspended bool           `gorm:"default:false" json:"is_suspended"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy   string         `gorm:"not null" json:"created_by"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
