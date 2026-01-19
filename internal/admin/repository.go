package admin

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

// Tenant Operations

func (r *Repository) CreateTenant(tenant *models.Tenant) error {
	return r.DB.Create(tenant).Error
}

func (r *Repository) GetTenantByID(id string) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := r.DB.Preload("Apps").First(&tenant, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (r *Repository) ListTenants(page, pageSize int) ([]models.Tenant, int64, error) {
	var tenants []models.Tenant
	var total int64

	if err := r.DB.Model(&models.Tenant{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := r.DB.Limit(pageSize).Offset(offset).Order("created_at desc").Find(&tenants).Error; err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

// App Operations

func (r *Repository) CreateApp(app *models.Application) error {
	return r.DB.Create(app).Error
}

func (r *Repository) GetAppByID(id string) (*models.Application, error) {
	var app models.Application
	if err := r.DB.Preload("OAuthProviderConfigs").First(&app, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) ListAppsByTenant(tenantID string) ([]models.Application, error) {
	var apps []models.Application
	if err := r.DB.Where("tenant_id = ?", tenantID).Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

// OAuth Config Operations

func (r *Repository) UpsertOAuthConfig(config *models.OAuthProviderConfig) error {
	// Check if exists
	var existing models.OAuthProviderConfig
	err := r.DB.Where("app_id = ? AND provider = ?", config.AppID, config.Provider).First(&existing).Error
	
	if err == nil {
		// Update
		config.ID = existing.ID
		return r.DB.Save(config).Error
	}
	
	// Create
	return r.DB.Create(config).Error
}
