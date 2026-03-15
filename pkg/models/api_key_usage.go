package models

import (
	"time"

	"github.com/google/uuid"
)

// ApiKeyUsage tracks daily request counts per API key for usage analytics.
// One row is maintained per (api_key_id, period_date) pair using upsert semantics.
type ApiKeyUsage struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ApiKeyID     uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_api_key_usage_key_period" json:"api_key_id"`
	PeriodDate   time.Time `gorm:"type:date;not null;uniqueIndex:idx_api_key_usage_key_period" json:"period_date"` // Day bucket (YYYY-MM-DD)
	RequestCount int64     `gorm:"not null;default:0" json:"request_count"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName specifies the table name for ApiKeyUsage.
func (ApiKeyUsage) TableName() string {
	return "api_key_usages"
}
