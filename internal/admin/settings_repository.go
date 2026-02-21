package admin

import (
	"github.com/gjovanovicst/auth_api/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SettingsRepository handles CRUD operations for the system_settings table.
type SettingsRepository struct {
	DB *gorm.DB
}

// NewSettingsRepository creates a new SettingsRepository.
func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{DB: db}
}

// GetAllSettings returns all system settings from the database.
func (r *SettingsRepository) GetAllSettings() ([]models.SystemSetting, error) {
	var settings []models.SystemSetting
	if err := r.DB.Order("category asc, key asc").Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

// GetSettingsByCategory returns all settings for a given category.
func (r *SettingsRepository) GetSettingsByCategory(category string) ([]models.SystemSetting, error) {
	var settings []models.SystemSetting
	if err := r.DB.Where("category = ?", category).Order("key asc").Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

// GetSettingByKey returns a single setting by its key.
// Returns nil, nil if the key is not found in the database.
func (r *SettingsRepository) GetSettingByKey(key string) (*models.SystemSetting, error) {
	var setting models.SystemSetting
	if err := r.DB.First(&setting, "key = ?", key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &setting, nil
}

// UpsertSetting inserts or updates a setting by key.
// Uses PostgreSQL ON CONFLICT DO UPDATE (upsert).
func (r *SettingsRepository) UpsertSetting(key, value, category string) error {
	setting := models.SystemSetting{
		Key:      key,
		Value:    value,
		Category: category,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&setting).Error
}

// DeleteSetting removes a setting from the database (reverts to default behavior).
func (r *SettingsRepository) DeleteSetting(key string) error {
	return r.DB.Where("key = ?", key).Delete(&models.SystemSetting{}).Error
}
