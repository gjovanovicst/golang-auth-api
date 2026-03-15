package bruteforce

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BruteForceConfig is the resolved brute-force configuration for a single request.
// It is built by ResolveConfig() which merges per-app overrides (from the Application
// model) with global defaults (from environment variables / config singleton).
type BruteForceConfig struct {
	// Account Lockout
	LockoutEnabled   bool
	LockoutThreshold int
	LockoutDurations []time.Duration
	LockoutWindow    time.Duration
	LockoutTierTTL   time.Duration

	// Progressive Delay
	DelayEnabled    bool
	DelayStartAfter int
	DelayMaxSeconds int
	DelayTierTTL    time.Duration

	// CAPTCHA
	CaptchaEnabled   bool
	CaptchaSiteKey   string
	CaptchaSecretKey string
	CaptchaThreshold int
}

// ResolveConfig builds a BruteForceConfig by merging per-app overrides from the
// Application model with hardcoded defaults. NULL fields on the Application model
// mean "use default". The defaults match the values previously shipped in .env.
func ResolveConfig(app *models.Application) BruteForceConfig {
	cfg := BruteForceConfig{
		// Hardcoded defaults (previously read from .env)
		LockoutEnabled:   true,
		LockoutThreshold: 5,
		LockoutDurations: []time.Duration{15 * time.Minute, 30 * time.Minute, 1 * time.Hour, 24 * time.Hour},
		LockoutWindow:    15 * time.Minute,
		LockoutTierTTL:   24 * time.Hour,

		DelayEnabled:    true,
		DelayStartAfter: 2,
		DelayMaxSeconds: 16,
		DelayTierTTL:    30 * time.Minute,

		CaptchaEnabled:   true,
		CaptchaSiteKey:   "6LeIxAcTAAAAAJcZVRqyHh71UMIEGNQ_MXjiZKhI", // #nosec G101 -- Google's public reCAPTCHA v2 test site key, always passes; intended to be overridden per-app
		CaptchaSecretKey: "6LeIxAcTAAAAAGG-vFI1TnRWxMZNFuojJ4WifJWe", // #nosec G101 -- Google's public reCAPTCHA v2 test secret key, always passes; intended to be overridden per-app
		CaptchaThreshold: 3,
	}

	if app == nil {
		return cfg
	}

	// Per-app overrides — non-nil pointer means "override global"
	if app.BfLockoutEnabled != nil {
		cfg.LockoutEnabled = *app.BfLockoutEnabled
	}
	if app.BfLockoutThreshold != nil {
		cfg.LockoutThreshold = *app.BfLockoutThreshold
	}
	if app.BfLockoutDurations != nil {
		if parsed := parseDurations(*app.BfLockoutDurations); len(parsed) > 0 {
			cfg.LockoutDurations = parsed
		}
	}
	if app.BfLockoutWindow != nil {
		if d, err := time.ParseDuration(*app.BfLockoutWindow); err == nil {
			cfg.LockoutWindow = d
		}
	}
	if app.BfLockoutTierTTL != nil {
		if d, err := time.ParseDuration(*app.BfLockoutTierTTL); err == nil {
			cfg.LockoutTierTTL = d
		}
	}
	if app.BfDelayEnabled != nil {
		cfg.DelayEnabled = *app.BfDelayEnabled
	}
	if app.BfDelayStartAfter != nil {
		cfg.DelayStartAfter = *app.BfDelayStartAfter
	}
	if app.BfDelayMaxSeconds != nil {
		cfg.DelayMaxSeconds = *app.BfDelayMaxSeconds
	}
	if app.BfDelayTierTTL != nil {
		if d, err := time.ParseDuration(*app.BfDelayTierTTL); err == nil {
			cfg.DelayTierTTL = d
		}
	}
	if app.BfCaptchaEnabled != nil {
		cfg.CaptchaEnabled = *app.BfCaptchaEnabled
	}
	if app.BfCaptchaSiteKey != nil && *app.BfCaptchaSiteKey != "" {
		cfg.CaptchaSiteKey = *app.BfCaptchaSiteKey
	}
	if app.BfCaptchaSecretKey != nil && *app.BfCaptchaSecretKey != "" {
		cfg.CaptchaSecretKey = *app.BfCaptchaSecretKey
	}
	if app.BfCaptchaThreshold != nil {
		cfg.CaptchaThreshold = *app.BfCaptchaThreshold
	}

	return cfg
}

// parseDurations parses a comma-separated string of durations (e.g., "15m,30m,1h,24h").
func parseDurations(s string) []time.Duration {
	parts := strings.Split(s, ",")
	var durations []time.Duration
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if d, err := time.ParseDuration(p); err == nil {
			durations = append(durations, d)
		}
	}
	return durations
}

// Service orchestrates all brute-force protection features:
// account lockout, progressive delays, and CAPTCHA triggering.
type Service struct {
	DB *gorm.DB
}

// NewService creates a new BruteForce protection service.
func NewService(db *gorm.DB) *Service {
	return &Service{DB: db}
}

// ==================== Account Lockout ====================

// IsAccountLocked checks whether the user is currently locked out.
// It handles auto-unlock: if the lockout has expired, the DB fields are cleared
// and the function returns false (not locked).
// Returns: locked bool, lockedUntil *time.Time, error.
func (s *Service) IsAccountLocked(user *models.User) (bool, *time.Time, error) {
	if user.LockedAt == nil {
		return false, nil, nil
	}

	// Check if lock has expired
	if user.LockExpiresAt != nil && time.Now().UTC().After(*user.LockExpiresAt) {
		// Auto-unlock: clear lockout fields in DB
		if err := s.DB.Model(user).Updates(map[string]interface{}{
			"locked_at":       nil,
			"lock_reason":     "",
			"lock_expires_at": nil,
		}).Error; err != nil {
			return false, nil, fmt.Errorf("failed to auto-unlock user: %w", err)
		}
		user.LockedAt = nil
		user.LockReason = ""
		user.LockExpiresAt = nil
		return false, nil, nil
	}

	// Account is still locked
	return true, user.LockExpiresAt, nil
}

// HandleFailedLogin processes a failed login attempt for lockout purposes.
// It increments the failure counter and, if the threshold is reached, locks the account.
// Returns: wasLocked bool, lockExpiresAt *time.Time, failCount int64, error.
// The failCount is returned so callers can pass it to anomaly detection for notifications.
func (s *Service) HandleFailedLogin(appID uuid.UUID, email string, cfg BruteForceConfig) (bool, *time.Time, int64, error) {
	// Always increment the failure counter so it is tracked even if lockout is disabled.
	// This counter is shared with anomaly detection (same Redis key pattern).
	count, err := redis.IncrFailedLogin(appID.String(), email, cfg.LockoutWindow)
	if err != nil {
		return false, nil, 0, fmt.Errorf("failed to increment failed login count: %w", err)
	}

	if !cfg.LockoutEnabled {
		return false, nil, count, nil
	}

	if count < int64(cfg.LockoutThreshold) {
		return false, nil, count, nil
	}

	// Threshold reached — lock the account
	wasLocked, expiresAt, lockErr := s.lockAccount(appID, email, cfg)
	return wasLocked, expiresAt, count, lockErr
}

// lockAccount applies a lockout to the user with escalating duration.
func (s *Service) lockAccount(appID uuid.UUID, email string, cfg BruteForceConfig) (bool, *time.Time, error) {
	// Get and increment the lockout tier
	tier, err := redis.IncrLockoutTier(appID.String(), email, cfg.LockoutTierTTL)
	if err != nil {
		return false, nil, fmt.Errorf("failed to increment lockout tier: %w", err)
	}

	// Select duration based on tier (1-based from Redis INCR)
	durations := cfg.LockoutDurations
	tierIdx := int(tier) - 1 // Convert to 0-based index
	if tierIdx >= len(durations) {
		tierIdx = len(durations) - 1 // Cap at maximum duration
	}
	lockDuration := durations[tierIdx]

	now := time.Now().UTC()
	expiresAt := now.Add(lockDuration)
	reason := fmt.Sprintf("Too many failed login attempts (tier %d)", tier)

	// Update user record in DB
	result := s.DB.Model(&models.User{}).
		Where("app_id = ? AND email = ?", appID, email).
		Updates(map[string]interface{}{
			"locked_at":       now,
			"lock_reason":     reason,
			"lock_expires_at": expiresAt,
		})
	if result.Error != nil {
		return false, nil, fmt.Errorf("failed to lock user account: %w", result.Error)
	}

	// Reset the failure counter so the next window starts fresh after unlock
	_ = redis.ResetFailedLogins(appID.String(), email)

	return true, &expiresAt, nil
}

// UnlockAccount manually unlocks a user account (admin action).
// Also resets the lockout tier and failure counter.
func (s *Service) UnlockAccount(appID uuid.UUID, userID uuid.UUID, email string) error {
	if err := s.DB.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"locked_at":       nil,
			"lock_reason":     "",
			"lock_expires_at": nil,
		}).Error; err != nil {
		return fmt.Errorf("failed to unlock user account: %w", err)
	}

	// Reset Redis counters
	_ = redis.ResetLockoutTier(appID.String(), email)
	_ = redis.ResetFailedLogins(appID.String(), email)
	_ = redis.ResetDelayTier(appID.String(), email)

	return nil
}

// ResetOnSuccess clears all brute-force counters for a user after successful login.
// This resets delay tiers for both the email and IP identifiers.
func (s *Service) ResetOnSuccess(appID uuid.UUID, email, ipAddress string) {
	appIDStr := appID.String()
	_ = redis.ResetDelayTier(appIDStr, email)
	_ = redis.ResetDelayTier(appIDStr, ipAddress)
	// Note: failed login counter and lockout tier are NOT reset on success.
	// The failed login counter is reset by the anomaly detector.
	// The lockout tier persists for escalation within its TTL window.
}

// ==================== Progressive Delays ====================

// GetDelay calculates the progressive delay (in seconds) that should be applied
// before processing a login attempt. It checks both per-email and per-IP delay tiers
// and returns the higher of the two.
// Returns 0 if no delay should be applied.
func (s *Service) GetDelay(appID uuid.UUID, email, ipAddress string, cfg BruteForceConfig) (int, error) {
	if !cfg.DelayEnabled {
		return 0, nil
	}

	appIDStr := appID.String()

	emailTier, err := redis.GetDelayTier(appIDStr, email)
	if err != nil {
		return 0, err
	}

	ipTier, err := redis.GetDelayTier(appIDStr, ipAddress)
	if err != nil {
		return 0, err
	}

	// Use the higher tier
	tier := emailTier
	if ipTier > tier {
		tier = ipTier
	}

	// No delay if below the start threshold
	if tier < int64(cfg.DelayStartAfter) {
		return 0, nil
	}

	// Calculate exponential delay: 2^(tier - startAfter) seconds, capped at max
	exponent := tier - int64(cfg.DelayStartAfter)
	delay := int(math.Pow(2, float64(exponent)))
	if delay > cfg.DelayMaxSeconds {
		delay = cfg.DelayMaxSeconds
	}

	return delay, nil
}

// IncrementDelayTier increments the delay tier for both email and IP after a failed login.
func (s *Service) IncrementDelayTier(appID uuid.UUID, email, ipAddress string, cfg BruteForceConfig) {
	if !cfg.DelayEnabled {
		return
	}
	appIDStr := appID.String()
	_, _ = redis.IncrDelayTier(appIDStr, email, cfg.DelayTierTTL)
	_, _ = redis.IncrDelayTier(appIDStr, ipAddress, cfg.DelayTierTTL)
}

// ==================== CAPTCHA Triggering ====================

// IsCaptchaRequired checks whether CAPTCHA should be required for this login attempt.
// It checks the failure count for the email against the CAPTCHA threshold.
func (s *Service) IsCaptchaRequired(appID uuid.UUID, email string, cfg BruteForceConfig) (bool, error) {
	if !cfg.CaptchaEnabled {
		return false, nil
	}

	count, err := redis.GetFailedLoginCount(appID.String(), email)
	if err != nil {
		return false, err
	}

	return count >= int64(cfg.CaptchaThreshold), nil
}
