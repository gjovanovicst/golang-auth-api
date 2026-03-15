package log

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gjovanovicst/auth_api/internal/geoip"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AnomalyDetector checks for unusual user behavior patterns
type AnomalyDetector struct {
	db    *gorm.DB
	geoIP *geoip.Service // nil when GeoIP is not configured
}

// NewAnomalyDetector creates a new anomaly detector instance.
// geoIPService may be nil if GeoIP lookups are not available.
func NewAnomalyDetector(db *gorm.DB, geoIPService *geoip.Service) *AnomalyDetector {
	return &AnomalyDetector{
		db:    db,
		geoIP: geoIPService,
	}
}

// UserContext represents the current context of a user action
type UserContext struct {
	UserID    uuid.UUID
	AppID     uuid.UUID
	IPAddress string
	UserAgent string
	Timestamp time.Time
}

// NotificationDetails contains the human-readable details included in notification emails.
// These are the raw values (not hashed) so they can be presented to the user.
type NotificationDetails struct {
	IPAddress string // Raw IP address of the login attempt
	Location  string // Human-readable location string (e.g. "San Francisco, United States")
	Device    string // User-Agent string identifying the device/browser
	LoginTime string // Formatted timestamp of the login
	AlertType string // e.g. "new_device", "new_location", "brute_force"
	Details   string // Additional context (e.g. "5 failed attempts in 15 minutes")
}

// AnomalyResult indicates if an anomaly was detected and why
type AnomalyResult struct {
	IsAnomaly           bool
	Reasons             []string
	ShouldLog           bool
	Severity            string               // "low", "medium", "high", "critical"
	NotifyUser          bool                 // Whether to send a notification email
	NotificationType    string               // "new_device_login" or "suspicious_activity"
	NotificationDetails *NotificationDetails // Details for the notification email
}

// DetectAnomaly checks if the current user context represents an anomaly
// based on the user's historical activity patterns
func (ad *AnomalyDetector) DetectAnomaly(ctx UserContext, config AnomalyConfig) AnomalyResult {
	result := AnomalyResult{
		IsAnomaly: false,
		Reasons:   []string{},
		ShouldLog: false,
		Severity:  "low",
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

	// Track whether this is a new device/location for notification purposes
	isNewIP := false
	isNewUA := false
	isNewGeo := false

	// Check for new IP address
	if config.LogOnNewIP && !patterns.HasSeenIP(ctx.IPAddress) {
		result.IsAnomaly = true
		result.Reasons = append(result.Reasons, "new_ip_address")
		result.ShouldLog = true
		result.Severity = raiseSeverity(result.Severity, "medium")
		isNewIP = true
	}

	// Check for new user agent (device/browser change)
	if config.LogOnNewUserAgent && !patterns.HasSeenUserAgent(ctx.UserAgent) {
		result.IsAnomaly = true
		result.Reasons = append(result.Reasons, "new_user_agent")
		result.ShouldLog = true
		result.Severity = raiseSeverity(result.Severity, "medium")
		isNewUA = true
	}

	// Check for geographic location change (requires GeoIP)
	if config.LogOnGeographicChange && ad.geoIP != nil && ad.geoIP.IsAvailable() {
		currentCountry := ad.geoIP.LookupCountry(ctx.IPAddress)
		if currentCountry != "" && !patterns.HasSeenCountry(currentCountry) {
			result.IsAnomaly = true
			result.Reasons = append(result.Reasons, "new_geographic_location")
			result.ShouldLog = true
			result.Severity = raiseSeverity(result.Severity, "high")
			isNewGeo = true
		}
	}

	// Check for unusual time access (if enabled)
	if config.LogOnUnusualTimeAccess && ad.isUnusualTime(ctx.Timestamp, patterns) {
		result.IsAnomaly = true
		result.Reasons = append(result.Reasons, "unusual_time_access")
		result.ShouldLog = true
		result.Severity = raiseSeverity(result.Severity, "medium")
	}

	// Determine if we should notify the user about this anomaly
	if result.IsAnomaly {
		ad.populateNotification(&result, ctx, config, isNewIP, isNewUA, isNewGeo)
	}

	return result
}

// DetectBruteForce checks Redis for failed login counts and returns an anomaly result
// if the brute-force threshold has been exceeded. This should be called after a failed
// login attempt has been recorded.
func (ad *AnomalyDetector) DetectBruteForce(ctx UserContext, config AnomalyConfig, failedCount int64) AnomalyResult {
	result := AnomalyResult{
		IsAnomaly: false,
		Reasons:   []string{},
		ShouldLog: false,
		Severity:  "low",
	}

	if !config.BruteForceEnabled {
		return result
	}

	if failedCount >= int64(config.BruteForceThreshold) {
		result.IsAnomaly = true
		result.Reasons = append(result.Reasons, "brute_force_detected")
		result.ShouldLog = true
		result.Severity = "critical"

		if config.NotifyOnBruteForce {
			location := ad.resolveLocation(ctx.IPAddress)
			result.NotifyUser = true
			result.NotificationType = "suspicious_activity"
			result.NotificationDetails = &NotificationDetails{
				IPAddress: ctx.IPAddress,
				Location:  location,
				Device:    ctx.UserAgent,
				LoginTime: ctx.Timestamp.UTC().Format(time.RFC3339),
				AlertType: "brute_force",
				Details: fmt.Sprintf("%d failed login attempts detected within %s",
					failedCount, config.BruteForceWindow),
			}
		}
	}

	return result
}

// IncrementAndCheckBruteForce increments the failed login counter in Redis for a user
// and returns the new count. This is a convenience wrapper around the Redis function.
func (ad *AnomalyDetector) IncrementAndCheckBruteForce(appID uuid.UUID, identifier string, window time.Duration) (int64, error) {
	return redis.IncrFailedLogin(appID.String(), identifier, window)
}

// ResetBruteForceCounter resets the failed login counter for a user (call on successful login).
func (ad *AnomalyDetector) ResetBruteForceCounter(appID uuid.UUID, identifier string) error {
	return redis.ResetFailedLogins(appID.String(), identifier)
}

// populateNotification fills in the notification fields on the anomaly result
// based on what type of anomaly was detected and the config settings.
func (ad *AnomalyDetector) populateNotification(result *AnomalyResult, ctx UserContext, config AnomalyConfig, isNewIP, isNewUA, isNewGeo bool) {
	// Determine if we should send a new device/location notification
	shouldNotifyNewDevice := config.NotifyOnNewDevice && (isNewIP || isNewUA)
	shouldNotifyGeoChange := config.NotifyOnGeoChange && isNewGeo

	if !shouldNotifyNewDevice && !shouldNotifyGeoChange {
		return
	}

	location := ad.resolveLocation(ctx.IPAddress)

	result.NotifyUser = true

	// Geographic change is treated as suspicious activity; new device as a login notification
	if shouldNotifyGeoChange {
		result.NotificationType = "suspicious_activity"
		alertDetails := "Login detected from a new geographic location"
		if isNewIP {
			alertDetails += " and a new IP address"
		}
		result.NotificationDetails = &NotificationDetails{
			IPAddress: ctx.IPAddress,
			Location:  location,
			Device:    ctx.UserAgent,
			LoginTime: ctx.Timestamp.UTC().Format(time.RFC3339),
			AlertType: "new_location",
			Details:   alertDetails,
		}
	} else {
		result.NotificationType = "new_device_login"
		result.NotificationDetails = &NotificationDetails{
			IPAddress: ctx.IPAddress,
			Location:  location,
			Device:    ctx.UserAgent,
			LoginTime: ctx.Timestamp.UTC().Format(time.RFC3339),
			AlertType: "new_device",
			Details:   "Login from a new device or browser",
		}
	}
}

// resolveLocation uses the GeoIP service to get a human-readable location for an IP.
func (ad *AnomalyDetector) resolveLocation(ipAddress string) string {
	if ad.geoIP == nil || !ad.geoIP.IsAvailable() {
		return "Unknown location"
	}
	info := ad.geoIP.Lookup(ipAddress)
	if info == nil {
		return "Unknown location"
	}
	return info.String()
}

// AnomalyConfig holds configuration for anomaly detection
type AnomalyConfig struct {
	Enabled                bool
	LogOnNewIP             bool
	LogOnNewUserAgent      bool
	LogOnGeographicChange  bool
	LogOnUnusualTimeAccess bool
	SessionWindow          time.Duration

	// Brute-force detection
	BruteForceEnabled   bool
	BruteForceThreshold int
	BruteForceWindow    time.Duration

	// Notification settings
	NotifyOnBruteForce   bool
	NotifyOnNewDevice    bool
	NotifyOnGeoChange    bool
	NotificationCooldown time.Duration
}

// UserPatterns holds the user's normal activity patterns
type UserPatterns struct {
	IsFirstAccess      bool
	KnownIPHashes      map[string]bool
	KnownUserAgentHash map[string]bool
	KnownCountries     map[string]bool // ISO country codes seen in historical logs
	TypicalAccessHours []int           // Hour of day (0-23)
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

// HasSeenCountry checks if the user has logged in from this country before
func (up *UserPatterns) HasSeenCountry(countryCode string) bool {
	return up.KnownCountries[countryCode]
}

// getUserPatterns retrieves and analyzes the user's recent activity patterns
func (ad *AnomalyDetector) getUserPatterns(userID uuid.UUID, window time.Duration) (UserPatterns, error) {
	patterns := UserPatterns{
		IsFirstAccess:      false,
		KnownIPHashes:      make(map[string]bool),
		KnownUserAgentHash: make(map[string]bool),
		KnownCountries:     make(map[string]bool),
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

	for _, logEntry := range logs {
		// Track IP addresses (hashed for privacy)
		if logEntry.IPAddress != "" {
			patterns.KnownIPHashes[hashString(logEntry.IPAddress)] = true
		}

		// Track user agents (hashed for privacy)
		if logEntry.UserAgent != "" {
			patterns.KnownUserAgentHash[hashString(logEntry.UserAgent)] = true
		}

		// Track typical access hours
		hour := logEntry.Timestamp.Hour()
		hourCounts[hour]++

		// Track last activity
		if logEntry.Timestamp.After(patterns.LastActivityTime) {
			patterns.LastActivityTime = logEntry.Timestamp
		}
	}

	// Extract country information from historical logs using GeoIP
	// We look up countries for all known IPs in the historical data
	if ad.geoIP != nil && ad.geoIP.IsAvailable() {
		// Re-read unique IPs from recent logs to resolve their countries
		var recentLogs []models.ActivityLog
		err := ad.db.Where("user_id = ? AND timestamp >= ?", userID, cutoffTime).
			Select("DISTINCT ip_address").
			Find(&recentLogs).Error
		if err == nil {
			for _, logEntry := range recentLogs {
				if logEntry.IPAddress != "" {
					country := ad.geoIP.LookupCountry(logEntry.IPAddress)
					if country != "" {
						patterns.KnownCountries[country] = true
					}
				}
			}
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

// raiseSeverity returns the higher of two severity levels.
// Order: low < medium < high < critical
func raiseSeverity(current, proposed string) string {
	order := map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}
	if order[proposed] > order[current] {
		return proposed
	}
	return current
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
