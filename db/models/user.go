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

type Department string

const (
	TownPlanningDepartment Department = "TOWN_PLANNING"
	EngineeringDepartment  Department = "ENGINEERING"
	HealthDepartment       Department = "HEALTH"
	RoadsDepartment        Department = "ROADS"
	WaterDepartment        Department = "WATER"
	TreasuryDepartment     Department = "TREASURY"
	EstatesDepartment      Department = "ESTATES"
	GISDepartment          Department = "GIS"
	HousingDepartment      Department = "HOUSING"
)

// User represents system users with role-based access
type User struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	FirstName      string     `gorm:"not null" json:"first_name"`
	LastName       string     `gorm:"not null" json:"last_name"`
	Email          string     `gorm:"unique;not null" json:"email"`
	Phone          string     `gorm:"unique" json:"phone"`
	WhatsAppNumber *string    `json:"whatsapp_number"`
	Password       string     `json:"-"` // Never include in JSON responses
	AuthMethod     AuthMethod `gorm:"type:varchar(20);default:'PASSWORD'" json:"auth_method"`
	TOTPSecret     string     `json:"-" gorm:"column:totp_secret"`

	// Role and permissions
	Role        Role        `gorm:"type:varchar(30);not null" json:"role"`
	Department  *Department `gorm:"type:varchar(30)" json:"department"`
	Permissions []string    `gorm:"type:json" json:"permissions,omitempty"`

	// Status
	Active      bool       `gorm:"default:true" json:"active"`
	IsSuspended bool       `gorm:"default:false" json:"is_suspended"`
	LastLoginAt *time.Time `json:"last_login_at"`

	// Profile
	ProfilePictureURL *string `json:"profile_picture_url"`
	Notes             *string `gorm:"type:text" json:"notes"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
