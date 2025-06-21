package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents the core user entity in our system
type User struct {
	ID             uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Email          string          `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash   string         `gorm:"" json:"-"` // Stored hashed, not exposed via JSON - not required for social logins
	EmailVerified  bool            `gorm:"default:false" json:"email_verified"`
	CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
	SocialAccounts []SocialAccount `gorm:"foreignKey:UserID" json:"social_accounts"` // One-to-many relationship
}