package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// User represents the core user entity in our system
type User struct {
	ID                 uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID              uuid.UUID       `gorm:"type:uuid;not null;default:'00000000-0000-0000-0000-000000000001';index;uniqueIndex:idx_email_app_id" json:"app_id"`
	Email              string          `gorm:"uniqueIndex:idx_email_app_id;not null" json:"email"`
	PasswordHash       string          `gorm:"" json:"-"` // Stored hashed, not exposed via JSON - not required for social logins
	EmailVerified      bool            `gorm:"default:false" json:"email_verified"`
	IsActive           bool            `gorm:"default:true" json:"is_active"`
	Name               string          `gorm:"" json:"name"`            // Full name from social login or user input
	FirstName          string          `gorm:"" json:"first_name"`      // First name from social login
	LastName           string          `gorm:"" json:"last_name"`       // Last name from social login
	ProfilePicture     string          `gorm:"" json:"profile_picture"` // Profile picture URL from social login
	Locale             string          `gorm:"" json:"locale"`          // User's locale/language preference
	TwoFAEnabled       bool            `gorm:"default:false" json:"two_fa_enabled"`
	TwoFASecret        string          `gorm:"" json:"-"`           // Stored encrypted, not exposed via JSON
	TwoFARecoveryCodes datatypes.JSON  `gorm:"type:jsonb" json:"-"` // Stored encrypted, not exposed via JSON
	CreatedAt          time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
	SocialAccounts     []SocialAccount `gorm:"foreignKey:UserID" json:"social_accounts"` // One-to-many relationship
}
