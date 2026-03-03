package models

import (
	"time"

	"github.com/google/uuid"
)

// Role represents a named role scoped to an application
type Role struct {
	ID          uuid.UUID    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID       uuid.UUID    `gorm:"type:uuid;not null;index;uniqueIndex:idx_role_app_name" json:"app_id"`
	Name        string       `gorm:"not null;uniqueIndex:idx_role_app_name" json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `gorm:"default:false" json:"is_system"` // System roles cannot be deleted
	CreatedAt   time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

// Permission represents a granular permission (resource:action)
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Resource    string    `gorm:"not null;uniqueIndex:idx_permission_resource_action" json:"resource"`
	Action      string    `gorm:"not null;uniqueIndex:idx_permission_resource_action" json:"action"`
	Description string    `json:"description"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// UserRole represents the assignment of a role to a user within an application
type UserRole struct {
	UserID     uuid.UUID  `gorm:"type:uuid;not null;primaryKey;index" json:"user_id"`
	RoleID     uuid.UUID  `gorm:"type:uuid;not null;primaryKey" json:"role_id"`
	AppID      uuid.UUID  `gorm:"type:uuid;not null;index;index:idx_user_role_app_user,priority:1" json:"app_id"` // Denormalized from Role for fast lookup
	AssignedAt time.Time  `gorm:"autoCreateTime" json:"assigned_at"`
	AssignedBy *uuid.UUID `gorm:"type:uuid" json:"assigned_by,omitempty"` // Who assigned this role (nullable for system assignments)
	Role       Role       `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	User       User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
