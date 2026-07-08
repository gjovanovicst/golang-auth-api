package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ArchivedUser stores a pseudonymised or full-copy record of a deleted user
// for audit and GDPR compliance purposes.
//
// When archive_mode = "purge": PII fields (email, name, first_name, last_name)
// are anonymized before storage. Only audit-relevant metadata is preserved.
//
// When archive_mode = "archive": the full profile is preserved as-is.
//
// The original user row is hard-deleted from the users table; this table
// serves as the deletion audit trail and proof of erasure.
type ArchivedUser struct {
	ID              uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID          uuid.UUID      `gorm:"type:uuid;not null"        json:"user_id"`
	AppID           uuid.UUID      `gorm:"type:uuid;not null"        json:"app_id"`
	Email           string         `gorm:"not null"                  json:"email"`
	Name            string         `gorm:"not null;default:''"       json:"name"`
	FirstName       string         `gorm:"not null;default:''"       json:"first_name"`
	LastName        string         `gorm:"not null;default:''"       json:"last_name"`
	Locale          string         `gorm:"not null;default:''"       json:"locale"`
	EmailVerified   bool           `gorm:"not null;default:false"    json:"email_verified"`
	IsActive        bool           `gorm:"not null;default:true"     json:"is_active"`
	TwoFAEnabled    bool           `gorm:"not null;default:false"    json:"two_fa_enabled"`
	HasPassword     bool           `gorm:"not null;default:true"     json:"has_password"`
	SocialProviders string         `gorm:"not null;default:''"       json:"social_providers"`
	RegisteredAt    time.Time      `gorm:"not null"                  json:"registered_at"`
	DeletedAt       time.Time      `gorm:"not null"                  json:"deleted_at"`
	DeletedBy       string         `gorm:"not null;default:''"       json:"deleted_by"`
	ArchiveMode     string         `gorm:"not null;default:'purge'"  json:"archive_mode"`
	Metadata        datatypes.JSON `gorm:"type:jsonb;default:'{}'"   json:"-"`
	CreatedAt       time.Time      `gorm:"autoCreateTime"            json:"created_at"`
}

// TableName specifies the table name for ArchivedUser.
func (ArchivedUser) TableName() string {
	return "archived_users"
}
