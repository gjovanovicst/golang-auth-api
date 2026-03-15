package models

import (
	"time"

	"github.com/google/uuid"
)

// WebhookDelivery represents a single delivery attempt for a webhook event.
// Full response details are stored for debugging and retry purposes.
type WebhookDelivery struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	EndpointID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"endpoint_id"`
	AppID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"app_id"`
	EventType    string     `gorm:"not null;index" json:"event_type"`
	Payload      string     `gorm:"type:text;not null" json:"payload"`           // JSON payload sent
	Attempt      int        `gorm:"not null;default:1" json:"attempt"`           // Attempt number (1-based)
	StatusCode   int        `json:"status_code"`                                 // HTTP response code (0 = no response)
	ResponseBody string     `gorm:"type:text" json:"response_body"`              // First 1KB of response
	LatencyMs    int64      `json:"latency_ms"`                                  // Round-trip time in milliseconds
	Success      bool       `gorm:"not null;default:false;index" json:"success"` // true = 2xx response
	ErrorMessage string     `json:"error_message"`                               // Network or timeout error
	NextRetryAt  *time.Time `gorm:"index" json:"next_retry_at"`                  // nil = no more retries
	CreatedAt    time.Time  `json:"created_at"`

	// Relationship (for preloading, not always needed)
	Endpoint *WebhookEndpoint `gorm:"foreignKey:EndpointID" json:"endpoint,omitempty"`
}

// TableName specifies the table name for WebhookDelivery
func (WebhookDelivery) TableName() string {
	return "webhook_deliveries"
}
