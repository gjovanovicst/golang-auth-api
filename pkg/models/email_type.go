package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// EmailType represents a category of email that the system can send.
// System-defined types (is_system=true) cannot be deleted.
// The Variables field stores the list of available template variables as JSONB.
type EmailType struct {
	ID             uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Code           string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"code"`
	Name           string         `gorm:"type:varchar(100);not null" json:"name"`
	Description    string         `gorm:"type:text" json:"description"`
	DefaultSubject string         `gorm:"type:varchar(255)" json:"default_subject"`
	Variables      datatypes.JSON `gorm:"type:jsonb" json:"variables"` // [{name, description, required}]
	IsSystem       bool           `gorm:"default:true" json:"is_system"`
	IsActive       bool           `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for EmailType.
func (EmailType) TableName() string {
	return "email_types"
}

// EmailTypeVariable describes a single template variable available for an email type.
type EmailTypeVariable struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}
