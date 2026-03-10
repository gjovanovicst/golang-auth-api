package models

import (
	"time"

	"github.com/google/uuid"
)

// TrustedDevice represents a device that a user has opted to trust for 30 days,
// allowing 2FA to be skipped on subsequent logins from that device.
// Scoped per app + user to support multi-tenancy.
type TrustedDevice struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index:idx_trusted_device_user_app" json:"user_id"`
	AppID      uuid.UUID `gorm:"type:uuid;not null;index:idx_trusted_device_user_app" json:"app_id"`
	TokenHash  string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"` // SHA-256 hex of the plaintext device token (never stored in plain)
	Name       string    `gorm:"type:varchar(255)" json:"name"`                  // Human-readable label auto-generated from User-Agent
	UserAgent  string    `gorm:"type:text" json:"user_agent"`
	IPAddress  string    `gorm:"type:varchar(45)" json:"ip_address"` // IPv4 or IPv6
	LastUsedAt time.Time `gorm:"autoUpdateTime" json:"last_used_at"`
	ExpiresAt  time.Time `gorm:"index:idx_trusted_device_expires" json:"expires_at"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName overrides the default table name
func (TrustedDevice) TableName() string {
	return "trusted_devices"
}
