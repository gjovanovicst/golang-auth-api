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
	err := r.DB.Where("id = ?", id).First(&user).Error
	return &user, err
}

func (r *Repository) UpdateUserPassword(userID, hashedPassword string) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", hashedPassword).Error
}

func (r *Repository) UpdateUserEmailVerified(userID string, verified bool) error {
	return r.DB.Model(&models.User{}).Where("id = ?", userID).Update("email_verified", verified).Error
}