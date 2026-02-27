package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AdminAccount represents a system-level admin user for the Admin GUI.
// These are separate from regular User accounts — admin accounts are not
// scoped to any application and have full system access.
type AdminAccount struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Username     string     `gorm:"uniqueIndex;not null" json:"username"`
	Email        string     `gorm:"uniqueIndex" json:"email"`
	PasswordHash string     `gorm:"not null" json:"-"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`

	// Two-Factor Authentication fields
	TwoFAEnabled       bool           `gorm:"default:false" json:"two_fa_enabled"`
	TwoFAMethod        string         `gorm:"type:varchar(20)" json:"two_fa_method"`
	TwoFASecret        string         `gorm:"type:text" json:"-"`
	TwoFARecoveryCodes datatypes.JSON `gorm:"type:jsonb" json:"-"`
}

// TableName overrides the default table name
func (AdminAccount) TableName() string {
	return "admin_accounts"
}
