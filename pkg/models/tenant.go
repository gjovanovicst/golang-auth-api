package models

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents a customer or organization that owns applications
type Tenant struct {
	ID        uuid.UUID     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name      string        `gorm:"not null" json:"name"`
	CreatedAt time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
	Apps      []Application `gorm:"foreignKey:TenantID" json:"apps"`
}
