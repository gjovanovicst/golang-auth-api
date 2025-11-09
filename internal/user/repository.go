package user

import (
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

func (r *Repository) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.DB.Where("email = ?", email).First(&user).Error
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

func (r *Repository) UpdateUserEmailVerified(userID string, verified bool) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("email_verified", verified).Error
}

// 2FA related methods

// Enable2FA enables 2FA for a user and stores the secret and recovery codes
func (r *Repository) Enable2FA(userID, secret, recoveryCodes string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"two_fa_enabled":        true,
		"two_fa_secret":         secret,
		"two_fa_recovery_codes": recoveryCodes,
	}).Error
}

// Disable2FA disables 2FA for a user
func (r *Repository) Disable2FA(userID string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"two_fa_enabled":        false,
		"two_fa_secret":         "",
		"two_fa_recovery_codes": nil,
	}).Error
}

// UpdateRecoveryCodes updates the recovery codes for a user
func (r *Repository) UpdateRecoveryCodes(userID, recoveryCodes string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("two_fa_recovery_codes", recoveryCodes).Error
}
