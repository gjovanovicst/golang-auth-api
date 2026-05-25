package twofa

import (
	"encoding/json"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserApp2FARepository handles all database operations for the user_app_2fa table.
type UserApp2FARepository struct {
	DB *gorm.DB
}

func NewUserApp2FARepository(db *gorm.DB) *UserApp2FARepository {
	return &UserApp2FARepository{DB: db}
}

// Get returns the per-app 2FA record for a (user, app) pair.
// Returns nil (no error) when no record exists.
func (r *UserApp2FARepository) Get(userID, appID uuid.UUID) (*models.UserApp2FA, error) {
	var rec models.UserApp2FA
	err := r.DB.Where("user_id = ? AND application_id = ?", userID, appID).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &rec, err
}

// Upsert creates or fully replaces the per-app 2FA record.
func (r *UserApp2FARepository) Upsert(rec *models.UserApp2FA) error {
	return r.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "application_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"two_fa_enabled",
			"two_fa_method",
			"two_fa_secret",
			"recovery_codes",
			"previous_method",
			"previous_secret",
			"updated_at",
		}),
	}).Create(rec).Error
}

// Enable2FA enables 2FA for a (user, app) pair using TOTP.
// secret and recoveryCodesJSON are already in their final (encrypted) form.
func (r *UserApp2FARepository) Enable2FA(userID, appID uuid.UUID, secret, recoveryCodesJSON, method string) error {
	return r.DB.Model(&models.UserApp2FA{}).
		Where("user_id = ? AND application_id = ?", userID, appID).
		Updates(map[string]interface{}{
			"two_fa_enabled": true,
			"two_fa_method":  method,
			"two_fa_secret":  secret,
			"recovery_codes": recoveryCodesJSON,
		}).Error
}

// EnableOrCreate enables 2FA; creates the row if it does not yet exist.
func (r *UserApp2FARepository) EnableOrCreate(userID, appID uuid.UUID, secret, recoveryCodesJSON, method string) error {
	rec := &models.UserApp2FA{
		UserID:        userID,
		ApplicationID: appID,
		TwoFAEnabled:  true,
		TwoFAMethod:   method,
		TwoFASecret:   secret,
	}
	// Store the JSONB recovery codes directly into the model's datatypes.JSON field
	rec.RecoveryCodes = []byte(recoveryCodesJSON)

	return r.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "application_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"two_fa_enabled",
			"two_fa_method",
			"two_fa_secret",
			"recovery_codes",
			"updated_at",
		}),
	}).Create(rec).Error
}

// Disable2FA disables 2FA for a (user, app) pair and clears the secret.
func (r *UserApp2FARepository) Disable2FA(userID, appID uuid.UUID) error {
	return r.DB.Model(&models.UserApp2FA{}).
		Where("user_id = ? AND application_id = ?", userID, appID).
		Updates(map[string]interface{}{
			"two_fa_enabled":  false,
			"two_fa_method":   "",
			"two_fa_secret":   "",
			"recovery_codes":  nil,
			"previous_method": "",
			"previous_secret": "",
		}).Error
}

// UpdateRecoveryCodes replaces the recovery codes for a (user, app) pair.
func (r *UserApp2FARepository) UpdateRecoveryCodes(userID, appID uuid.UUID, recoveryCodesJSON string) error {
	return r.DB.Model(&models.UserApp2FA{}).
		Where("user_id = ? AND application_id = ?", userID, appID).
		Update("recovery_codes", recoveryCodesJSON).Error
}

// SavePreviousMethod saves the current method/secret as "previous" before switching
// to backup_email 2FA (so it can be restored later).
func (r *UserApp2FARepository) SavePreviousMethod(userID, appID uuid.UUID, previousMethod, previousSecret string) error {
	return r.DB.Model(&models.UserApp2FA{}).
		Where("user_id = ? AND application_id = ?", userID, appID).
		Updates(map[string]interface{}{
			"previous_method": previousMethod,
			"previous_secret": previousSecret,
		}).Error
}

// RestorePreviousMethod sets two_fa_method/secret back to the saved previous values
// and clears the "previous" columns.  Used when disabling backup_email 2FA.
func (r *UserApp2FARepository) RestorePreviousMethod(userID, appID uuid.UUID) error {
	// Load the current record first so we can read previous_method / previous_secret.
	var rec models.UserApp2FA
	if err := r.DB.Where("user_id = ? AND application_id = ?", userID, appID).First(&rec).Error; err != nil {
		return err
	}

	updates := map[string]interface{}{
		"previous_method": "",
		"previous_secret": "",
	}
	if rec.PreviousMethod != "" {
		updates["two_fa_method"] = rec.PreviousMethod
		updates["two_fa_secret"] = rec.PreviousSecret
		updates["two_fa_enabled"] = true
	} else {
		// No prior method — fully disable 2FA
		updates["two_fa_enabled"] = false
		updates["two_fa_method"] = ""
		updates["two_fa_secret"] = ""
		updates["recovery_codes"] = nil
	}

	return r.DB.Model(&models.UserApp2FA{}).
		Where("user_id = ? AND application_id = ?", userID, appID).
		Updates(updates).Error
}

// GetRecoveryCodes parses and returns the recovery codes for a (user, app) pair.
func (r *UserApp2FARepository) GetRecoveryCodes(userID, appID uuid.UUID) ([]string, error) {
	var rec models.UserApp2FA
	if err := r.DB.Select("recovery_codes").
		Where("user_id = ? AND application_id = ?", userID, appID).
		First(&rec).Error; err != nil {
		return nil, err
	}
	var codes []string
	if err := json.Unmarshal(rec.RecoveryCodes, &codes); err != nil {
		return nil, err
	}
	return codes, nil
}

// ConsumeRecoveryCode verifies and removes a single recovery code (one-time use).
func (r *UserApp2FARepository) ConsumeRecoveryCode(userID, appID uuid.UUID, code string) error {
	codes, err := r.GetRecoveryCodes(userID, appID)
	if err != nil {
		return err
	}
	for i, c := range codes {
		if c == code {
			codes = append(codes[:i], codes[i+1:]...)
			updated, _ := json.Marshal(codes)
			return r.UpdateRecoveryCodes(userID, appID, string(updated))
		}
	}
	return gorm.ErrRecordNotFound // reuse sentinel for "not found"
}
