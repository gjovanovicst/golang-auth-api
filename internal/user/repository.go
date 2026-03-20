package user

import (
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"gorm.io/gorm"
)

type Repository struct {
	DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) CreateUser(user *models.User) error {
	return r.DB.Create(user).Error
}

func (r *Repository) GetUserByEmail(appID, email string) (*models.User, error) {
	var user models.User
	err := r.DB.Where("app_id = ? AND email = ?", appID, email).First(&user).Error
	return &user, err
}

func (r *Repository) GetUserByID(id string) (*models.User, error) {
	var user models.User
	err := r.DB.Preload("SocialAccounts").Where("id = ?", id).First(&user).Error
	return &user, err
}

func (r *Repository) UpdateUser(user *models.User) error {
	return r.DB.Save(user).Error
}

func (r *Repository) UpdateUserPassword(userID, hashedPassword string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", hashedPassword).Error
}

// UpdateUserPasswordWithHistory sets password_hash, password_history, and password_changed_at atomically.
func (r *Repository) UpdateUserPasswordWithHistory(userID, hashedPassword string, history []byte) error {
	now := time.Now()
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"password_hash":       hashedPassword,
		"password_history":    history,
		"password_changed_at": &now,
	}).Error
}

func (r *Repository) UpdateUserEmailVerified(userID string, verified bool) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("email_verified", verified).Error
}

// 2FA related methods

// Enable2FA enables 2FA for a user and stores the secret and recovery codes.
// Defaults to TOTP method for backward compatibility.
func (r *Repository) Enable2FA(userID, secret, recoveryCodes string) error {
	return r.Enable2FAWithMethod(userID, secret, recoveryCodes, "totp")
}

// Enable2FAWithMethod enables 2FA for a user with a specific method ("totp" or "email").
func (r *Repository) Enable2FAWithMethod(userID, secret, recoveryCodes, method string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"two_fa_enabled":        true,
		"two_fa_secret":         secret,
		"two_fa_recovery_codes": recoveryCodes,
		"two_fa_method":         method,
	}).Error
}

// Disable2FA disables 2FA for a user
func (r *Repository) Disable2FA(userID string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"two_fa_enabled":        false,
		"two_fa_secret":         "",
		"two_fa_recovery_codes": nil,
		"two_fa_method":         "",
	}).Error
}

// UpdateRecoveryCodes updates the recovery codes for a user
func (r *Repository) UpdateRecoveryCodes(userID, recoveryCodes string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("two_fa_recovery_codes", recoveryCodes).Error
}

// DeleteUser deletes a user and all related data within a transaction.
// FK-constrained tables (social_accounts, user_roles) are deleted first to avoid
// "update or delete violates foreign key constraint" errors from NO ACTION constraints.
func (r *Repository) DeleteUser(userID string) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// 1. social_accounts.user_id → users.id (NOT NULL, NO ACTION) — must delete first
		if err := tx.Exec("DELETE FROM social_accounts WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		// 2. user_roles.user_id → users.id (NOT NULL, NO ACTION) — must delete first
		if err := tx.Exec("DELETE FROM user_roles WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		// 3. trusted_devices — no FK constraint, but clean up
		if err := tx.Exec("DELETE FROM trusted_devices WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		// 4. web_authn_credentials — no FK constraint, but clean up
		if err := tx.Exec("DELETE FROM web_authn_credentials WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		// 5. activity_logs — no FK constraint, but clean up
		if err := tx.Exec("DELETE FROM activity_logs WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		// 6. Finally hard-delete the user row
		return tx.Where("id = ?", userID).Delete(&models.User{}).Error
	})
}

// UpdateUserProfile updates user profile fields (name, first_name, last_name, profile_picture, locale)
func (r *Repository) UpdateUserProfile(userID string, updates map[string]interface{}) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error
}

// UpdateUserEmail updates user email and sets email_verified to false
func (r *Repository) UpdateUserEmail(userID, newEmail string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"email":          newEmail,
		"email_verified": false,
	}).Error
}

// ClearLockout clears the lockout fields for a user (auto-unlock on expired lockout).
func (r *Repository) ClearLockout(userID string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"locked_at":       nil,
		"lock_reason":     "",
		"lock_expires_at": nil,
	}).Error
}

// SetBackupEmail sets the pending backup email for a user (not yet verified).
func (r *Repository) SetBackupEmail(userID, backupEmail string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"backup_email":          backupEmail,
		"backup_email_verified": false,
	}).Error
}

// VerifyBackupEmail marks the backup email as verified.
func (r *Repository) VerifyBackupEmail(userID string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("backup_email_verified", true).Error
}

// ClearBackupEmail removes the backup email and its verified status.
func (r *Repository) ClearBackupEmail(userID string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"backup_email":          "",
		"backup_email_verified": false,
	}).Error
}

// SaveAndSwitchToBackupEmail2FA atomically saves the user's current 2FA method/secret as
// "previous" fields and switches the active method to backup_email.
// This allows DisableBackupEmail2FAMethod to fully restore the prior configuration.
func (r *Repository) SaveAndSwitchToBackupEmail2FA(userID, previousMethod, previousSecret, recoveryCodes string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"two_fa_previous_method": previousMethod,
		"two_fa_previous_secret": previousSecret,
		"two_fa_enabled":         true,
		"two_fa_method":          "backup_email",
		"two_fa_secret":          "",
		"two_fa_recovery_codes":  recoveryCodes,
	}).Error
}

// RestorePreviousTwoFAMethod reverts a user from backup_email 2FA back to their prior method.
// It reads the previously saved method/secret, restores them, and clears the "previous" fields.
// If no prior method was saved the user ends up with 2FA disabled.
func (r *Repository) RestorePreviousTwoFAMethod(userID string) error {
	var user models.User
	if err := r.DB.Select("two_fa_previous_method, two_fa_previous_secret").
		Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}

	enabled := user.TwoFAPreviousMethod != ""
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"two_fa_method":          user.TwoFAPreviousMethod,
		"two_fa_secret":          user.TwoFAPreviousSecret,
		"two_fa_enabled":         enabled,
		"two_fa_previous_method": "",
		"two_fa_previous_secret": "",
		// Keep recovery codes — they remain valid for the restored method.
	}).Error
}

// SetPhoneNumber sets the phone number for a user (not yet verified).
func (r *Repository) SetPhoneNumber(userID, phone string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"phone_number":   phone,
		"phone_verified": false,
	}).Error
}

// VerifyPhoneNumber marks the phone number as verified.
func (r *Repository) VerifyPhoneNumber(userID string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("phone_verified", true).Error
}

// ClearPhone removes the phone number and its verified status.
func (r *Repository) ClearPhone(userID string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"phone_number":   "",
		"phone_verified": false,
	}).Error
}
