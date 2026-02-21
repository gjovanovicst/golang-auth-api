package admin

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gjovanovicst/auth_api/internal/database"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/spf13/viper"
)

// SettingType defines the expected value type for a setting.
type SettingType string

const (
	SettingTypeString   SettingType = "string"
	SettingTypeInt      SettingType = "int"
	SettingTypeBool     SettingType = "bool"
	SettingTypeFloat    SettingType = "float"
	SettingTypeDuration SettingType = "duration"
)

// SettingSource indicates where a resolved setting value came from.
type SettingSource string

const (
	SourceEnv     SettingSource = "env"
	SourceDB      SettingSource = "db"
	SourceDefault SettingSource = "default"
)

// SettingDefinition describes a known system setting.
type SettingDefinition struct {
	Key             string // Setting key (matches env var name)
	EnvVar          string // Environment variable name (usually same as Key)
	Category        string // Category slug for grouping
	Type            SettingType
	DefaultValue    string // Default as string
	Label           string // Human-readable label
	Description     string // Help text
	Sensitive       bool   // If true, value is masked in display
	RequiresRestart bool   // If true, changes need app restart
}

// ResolvedSetting holds a setting definition with its resolved value and source.
type ResolvedSetting struct {
	Definition SettingDefinition
	Value      string        // Resolved value (masked if sensitive + env source)
	RawValue   string        // Actual value (for editing; empty if sensitive + env)
	Source     SettingSource // Where the value came from
	DBValue    *string       // Value stored in DB (nil if not in DB)
}

// SettingsCategory groups resolved settings under a category.
type SettingsCategory struct {
	Slug     string // URL-safe identifier
	Label    string // Human-readable name
	Icon     string // Bootstrap icon class
	Settings []ResolvedSetting
}

// SystemInfo holds read-only system information for display.
type SystemInfo struct {
	GoVersion   string
	GOOS        string
	GOARCH      string
	NumCPU      int
	DBHost      string
	DBPort      string
	DBName      string
	DBStatus    string // "Connected" or error message
	RedisAddr   string
	RedisStatus string // "Connected" or error message
	Uptime      string
	StartTime   time.Time
	ServerPort  string
	GinMode     string
}

// SettingsService handles settings resolution and management.
type SettingsService struct {
	repo      *SettingsRepository
	startTime time.Time
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(repo *SettingsRepository) *SettingsService {
	return &SettingsService{
		repo:      repo,
		startTime: time.Now(),
	}
}

// categoryMeta defines the display order and metadata for categories.
var categoryMeta = []struct {
	Slug  string
	Label string
	Icon  string
}{
	{"general", "General", "bi-gear"},
	{"jwt", "JWT & Tokens", "bi-shield-lock"},
	{"admin", "Admin Session", "bi-person-lock"},
	{"email", "Email / SMTP", "bi-envelope"},
	{"log_retention", "Log Retention", "bi-archive"},
	{"log_cleanup", "Log Cleanup", "bi-trash"},
	{"log_behavior", "Log Behavior", "bi-toggles"},
	{"oauth_redirect", "OAuth Redirects", "bi-box-arrow-up-right"},
}

// settingsRegistry is the single source of truth for all known settings.
var settingsRegistry = []SettingDefinition{
	// --- General ---
	{Key: "APP_NAME", EnvVar: "APP_NAME", Category: "general", Type: SettingTypeString, DefaultValue: "Auth API", Label: "Application Name", Description: "Display name used in emails, TOTP, and UI.", Sensitive: false, RequiresRestart: false},
	{Key: "PORT", EnvVar: "PORT", Category: "general", Type: SettingTypeInt, DefaultValue: "8080", Label: "Server Port", Description: "HTTP port the server listens on.", Sensitive: false, RequiresRestart: true},
	{Key: "GIN_MODE", EnvVar: "GIN_MODE", Category: "general", Type: SettingTypeString, DefaultValue: "debug", Label: "Gin Mode", Description: "Gin framework mode: debug, release, or test.", Sensitive: false, RequiresRestart: true},
	{Key: "FRONTEND_URL", EnvVar: "FRONTEND_URL", Category: "general", Type: SettingTypeString, DefaultValue: "", Label: "Frontend URL", Description: "Frontend application URL for CORS and redirects.", Sensitive: false, RequiresRestart: true},

	// --- JWT & Tokens ---
	{Key: "JWT_SECRET", EnvVar: "JWT_SECRET", Category: "jwt", Type: SettingTypeString, DefaultValue: "", Label: "JWT Secret", Description: "Secret key used to sign JWT tokens. Keep this secure.", Sensitive: true, RequiresRestart: true},
	{Key: "ACCESS_TOKEN_EXPIRATION_MINUTES", EnvVar: "ACCESS_TOKEN_EXPIRATION_MINUTES", Category: "jwt", Type: SettingTypeInt, DefaultValue: "15", Label: "Access Token Expiration (minutes)", Description: "How long access tokens remain valid.", Sensitive: false, RequiresRestart: false},
	{Key: "REFRESH_TOKEN_EXPIRATION_HOURS", EnvVar: "REFRESH_TOKEN_EXPIRATION_HOURS", Category: "jwt", Type: SettingTypeInt, DefaultValue: "720", Label: "Refresh Token Expiration (hours)", Description: "How long refresh tokens remain valid (720 = 30 days).", Sensitive: false, RequiresRestart: false},

	// --- Admin Session ---
	{Key: "ADMIN_SESSION_EXPIRATION_HOURS", EnvVar: "ADMIN_SESSION_EXPIRATION_HOURS", Category: "admin", Type: SettingTypeInt, DefaultValue: "8", Label: "Session Expiration (hours)", Description: "How long admin GUI sessions remain active.", Sensitive: false, RequiresRestart: false},

	// --- Email / SMTP ---
	{Key: "EMAIL_HOST", EnvVar: "EMAIL_HOST", Category: "email", Type: SettingTypeString, DefaultValue: "", Label: "SMTP Host", Description: "SMTP server hostname (e.g., smtp.gmail.com).", Sensitive: false, RequiresRestart: false},
	{Key: "EMAIL_PORT", EnvVar: "EMAIL_PORT", Category: "email", Type: SettingTypeInt, DefaultValue: "587", Label: "SMTP Port", Description: "SMTP server port (587 for TLS, 465 for SSL).", Sensitive: false, RequiresRestart: false},
	{Key: "EMAIL_USERNAME", EnvVar: "EMAIL_USERNAME", Category: "email", Type: SettingTypeString, DefaultValue: "", Label: "SMTP Username", Description: "Username for SMTP authentication.", Sensitive: false, RequiresRestart: false},
	{Key: "EMAIL_PASSWORD", EnvVar: "EMAIL_PASSWORD", Category: "email", Type: SettingTypeString, DefaultValue: "", Label: "SMTP Password", Description: "Password for SMTP authentication.", Sensitive: true, RequiresRestart: false},
	{Key: "EMAIL_FROM", EnvVar: "EMAIL_FROM", Category: "email", Type: SettingTypeString, DefaultValue: "", Label: "From Address", Description: "Email address shown as the sender.", Sensitive: false, RequiresRestart: false},

	// --- Log Retention ---
	{Key: "LOG_RETENTION_CRITICAL", EnvVar: "LOG_RETENTION_CRITICAL", Category: "log_retention", Type: SettingTypeInt, DefaultValue: "365", Label: "Critical Events (days)", Description: "Retention period for critical events (login, password change, etc.).", Sensitive: false, RequiresRestart: false},
	{Key: "LOG_RETENTION_IMPORTANT", EnvVar: "LOG_RETENTION_IMPORTANT", Category: "log_retention", Type: SettingTypeInt, DefaultValue: "180", Label: "Important Events (days)", Description: "Retention period for important events (email verify, social login, etc.).", Sensitive: false, RequiresRestart: false},
	{Key: "LOG_RETENTION_INFORMATIONAL", EnvVar: "LOG_RETENTION_INFORMATIONAL", Category: "log_retention", Type: SettingTypeInt, DefaultValue: "90", Label: "Informational Events (days)", Description: "Retention period for routine events (token refresh, profile access).", Sensitive: false, RequiresRestart: false},

	// --- Log Cleanup ---
	{Key: "LOG_CLEANUP_ENABLED", EnvVar: "LOG_CLEANUP_ENABLED", Category: "log_cleanup", Type: SettingTypeBool, DefaultValue: "true", Label: "Cleanup Enabled", Description: "Enable automatic cleanup of expired activity logs.", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_CLEANUP_INTERVAL", EnvVar: "LOG_CLEANUP_INTERVAL", Category: "log_cleanup", Type: SettingTypeDuration, DefaultValue: "24h", Label: "Cleanup Interval", Description: "How often the cleanup job runs (e.g., 24h, 12h, 1h).", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_CLEANUP_BATCH_SIZE", EnvVar: "LOG_CLEANUP_BATCH_SIZE", Category: "log_cleanup", Type: SettingTypeInt, DefaultValue: "1000", Label: "Cleanup Batch Size", Description: "Number of records deleted per cleanup batch.", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_ARCHIVE_BEFORE_CLEANUP", EnvVar: "LOG_ARCHIVE_BEFORE_CLEANUP", Category: "log_cleanup", Type: SettingTypeBool, DefaultValue: "false", Label: "Archive Before Cleanup", Description: "Archive logs to a file before deleting them.", Sensitive: false, RequiresRestart: true},

	// --- Log Behavior ---
	{Key: "LOG_TOKEN_REFRESH", EnvVar: "LOG_TOKEN_REFRESH", Category: "log_behavior", Type: SettingTypeBool, DefaultValue: "false", Label: "Log Token Refresh", Description: "Enable logging of token refresh events (high frequency).", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_PROFILE_ACCESS", EnvVar: "LOG_PROFILE_ACCESS", Category: "log_behavior", Type: SettingTypeBool, DefaultValue: "false", Label: "Log Profile Access", Description: "Enable logging of profile access events (high frequency).", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_SAMPLE_TOKEN_REFRESH", EnvVar: "LOG_SAMPLE_TOKEN_REFRESH", Category: "log_behavior", Type: SettingTypeFloat, DefaultValue: "0.01", Label: "Token Refresh Sample Rate", Description: "Sampling rate for token refresh events (0.0 to 1.0, where 1.0 = 100%).", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_SAMPLE_PROFILE_ACCESS", EnvVar: "LOG_SAMPLE_PROFILE_ACCESS", Category: "log_behavior", Type: SettingTypeFloat, DefaultValue: "0.01", Label: "Profile Access Sample Rate", Description: "Sampling rate for profile access events (0.0 to 1.0, where 1.0 = 100%).", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_DISABLED_EVENTS", EnvVar: "LOG_DISABLED_EVENTS", Category: "log_behavior", Type: SettingTypeString, DefaultValue: "", Label: "Disabled Events", Description: "Comma-separated list of event types to disable (e.g., TOKEN_REFRESH,PROFILE_ACCESS).", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_ANOMALY_DETECTION_ENABLED", EnvVar: "LOG_ANOMALY_DETECTION_ENABLED", Category: "log_behavior", Type: SettingTypeBool, DefaultValue: "true", Label: "Anomaly Detection", Description: "Enable anomaly-based conditional logging.", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_ANOMALY_NEW_IP", EnvVar: "LOG_ANOMALY_NEW_IP", Category: "log_behavior", Type: SettingTypeBool, DefaultValue: "true", Label: "Log New IP", Description: "Log when a user connects from a new IP address.", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_ANOMALY_NEW_USER_AGENT", EnvVar: "LOG_ANOMALY_NEW_USER_AGENT", Category: "log_behavior", Type: SettingTypeBool, DefaultValue: "true", Label: "Log New User Agent", Description: "Log when a user connects from a new browser/device.", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_ANOMALY_GEO_CHANGE", EnvVar: "LOG_ANOMALY_GEO_CHANGE", Category: "log_behavior", Type: SettingTypeBool, DefaultValue: "false", Label: "Log Geo Change", Description: "Log when a user connects from a new geographic location (requires GeoIP).", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_ANOMALY_UNUSUAL_TIME", EnvVar: "LOG_ANOMALY_UNUSUAL_TIME", Category: "log_behavior", Type: SettingTypeBool, DefaultValue: "false", Label: "Log Unusual Time", Description: "Log when a user accesses at an unusual time of day.", Sensitive: false, RequiresRestart: true},
	{Key: "LOG_ANOMALY_SESSION_WINDOW", EnvVar: "LOG_ANOMALY_SESSION_WINDOW", Category: "log_behavior", Type: SettingTypeDuration, DefaultValue: "720h", Label: "Anomaly Session Window", Description: "How long to remember user patterns for anomaly detection (e.g., 720h = 30 days).", Sensitive: false, RequiresRestart: true},

	// --- OAuth Redirects ---
	{Key: "ALLOWED_REDIRECT_DOMAINS", EnvVar: "ALLOWED_REDIRECT_DOMAINS", Category: "oauth_redirect", Type: SettingTypeString, DefaultValue: "", Label: "Allowed Redirect Domains", Description: "Comma-separated list of domains allowed for OAuth redirect URIs.", Sensitive: false, RequiresRestart: false},
	{Key: "DEFAULT_REDIRECT_URI", EnvVar: "DEFAULT_REDIRECT_URI", Category: "oauth_redirect", Type: SettingTypeString, DefaultValue: "", Label: "Default Redirect URI", Description: "Default URI to redirect to after OAuth authentication.", Sensitive: false, RequiresRestart: false},
}

// GetSettingDefinition returns the definition for a given key, or nil if not found.
func GetSettingDefinition(key string) *SettingDefinition {
	for i := range settingsRegistry {
		if settingsRegistry[i].Key == key {
			return &settingsRegistry[i]
		}
	}
	return nil
}

// GetSystemInfo returns read-only system information.
func (s *SettingsService) GetSystemInfo() SystemInfo {
	info := SystemInfo{
		GoVersion:  runtime.Version(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		NumCPU:     runtime.NumCPU(),
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBName:     os.Getenv("DB_NAME"),
		RedisAddr:  viper.GetString("REDIS_ADDR"),
		StartTime:  s.startTime,
		ServerPort: viper.GetString("PORT"),
		GinMode:    viper.GetString("GIN_MODE"),
	}

	// Calculate uptime
	uptime := time.Since(s.startTime)
	info.Uptime = formatUptime(uptime)

	// Check DB connection
	if database.DB != nil {
		sqlDB, err := database.DB.DB()
		if err == nil {
			if err := sqlDB.Ping(); err == nil {
				info.DBStatus = "Connected"
			} else {
				info.DBStatus = "Error: " + err.Error()
			}
		} else {
			info.DBStatus = "Error: " + err.Error()
		}
	} else {
		info.DBStatus = "Not initialized"
	}

	// Check Redis connection
	if redis.Rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := redis.Rdb.Ping(ctx).Err(); err == nil {
			info.RedisStatus = "Connected"
		} else {
			info.RedisStatus = "Error: " + err.Error()
		}
	} else {
		info.RedisStatus = "Not initialized"
	}

	return info
}

// ResolveAllByCategory returns all settings grouped by category with resolved values.
func (s *SettingsService) ResolveAllByCategory() ([]SettingsCategory, error) {
	// Load all DB settings into a map for fast lookup
	dbSettings, err := s.repo.GetAllSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings from database: %w", err)
	}
	dbMap := make(map[string]*models.SystemSetting, len(dbSettings))
	for i := range dbSettings {
		dbMap[dbSettings[i].Key] = &dbSettings[i]
	}

	// Build categories in display order
	catSettingsMap := make(map[string][]ResolvedSetting)
	for _, def := range settingsRegistry {
		resolved := s.resolveSetting(def, dbMap[def.Key])
		catSettingsMap[def.Category] = append(catSettingsMap[def.Category], resolved)
	}

	var categories []SettingsCategory
	for _, meta := range categoryMeta {
		settings, exists := catSettingsMap[meta.Slug]
		if !exists {
			continue
		}
		categories = append(categories, SettingsCategory{
			Slug:     meta.Slug,
			Label:    meta.Label,
			Icon:     meta.Icon,
			Settings: settings,
		})
	}

	return categories, nil
}

// ResolveCategorySettings returns resolved settings for a single category.
func (s *SettingsService) ResolveCategorySettings(categorySlug string) (*SettingsCategory, error) {
	// Find category metadata
	var meta *struct {
		Slug  string
		Label string
		Icon  string
	}
	for i := range categoryMeta {
		if categoryMeta[i].Slug == categorySlug {
			meta = &categoryMeta[i]
			break
		}
	}
	if meta == nil {
		return nil, fmt.Errorf("unknown category: %s", categorySlug)
	}

	// Get all DB settings for this category
	dbSettings, err := s.repo.GetSettingsByCategory(categorySlug)
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}
	dbMap := make(map[string]*models.SystemSetting, len(dbSettings))
	for i := range dbSettings {
		dbMap[dbSettings[i].Key] = &dbSettings[i]
	}

	var settings []ResolvedSetting
	for _, def := range settingsRegistry {
		if def.Category != categorySlug {
			continue
		}
		settings = append(settings, s.resolveSetting(def, dbMap[def.Key]))
	}

	return &SettingsCategory{
		Slug:     meta.Slug,
		Label:    meta.Label,
		Icon:     meta.Icon,
		Settings: settings,
	}, nil
}

// resolveSetting resolves a single setting using priority: env > db > default.
func (s *SettingsService) resolveSetting(def SettingDefinition, dbSetting *models.SystemSetting) ResolvedSetting {
	resolved := ResolvedSetting{
		Definition: def,
	}

	// Check DB value
	if dbSetting != nil {
		val := dbSetting.Value
		resolved.DBValue = &val
	}

	// Resolve: env > db > default
	envVal := getEnvValue(def.EnvVar)
	if envVal != "" {
		resolved.Value = envVal
		resolved.RawValue = envVal
		resolved.Source = SourceEnv
		// For sensitive env-sourced values, mask the raw value
		if def.Sensitive {
			resolved.RawValue = ""
		}
	} else if dbSetting != nil {
		resolved.Value = dbSetting.Value
		resolved.RawValue = dbSetting.Value
		resolved.Source = SourceDB
	} else {
		resolved.Value = def.DefaultValue
		resolved.RawValue = def.DefaultValue
		resolved.Source = SourceDefault
	}

	return resolved
}

// UpdateSetting validates and persists a setting value.
func (s *SettingsService) UpdateSetting(key, value string) error {
	def := GetSettingDefinition(key)
	if def == nil {
		return fmt.Errorf("unknown setting key: %s", key)
	}

	// Validate value against type
	if err := validateSettingValue(def.Type, value); err != nil {
		return fmt.Errorf("invalid value for %s: %w", key, err)
	}

	return s.repo.UpsertSetting(key, value, def.Category)
}

// ResetSetting removes the DB override for a setting, reverting to env var or default.
func (s *SettingsService) ResetSetting(key string) error {
	def := GetSettingDefinition(key)
	if def == nil {
		return fmt.Errorf("unknown setting key: %s", key)
	}
	return s.repo.DeleteSetting(key)
}

// validateSettingValue checks if a value is valid for the given type.
func validateSettingValue(settingType SettingType, value string) error {
	switch settingType {
	case SettingTypeInt:
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("must be a valid integer")
		}
	case SettingTypeBool:
		lower := strings.ToLower(value)
		if lower != "true" && lower != "false" {
			return fmt.Errorf("must be true or false")
		}
	case SettingTypeFloat:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("must be a valid number")
		}
	case SettingTypeDuration:
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf("must be a valid duration (e.g., 24h, 30m, 1h30m)")
		}
	case SettingTypeString:
		// Any string is valid
	}
	return nil
}

// getEnvValue reads an environment variable value.
// Returns empty string if not set. Uses os.Getenv directly because
// some settings (LOG_*) use os.Getenv rather than Viper.
func getEnvValue(envVar string) string {
	return os.Getenv(envVar)
}

// formatUptime formats a duration into a human-readable uptime string.
func formatUptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	parts = append(parts, fmt.Sprintf("%dm", minutes))

	return strings.Join(parts, " ")
}
