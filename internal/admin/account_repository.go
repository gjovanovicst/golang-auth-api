package admin

import (
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"gorm.io/gorm"
)

// AccountRepository handles database operations for admin accounts
type AccountRepository struct {
	DB *gorm.DB
}

// NewAccountRepository creates a new AccountRepository
func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{DB: db}
}

// Create stores a new admin account in the database
func (r *AccountRepository) Create(account *models.AdminAccount) error {
	return r.DB.Create(account).Error
}

// GetByUsername retrieves an admin account by username
func (r *AccountRepository) GetByUsername(username string) (*models.AdminAccount, error) {
	var account models.AdminAccount
	if err := r.DB.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// GetByID retrieves an admin account by ID
func (r *AccountRepository) GetByID(id string) (*models.AdminAccount, error) {
	var account models.AdminAccount
	if err := r.DB.First(&account, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// UpdateLastLogin sets the LastLoginAt timestamp for an admin account
func (r *AccountRepository) UpdateLastLogin(id string) error {
	now := time.Now()
	return r.DB.Model(&models.AdminAccount{}).Where("id = ?", id).Update("last_login_at", now).Error
}

// ListAll retrieves all admin accounts ordered by creation date
func (r *AccountRepository) ListAll() ([]models.AdminAccount, error) {
	var accounts []models.AdminAccount
	if err := r.DB.Order("created_at asc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

// Count returns the total number of admin accounts
func (r *AccountRepository) Count() (int64, error) {
	var count int64
	if err := r.DB.Model(&models.AdminAccount{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteByID removes an admin account by ID
func (r *AccountRepository) DeleteByID(id string) error {
	return r.DB.Delete(&models.AdminAccount{}, "id = ?", id).Error
}

// UpdateEmail sets the email address for an admin account
func (r *AccountRepository) UpdateEmail(id, email string) error {
	return r.DB.Model(&models.AdminAccount{}).Where("id = ?", id).Update("email", email).Error
}

// UpdatePassword updates the password hash for an admin account
func (r *AccountRepository) UpdatePassword(id, passwordHash string) error {
	return r.DB.Model(&models.AdminAccount{}).Where("id = ?", id).Update("password_hash", passwordHash).Error
}

// Enable2FA activates two-factor authentication for an admin account.
// It sets the method, secret, and recovery codes in a single update.
func (r *AccountRepository) Enable2FA(id, method, secret string, recoveryCodes []byte) error {
	return r.DB.Model(&models.AdminAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"two_fa_enabled":        true,
		"two_fa_method":         method,
		"two_fa_secret":         secret,
		"two_fa_recovery_codes": recoveryCodes,
	}).Error
}

// Disable2FA deactivates two-factor authentication for an admin account,
// clearing all 2FA-related fields.
func (r *AccountRepository) Disable2FA(id string) error {
	return r.DB.Model(&models.AdminAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"two_fa_enabled":        false,
		"two_fa_method":         "",
		"two_fa_secret":         "",
		"two_fa_recovery_codes": []byte("[]"),
	}).Error
}

// UpdateRecoveryCodes replaces the recovery codes for an admin account.
func (r *AccountRepository) UpdateRecoveryCodes(id string, codes []byte) error {
	return r.DB.Model(&models.AdminAccount{}).Where("id = ?", id).Update("two_fa_recovery_codes", codes).Error
}
