package webauthn

import (
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository provides database access for WebAuthn credentials.
type Repository struct {
	DB *gorm.DB
}

// NewRepository creates a new WebAuthn repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// CreateCredential stores a new WebAuthn credential in the database.
func (r *Repository) CreateCredential(cred *models.WebAuthnCredential) error {
	return r.DB.Create(cred).Error
}

// GetCredentialsByUserID returns all WebAuthn credentials for a user.
func (r *Repository) GetCredentialsByUserID(userID uuid.UUID) ([]models.WebAuthnCredential, error) {
	var creds []models.WebAuthnCredential
	err := r.DB.Where("user_id = ?", userID).Order("created_at ASC").Find(&creds).Error
	return creds, err
}

// GetCredentialsByUserAndApp returns all WebAuthn credentials for a user within a specific app.
func (r *Repository) GetCredentialsByUserAndApp(userID, appID uuid.UUID) ([]models.WebAuthnCredential, error) {
	var creds []models.WebAuthnCredential
	err := r.DB.Where("user_id = ? AND app_id = ?", userID, appID).Order("created_at ASC").Find(&creds).Error
	return creds, err
}

// GetCredentialByCredentialID looks up a credential by its WebAuthn credential ID bytes.
func (r *Repository) GetCredentialByCredentialID(credentialID []byte) (*models.WebAuthnCredential, error) {
	var cred models.WebAuthnCredential
	err := r.DB.Where("credential_id = ?", credentialID).First(&cred).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// GetCredentialByAppAndCredentialID looks up a credential by app ID and WebAuthn credential ID.
func (r *Repository) GetCredentialByAppAndCredentialID(appID uuid.UUID, credentialID []byte) (*models.WebAuthnCredential, error) {
	var cred models.WebAuthnCredential
	err := r.DB.Where("app_id = ? AND credential_id = ?", appID, credentialID).First(&cred).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// GetCredentialByID looks up a credential by its primary key UUID.
func (r *Repository) GetCredentialByID(id uuid.UUID) (*models.WebAuthnCredential, error) {
	var cred models.WebAuthnCredential
	err := r.DB.Where("id = ?", id).First(&cred).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// UpdateCredentialSignCount updates the sign count and last used timestamp.
func (r *Repository) UpdateCredentialSignCount(id uuid.UUID, signCount uint32) error {
	return r.DB.Model(&models.WebAuthnCredential{}).Where("id = ?", id).Updates(map[string]interface{}{
		"sign_count":   signCount,
		"last_used_at": gorm.Expr("NOW()"),
	}).Error
}

// DeleteCredential removes a credential, scoped to the owning user for safety.
func (r *Repository) DeleteCredential(id, userID uuid.UUID) error {
	return r.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.WebAuthnCredential{}).Error
}

// CountCredentialsByUserAndApp returns the number of passkeys a user has for an app.
func (r *Repository) CountCredentialsByUserAndApp(userID, appID uuid.UUID) (int64, error) {
	var count int64
	err := r.DB.Model(&models.WebAuthnCredential{}).Where("user_id = ? AND app_id = ?", userID, appID).Count(&count).Error
	return count, err
}

// RenameCredential updates the user-friendly name of a credential.
func (r *Repository) RenameCredential(id, userID uuid.UUID, name string) error {
	result := r.DB.Model(&models.WebAuthnCredential{}).Where("id = ? AND user_id = ?", id, userID).Update("name", name)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ============================================================
// Admin Account Passkey Operations
// ============================================================

// GetCredentialsByAdminID returns all WebAuthn credentials for an admin account.
func (r *Repository) GetCredentialsByAdminID(adminID uuid.UUID) ([]models.WebAuthnCredential, error) {
	var creds []models.WebAuthnCredential
	err := r.DB.Where("admin_id = ?", adminID).Order("created_at ASC").Find(&creds).Error
	return creds, err
}

// DeleteAdminCredential removes a credential scoped to the owning admin for safety.
func (r *Repository) DeleteAdminCredential(id, adminID uuid.UUID) error {
	return r.DB.Where("id = ? AND admin_id = ?", id, adminID).Delete(&models.WebAuthnCredential{}).Error
}

// RenameAdminCredential updates the user-friendly name of an admin passkey.
func (r *Repository) RenameAdminCredential(id, adminID uuid.UUID, name string) error {
	result := r.DB.Model(&models.WebAuthnCredential{}).Where("id = ? AND admin_id = ?", id, adminID).Update("name", name)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetCredentialByAdminAndCredentialID looks up a credential by admin ID and WebAuthn credential ID.
func (r *Repository) GetCredentialByAdminAndCredentialID(adminID uuid.UUID, credentialID []byte) (*models.WebAuthnCredential, error) {
	var cred models.WebAuthnCredential
	err := r.DB.Where("admin_id = ? AND credential_id = ?", adminID, credentialID).First(&cred).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}
