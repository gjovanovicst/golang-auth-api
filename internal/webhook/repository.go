package webhook

import (
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles all database operations for webhook endpoints and deliveries.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new webhook repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ============================================================================
// Endpoint operations
// ============================================================================

// CreateEndpoint persists a new webhook endpoint.
func (r *Repository) CreateEndpoint(ep *models.WebhookEndpoint) error {
	return r.db.Create(ep).Error
}

// GetEndpointByID returns a webhook endpoint by its primary key.
func (r *Repository) GetEndpointByID(id uuid.UUID) (*models.WebhookEndpoint, error) {
	var ep models.WebhookEndpoint
	if err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&ep).Error; err != nil {
		return nil, err
	}
	return &ep, nil
}

// GetEndpointByAppAndEvent returns the endpoint for a specific (appID, eventType) pair.
func (r *Repository) GetEndpointByAppAndEvent(appID uuid.UUID, eventType string) (*models.WebhookEndpoint, error) {
	var ep models.WebhookEndpoint
	if err := r.db.Where("app_id = ? AND event_type = ? AND deleted_at IS NULL", appID, eventType).First(&ep).Error; err != nil {
		return nil, err
	}
	return &ep, nil
}

// ListEndpointsByApp returns all (non-deleted) webhook endpoints for an application.
func (r *Repository) ListEndpointsByApp(appID uuid.UUID, page, pageSize int) ([]models.WebhookEndpoint, int64, error) {
	var endpoints []models.WebhookEndpoint
	var total int64

	query := r.db.Model(&models.WebhookEndpoint{}).Where("app_id = ? AND deleted_at IS NULL", appID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&endpoints).Error; err != nil {
		return nil, 0, err
	}
	return endpoints, total, nil
}

// ListAllEndpoints returns all non-deleted webhook endpoints (admin use).
func (r *Repository) ListAllEndpoints(page, pageSize int) ([]models.WebhookEndpoint, int64, error) {
	var endpoints []models.WebhookEndpoint
	var total int64

	query := r.db.Model(&models.WebhookEndpoint{}).Where("deleted_at IS NULL")
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&endpoints).Error; err != nil {
		return nil, 0, err
	}
	return endpoints, total, nil
}

// GetActiveEndpointsForEvent returns all active (non-deleted, is_active=true) endpoints
// for a given appID and eventType. Used by the dispatch path.
func (r *Repository) GetActiveEndpointsForEvent(appID uuid.UUID, eventType string) ([]models.WebhookEndpoint, error) {
	var endpoints []models.WebhookEndpoint
	if err := r.db.Where("app_id = ? AND event_type = ? AND is_active = true AND deleted_at IS NULL", appID, eventType).
		Find(&endpoints).Error; err != nil {
		return nil, err
	}
	return endpoints, nil
}

// UpdateEndpointActive sets the is_active flag on an endpoint.
func (r *Repository) UpdateEndpointActive(id uuid.UUID, isActive bool) error {
	return r.db.Model(&models.WebhookEndpoint{}).Where("id = ?", id).Update("is_active", isActive).Error
}

// SoftDeleteEndpoint sets deleted_at on an endpoint.
func (r *Repository) SoftDeleteEndpoint(id uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.Model(&models.WebhookEndpoint{}).Where("id = ?", id).Update("deleted_at", now).Error
}

// ============================================================================
// Delivery operations
// ============================================================================

// CreateDelivery persists a delivery record.
func (r *Repository) CreateDelivery(d *models.WebhookDelivery) error {
	return r.db.Create(d).Error
}

// GetDeliveriesByEndpoint returns delivery history for a specific endpoint, paginated.
func (r *Repository) GetDeliveriesByEndpoint(endpointID uuid.UUID, page, pageSize int) ([]models.WebhookDelivery, int64, error) {
	var deliveries []models.WebhookDelivery
	var total int64

	query := r.db.Model(&models.WebhookDelivery{}).Where("endpoint_id = ?", endpointID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&deliveries).Error; err != nil {
		return nil, 0, err
	}
	return deliveries, total, nil
}

// GetDeliveriesByApp returns delivery history across all endpoints for an app, paginated.
func (r *Repository) GetDeliveriesByApp(appID uuid.UUID, page, pageSize int) ([]models.WebhookDelivery, int64, error) {
	var deliveries []models.WebhookDelivery
	var total int64

	query := r.db.Model(&models.WebhookDelivery{}).Where("app_id = ?", appID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&deliveries).Error; err != nil {
		return nil, 0, err
	}
	return deliveries, total, nil
}

// GetPendingRetries returns all delivery records where next_retry_at <= now and success = false.
// Used by the background retry worker.
func (r *Repository) GetPendingRetries(now time.Time, limit int) ([]models.WebhookDelivery, error) {
	var deliveries []models.WebhookDelivery
	if err := r.db.
		Where("success = false AND next_retry_at IS NOT NULL AND next_retry_at <= ?", now).
		Order("next_retry_at ASC").
		Limit(limit).
		Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}

// ClearRetrySchedule nullifies next_retry_at (used after all retries exhausted or on success).
func (r *Repository) ClearRetrySchedule(id uuid.UUID) error {
	return r.db.Model(&models.WebhookDelivery{}).Where("id = ?", id).Update("next_retry_at", nil).Error
}
