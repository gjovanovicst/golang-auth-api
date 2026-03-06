package models

import (
	"time"

	"github.com/google/uuid"
)

// WebhookEndpoint represents a registered webhook URL for a specific application and event type.
// One endpoint per (AppID, EventType) composite — explicit and simple model.
// The signing secret is stored as an HMAC-SHA256 key and shown only once at creation time.
type WebhookEndpoint struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID     uuid.UUID  `gorm:"type:uuid;not null;index:idx_webhook_app_event,unique,composite:app_event" json:"app_id"`
	EventType string     `gorm:"not null;index:idx_webhook_app_event,unique,composite:app_event" json:"event_type"`
	URL       string     `gorm:"not null" json:"url"`
	Secret    string     `gorm:"not null" json:"-"` // HMAC-SHA256 key, never exposed after creation
	IsActive  bool       `gorm:"not null;default:true" json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"-"`
}

// TableName specifies the table name for WebhookEndpoint
func (WebhookEndpoint) TableName() string {
	return "webhook_endpoints"
}

// ValidEventTypes returns the list of supported webhook event types.
var ValidEventTypes = []string{
	"user.registered",
	"user.verified",
	"user.login",
	"user.password_changed",
	"2fa.enabled",
	"2fa.disabled",
	"social.linked",
	"social.unlinked",
}
