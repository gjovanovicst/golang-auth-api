package models

import (
	"time"

	"github.com/google/uuid"
)

// SessionGroup is a named group of applications that share authentication state.
// When a user is authenticated in any app in the group they can obtain tokens
// for any other app in the group via the SSO exchange flow without re-entering
// credentials (similar to Google's cross-product SSO).
type SessionGroup struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID    uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	// GlobalLogout controls whether logging out of one app in the group
	// revokes the user's sessions in all other apps of the group.
	GlobalLogout bool              `gorm:"default:true" json:"global_logout"`
	Apps         []SessionGroupApp `gorm:"foreignKey:SessionGroupID" json:"apps,omitempty"`
	CreatedAt    time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

// SessionGroupApp is the join table linking an Application to a SessionGroup.
// An application can belong to at most one session group (enforced by the
// unique index on AppID).
type SessionGroupApp struct {
	SessionGroupID uuid.UUID   `gorm:"type:uuid;not null;primaryKey" json:"session_group_id"`
	AppID          uuid.UUID   `gorm:"type:uuid;not null;primaryKey;uniqueIndex:idx_session_group_app_id" json:"app_id"`
	App            Application `gorm:"foreignKey:AppID" json:"app,omitempty"`
	AddedAt        time.Time   `gorm:"autoCreateTime" json:"added_at"`
}
