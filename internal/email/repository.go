package email

import (
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles database operations for the email system.
type Repository struct {
	DB *gorm.DB
}

// NewRepository creates a new email Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// ============================================================================
// Email Server Config operations
// ============================================================================

// GetServerConfig returns the default active SMTP configuration for a specific application.
// Returns nil, nil if no per-app config exists.
func (r *Repository) GetServerConfig(appID uuid.UUID) (*models.EmailServerConfig, error) {
	var config models.EmailServerConfig
	err := r.DB.Where("app_id = ? AND is_active = ? AND is_default = ?", appID, true, true).First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Fallback: try any active config for this app (if no default is flagged)
			err = r.DB.Where("app_id = ? AND is_active = ?", appID, true).First(&config).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil, nil
				}
				return nil, err
			}
			return &config, nil
		}
		return nil, err
	}
	return &config, nil
}

// GetServerConfigByID returns an SMTP configuration by its ID.
func (r *Repository) GetServerConfigByID(id uuid.UUID) (*models.EmailServerConfig, error) {
	var config models.EmailServerConfig
	err := r.DB.First(&config, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// GetServerConfigAny returns the first SMTP configuration for an application regardless of is_active.
// Used for admin listing and backward compatibility checks.
// Returns nil, nil if no config exists for the app.
func (r *Repository) GetServerConfigAny(appID uuid.UUID) (*models.EmailServerConfig, error) {
	var config models.EmailServerConfig
	err := r.DB.Where("app_id = ?", appID).First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// GetServerConfigsByApp returns all SMTP configurations for a specific application.
func (r *Repository) GetServerConfigsByApp(appID uuid.UUID) ([]models.EmailServerConfig, error) {
	var configs []models.EmailServerConfig
	err := r.DB.Where("app_id = ?", appID).Order("is_default DESC, name ASC").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// GetAllServerConfigs returns all SMTP configurations across all applications.
func (r *Repository) GetAllServerConfigs() ([]models.EmailServerConfig, error) {
	var configs []models.EmailServerConfig
	err := r.DB.Order("app_id, is_default DESC, name ASC").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// CreateServerConfig creates a new SMTP configuration for an application.
func (r *Repository) CreateServerConfig(config *models.EmailServerConfig) error {
	return r.DB.Create(config).Error
}

// UpdateServerConfig updates an existing SMTP configuration.
func (r *Repository) UpdateServerConfig(config *models.EmailServerConfig) error {
	return r.DB.Save(config).Error
}

// DeleteServerConfig removes the SMTP configuration for an application.
func (r *Repository) DeleteServerConfig(appID uuid.UUID) error {
	return r.DB.Where("app_id = ?", appID).Delete(&models.EmailServerConfig{}).Error
}

// DeleteServerConfigByID removes a specific SMTP configuration by its ID.
func (r *Repository) DeleteServerConfigByID(id uuid.UUID) error {
	return r.DB.Where("id = ?", id).Delete(&models.EmailServerConfig{}).Error
}

// ClearDefaultFlag unsets is_default on all configs for an app.
func (r *Repository) ClearDefaultFlag(appID uuid.UUID) error {
	return r.DB.Model(&models.EmailServerConfig{}).
		Where("app_id = ?", appID).
		Update("is_default", false).Error
}

// ============================================================================
// Email Type operations
// ============================================================================

// GetAllEmailTypes returns all email types.
func (r *Repository) GetAllEmailTypes() ([]models.EmailType, error) {
	var types []models.EmailType
	if err := r.DB.Order("name asc").Find(&types).Error; err != nil {
		return nil, err
	}
	return types, nil
}

// GetActiveEmailTypes returns all active email types.
func (r *Repository) GetActiveEmailTypes() ([]models.EmailType, error) {
	var types []models.EmailType
	if err := r.DB.Where("is_active = ?", true).Order("name asc").Find(&types).Error; err != nil {
		return nil, err
	}
	return types, nil
}

// GetEmailTypeByCode returns an email type by its unique code.
func (r *Repository) GetEmailTypeByCode(code string) (*models.EmailType, error) {
	var emailType models.EmailType
	err := r.DB.Where("code = ?", code).First(&emailType).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &emailType, nil
}

// GetEmailTypeByID returns an email type by its ID.
func (r *Repository) GetEmailTypeByID(id uuid.UUID) (*models.EmailType, error) {
	var emailType models.EmailType
	err := r.DB.First(&emailType, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &emailType, nil
}

// CreateEmailType creates a new email type.
func (r *Repository) CreateEmailType(emailType *models.EmailType) error {
	return r.DB.Create(emailType).Error
}

// UpdateEmailType updates an existing email type.
func (r *Repository) UpdateEmailType(emailType *models.EmailType) error {
	return r.DB.Save(emailType).Error
}

// DeleteEmailType deletes a custom email type (only non-system types).
// Also deletes any templates associated with this type.
func (r *Repository) DeleteEmailType(id uuid.UUID) error {
	// Delete associated templates first
	if err := r.DB.Where("email_type_id = ?", id).Delete(&models.EmailTemplate{}).Error; err != nil {
		return err
	}
	return r.DB.Where("id = ? AND is_system = ?", id, false).Delete(&models.EmailType{}).Error
}

// ============================================================================
// Email Template operations
// ============================================================================

// GetTemplate resolves the template for a given app and email type code.
// Resolution order: app-specific -> global default (app_id IS NULL) -> nil (use hardcoded).
func (r *Repository) GetTemplate(appID uuid.UUID, typeCode string) (*models.EmailTemplate, error) {
	// First get the email type
	emailType, err := r.GetEmailTypeByCode(typeCode)
	if err != nil {
		return nil, err
	}
	if emailType == nil {
		return nil, nil
	}

	// Try app-specific template first
	var template models.EmailTemplate
	err = r.DB.Where("app_id = ? AND email_type_id = ? AND is_active = ?", appID, emailType.ID, true).
		First(&template).Error
	if err == nil {
		template.EmailType = *emailType
		return &template, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Fall back to global default template (app_id IS NULL)
	err = r.DB.Where("app_id IS NULL AND email_type_id = ? AND is_active = ?", emailType.ID, true).
		First(&template).Error
	if err == nil {
		template.EmailType = *emailType
		return &template, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return nil, nil
}

// GetTemplatesByApp returns all templates for a specific application.
func (r *Repository) GetTemplatesByApp(appID uuid.UUID) ([]models.EmailTemplate, error) {
	var templates []models.EmailTemplate
	err := r.DB.Preload("EmailType").Where("app_id = ?", appID).
		Order("created_at asc").Find(&templates).Error
	if err != nil {
		return nil, err
	}
	return templates, nil
}

// GetGlobalDefaultTemplates returns all global default templates (app_id IS NULL).
func (r *Repository) GetGlobalDefaultTemplates() ([]models.EmailTemplate, error) {
	var templates []models.EmailTemplate
	err := r.DB.Preload("EmailType").Where("app_id IS NULL").
		Order("created_at asc").Find(&templates).Error
	if err != nil {
		return nil, err
	}
	return templates, nil
}

// GetTemplateByID returns a template by its ID.
func (r *Repository) GetTemplateByID(id uuid.UUID) (*models.EmailTemplate, error) {
	var template models.EmailTemplate
	err := r.DB.Preload("EmailType").First(&template, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

// CreateTemplate creates a new email template.
func (r *Repository) CreateTemplate(template *models.EmailTemplate) error {
	return r.DB.Create(template).Error
}

// UpdateTemplate updates an existing email template.
func (r *Repository) UpdateTemplate(template *models.EmailTemplate) error {
	return r.DB.Save(template).Error
}

// DeleteTemplate removes an email template.
func (r *Repository) DeleteTemplate(id uuid.UUID) error {
	return r.DB.Where("id = ?", id).Delete(&models.EmailTemplate{}).Error
}

// UpsertAppTemplate creates or updates a template for a specific app and email type.
func (r *Repository) UpsertAppTemplate(appID uuid.UUID, emailTypeID uuid.UUID, template *models.EmailTemplate) error {
	// Check if one already exists
	var existing models.EmailTemplate
	err := r.DB.Where("app_id = ? AND email_type_id = ?", appID, emailTypeID).First(&existing).Error
	if err == nil {
		// Update existing
		existing.Name = template.Name
		existing.Subject = template.Subject
		existing.BodyHTML = template.BodyHTML
		existing.BodyText = template.BodyText
		existing.TemplateEngine = template.TemplateEngine
		existing.FromEmail = template.FromEmail
		existing.FromName = template.FromName
		existing.ServerConfigID = template.ServerConfigID
		existing.IsActive = template.IsActive
		return r.DB.Save(&existing).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Create new
	template.AppID = &appID
	template.EmailTypeID = emailTypeID
	return r.DB.Create(template).Error
}

// UpsertGlobalTemplate creates or updates a global default template for an email type.
func (r *Repository) UpsertGlobalTemplate(emailTypeID uuid.UUID, template *models.EmailTemplate) error {
	var existing models.EmailTemplate
	err := r.DB.Where("app_id IS NULL AND email_type_id = ?", emailTypeID).First(&existing).Error
	if err == nil {
		existing.Name = template.Name
		existing.Subject = template.Subject
		existing.BodyHTML = template.BodyHTML
		existing.BodyText = template.BodyText
		existing.TemplateEngine = template.TemplateEngine
		existing.FromEmail = template.FromEmail
		existing.FromName = template.FromName
		existing.ServerConfigID = template.ServerConfigID
		existing.IsActive = template.IsActive
		return r.DB.Save(&existing).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	template.AppID = nil
	template.EmailTypeID = emailTypeID
	return r.DB.Create(template).Error
}
