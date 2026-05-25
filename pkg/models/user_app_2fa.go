package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// UserApp2FA stores per-application 2FA configuration for a user.
// Each (user_id, application_id) pair has its own secret, recovery codes,
// and enabled flag so that enabling 2FA in one app does not automatically
// affect other apps.
type UserApp2FA struct {
	ID            uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_user_app_2fa" json:"user_id"`
	ApplicationID uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_user_app_2fa" json:"application_id"`
	TwoFAEnabled  bool           `gorm:"default:false" json:"two_fa_enabled"`
	TwoFAMethod   string         `gorm:"type:varchar(20);default:''" json:"two_fa_method"` // "totp", "email", "sms", "passkey", "backup_email"
	TwoFASecret   string         `gorm:"type:text;default:''" json:"-"`                    // Encrypted TOTP secret, never exposed
	RecoveryCodes datatypes.JSON `gorm:"type:jsonb" json:"-"`                              // Encrypted, never exposed

	// When the user switches to backup_email as their active 2FA method we save
	// the previous method/secret here so it can be restored on disable.
	PreviousMethod string `gorm:"type:varchar(20);default:''" json:"-"`
	PreviousSecret string `gorm:"type:text;default:''" json:"-"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName overrides GORM's auto-pluralisation which would otherwise produce
// "user_app2_fas" instead of the correct "user_app_2fa".
func (UserApp2FA) TableName() string { return "user_app_2fa" }
