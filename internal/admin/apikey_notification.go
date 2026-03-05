package admin

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gjovanovicst/auth_api/internal/email"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// ApiKeyNotificationService sends expiry-warning emails for API keys that are
// about to expire. It runs as an in-process background goroutine, checking once
// per day (same pattern as internal/log/cleanup.go).
//
// Notifications are sent:
//   - 7 days before expiry  (deduplicated via notified_7_days_at column)
//   - 1 day  before expiry  (deduplicated via notified_1_day_at  column)
//
// The recipient is the system admin email configured via the ADMIN_EMAIL
// environment variable. If the variable is empty, notifications are skipped.
type ApiKeyNotificationService struct {
	repo         *Repository
	emailService *email.Service
	ctx          context.Context
	cancel       context.CancelFunc
	ticker       *time.Ticker
}

// NewApiKeyNotificationService creates the service but does not start it.
func NewApiKeyNotificationService(repo *Repository, emailSvc *email.Service) *ApiKeyNotificationService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ApiKeyNotificationService{
		repo:         repo,
		emailService: emailSvc,
		ctx:          ctx,
		cancel:       cancel,
		ticker:       time.NewTicker(24 * time.Hour),
	}
}

// Start launches the background worker goroutine.
func (s *ApiKeyNotificationService) Start() {
	go s.worker()
	log.Println("API key expiry notification service started (interval: 24h)")
}

// Shutdown stops the background worker.
func (s *ApiKeyNotificationService) Shutdown() {
	if s == nil {
		return
	}
	log.Println("Shutting down API key notification service...")
	if s.cancel != nil {
		s.cancel()
	}
	if s.ticker != nil {
		s.ticker.Stop()
	}
}

// worker runs the notification check on a 24-hour schedule.
func (s *ApiKeyNotificationService) worker() {
	// Run an initial check shortly after startup.
	time.Sleep(2 * time.Minute)
	s.runCheck()

	for {
		select {
		case <-s.ctx.Done():
			log.Println("API key notification service shutting down...")
			return
		case <-s.ticker.C:
			s.runCheck()
		}
	}
}

// runCheck queries for keys expiring within 7 days and sends any outstanding
// notification emails.
func (s *ApiKeyNotificationService) runCheck() {
	adminEmail := viper.GetString("ADMIN_EMAIL")
	if adminEmail == "" {
		// No recipient configured — nothing to do.
		return
	}

	log.Println("Running API key expiry notification check...")

	keys, err := s.repo.GetKeysExpiringWithin(7)
	if err != nil {
		log.Printf("API key notification: failed to query expiring keys: %v", err)
		return
	}

	now := time.Now().UTC()
	sent := 0

	for _, key := range keys {
		if key.ExpiresAt == nil {
			continue
		}
		daysLeft := int(key.ExpiresAt.UTC().Sub(now).Hours() / 24)

		// 7-day warning
		if daysLeft <= 7 && key.Notified7DaysAt == nil {
			if err := s.sendNotification(adminEmail, key.ID, key.Name, key.KeyPrefix, string(key.KeyType), *key.ExpiresAt, daysLeft); err != nil {
				log.Printf("API key notification: failed to send 7-day warning for key %s: %v", key.ID, err)
			} else {
				if markErr := s.repo.MarkApiKeyNotified7Days(key.ID); markErr != nil {
					log.Printf("API key notification: failed to mark 7-day notified for key %s: %v", key.ID, markErr)
				}
				sent++
			}
		}

		// 1-day warning
		if daysLeft <= 1 && key.Notified1DayAt == nil {
			if err := s.sendNotification(adminEmail, key.ID, key.Name, key.KeyPrefix, string(key.KeyType), *key.ExpiresAt, daysLeft); err != nil {
				log.Printf("API key notification: failed to send 1-day warning for key %s: %v", key.ID, err)
			} else {
				if markErr := s.repo.MarkApiKeyNotified1Day(key.ID); markErr != nil {
					log.Printf("API key notification: failed to mark 1-day notified for key %s: %v", key.ID, markErr)
				}
				sent++
			}
		}
	}

	if sent > 0 {
		log.Printf("API key notification: sent %d expiry warning email(s)", sent)
	}
}

// sendNotification sends a single api_key_expiring_soon email.
func (s *ApiKeyNotificationService) sendNotification(
	toEmail string,
	keyID uuid.UUID,
	keyName, keyPrefix, keyType string,
	expiresAt time.Time,
	daysLeft int,
) error {
	vars := map[string]string{
		email.VarApiKeyName:      keyName,
		email.VarApiKeyPrefix:    keyPrefix,
		email.VarApiKeyType:      keyType,
		email.VarApiKeyExpiresAt: expiresAt.UTC().Format(time.RFC1123),
		email.VarDaysUntilExpiry: fmt.Sprintf("%d", daysLeft),
	}

	// Send in the system (nil) app context — uses the global SMTP config.
	return s.emailService.SendEmail(uuid.Nil, email.TypeApiKeyExpiringSoon, toEmail, vars)
}
