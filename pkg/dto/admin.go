package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateTenantRequest represents the payload for creating a new tenant
type CreateTenantRequest struct {
	Name string `json:"name" binding:"required"`
}

// TenantResponse represents the tenant data returned to clients
type TenantResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateAppRequest represents the payload for creating a new application
type CreateAppRequest struct {
	TenantID    string `json:"tenant_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// AppResponse represents the application data returned to clients
type AppResponse struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UpsertOAuthConfigRequest represents the payload for setting OAuth credentials
type UpsertOAuthConfigRequest struct {
	Provider     string `json:"provider" binding:"required"` // e.g., "google", "github"
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"` // #nosec G101 -- This is a DTO field, not a hardcoded credential
	RedirectURL  string `json:"redirect_url" binding:"required"`
}

// OAuthConfigResponse represents the OAuth config data returned (excluding secret)
type OAuthConfigResponse struct {
	ID          uuid.UUID `json:"id"`
	AppID       uuid.UUID `json:"app_id"`
	Provider    string    `json:"provider"`
	ClientID    string    `json:"client_id"`
	RedirectURL string    `json:"redirect_url"`
	IsEnabled   bool      `json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
