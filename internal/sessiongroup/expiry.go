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
	RevokeAllUserSessionsInGroup(appID, userEmail string)
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
	pubsub := redis.Rdb.PSubscribe(s.ctx, "__keyevent@0__:expired")
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

	// Revoke sessions in all other apps in the group
	s.handler.RevokeAllUserSessionsInGroup(appID, user.Email)
}

// periodicScanner periodically scans for expired session_meta keys
func (s *ExpiryService) periodicScanner() {
	// Run initial scan after a short delay
	time.Sleep(30 * time.Second)
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

// scanForExpiredSessions scans Redis for expired session_meta keys and processes them
func (s *ExpiryService) scanForExpiredSessions() {
	if !s.isRunning {
		return
	}

	log.Println("[SessionGroup] Starting periodic scan for expired sessions...")
	startTime := time.Now()

	expiredKeys, err := redis.GetExpiredSessionMetaKeys()
	if err != nil {
		log.Printf("[SessionGroup] Failed to get expired session keys: %v", err)
		return
	}

	if len(expiredKeys) == 0 {
		log.Printf("[SessionGroup] No expired sessions found (scan took %v)", time.Since(startTime))
		return
	}

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
