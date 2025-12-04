package log

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gjovanovicst/auth_api/internal/database"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AnomalyDetector checks for unusual user behavior patterns
type AnomalyDetector struct {
	db *gorm.DB
}

// NewAnomalyDetector creates a new anomaly detector instance
func NewAnomalyDetector(db *gorm.DB) *AnomalyDetector {
	return &AnomalyDetector{
		db: db,
	}
}

// UserContext represents the current context of a user action
type UserContext struct {
	UserID    uuid.UUID
	IPAddress string
	UserAgent string
	Timestamp time.Time
}

// AnomalyResult indicates if an anomaly was detected and why
type AnomalyResult struct {
	IsAnomaly bool
	Reasons   []string
	ShouldLog bool
}

// DetectAnomaly checks if the current user context represents an anomaly
// based on the user's historical activity patterns
func (ad *AnomalyDetector) DetectAnomaly(ctx UserContext, config AnomalyConfig) AnomalyResult {
	result := AnomalyResult{
		IsAnomaly: false,
		Reasons:   []string{},
		ShouldLog: false,
	}

	if !config.Enabled {
		return result
	}

	// Get user's recent activity patterns
	patterns, err := ad.getUserPatterns(ctx.UserID, config.SessionWindow)
	if err != nil {
		// On error, default to logging (fail-safe approach)
		result.ShouldLog = true
		return result
	}

	// If no historical data exists, this is inherently anomalous (first access)
	if patterns.IsFirstAccess {
		result.IsAnomaly = true
		result.Reasons = append(result.Reasons, "first_access")
		result.ShouldLog = true
		return result
	}

	// Check for new IP address
	if config.LogOnNewIP && !patterns.HasSeenIP(ctx.IPAddress) {
		result.IsAnomaly = true
		result.Reasons = append(result.Reasons, "new_ip_address")
		result.ShouldLog = true
	}

	// Check for new user agent (device/browser change)
	if config.LogOnNewUserAgent && !patterns.HasSeenUserAgent(ctx.UserAgent) {
		result.IsAnomaly = true
		result.Reasons = append(result.Reasons, "new_user_agent")
		result.ShouldLog = true
	}

	// Check for unusual time access (if enabled)
	if config.LogOnUnusualTimeAccess && ad.isUnusualTime(ctx.Timestamp, patterns) {
		result.IsAnomaly = true
		result.Reasons = append(result.Reasons, "unusual_time_access")
		result.ShouldLog = true
	}

	return result
}

// AnomalyConfig holds configuration for anomaly detection
type AnomalyConfig struct {
	Enabled                bool
	LogOnNewIP             bool
	LogOnNewUserAgent      bool
	LogOnUnusualTimeAccess bool
	SessionWindow          time.Duration
}

// UserPatterns holds the user's normal activity patterns
type UserPatterns struct {
	IsFirstAccess      bool
	KnownIPHashes      map[string]bool
	KnownUserAgentHash map[string]bool
	TypicalAccessHours []int // Hour of day (0-23)
	LastActivityTime   time.Time
}

// HasSeenIP checks if the user has used this IP before
func (up *UserPatterns) HasSeenIP(ipAddress string) bool {
	hash := hashString(ipAddress)
	return up.KnownIPHashes[hash]
}

// HasSeenUserAgent checks if the user has used this user agent before
func (up *UserPatterns) HasSeenUserAgent(userAgent string) bool {
	hash := hashString(userAgent)
	return up.KnownUserAgentHash[hash]
}

// getUserPatterns retrieves and analyzes the user's recent activity patterns
func (ad *AnomalyDetector) getUserPatterns(userID uuid.UUID, window time.Duration) (UserPatterns, error) {
	patterns := UserPatterns{
		IsFirstAccess:      false,
		KnownIPHashes:      make(map[string]bool),
		KnownUserAgentHash: make(map[string]bool),
		TypicalAccessHours: []int{},
	}

	// Query recent activity logs
	var logs []models.ActivityLog
	cutoffTime := time.Now().UTC().Add(-window)

	err := ad.db.Where("user_id = ? AND timestamp >= ?", userID, cutoffTime).
		Order("timestamp DESC").
		Limit(100). // Limit to last 100 entries for performance
		Find(&logs).Error

	if err != nil {
		return patterns, err
	}

	// If no logs found, this is first access
	if len(logs) == 0 {
		patterns.IsFirstAccess = true
		return patterns, nil
	}

	// Build pattern maps
	hourCounts := make(map[int]int)

	for _, log := range logs {
		// Track IP addresses (hashed for privacy)
		if log.IPAddress != "" {
			patterns.KnownIPHashes[hashString(log.IPAddress)] = true
		}

		// Track user agents (hashed for privacy)
		if log.UserAgent != "" {
			patterns.KnownUserAgentHash[hashString(log.UserAgent)] = true
		}

		// Track typical access hours
		hour := log.Timestamp.Hour()
		hourCounts[hour]++

		// Track last activity
		if log.Timestamp.After(patterns.LastActivityTime) {
			patterns.LastActivityTime = log.Timestamp
		}
	}

	// Determine typical access hours (hours with activity)
	for hour := range hourCounts {
		patterns.TypicalAccessHours = append(patterns.TypicalAccessHours, hour)
	}

	return patterns, nil
}

// isUnusualTime checks if the access time is unusual for this user
func (ad *AnomalyDetector) isUnusualTime(timestamp time.Time, patterns UserPatterns) bool {
	// If we don't have enough data, don't flag as unusual
	if len(patterns.TypicalAccessHours) == 0 {
		return false
	}

	// Check if current hour is in typical access hours
	currentHour := timestamp.Hour()
	for _, typicalHour := range patterns.TypicalAccessHours {
		// Allow +/- 2 hours window
		if abs(currentHour-typicalHour) <= 2 || abs(currentHour-typicalHour) >= 22 {
			return false
		}
	}

	// If we have very few typical hours and this is outside them, flag it
	if len(patterns.TypicalAccessHours) < 6 {
		return true
	}

	return false
}

// hashString creates a SHA-256 hash of a string for privacy-preserving comparison
func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// GetGlobalAnomalyDetector returns a singleton anomaly detector instance
var globalAnomalyDetector *AnomalyDetector

func GetAnomalyDetector() *AnomalyDetector {
	if globalAnomalyDetector == nil {
		globalAnomalyDetector = NewAnomalyDetector(database.DB)
	}
	return globalAnomalyDetector
}
