package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ActivityLog captures essential details about each user action
type ActivityLog struct {
	ID        uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID     uuid.UUID       `gorm:"type:uuid;not null;default:'00000000-0000-0000-0000-000000000001';index" json:"app_id"`
	UserID    uuid.UUID       `gorm:"index:idx_user_timestamp;index:idx_cleanup" json:"user_id"` // Composite indexes for performance
	EventType string          `gorm:"index;not null" json:"event_type"`
	Timestamp time.Time       `gorm:"index:idx_user_timestamp;index:idx_cleanup;not null" json:"timestamp"`
	IPAddress string          `json:"ip_address"`
	UserAgent string          `json:"user_agent"`
	Details   json.RawMessage `gorm:"type:jsonb" json:"details"` // Use json.RawMessage for flexible JSONB

	// New fields for smart logging
	Severity  string     `gorm:"index:idx_cleanup;not null;default:'INFORMATIONAL'" json:"severity"` // CRITICAL, IMPORTANT, INFORMATIONAL
	ExpiresAt *time.Time `gorm:"index:idx_expires" json:"expires_at"`                                // Automatic expiration timestamp for cleanup
	IsAnomaly bool       `gorm:"default:false" json:"is_anomaly"`                                    // Flag if this was logged due to anomaly detection
}

// TableName specifies the table name for ActivityLog
func (ActivityLog) TableName() string {
	return "activity_logs"
}
