package dto

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Role DTOs
// ============================================================

// CreateRoleRequest represents the payload for creating a new role.
type CreateRoleRequest struct {
	AppID       string `json:"app_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateRoleRequest represents the payload for updating a role.
type UpdateRoleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// RoleResponse represents a role returned to clients.
type RoleResponse struct {
	ID          uuid.UUID            `json:"id"`
	AppID       uuid.UUID            `json:"app_id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	IsSystem    bool                 `json:"is_system"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// ============================================================
// Permission DTOs
// ============================================================

// CreatePermissionRequest represents the payload for creating a custom permission.
type CreatePermissionRequest struct {
	Resource    string `json:"resource" binding:"required"`
	Action      string `json:"action" binding:"required"`
	Description string `json:"description"`
}

// PermissionResponse represents a permission returned to clients.
type PermissionResponse struct {
	ID          uuid.UUID `json:"id"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
}

// SetRolePermissionsRequest represents the payload for setting role permissions.
type SetRolePermissionsRequest struct {
	PermissionIDs []string `json:"permission_ids" binding:"required"`
}

// ============================================================
// User-Role Assignment DTOs
// ============================================================

// AssignRoleRequest represents the payload for assigning a role to a user.
type AssignRoleRequest struct {
	UserID string `json:"user_id" binding:"required"`
	RoleID string `json:"role_id" binding:"required"`
}

// RevokeRoleRequest represents the payload for revoking a role from a user.
type RevokeRoleRequest struct {
	RoleID string `json:"role_id" binding:"required"`
}

// UserRolesResponse represents the roles assigned to a user.
type UserRolesResponse struct {
	UserID uuid.UUID      `json:"user_id"`
	AppID  uuid.UUID      `json:"app_id"`
	Roles  []RoleResponse `json:"roles"`
}
