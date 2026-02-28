package models

import (
	"time"

	"github.com/google/uuid"
)

// EmailServerConfig stores SMTP server configuration.
// Configs can be scoped to a specific application (AppID set) or global/system-level (AppID nil).
// Multiple configs can exist per application (e.g., transactional, marketing, finance).
// One config per scope is marked as the default (is_default=true).
// Resolution chain for sending emails: app-specific config -> global config -> dev mode (log to stdout).
type EmailServerConfig struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID        *uuid.UUID `gorm:"type:uuid;index" json:"app_id"`                            // NULL = global/system-level config
	Name         string     `gorm:"type:varchar(100);not null;default:'Default'" json:"name"` // Label (e.g., "Transactional", "Marketing")
	SMTPHost     string     `gorm:"type:varchar(255);not null" json:"smtp_host"`
	SMTPPort     int        `gorm:"not null;default:587" json:"smtp_port"`
	SMTPUsername string     `gorm:"type:varchar(255)" json:"smtp_username"`
	SMTPPassword string     `gorm:"type:text" json:"-"` // Not exposed in JSON responses
	FromAddress  string     `gorm:"type:varchar(255);not null" json:"from_address"`
	FromName     string     `gorm:"type:varchar(100)" json:"from_name"`
	UseTLS       bool       `gorm:"default:true" json:"use_tls"`
	IsDefault    bool       `gorm:"default:true" json:"is_default"` // Only one default per scope (app or global)
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for EmailServerConfig.
func (EmailServerConfig) TableName() string {
	return "email_server_configs"
}
