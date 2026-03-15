package oidc

import (
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles all OIDC-related database operations.
type Repository struct {
	DB *gorm.DB
}

// NewRepository constructs a new OIDC Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// ─── Application ───────────────────────────────────────────────────────────────

// GetApplication fetches an Application by UUID.
func (r *Repository) GetApplication(appID uuid.UUID) (*models.Application, error) {
	var app models.Application
	err := r.DB.Where("id = ?", appID).First(&app).Error
	return &app, err
}

// SaveRSAPrivateKey persists the PEM-encoded RSA private key for an application.
func (r *Repository) SaveRSAPrivateKey(appID uuid.UUID, pemKey string) error {
	return r.DB.Model(&models.Application{}).
		Where("id = ?", appID).
		Update("oidc_rsa_private_key", pemKey).Error
}

// ─── OIDCClient CRUD ────────────────────────────────────────────────────────────

// CreateClient inserts a new OIDCClient.
func (r *Repository) CreateClient(client *models.OIDCClient) error {
	return r.DB.Create(client).Error
}

// GetClientByID fetches an OIDCClient by its primary key UUID.
func (r *Repository) GetClientByID(id uuid.UUID) (*models.OIDCClient, error) {
	var c models.OIDCClient
	err := r.DB.Where("id = ?", id).First(&c).Error
	return &c, err
}

// GetClientByClientID fetches an OIDCClient by its public client_id string.
func (r *Repository) GetClientByClientID(clientID string) (*models.OIDCClient, error) {
	var c models.OIDCClient
	err := r.DB.Where("client_id = ?", clientID).First(&c).Error
	return &c, err
}

// ListClientsByApp returns all OIDC clients registered for a given application.
func (r *Repository) ListClientsByApp(appID uuid.UUID) ([]models.OIDCClient, error) {
	var clients []models.OIDCClient
	err := r.DB.Where("app_id = ?", appID).Order("created_at ASC").Find(&clients).Error
	return clients, err
}

// UpdateClient persists changes to an OIDCClient.
func (r *Repository) UpdateClient(client *models.OIDCClient) error {
	return r.DB.Save(client).Error
}

// DeleteClient hard-deletes an OIDCClient by UUID.
func (r *Repository) DeleteClient(id uuid.UUID) error {
	return r.DB.Where("id = ?", id).Delete(&models.OIDCClient{}).Error
}

// ─── OIDCAuthCode ──────────────────────────────────────────────────────────────

// CreateAuthCode inserts a new authorization code record.
func (r *Repository) CreateAuthCode(code *models.OIDCAuthCode) error {
	return r.DB.Create(code).Error
}

// GetAuthCode fetches a single authorization code by its code string.
func (r *Repository) GetAuthCode(code string) (*models.OIDCAuthCode, error) {
	var ac models.OIDCAuthCode
	err := r.DB.Where("code = ?", code).First(&ac).Error
	return &ac, err
}

// MarkAuthCodeUsed marks an authorization code as used (prevents replay).
func (r *Repository) MarkAuthCodeUsed(id uuid.UUID) error {
	return r.DB.Model(&models.OIDCAuthCode{}).
		Where("id = ?", id).
		Update("used", true).Error
}

// DeleteExpiredAuthCodes removes all expired authorization codes.
// Call periodically to keep the table small.
func (r *Repository) DeleteExpiredAuthCodes() error {
	return r.DB.Where("expires_at < ?", time.Now()).Delete(&models.OIDCAuthCode{}).Error
}

// ─── User lookup (needed by service layer) ─────────────────────────────────────

// GetUserByID fetches a User by UUID string.
func (r *Repository) GetUserByID(id string) (*models.User, error) {
	var u models.User
	err := r.DB.Where("id = ?", id).First(&u).Error
	return &u, err
}

// GetUserByEmail fetches a User by appID + email.
func (r *Repository) GetUserByEmail(appID, email string) (*models.User, error) {
	var u models.User
	err := r.DB.Where("app_id = ? AND email = ?", appID, email).First(&u).Error
	return &u, err
}
