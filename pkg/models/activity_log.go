package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ActivityLog captures essential details about each user action
type ActivityLog struct {
	ID        uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID       `gorm:"index" json:"user_id"` // Consider not making it a foreign key constraint for performance if high volume
	EventType string          `gorm:"index;not null" json:"event_type"`
	Timestamp time.Time       `gorm:"index;not null" json:"timestamp"`
	IPAddress string          `json:"ip_address"`
	UserAgent string          `json:"user_agent"`
	Details   json.RawMessage `gorm:"type:jsonb" json:"details"` // Use json.RawMessage for flexible JSONB
}
