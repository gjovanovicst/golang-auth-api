package log

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/gjovanovicst/auth_api/internal/config"
	"github.com/gjovanovicst/auth_api/internal/database"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

// Event types constants for consistency
const (
	EventLogin            = "LOGIN"
	EventLogout           = "LOGOUT"
	EventRegister         = "REGISTER"
	EventPasswordChange   = "PASSWORD_CHANGE"
	EventPasswordReset    = "PASSWORD_RESET"
	EventEmailVerify      = "EMAIL_VERIFY"
	EventEmailChange      = "EMAIL_CHANGE"
	Event2FAEnable        = "2FA_ENABLE"
	Event2FADisable       = "2FA_DISABLE"
	Event2FALogin         = "2FA_LOGIN"
	EventTokenRefresh     = "TOKEN_REFRESH"
	EventSocialLogin      = "SOCIAL_LOGIN"
	EventProfileAccess    = "PROFILE_ACCESS"
	EventProfileUpdate    = "PROFILE_UPDATE"
	EventAccountDeletion  = "ACCOUNT_DELETION"
	EventRecoveryCodeUsed = "RECOVERY_CODE_USED"
	EventRecoveryCodeGen  = "RECOVERY_CODE_GENERATE"
)

// LogEntry represents a log entry to be processed
type LogEntry struct {
	AppID     uuid.UUID
	UserID    uuid.UUID
	EventType string
	IPAddress string
	UserAgent string
	Details   map[string]interface{}
	Timestamp time.Time
	IsAnomaly bool
}

// Service handles asynchronous activity logging
type Service struct {
	logChannel chan LogEntry
	ctx        context.Context
	cancel     context.CancelFunc
}

var serviceInstance *Service

// InitializeLogService initializes the global log service
func InitializeLogService() *Service {
	if serviceInstance != nil {
		return serviceInstance
	}

	ctx, cancel := context.WithCancel(context.Background())

	serviceInstance = &Service{
		logChannel: make(chan LogEntry, 1000), // Buffer for 1000 log entries
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start the background worker
	go serviceInstance.worker()

	return serviceInstance
}

// GetLogService returns the singleton log service instance
func GetLogService() *Service {
	if serviceInstance == nil {
		return InitializeLogService()
	}
	return serviceInstance
}

// LogActivity logs a user activity asynchronously with smart filtering
func (s *Service) LogActivity(appID, userID uuid.UUID, eventType, ipAddress, userAgent string, details map[string]interface{}) {
	// Get logging configuration
	cfg := config.GetLoggingConfig()

	// Check if event is enabled
	if !cfg.IsEventEnabled(eventType) {
		return
	}

	// Check sampling rate for high-frequency events
	samplingRate := cfg.GetSamplingRate(eventType)
	// #nosec G404 -- Using math/rand for non-cryptographic sampling is acceptable
	if samplingRate < 1.0 && rand.Float64() > samplingRate {
		// Skip this log entry based on sampling
		return
	}

	// For informational events with anomaly detection enabled, check for anomalies
	isAnomaly := false
	if cfg.AnomalyDetection.Enabled {
		severity := cfg.GetEventSeverity(eventType)
		if severity == config.SeverityInformational {
			detector := GetAnomalyDetector()
			ctx := UserContext{
				UserID:    userID,
				IPAddress: ipAddress,
				UserAgent: userAgent,
				Timestamp: time.Now().UTC(),
			}
			anomalyCfg := AnomalyConfig{
				Enabled:                cfg.AnomalyDetection.Enabled,
				LogOnNewIP:             cfg.AnomalyDetection.LogOnNewIP,
				LogOnNewUserAgent:      cfg.AnomalyDetection.LogOnNewUserAgent,
				LogOnUnusualTimeAccess: cfg.AnomalyDetection.LogOnUnusualTimeAccess,
				SessionWindow:          cfg.AnomalyDetection.SessionWindow,
			}
			result := detector.DetectAnomaly(ctx, anomalyCfg)

			// If no anomaly detected and this is informational, skip logging
			if !result.ShouldLog {
				return
			}

			isAnomaly = result.IsAnomaly
			if isAnomaly && details == nil {
				details = make(map[string]interface{})
			}
			if isAnomaly {
				details["anomaly_reasons"] = result.Reasons
			}
		}
	}

	logEntry := LogEntry{
		AppID:     appID,
		UserID:    userID,
		EventType: eventType,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   details,
		Timestamp: time.Now().UTC(),
		IsAnomaly: isAnomaly,
	}

	// Non-blocking send to channel
	select {
	case s.logChannel <- logEntry:
		// Successfully queued
	default:
		// Channel is full, log the error but don't block the main request
		log.Printf("Warning: Activity log channel is full, dropping log entry for user %s, event %s", userID, eventType)
	}
}

// worker processes log entries from the channel
func (s *Service) worker() {
	for {
		select {
		case <-s.ctx.Done():
			// Service is shutting down
			log.Println("Activity log service shutting down...")
			return
		case entry := <-s.logChannel:
			s.processLogEntry(entry)
		}
	}
}

// processLogEntry writes a single log entry to the database with retry logic
func (s *Service) processLogEntry(entry LogEntry) {
	const maxRetries = 3
	const retryDelay = time.Second * 2

	var detailsJSON json.RawMessage
	if entry.Details != nil {
		jsonBytes, err := json.Marshal(entry.Details)
		if err != nil {
			log.Printf("Error marshaling log details for user %s, event %s: %v", entry.UserID, entry.EventType, err)
			// Create empty JSON object if marshaling fails
			detailsJSON = json.RawMessage("{}")
		} else {
			detailsJSON = json.RawMessage(jsonBytes)
		}
	} else {
		detailsJSON = json.RawMessage("{}")
	}

	// Get logging configuration for severity and retention
	cfg := config.GetLoggingConfig()
	severity := cfg.GetEventSeverity(entry.EventType)
	retentionDays := cfg.GetRetentionDays(severity)

	// Calculate expiration time
	expiresAt := entry.Timestamp.AddDate(0, 0, retentionDays)

	activityLog := models.ActivityLog{
		AppID:     entry.AppID,
		UserID:    entry.UserID,
		EventType: entry.EventType,
		Timestamp: entry.Timestamp,
		IPAddress: entry.IPAddress,
		UserAgent: entry.UserAgent,
		Details:   detailsJSON,
		Severity:  string(severity),
		ExpiresAt: &expiresAt,
		IsAnomaly: entry.IsAnomaly,
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := database.DB.Create(&activityLog).Error
		if err == nil {
			// Successfully logged
			return
		}

		lastErr = err
		log.Printf("Attempt %d/%d failed to log activity for user %s, event %s: %v",
			attempt, maxRetries, entry.UserID, entry.EventType, err)

		if attempt < maxRetries {
			// Wait before retry
			time.Sleep(retryDelay * time.Duration(attempt))
		}
	}

	// All retries failed, log the final error
	log.Printf("Failed to log activity after %d attempts for user %s, event %s: %v",
		maxRetries, entry.UserID, entry.EventType, lastErr)

	// In a production environment, you might want to send this to a dead letter queue
	// or persistent error log for manual intervention
}

// Shutdown gracefully shuts down the log service
func (s *Service) Shutdown() {
	if s.cancel != nil {
		s.cancel()
	}

	// Process remaining entries in the channel with a timeout
	timeout := time.After(10 * time.Second)
	for {
		select {
		case entry := <-s.logChannel:
			s.processLogEntry(entry)
		case <-timeout:
			log.Println("Activity log service shutdown timeout reached")
			return
		default:
			// Channel is empty
			log.Println("Activity log service shutdown complete")
			return
		}
	}
}

// Helper functions for common logging scenarios

// LogLogin logs a successful login event
func LogLogin(appID, userID uuid.UUID, ipAddress, userAgent string, details map[string]interface{}) {
	GetLogService().LogActivity(appID, userID, EventLogin, ipAddress, userAgent, details)
}

// LogLogout logs a logout event
func LogLogout(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventLogout, ipAddress, userAgent, nil)
}

// LogRegister logs a user registration event
func LogRegister(appID, userID uuid.UUID, ipAddress, userAgent string, email string) {
	details := map[string]interface{}{
		"email": email,
	}
	GetLogService().LogActivity(appID, userID, EventRegister, ipAddress, userAgent, details)
}

// LogPasswordChange logs a password change event
func LogPasswordChange(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventPasswordChange, ipAddress, userAgent, nil)
}

// LogPasswordReset logs a password reset event
func LogPasswordReset(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventPasswordReset, ipAddress, userAgent, nil)
}

// LogEmailVerify logs an email verification event
func LogEmailVerify(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventEmailVerify, ipAddress, userAgent, nil)
}

// Log2FAEnable logs a 2FA enable event
func Log2FAEnable(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, Event2FAEnable, ipAddress, userAgent, nil)
}

// Log2FADisable logs a 2FA disable event
func Log2FADisable(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, Event2FADisable, ipAddress, userAgent, nil)
}

// Log2FALogin logs a successful 2FA login event
func Log2FALogin(appID, userID uuid.UUID, ipAddress, userAgent string, method string) {
	details := map[string]interface{}{
		"method": method, // "totp" or "recovery_code"
	}
	GetLogService().LogActivity(appID, userID, Event2FALogin, ipAddress, userAgent, details)
}

// LogTokenRefresh logs a token refresh event
func LogTokenRefresh(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventTokenRefresh, ipAddress, userAgent, nil)
}

// LogSocialLogin logs a social login event
func LogSocialLogin(appID, userID uuid.UUID, ipAddress, userAgent string, provider string) {
	details := map[string]interface{}{
		"provider": provider,
	}
	GetLogService().LogActivity(appID, userID, EventSocialLogin, ipAddress, userAgent, details)
}

// LogProfileAccess logs profile access (optional, for high-security environments)
func LogProfileAccess(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventProfileAccess, ipAddress, userAgent, nil)
}

// LogRecoveryCodeUsed logs when a recovery code is used
func LogRecoveryCodeUsed(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventRecoveryCodeUsed, ipAddress, userAgent, nil)
}

// LogRecoveryCodeGenerate logs when new recovery codes are generated
func LogRecoveryCodeGenerate(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventRecoveryCodeGen, ipAddress, userAgent, nil)
}

// LogEmailChange logs an email change event
func LogEmailChange(appID, userID uuid.UUID, ipAddress, userAgent string, details map[string]interface{}) {
	GetLogService().LogActivity(appID, userID, EventEmailChange, ipAddress, userAgent, details)
}

// LogProfileUpdate logs a profile update event
func LogProfileUpdate(appID, userID uuid.UUID, ipAddress, userAgent string, details map[string]interface{}) {
	GetLogService().LogActivity(appID, userID, EventProfileUpdate, ipAddress, userAgent, details)
}

// LogAccountDeletion logs an account deletion event
func LogAccountDeletion(appID, userID uuid.UUID, ipAddress, userAgent string) {
	GetLogService().LogActivity(appID, userID, EventAccountDeletion, ipAddress, userAgent, nil)
}
