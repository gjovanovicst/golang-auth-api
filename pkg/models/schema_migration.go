package models

import "time"

// SchemaMigration tracks which database migrations have been applied
type SchemaMigration struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Version         string    `gorm:"uniqueIndex;not null;size:255" json:"version"` // YYYYMMDD_HHMMSS format
	Name            string    `gorm:"not null;size:255" json:"name"`
	AppliedAt       time.Time `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"applied_at"`
	ExecutionTimeMs int       `json:"execution_time_ms"` // How long the migration took
	Success         bool      `gorm:"not null;default:true" json:"success"`
	ErrorMessage    string    `gorm:"type:text" json:"error_message,omitempty"`
	Checksum        string    `gorm:"size:64" json:"checksum,omitempty"` // SHA256 of migration file
}

// TableName specifies the table name for GORM
func (SchemaMigration) TableName() string {
	return "schema_migrations"
}
