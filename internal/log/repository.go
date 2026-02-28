package log

import (
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// ListUserActivityLogs retrieves activity logs for a specific user with pagination and filtering
func (r *Repository) ListUserActivityLogs(userID uuid.UUID, page, limit int, eventType string, startDate, endDate *time.Time) ([]models.ActivityLog, int64, error) {
	var logs []models.ActivityLog
	var totalCount int64

	// Build the base query
	query := r.DB.Where("user_id = ?", userID)

	// Apply event type filter if provided
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	// Apply date range filters if provided
	if startDate != nil {
		query = query.Where("timestamp >= ?", startDate)
	}
	if endDate != nil {
		// Add 1 day to end date to include the entire end date
		endOfDay := endDate.Add(24 * time.Hour)
		query = query.Where("timestamp < ?", endOfDay)
	}

	// Get total count for pagination
	if err := query.Model(&models.ActivityLog{}).Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	offset := (page - 1) * limit
	if err := query.Order("timestamp DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, totalCount, nil
}

// ListAllActivityLogs retrieves activity logs for all users (admin functionality) with pagination and filtering
func (r *Repository) ListAllActivityLogs(page, limit int, eventType string, startDate, endDate *time.Time) ([]models.ActivityLog, int64, error) {
	var logs []models.ActivityLog
	var totalCount int64

	// Build the base query
	query := r.DB.Model(&models.ActivityLog{})

	// Apply event type filter if provided
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	// Apply date range filters if provided
	if startDate != nil {
		query = query.Where("timestamp >= ?", startDate)
	}
	if endDate != nil {
		// Add 1 day to end date to include the entire end date
		endOfDay := endDate.Add(24 * time.Hour)
		query = query.Where("timestamp < ?", endOfDay)
	}

	// Get total count for pagination
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	offset := (page - 1) * limit
	if err := query.Order("timestamp DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, totalCount, nil
}

// GetActivityLogByID retrieves a specific activity log by ID
func (r *Repository) GetActivityLogByID(id uuid.UUID) (*models.ActivityLog, error) {
	var log models.ActivityLog
	if err := r.DB.Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}
