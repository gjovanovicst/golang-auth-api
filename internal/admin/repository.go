package admin

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

// TenantListItem holds a tenant with its app count for list views.
type TenantListItem struct {
	ID        uuid.UUID
	Name      string
	AppCount  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ListTenantsWithAppCount returns paginated tenants with their application counts.
func (r *Repository) ListTenantsWithAppCount(page, pageSize int) ([]TenantListItem, int64, error) {
	var total int64
	if err := r.DB.Model(&models.Tenant{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []TenantListItem
	offset := (page - 1) * pageSize

	err := r.DB.Model(&models.Tenant{}).
		Select("tenants.id, tenants.name, tenants.created_at, tenants.updated_at, COUNT(applications.id) as app_count").
		Joins("LEFT JOIN applications ON applications.tenant_id = tenants.id").
		Group("tenants.id").
		Order("tenants.created_at desc").
		Limit(pageSize).
		Offset(offset).
		Scan(&items).Error

	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *Repository) UpdateTenant(id string, name string) error {
	return r.DB.Model(&models.Tenant{}).Where("id = ?", id).Update("name", name).Error
}

func (r *Repository) DeleteTenant(id string) error {
	return r.DB.Where("id = ?", id).Delete(&models.Tenant{}).Error
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

// AppListItem holds an application with its tenant name and OAuth config count for list views.
type AppListItem struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Name             string
	Description      string
	TenantName       string
	OAuthConfigCount int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ListAppsWithDetails returns paginated applications with tenant name and OAuth config count.
// If tenantID is non-empty, results are filtered to that tenant.
func (r *Repository) ListAppsWithDetails(page, pageSize int, tenantID string) ([]AppListItem, int64, error) {
	var total int64

	countQuery := r.DB.Model(&models.Application{})
	if tenantID != "" {
		countQuery = countQuery.Where("applications.tenant_id = ?", tenantID)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []AppListItem
	offset := (page - 1) * pageSize

	query := r.DB.Model(&models.Application{}).
		Select(`applications.id, applications.tenant_id, applications.name, applications.description,
			applications.created_at, applications.updated_at,
			tenants.name as tenant_name,
			COUNT(oauth_provider_configs.id) as o_auth_config_count`).
		Joins("LEFT JOIN tenants ON tenants.id = applications.tenant_id").
		Joins("LEFT JOIN oauth_provider_configs ON oauth_provider_configs.app_id = applications.id").
		Group("applications.id, tenants.name").
		Order("applications.created_at desc").
		Limit(pageSize).
		Offset(offset)

	if tenantID != "" {
		query = query.Where("applications.tenant_id = ?", tenantID)
	}

	if err := query.Scan(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *Repository) UpdateApp(id string, name string, description string) error {
	return r.DB.Model(&models.Application{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"name":        name,
			"description": description,
		}).Error
}

func (r *Repository) DeleteApp(id string) error {
	return r.DB.Where("id = ?", id).Delete(&models.Application{}).Error
}

// ListAllTenants returns all tenants (ID and Name only), ordered by name.
// Used for populating dropdown selects in forms and filters.
func (r *Repository) ListAllTenants() ([]models.Tenant, error) {
	var tenants []models.Tenant
	if err := r.DB.Select("id, name").Order("name asc").Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

// Count Operations (used by Dashboard)

func (r *Repository) CountTenants() (int64, error) {
	var count int64
	if err := r.DB.Model(&models.Tenant{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) CountApps() (int64, error) {
	var count int64
	if err := r.DB.Model(&models.Application{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
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

// OAuthConfigListItem holds an OAuth config with app and tenant names for list views.
type OAuthConfigListItem struct {
	ID          uuid.UUID
	AppID       uuid.UUID
	AppName     string
	TenantName  string
	Provider    string
	ClientID    string
	RedirectURL string
	IsEnabled   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListOAuthConfigsWithDetails returns paginated OAuth configs with app and tenant names.
// If appID is non-empty, results are filtered to that application.
func (r *Repository) ListOAuthConfigsWithDetails(page, pageSize int, appID string) ([]OAuthConfigListItem, int64, error) {
	var total int64

	countQuery := r.DB.Model(&models.OAuthProviderConfig{})
	if appID != "" {
		countQuery = countQuery.Where("oauth_provider_configs.app_id = ?", appID)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []OAuthConfigListItem
	offset := (page - 1) * pageSize

	query := r.DB.Model(&models.OAuthProviderConfig{}).
		Select(`oauth_provider_configs.id, oauth_provider_configs.app_id,
			oauth_provider_configs.provider, oauth_provider_configs.client_id,
			oauth_provider_configs.redirect_url, oauth_provider_configs.is_enabled,
			oauth_provider_configs.created_at, oauth_provider_configs.updated_at,
			applications.name as app_name,
			tenants.name as tenant_name`).
		Joins("LEFT JOIN applications ON applications.id = oauth_provider_configs.app_id").
		Joins("LEFT JOIN tenants ON tenants.id = applications.tenant_id").
		Order("oauth_provider_configs.created_at desc").
		Limit(pageSize).
		Offset(offset)

	if appID != "" {
		query = query.Where("oauth_provider_configs.app_id = ?", appID)
	}

	if err := query.Scan(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetOAuthConfigByID returns a single OAuth config by ID.
func (r *Repository) GetOAuthConfigByID(id string) (*models.OAuthProviderConfig, error) {
	var config models.OAuthProviderConfig
	if err := r.DB.First(&config, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// UpdateOAuthConfigByID updates an OAuth config by primary key.
// If clientSecret is empty, the existing secret is preserved.
func (r *Repository) UpdateOAuthConfigByID(id string, clientID string, clientSecret string, redirectURL string, isEnabled bool) error {
	updates := map[string]interface{}{
		"client_id":    clientID,
		"redirect_url": redirectURL,
		"is_enabled":   isEnabled,
	}
	if clientSecret != "" {
		updates["client_secret"] = clientSecret
	}
	return r.DB.Model(&models.OAuthProviderConfig{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteOAuthConfig deletes an OAuth config by ID.
func (r *Repository) DeleteOAuthConfig(id string) error {
	return r.DB.Where("id = ?", id).Delete(&models.OAuthProviderConfig{}).Error
}

// ToggleOAuthConfigEnabled flips the IsEnabled flag for an OAuth config.
func (r *Repository) ToggleOAuthConfigEnabled(id string) (*models.OAuthProviderConfig, error) {
	var config models.OAuthProviderConfig
	if err := r.DB.First(&config, "id = ?", id).Error; err != nil {
		return nil, err
	}
	config.IsEnabled = !config.IsEnabled
	if err := r.DB.Model(&config).Update("is_enabled", config.IsEnabled).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// AppWithTenant holds an application ID, name, and its tenant name for dropdown selects.
type AppWithTenant struct {
	ID         uuid.UUID
	Name       string
	TenantName string
}

// ListAllAppsWithTenantName returns all applications with their tenant name, ordered by tenant then app name.
// Used for populating dropdown selects in forms and filters.
func (r *Repository) ListAllAppsWithTenantName() ([]AppWithTenant, error) {
	var items []AppWithTenant
	err := r.DB.Model(&models.Application{}).
		Select("applications.id, applications.name, tenants.name as tenant_name").
		Joins("LEFT JOIN tenants ON tenants.id = applications.tenant_id").
		Order("tenants.name asc, applications.name asc").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ============================================================
// User Operations (Admin GUI - read + toggle only)
// ============================================================

// UserListItem represents a user row in the admin GUI list view
type UserListItem struct {
	ID                 uuid.UUID `json:"id"`
	Email              string    `json:"email"`
	Name               string    `json:"name"`
	AppID              uuid.UUID `json:"app_id"`
	AppName            string    `json:"app_name"`
	TenantName         string    `json:"tenant_name"`
	IsActive           bool      `json:"is_active"`
	EmailVerified      bool      `json:"email_verified"`
	TwoFAEnabled       bool      `json:"two_fa_enabled"`
	HasPassword        bool      `json:"has_password"`
	SocialAccountCount int       `json:"social_account_count"`
	CreatedAt          time.Time `json:"created_at"`
}

// UserDetail represents a full user view with social accounts for the admin GUI detail panel
type UserDetail struct {
	ID             uuid.UUID              `json:"id"`
	Email          string                 `json:"email"`
	Name           string                 `json:"name"`
	FirstName      string                 `json:"first_name"`
	LastName       string                 `json:"last_name"`
	ProfilePicture string                 `json:"profile_picture"`
	Locale         string                 `json:"locale"`
	AppID          uuid.UUID              `json:"app_id"`
	AppName        string                 `json:"app_name"`
	TenantName     string                 `json:"tenant_name"`
	IsActive       bool                   `json:"is_active"`
	EmailVerified  bool                   `json:"email_verified"`
	TwoFAEnabled   bool                   `json:"two_fa_enabled"`
	HasPassword    bool                   `json:"has_password"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	SocialAccounts []models.SocialAccount `json:"social_accounts" gorm:"-"`
}

// UserStatusCounts holds active/inactive user counts for dashboard display
type UserStatusCounts struct {
	ActiveUsers   int64 `json:"active_users"`
	InactiveUsers int64 `json:"inactive_users"`
}

// ListUsersWithDetails returns a paginated list of users with app/tenant info and social account counts.
// Supports optional filtering by appID and text search on email/name.
func (r *Repository) ListUsersWithDetails(page, pageSize int, appID, search string) ([]UserListItem, int64, error) {
	var items []UserListItem
	var total int64

	// Build base conditions for reuse in both count and data queries
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Joins("LEFT JOIN applications ON applications.id = users.app_id").
			Joins("LEFT JOIN tenants ON tenants.id = applications.tenant_id").
			Joins("LEFT JOIN (SELECT user_id, COUNT(*) as count FROM social_accounts GROUP BY user_id) sa_count ON sa_count.user_id = users.id")
		if appID != "" {
			q = q.Where("users.app_id = ?", appID)
		}
		if search != "" {
			searchTerm := "%" + search + "%"
			q = q.Where("(users.email ILIKE ? OR users.name ILIKE ?)", searchTerm, searchTerm)
		}
		return q
	}

	// Count total matching records (separate query)
	countQuery := applyFilters(r.DB.Model(&models.User{}))
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated results
	dataQuery := applyFilters(r.DB.Model(&models.User{}).
		Select(`users.id, users.email, users.name, users.app_id,
			applications.name as app_name,
			COALESCE(tenants.name, '') as tenant_name,
			users.is_active, users.email_verified, users.two_fa_enabled,
			(users.password_hash != '') as has_password,
			COALESCE(sa_count.count, 0) as social_account_count,
			users.created_at`))

	offset := (page - 1) * pageSize
	if err := dataQuery.Order("users.created_at desc").Offset(offset).Limit(pageSize).Scan(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetUserDetailByID returns a full user detail view with social accounts, app name, and tenant name.
func (r *Repository) GetUserDetailByID(id string) (*UserDetail, error) {
	var detail UserDetail

	err := r.DB.Model(&models.User{}).
		Select(`users.id, users.email, users.name, users.first_name, users.last_name,
			users.profile_picture, users.locale, users.app_id,
			applications.name as app_name,
			COALESCE(tenants.name, '') as tenant_name,
			users.is_active, users.email_verified, users.two_fa_enabled,
			(users.password_hash != '') as has_password,
			users.created_at, users.updated_at`).
		Joins("LEFT JOIN applications ON applications.id = users.app_id").
		Joins("LEFT JOIN tenants ON tenants.id = applications.tenant_id").
		Where("users.id = ?", id).
		Scan(&detail).Error
	if err != nil {
		return nil, err
	}

	// Check if user was found (GORM Scan doesn't return ErrRecordNotFound)
	if detail.ID == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}

	// Load social accounts
	var socialAccounts []models.SocialAccount
	if err := r.DB.Where("user_id = ?", id).Order("created_at asc").Find(&socialAccounts).Error; err != nil {
		return nil, err
	}
	detail.SocialAccounts = socialAccounts

	return &detail, nil
}

// ToggleUserActive toggles the is_active flag for a user and returns the new value along with the user's app_id.
func (r *Repository) ToggleUserActive(id string) (isActive bool, appID string, err error) {
	var user models.User
	if err := r.DB.Select("id, is_active, app_id").First(&user, "id = ?", id).Error; err != nil {
		return false, "", err
	}

	newActive := !user.IsActive
	if err := r.DB.Model(&user).Update("is_active", newActive).Error; err != nil {
		return false, "", err
	}

	return newActive, user.AppID.String(), nil
}

// CountUsersByStatus returns the count of active and inactive users.
func (r *Repository) CountUsersByStatus() (*UserStatusCounts, error) {
	var counts UserStatusCounts

	if err := r.DB.Model(&models.User{}).Where("is_active = ?", true).Count(&counts.ActiveUsers).Error; err != nil {
		return nil, err
	}
	if err := r.DB.Model(&models.User{}).Where("is_active = ?", false).Count(&counts.InactiveUsers).Error; err != nil {
		return nil, err
	}

	return &counts, nil
}

// ============================================================
// Activity Log Operations (Admin GUI - read only)
// ============================================================

// ActivityLogListItem represents an activity log row in the admin GUI list view.
type ActivityLogListItem struct {
	ID        uuid.UUID `json:"id"`
	AppID     uuid.UUID `json:"app_id"`
	AppName   string    `json:"app_name"`
	UserID    uuid.UUID `json:"user_id"`
	UserEmail string    `json:"user_email"`
	EventType string    `json:"event_type"`
	Severity  string    `json:"severity"`
	IPAddress string    `json:"ip_address"`
	IsAnomaly bool      `json:"is_anomaly"`
	Timestamp time.Time `json:"timestamp"`
}

// ActivityLogDetail represents a full activity log view for the admin GUI detail panel.
type ActivityLogDetail struct {
	ID        uuid.UUID  `json:"id"`
	AppID     uuid.UUID  `json:"app_id"`
	AppName   string     `json:"app_name"`
	UserID    uuid.UUID  `json:"user_id"`
	UserEmail string     `json:"user_email"`
	EventType string     `json:"event_type"`
	Severity  string     `json:"severity"`
	IPAddress string     `json:"ip_address"`
	UserAgent string     `json:"user_agent"`
	Details   string     `json:"details"`
	IsAnomaly bool       `json:"is_anomaly"`
	ExpiresAt *time.Time `json:"expires_at"`
	Timestamp time.Time  `json:"timestamp"`
}

// ListActivityLogs returns a paginated list of activity logs with user email and app name.
// Supports optional filtering by eventType, severity, appID, date range, and text search on user email.
func (r *Repository) ListActivityLogs(page, pageSize int, eventType, severity, appID, search, startDate, endDate string) ([]ActivityLogListItem, int64, error) {
	var items []ActivityLogListItem
	var total int64

	// Build base conditions for reuse in both count and data queries
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Joins("LEFT JOIN users ON users.id = activity_logs.user_id::uuid").
			Joins("LEFT JOIN applications ON applications.id = activity_logs.app_id::uuid")
		if eventType != "" {
			q = q.Where("activity_logs.event_type = ?", eventType)
		}
		if severity != "" {
			q = q.Where("activity_logs.severity = ?", severity)
		}
		if appID != "" {
			q = q.Where("activity_logs.app_id = ?", appID)
		}
		if search != "" {
			q = q.Where("users.email ILIKE ?", "%"+search+"%")
		}
		if startDate != "" {
			q = q.Where("activity_logs.timestamp >= ?", startDate)
		}
		if endDate != "" {
			q = q.Where("activity_logs.timestamp <= ?", endDate+" 23:59:59")
		}
		return q
	}

	// Count total matching records (separate query)
	countQuery := applyFilters(r.DB.Model(&models.ActivityLog{}))
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated results
	dataQuery := applyFilters(r.DB.Model(&models.ActivityLog{}).
		Select(`activity_logs.id, activity_logs.app_id,
			COALESCE(applications.name, '') as app_name,
			activity_logs.user_id,
			COALESCE(users.email, '') as user_email,
			activity_logs.event_type, activity_logs.severity,
			activity_logs.ip_address, activity_logs.is_anomaly,
			activity_logs.timestamp`))

	offset := (page - 1) * pageSize
	if err := dataQuery.Order("activity_logs.timestamp desc").Offset(offset).Limit(pageSize).Scan(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetActivityLogDetail returns a full activity log detail view with user email and app name.
func (r *Repository) GetActivityLogDetail(id string) (*ActivityLogDetail, error) {
	var detail ActivityLogDetail

	err := r.DB.Model(&models.ActivityLog{}).
		Select(`activity_logs.id, activity_logs.app_id,
			COALESCE(applications.name, '') as app_name,
			activity_logs.user_id,
			COALESCE(users.email, '') as user_email,
			activity_logs.event_type, activity_logs.severity,
			activity_logs.ip_address, activity_logs.user_agent,
			COALESCE(activity_logs.details::text, '') as details,
			activity_logs.is_anomaly, activity_logs.expires_at,
			activity_logs.timestamp`).
		Joins("LEFT JOIN users ON users.id = activity_logs.user_id::uuid").
		Joins("LEFT JOIN applications ON applications.id = activity_logs.app_id::uuid").
		Where("activity_logs.id = ?", id).
		Scan(&detail).Error
	if err != nil {
		return nil, err
	}

	// Check if log was found (GORM Scan doesn't return ErrRecordNotFound)
	if detail.ID == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}

	return &detail, nil
}

// ListDistinctEventTypes returns all distinct event types currently in the activity_logs table.
func (r *Repository) ListDistinctEventTypes() ([]string, error) {
	var eventTypes []string
	err := r.DB.Model(&models.ActivityLog{}).
		Distinct("event_type").
		Order("event_type asc").
		Pluck("event_type", &eventTypes).Error
	if err != nil {
		return nil, err
	}
	return eventTypes, nil
}

// ListDistinctSeverities returns all distinct severity levels currently in the activity_logs table.
func (r *Repository) ListDistinctSeverities() ([]string, error) {
	var severities []string
	err := r.DB.Model(&models.ActivityLog{}).
		Distinct("severity").
		Order("severity asc").
		Pluck("severity", &severities).Error
	if err != nil {
		return nil, err
	}
	return severities, nil
}

// ============================================================
// API Key Operations (Admin GUI - full CRUD)
// ============================================================

// ApiKeyListItem represents an API key row in the admin GUI list view.
type ApiKeyListItem struct {
	ID         uuid.UUID  `json:"id"`
	KeyType    string     `json:"key_type"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	KeySuffix  string     `json:"key_suffix"`
	AppID      *uuid.UUID `json:"app_id"`
	AppName    string     `json:"app_name"`
	TenantName string     `json:"tenant_name"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	IsRevoked  bool       `json:"is_revoked"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateApiKey inserts a new API key record.
func (r *Repository) CreateApiKey(apiKey *models.ApiKey) error {
	return r.DB.Create(apiKey).Error
}

// ListApiKeys returns a paginated list of API keys with optional type filter.
func (r *Repository) ListApiKeys(page, pageSize int, keyType string) ([]ApiKeyListItem, int64, error) {
	var items []ApiKeyListItem
	var total int64

	// Build base conditions for reuse in both count and data queries
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Joins("LEFT JOIN applications ON applications.id = api_keys.app_id").
			Joins("LEFT JOIN tenants ON tenants.id = applications.tenant_id")
		if keyType != "" {
			q = q.Where("api_keys.key_type = ?", keyType)
		}
		return q
	}

	// Count total matching records (separate query)
	countQuery := applyFilters(r.DB.Model(&models.ApiKey{}))
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated results
	dataQuery := applyFilters(r.DB.Model(&models.ApiKey{}).
		Select(`api_keys.id, api_keys.key_type, api_keys.name,
			api_keys.key_prefix, api_keys.key_suffix,
			api_keys.app_id, api_keys.expires_at,
			api_keys.last_used_at, api_keys.is_revoked,
			api_keys.created_at,
			COALESCE(applications.name, '') as app_name,
			COALESCE(tenants.name, '') as tenant_name`))

	offset := (page - 1) * pageSize
	if err := dataQuery.Order("api_keys.created_at desc").Offset(offset).Limit(pageSize).Scan(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetApiKeyByID returns a single API key by ID.
func (r *Repository) GetApiKeyByID(id string) (*models.ApiKey, error) {
	var apiKey models.ApiKey
	if err := r.DB.First(&apiKey, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &apiKey, nil
}

// RevokeApiKey sets the is_revoked flag to true for an API key.
func (r *Repository) RevokeApiKey(id string) error {
	return r.DB.Model(&models.ApiKey{}).Where("id = ?", id).Update("is_revoked", true).Error
}

// DeleteApiKey permanently deletes an API key by ID.
func (r *Repository) DeleteApiKey(id string) error {
	return r.DB.Where("id = ?", id).Delete(&models.ApiKey{}).Error
}

// FindActiveKeyByHash looks up an active (non-revoked, non-expired) API key by its SHA-256 hash.
// Returns nil, nil if no matching key is found.
func (r *Repository) FindActiveKeyByHash(keyHash string) (*models.ApiKey, error) {
	var apiKey models.ApiKey
	query := r.DB.Where("key_hash = ? AND is_revoked = ?", keyHash, false)

	if err := query.First(&apiKey).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	// Check expiration
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, nil // Expired
	}

	return &apiKey, nil
}

// UpdateApiKeyLastUsed sets the last_used_at timestamp to now.
func (r *Repository) UpdateApiKeyLastUsed(id uuid.UUID) {
	// Fire-and-forget update; errors are non-critical
	r.DB.Model(&models.ApiKey{}).Where("id = ?", id).Update("last_used_at", time.Now())
}
