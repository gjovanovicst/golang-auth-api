package twofa

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TrustedDeviceRepository handles all database operations for TrustedDevice records.
type TrustedDeviceRepository struct {
	DB *gorm.DB
}

// NewTrustedDeviceRepository creates a new TrustedDeviceRepository.
func NewTrustedDeviceRepository(db *gorm.DB) *TrustedDeviceRepository {
	return &TrustedDeviceRepository{DB: db}
}

// hashToken returns the SHA-256 hex hash of a plaintext device token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

// Create persists a new TrustedDevice record. The caller is responsible for hashing
// the device token before storing it (use hashToken).
func (r *TrustedDeviceRepository) Create(device *models.TrustedDevice) error {
	return r.DB.Create(device).Error
}

// FindByTokenHash looks up a trusted device by its hashed token.
// Returns nil, nil when not found.
func (r *TrustedDeviceRepository) FindByTokenHash(tokenHash string) (*models.TrustedDevice, error) {
	var d models.TrustedDevice
	err := r.DB.Where("token_hash = ?", tokenHash).First(&d).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &d, err
}

// FindByUserAndApp returns all non-expired trusted devices for a given user + app.
func (r *TrustedDeviceRepository) FindByUserAndApp(userID, appID uuid.UUID) ([]models.TrustedDevice, error) {
	var devices []models.TrustedDevice
	err := r.DB.Where("user_id = ? AND app_id = ? AND expires_at > ?", userID, appID, time.Now().UTC()).
		Order("last_used_at DESC").
		Find(&devices).Error
	return devices, err
}

// FindByID returns a single trusted device by its primary key.
// Returns nil, nil when not found.
func (r *TrustedDeviceRepository) FindByID(id uuid.UUID) (*models.TrustedDevice, error) {
	var d models.TrustedDevice
	err := r.DB.First(&d, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &d, err
}

// TouchLastUsed updates last_used_at to the current time for a device.
func (r *TrustedDeviceRepository) TouchLastUsed(id uuid.UUID) error {
	return r.DB.Model(&models.TrustedDevice{}).Where("id = ?", id).
		Update("last_used_at", time.Now().UTC()).Error
}

// DeleteByID removes a single trusted device by its primary key.
func (r *TrustedDeviceRepository) DeleteByID(id uuid.UUID) error {
	return r.DB.Where("id = ?", id).Delete(&models.TrustedDevice{}).Error
}

// DeleteAllForUser removes all trusted devices for a user in a given app.
func (r *TrustedDeviceRepository) DeleteAllForUser(userID, appID uuid.UUID) error {
	return r.DB.Where("user_id = ? AND app_id = ?", userID, appID).
		Delete(&models.TrustedDevice{}).Error
}

// DeleteByUserAppAndUserAgent removes all existing device records (expired or active)
// for a given user + app + user-agent combination. Used for deduplication before
// inserting a fresh trusted device row on repeated "Remember this device" logins.
func (r *TrustedDeviceRepository) DeleteByUserAppAndUserAgent(userID, appID uuid.UUID, userAgent string) error {
	return r.DB.Where("user_id = ? AND app_id = ? AND user_agent = ?", userID, appID, userAgent).
		Delete(&models.TrustedDevice{}).Error
}

// DeleteExpired removes all trusted devices whose ExpiresAt is in the past.
// This can be called periodically as a cleanup job.
func (r *TrustedDeviceRepository) DeleteExpired() (int64, error) {
	result := r.DB.Where("expires_at < ?", time.Now().UTC()).Delete(&models.TrustedDevice{})
	return result.RowsAffected, result.Error
}

// CountByUserAndApp returns the number of active (non-expired) trusted devices for a user.
func (r *TrustedDeviceRepository) CountByUserAndApp(userID, appID uuid.UUID) (int64, error) {
	var count int64
	err := r.DB.Model(&models.TrustedDevice{}).
		Where("user_id = ? AND app_id = ? AND expires_at > ?", userID, appID, time.Now().UTC()).
		Count(&count).Error
	return count, err
}

// CountAllActive returns the total count of non-expired trusted devices across all apps/users.
// Used for dashboard stats.
func (r *TrustedDeviceRepository) CountAllActive() (int64, error) {
	var count int64
	err := r.DB.Model(&models.TrustedDevice{}).
		Where("expires_at > ?", time.Now().UTC()).
		Count(&count).Error
	return count, err
}

// FindAllForUser returns all non-expired trusted devices for a user across all apps.
// Used by the admin panel to list and revoke devices.
func (r *TrustedDeviceRepository) FindAllForUser(userID uuid.UUID) ([]models.TrustedDevice, error) {
	var devices []models.TrustedDevice
	err := r.DB.Where("user_id = ? AND expires_at > ?", userID, time.Now().UTC()).
		Order("last_used_at DESC").
		Find(&devices).Error
	return devices, err
}
