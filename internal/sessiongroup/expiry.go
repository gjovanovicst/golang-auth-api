package sessiongroup

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/spf13/viper"
)

// ExpiryService handles detection of expired sessions and triggers group-wide revocation
type ExpiryService struct {
	handler          ExpiryHandlerInterface
	ctx              context.Context
	cancel           context.CancelFunc
	ticker           *time.Ticker
	isRunning        bool
	useKeyspaceNotif bool
	scanInterval     time.Duration
	scanCount        int64 // incremented each scan cycle, used for periodic orphan cleanup
}

// Config holds configuration for the expiry service
type Config struct {
	Enabled              bool
	UseKeyspaceNotif     bool
	ScanInterval         time.Duration
	KeyspaceNotifEnabled bool
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	redisNotify := viper.GetString("REDIS_NOTIFY_KEYSPACE_EVENTS")
	useKeyspaceNotif := redisNotify != "" && strings.Contains(redisNotify, "E")

	return Config{
		Enabled:              viper.GetBool("SESSION_GROUP_EXPIRY_REVOCATION_ENABLED"),
		UseKeyspaceNotif:     useKeyspaceNotif,
		ScanInterval:         viper.GetDuration("SESSION_GROUP_EXPIRY_SCAN_INTERVAL"),
		KeyspaceNotifEnabled: viper.GetBool("SESSION_GROUP_KEYSYSPACE_NOTIF_ENABLED"),
	}
}

// ExpiryHandlerInterface defines the interface for handling session expiry
type ExpiryHandlerInterface interface {
	ShouldRevokeGroupSessions(appID string) (bool, *models.SessionGroup)
	RevokeAllUserSessionsInGroup(appID, userEmail, reason, deviceID string)
	GetUserByID(userID string) (*models.User, error)
}

// NewExpiryService creates a new expiry detection service
func NewExpiryService(handler ExpiryHandlerInterface) *ExpiryService {
	config := DefaultConfig()

	// Set defaults if not configured
	if config.ScanInterval == 0 {
		config.ScanInterval = 5 * time.Minute
	}
	if !config.Enabled {
		config.Enabled = true
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ExpiryService{
		handler:          handler,
		ctx:              ctx,
		cancel:           cancel,
		ticker:           time.NewTicker(config.ScanInterval),
		isRunning:        false,
		useKeyspaceNotif: config.UseKeyspaceNotif && config.KeyspaceNotifEnabled,
		scanInterval:     config.ScanInterval,
	}
}

// Start begins the expiry detection service
func (s *ExpiryService) Start() {
	if s.isRunning {
		return
	}

	// Clean up any orphaned session_meta keys left over from incomplete session
	// deletions (e.g. DeleteAllUserSessions before the session_meta cleanup fix).
	// This MUST run before the scanners start, otherwise the first keyspace
	// notification or periodic scan for an orphaned key will trigger a cascading
	// group-wide revocation of every active session for affected users.
	cleaned, err := redis.CleanOrphanedSessionMetaKeys()
	if err != nil {
		log.Printf("[SessionGroup] Warning: orphaned session_meta cleanup failed: %v", err)
	} else if cleaned > 0 {
		log.Printf("[SessionGroup] Cleaned up %d orphaned session_meta keys at startup", cleaned)
	}

	s.isRunning = true

	// Start keyspace notification listener if enabled
	if s.useKeyspaceNotif {
		go s.listenForKeyExpirations()
		log.Println("[SessionGroup] Started keyspace notification listener for session expiry")
	} else {
		log.Println("[SessionGroup] Keyspace notifications disabled, using periodic scanning")
	}

	// Start periodic scanner (always runs as fallback)
	go s.periodicScanner()

	log.Printf("[SessionGroup] Expiry detection service started (scan interval: %v)", s.scanInterval)
}

// Stop gracefully stops the expiry detection service
func (s *ExpiryService) Stop() {
	if !s.isRunning {
		return
	}

	log.Println("[SessionGroup] Stopping expiry detection service...")

	s.cancel()
	s.ticker.Stop()

	// Wait for goroutines to finish
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for s.isRunning {
		select {
		case <-timeout:
			log.Println("[SessionGroup] Service stop timeout reached")
			return
		case <-ticker.C:
			// Continue waiting
		}
	}

	log.Println("[SessionGroup] Expiry detection service stopped")
}

// listenForKeyExpirations subscribes to Redis keyspace notifications for expired keys
func (s *ExpiryService) listenForKeyExpirations() {
	pubsub := redis.PSubscribeKeyExpiry(s.ctx, "__keyevent@0__:expired")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			s.handleExpiredKey(msg.Payload)
		}
	}
}

// handleExpiredKey processes an expired key notification
func (s *ExpiryService) handleExpiredKey(key string) {
	// Only process session_meta keys
	if !strings.HasPrefix(key, "session_meta:") {
		return
	}

	appID, userID, sessionID, err := redis.ParseSessionMetaKey(key)
	if err != nil {
		log.Printf("[SessionGroup] Failed to parse expired key %s: %v", key, err)
		return
	}

	log.Printf("[SessionGroup] Session expired: app=%s, user=%s, session=%s", appID, userID, sessionID)

	// Defence-in-depth: before triggering a group-wide revocation, verify that the
	// corresponding session hash is genuinely expiring right now. If the session hash
	// still has a substantial TTL, the session_meta key expiry was spurious (TTL
	// mismatch, manual deletion, or a Redis edge case) and we MUST skip revocation to
	// avoid killing active user sessions across all apps in the group.
	//
	// Redis TTL semantics:
	//   > 0  → key is alive with remaining TTL
	//   = 0  → key exists but has no remaining TTL (expiring right now)
	//   = -1 → key exists but has NO expiry set (persistent/anomalous)
	//   = -2 → key does not exist (already cleanly expired or explicitly deleted)
	//
	// Only TTL == 0 (genuinely expiring right now) should trigger group revocation.
	// TTL == -2 means the session was already cleaned up — revoking the user's other
	// sessions at this point would kill brand-new active sessions created after the
	// stale session expired naturally.
	sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	if sessionTTL, ttlErr := redis.Rdb.TTL(context.Background(), sessionKey).Result(); ttlErr == nil {
		// TTL == -2: key does not exist — session was already cleanly expired or
		// explicitly revoked. There is nothing left to revoke, and proceeding would
		// kill the user's brand-new active sessions in peer apps.
		if sessionTTL == -2 {
			log.Printf("[SessionGroup] SKIPPING group revocation: session hash %s does not exist — session already cleanly expired/revoked", sessionKey)
			// Clean up the orphaned session_meta key so it doesn't fire again
			redis.Rdb.Del(context.Background(), key)
			return
		}
		// TTL == -1: key exists but has no expiry set. This is anomalous for session
		// keys (they always get a TTL at creation). Treat as a spurious notification.
		if sessionTTL == -1 {
			log.Printf("[SessionGroup] SKIPPING group revocation: session hash %s has no TTL (persistent key — anomaly)", sessionKey)
			return
		}
		// TTL > 1 min: session is still alive — spurious session_meta expiry
		// (TTL mismatch between session_meta and session hash).
		if sessionTTL > time.Minute {
			log.Printf("[SessionGroup] SKIPPING group revocation: session hash %s still has %v TTL — spurious session_meta expiry", sessionKey, sessionTTL)
			return
		}
		// TTL is between 0 and 1 minute: session genuinely expiring within the
		// threshold. Proceed with group revocation.
		if sessionTTL > 0 {
			log.Printf("[SessionGroup] Session hash %s has only %v remaining — proceeding with group revocation", sessionKey, sessionTTL)
		} else {
			// TTL == 0: just expired, proceed
			log.Printf("[SessionGroup] Session hash %s TTL=0 (just expired) — proceeding with group revocation", sessionKey)
		}
	}

	// Check if this app belongs to a session group with GlobalLogout enabled
	shouldRevoke, group := s.handler.ShouldRevokeGroupSessions(appID)
	if !shouldRevoke || group == nil {
		return
	}

	// Get user email to revoke sessions in other apps
	user, err := s.handler.GetUserByID(userID)
	if err != nil || user == nil {
		log.Printf("[SessionGroup] Failed to get user %s for group revocation: %v", userID, err)
		return
	}

	// Revoke sessions in all other apps in the group, marking this as natural expiry
	// (not admin revocation) so the frontend shows the amber "Session Expired" modal.
	s.handler.RevokeAllUserSessionsInGroup(appID, user.Email, "expired", "")
}

// periodicScanner periodically scans for expired session_meta keys
func (s *ExpiryService) periodicScanner() {
	// Delay the initial scan to give the server time to stabilise after startup.
	// Before this was 30 s, which could race with the very first login request
	// if session_meta keys from a prior run persisted in Redis.
	time.Sleep(2 * time.Minute)
	s.scanForExpiredSessions()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
			s.scanForExpiredSessions()
		}
	}
}

// scanForExpiredSessions scans Redis for expired session_meta keys and processes them.
// Every 6th scan (≈30 min at the default 5 min interval) it also runs a full orphan
// cleanup to remove any session_meta keys whose session hash was deleted without
// cleaning up the meta key — a belt-and-suspenders complement to the per-scan
// proactive cleanup inside GetExpiredSessionMetaKeys.
func (s *ExpiryService) scanForExpiredSessions() {
	if !s.isRunning {
		return
	}

	// Track scan count for periodic full orphan cleanup
	scanCount := s.incrementScanCount()

	log.Println("[SessionGroup] Starting periodic scan for expired sessions...")
	startTime := time.Now()

	expiredKeys, err := redis.GetExpiredSessionMetaKeys()
	if err != nil {
		log.Printf("[SessionGroup] Failed to get expired session keys: %v", err)
		return
	}

	if len(expiredKeys) == 0 {
		log.Printf("[SessionGroup] No expired sessions found (scan took %v)", time.Since(startTime))
	} else {
		log.Printf("[SessionGroup] Found %d expired sessions", len(expiredKeys))

		processed := 0
		for _, key := range expiredKeys {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.handleExpiredKey(key)
				processed++
			}
		}

		log.Printf("[SessionGroup] Processed %d expired sessions (scan took %v)", processed, time.Since(startTime))
	}

	// Every 6th scan (≈30 min), run a full orphan cleanup as belt-and-suspenders.
	// GetExpiredSessionMetaKeys already handles -1 TTL orphans inline, but a full
	// sweep catches any remaining edge cases (e.g. session_meta keys with positive
	// TTL whose session hash was explicitly deleted without cleaning the meta key).
	if scanCount%6 == 0 {
		if cleaned, cleanErr := redis.CleanOrphanedSessionMetaKeys(); cleanErr != nil {
			log.Printf("[SessionGroup] Periodic orphan cleanup failed: %v", cleanErr)
		} else if cleaned > 0 {
			log.Printf("[SessionGroup] Periodic orphan cleanup: removed %d orphaned session_meta keys", cleaned)
		}
	}
}

// incrementScanCount atomically increments and returns the scan counter.
func (s *ExpiryService) incrementScanCount() int64 {
	// scanCount is only accessed from the single periodicScanner goroutine,
	// so a plain increment is safe without atomic operations.
	s.scanCount++
	return s.scanCount
}

// ForceScan triggers an immediate scan for expired sessions
func (s *ExpiryService) ForceScan() (int, error) {
	if !s.isRunning {
		return 0, fmt.Errorf("service not running")
	}

	expiredKeys, err := redis.GetExpiredSessionMetaKeys()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, key := range expiredKeys {
		s.handleExpiredKey(key)
		count++
	}

	return count, nil
}

// IsRunning returns whether the service is currently running
func (s *ExpiryService) IsRunning() bool {
	return s.isRunning
}

// GetConfig returns the current service configuration
func (s *ExpiryService) GetConfig() Config {
	return Config{
		Enabled:              s.isRunning,
		UseKeyspaceNotif:     s.useKeyspaceNotif,
		ScanInterval:         s.scanInterval,
		KeyspaceNotifEnabled: s.useKeyspaceNotif,
	}
}
