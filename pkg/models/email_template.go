package models

import (
	"time"

	"github.com/google/uuid"
)

// EmailTemplate stores email templates that can be per-application or global defaults.
// Resolution order: app-specific (app_id set) -> global default (app_id NULL) -> hardcoded fallback.
type EmailTemplate struct {
	ID             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID          *uuid.UUID `gorm:"type:uuid;index;uniqueIndex:idx_app_email_type" json:"app_id"` // NULL = global default template
	EmailTypeID    uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_app_email_type" json:"email_type_id"`
	Name           string     `gorm:"type:varchar(100);not null" json:"name"`
	Subject        string     `gorm:"type:varchar(255);not null" json:"subject"`
	BodyHTML       string     `gorm:"type:text" json:"body_html"`
	BodyText       string     `gorm:"type:text" json:"body_text"`
	TemplateEngine string     `gorm:"type:varchar(20);not null;default:'go_template'" json:"template_engine"` // go_template | placeholder | raw_html
	FromEmail      string     `gorm:"type:varchar(255);default:''" json:"from_email,omitempty"`               // Optional sender override
	FromName       string     `gorm:"type:varchar(255);default:''" json:"from_name,omitempty"`                // Optional sender name override
	ServerConfigID *uuid.UUID `gorm:"type:uuid" json:"server_config_id,omitempty"`                            // Optional link to specific SMTP config
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	EmailType    EmailType          `gorm:"foreignKey:EmailTypeID" json:"email_type,omitempty"`
	ServerConfig *EmailServerConfig `gorm:"foreignKey:ServerConfigID" json:"server_config,omitempty"`
}

// TableName specifies the table name for EmailTemplate.
func (EmailTemplate) TableName() string {
	return "email_templates"
}

// Template engine constants
const (
	TemplateEngineGoTemplate  = "go_template" // Go html/template syntax: {{.VarName}}
	TemplateEnginePlaceholder = "placeholder" // Simple replacement: {var_name}
	TemplateEngineRawHTML     = "raw_html"    // Raw HTML with {{.VarName}} substitution
)
