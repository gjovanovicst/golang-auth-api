package models

import (
	"time"

	"github.com/google/uuid"
)

// WebAuthnCredential stores a FIDO2/WebAuthn passkey credential.
// Supports both regular users (UserID+AppID) and admin accounts (AdminID).
// For regular users: UserID and AppID are set, AdminID is nil.
// For admin accounts: AdminID is set, UserID and AppID are nil.
type WebAuthnCredential struct {
	ID              uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID          *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`  // Regular user (nullable for admin passkeys)
	AppID           *uuid.UUID `gorm:"type:uuid;index" json:"app_id,omitempty"`   // Application (nullable for admin passkeys)
	AdminID         *uuid.UUID `gorm:"type:uuid;index" json:"admin_id,omitempty"` // Admin account (nullable for user passkeys)
	CredentialID    []byte     `gorm:"type:bytea;not null;uniqueIndex" json:"-"`
	PublicKey       []byte     `gorm:"type:bytea;not null" json:"-"`
	AttestationType string     `gorm:"type:varchar(50)" json:"attestation_type"`
	AAGUID          []byte     `gorm:"type:bytea" json:"-"` // Authenticator identifier
	SignCount       uint32     `gorm:"default:0" json:"-"`
	Name            string     `gorm:"type:varchar(100)" json:"name"`       // User-friendly name ("My MacBook", "YubiKey")
	Transports      string     `gorm:"type:varchar(255)" json:"transports"` // Comma-separated: "usb,ble,nfc,internal"
	BackupEligible  bool       `gorm:"default:false" json:"backup_eligible"`
	BackupState     bool       `gorm:"default:false" json:"backup_state"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`
}
