package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// EventSeverity represents the criticality level of an event
type EventSeverity string

const (
	SeverityCritical      EventSeverity = "CRITICAL"
	SeverityImportant     EventSeverity = "IMPORTANT"
	SeverityInformational EventSeverity = "INFORMATIONAL"
)

// LoggingConfig holds all configuration for the activity logging system
type LoggingConfig struct {
	// Event severity mappings
	EventSeverities map[string]EventSeverity

	// Enabled events - if false, event won't be logged at all
	EnabledEvents map[string]bool

	// Sampling rates for high-frequency events (0.0 to 1.0)
	// 1.0 = log all, 0.01 = log 1%, 0.0 = log none
	SamplingRates map[string]float64

	// Anomaly detection settings
	AnomalyDetection AnomalyDetectionConfig

	// Retention policies (in days)
	RetentionPolicies map[EventSeverity]int

	// Cleanup settings
	CleanupEnabled       bool
	CleanupInterval      time.Duration
	CleanupBatchSize     int
	ArchiveBeforeCleanup bool
}

// AnomalyDetectionConfig holds settings for anomaly-based conditional logging
type AnomalyDetectionConfig struct {
	Enabled                bool
	LogOnNewIP             bool
	LogOnNewUserAgent      bool
	LogOnGeographicChange  bool
	LogOnUnusualTimeAccess bool
	SessionWindow          time.Duration // How long to remember user's normal patterns
}

var defaultConfig *LoggingConfig

// GetLoggingConfig returns the singleton logging configuration
func GetLoggingConfig() *LoggingConfig {
	if defaultConfig == nil {
		defaultConfig = initializeLoggingConfig()
	}
	return defaultConfig
}

// initializeLoggingConfig creates the default logging configuration
func initializeLoggingConfig() *LoggingConfig {
	config := &LoggingConfig{
		EventSeverities:   initializeEventSeverities(),
		EnabledEvents:     initializeEnabledEvents(),
		SamplingRates:     initializeSamplingRates(),
		AnomalyDetection:  initializeAnomalyDetection(),
		RetentionPolicies: initializeRetentionPolicies(),

		CleanupEnabled:       getEnvBool("LOG_CLEANUP_ENABLED", true),
		CleanupInterval:      getEnvDuration("LOG_CLEANUP_INTERVAL", 24*time.Hour),
		CleanupBatchSize:     getEnvInt("LOG_CLEANUP_BATCH_SIZE", 1000),
		ArchiveBeforeCleanup: getEnvBool("LOG_ARCHIVE_BEFORE_CLEANUP", false),
	}

	return config
}

// initializeEventSeverities maps event types to their severity levels
func initializeEventSeverities() map[string]EventSeverity {
	return map[string]EventSeverity{
		// Critical events - always important for security audits
		"LOGIN":              SeverityCritical,
		"LOGOUT":             SeverityCritical,
		"REGISTER":           SeverityCritical,
		"PASSWORD_CHANGE":    SeverityCritical,
		"PASSWORD_RESET":     SeverityCritical,
		"EMAIL_CHANGE":       SeverityCritical,
		"2FA_ENABLE":         SeverityCritical,
		"2FA_DISABLE":        SeverityCritical,
		"ACCOUNT_DELETION":   SeverityCritical,
		"RECOVERY_CODE_USED": SeverityCritical,

		// Important events - significant but not critical
		"EMAIL_VERIFY":           SeverityImportant,
		"2FA_LOGIN":              SeverityImportant,
		"SOCIAL_LOGIN":           SeverityImportant,
		"PROFILE_UPDATE":         SeverityImportant,
		"RECOVERY_CODE_GENERATE": SeverityImportant,

		// Informational events - routine operations
		"TOKEN_REFRESH":  SeverityInformational,
		"PROFILE_ACCESS": SeverityInformational,
	}
}

// initializeEnabledEvents determines which events are enabled by default
func initializeEnabledEvents() map[string]bool {
	// Check environment variable for disabled events
	disabledEventsStr := os.Getenv("LOG_DISABLED_EVENTS")
	disabledEvents := make(map[string]bool)
	if disabledEventsStr != "" {
		for _, event := range strings.Split(disabledEventsStr, ",") {
			disabledEvents[strings.TrimSpace(event)] = true
		}
	}

	enabled := map[string]bool{
		"LOGIN":                  true,
		"LOGOUT":                 true,
		"REGISTER":               true,
		"PASSWORD_CHANGE":        true,
		"PASSWORD_RESET":         true,
		"EMAIL_VERIFY":           true,
		"EMAIL_CHANGE":           true,
		"2FA_ENABLE":             true,
		"2FA_DISABLE":            true,
		"2FA_LOGIN":              true,
		"TOKEN_REFRESH":          getEnvBool("LOG_TOKEN_REFRESH", false), // Disabled by default
		"SOCIAL_LOGIN":           true,
		"PROFILE_ACCESS":         getEnvBool("LOG_PROFILE_ACCESS", false), // Disabled by default
		"PROFILE_UPDATE":         true,
		"ACCOUNT_DELETION":       true,
		"RECOVERY_CODE_USED":     true,
		"RECOVERY_CODE_GENERATE": true,
	}

	// Apply disabled events from environment
	for event := range disabledEvents {
		enabled[event] = false
	}

	return enabled
}

// initializeSamplingRates sets sampling rates for high-frequency events
func initializeSamplingRates() map[string]float64 {
	return map[string]float64{
		// Only sample token refresh if enabled
		"TOKEN_REFRESH": getEnvFloat("LOG_SAMPLE_TOKEN_REFRESH", 0.01), // 1% by default

		// Sample profile access if enabled
		"PROFILE_ACCESS": getEnvFloat("LOG_SAMPLE_PROFILE_ACCESS", 0.01), // 1% by default

		// All other events are logged at 100% (no sampling)
	}
}

// initializeAnomalyDetection configures anomaly detection settings
func initializeAnomalyDetection() AnomalyDetectionConfig {
	return AnomalyDetectionConfig{
		Enabled:                getEnvBool("LOG_ANOMALY_DETECTION_ENABLED", true),
		LogOnNewIP:             getEnvBool("LOG_ANOMALY_NEW_IP", true),
		LogOnNewUserAgent:      getEnvBool("LOG_ANOMALY_NEW_USER_AGENT", true),
		LogOnGeographicChange:  getEnvBool("LOG_ANOMALY_GEO_CHANGE", false), // Requires GeoIP
		LogOnUnusualTimeAccess: getEnvBool("LOG_ANOMALY_UNUSUAL_TIME", false),
		SessionWindow:          getEnvDuration("LOG_ANOMALY_SESSION_WINDOW", 30*24*time.Hour), // 30 days
	}
}

// initializeRetentionPolicies sets retention periods for different severity levels
func initializeRetentionPolicies() map[EventSeverity]int {
	return map[EventSeverity]int{
		SeverityCritical:      getEnvInt("LOG_RETENTION_CRITICAL", 365),     // 1 year
		SeverityImportant:     getEnvInt("LOG_RETENTION_IMPORTANT", 180),    // 6 months
		SeverityInformational: getEnvInt("LOG_RETENTION_INFORMATIONAL", 90), // 3 months
	}
}

// GetEventSeverity returns the severity level for a given event type
func (c *LoggingConfig) GetEventSeverity(eventType string) EventSeverity {
	if severity, exists := c.EventSeverities[eventType]; exists {
		return severity
	}
	// Default to informational if not specified
	return SeverityInformational
}

// IsEventEnabled checks if an event type should be logged
func (c *LoggingConfig) IsEventEnabled(eventType string) bool {
	if enabled, exists := c.EnabledEvents[eventType]; exists {
		return enabled
	}
	// Default to enabled if not specified
	return true
}

// GetSamplingRate returns the sampling rate for an event type
func (c *LoggingConfig) GetSamplingRate(eventType string) float64 {
	if rate, exists := c.SamplingRates[eventType]; exists {
		return rate
	}
	// Default to 100% (log everything)
	return 1.0
}

// GetRetentionDays returns the retention period in days for a given severity
func (c *LoggingConfig) GetRetentionDays(severity EventSeverity) int {
	if days, exists := c.RetentionPolicies[severity]; exists {
		return days
	}
	// Default to 90 days if not specified
	return 90
}

// Helper functions to read environment variables with defaults

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
