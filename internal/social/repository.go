package social

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

func (r *Repository) CreateSocialAccount(socialAccount *models.SocialAccount) error {
	return r.DB.Create(socialAccount).Error
}

func (r *Repository) GetSocialAccountByProviderAndUserID(provider, providerUserID string) (*models.SocialAccount, error) {
	var socialAccount models.SocialAccount
	err := r.DB.Where("provider = ? AND provider_user_id = ?", provider, providerUserID).First(&socialAccount).Error
	return &socialAccount, err
}

func (r *Repository) GetSocialAccountsByUserID(userID string) ([]models.SocialAccount, error) {
	var socialAccounts []models.SocialAccount
	err := r.DB.Where("user_id = ?", userID).Find(&socialAccounts).Error
	return socialAccounts, err
}

func (r *Repository) UpdateSocialAccount(socialAccount *models.SocialAccount) error {
	return r.DB.Save(socialAccount).Error
}

func (r *Repository) UpdateSocialAccountTokens(id string, accessToken, refreshToken string) error {
	return r.DB.Model(&models.SocialAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	}).Error
}

func (r *Repository) DeleteSocialAccount(id string) error {
	return r.DB.Delete(&models.SocialAccount{}, "id = ?", id).Error
}