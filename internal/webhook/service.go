package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

const (
	maxAttempts      = 5
	retryWorkerPoll  = 30 * time.Second
	deliveryTimeout  = 10 * time.Second
	maxResponseBytes = 1024 // Store first 1 KB of response body
	secretPrefix     = "whsec_"
)

// Service manages webhook endpoint registration, event dispatching, HMAC signing,
// and the background retry worker.
type Service struct {
	repo     *Repository
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewService creates a new webhook service and starts the background retry worker.
func NewService(repo *Repository) *Service {
	s := &Service{
		repo:   repo,
		stopCh: make(chan struct{}),
	}
	go s.retryWorker()
	return s
}

// Shutdown signals the background retry worker to stop.
func (s *Service) Shutdown() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

// ============================================================================
// Endpoint management
// ============================================================================

// RegisterEndpoint creates a new webhook endpoint.
// Returns the created endpoint AND the plaintext secret (shown only once).
func (s *Service) RegisterEndpoint(appID uuid.UUID, eventType, url string) (*models.WebhookEndpoint, string, error) {
	// Validate event type
	if !isValidEventType(eventType) {
		return nil, "", fmt.Errorf("unsupported event type: %s", eventType)
	}

	// Generate a random 32-byte secret and encode as hex with prefix
	rawSecret := make([]byte, 32)
	if _, err := rand.Read(rawSecret); err != nil {
		return nil, "", fmt.Errorf("failed to generate webhook secret: %w", err)
	}
	plaintextSecret := secretPrefix + hex.EncodeToString(rawSecret)

	ep := &models.WebhookEndpoint{
		AppID:     appID,
		EventType: eventType,
		URL:       url,
		Secret:    plaintextSecret, // stored as-is; HMAC is derived at delivery time
		IsActive:  true,
	}

	if err := s.repo.CreateEndpoint(ep); err != nil {
		return nil, "", fmt.Errorf("failed to create webhook endpoint: %w", err)
	}

	return ep, plaintextSecret, nil
}

// GetEndpoint returns a single webhook endpoint by ID.
func (s *Service) GetEndpoint(id uuid.UUID) (*models.WebhookEndpoint, error) {
	return s.repo.GetEndpointByID(id)
}

// ListEndpointsByApp returns paginated endpoints for an application.
func (s *Service) ListEndpointsByApp(appID uuid.UUID, page, pageSize int) ([]models.WebhookEndpoint, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListEndpointsByApp(appID, page, pageSize)
}

// ListAllEndpoints returns paginated endpoints across all apps (admin use).
func (s *Service) ListAllEndpoints(page, pageSize int) ([]models.WebhookEndpoint, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListAllEndpoints(page, pageSize)
}

// SetEndpointActive enables or disables a webhook endpoint.
func (s *Service) SetEndpointActive(id uuid.UUID, isActive bool) error {
	return s.repo.UpdateEndpointActive(id, isActive)
}

// DeleteEndpoint soft-deletes a webhook endpoint.
func (s *Service) DeleteEndpoint(id uuid.UUID) error {
	return s.repo.SoftDeleteEndpoint(id)
}

// ============================================================================
// Delivery log queries
// ============================================================================

// ListDeliveriesByEndpoint returns paginated delivery history for an endpoint.
func (s *Service) ListDeliveriesByEndpoint(endpointID uuid.UUID, page, pageSize int) ([]models.WebhookDelivery, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.GetDeliveriesByEndpoint(endpointID, page, pageSize)
}

// ListDeliveriesByApp returns paginated delivery history across all endpoints for an app.
func (s *Service) ListDeliveriesByApp(appID uuid.UUID, page, pageSize int) ([]models.WebhookDelivery, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.GetDeliveriesByApp(appID, page, pageSize)
}

// ============================================================================
// Event dispatch
// ============================================================================

// Payload is the body sent to webhook consumers.
type Payload struct {
	EventType string          `json:"event_type"`
	AppID     string          `json:"app_id"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// Dispatch sends event payloads to all active endpoints registered for (appID, eventType).
// Each delivery is fire-and-forget in a goroutine — the call returns immediately.
func (s *Service) Dispatch(appID uuid.UUID, eventType string, data interface{}) {
	log.Printf("[webhook] Dispatch called: appID=%s eventType=%s", appID, eventType)
	endpoints, err := s.repo.GetActiveEndpointsForEvent(appID, eventType)
	if err != nil {
		log.Printf("[webhook] failed to fetch endpoints for %s/%s: %v", appID, eventType, err)
		return
	}
	if len(endpoints) == 0 {
		log.Printf("[webhook] no active endpoints for %s/%s", appID, eventType)
		return
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("[webhook] failed to marshal payload data for %s/%s: %v", appID, eventType, err)
		return
	}

	payload := Payload{
		EventType: eventType,
		AppID:     appID.String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      dataJSON,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[webhook] failed to marshal full payload for %s/%s: %v", appID, eventType, err)
		return
	}

	log.Printf("[webhook] dispatching %s/%s to %d endpoint(s)", appID, eventType, len(endpoints))

	for i := range endpoints {
		ep := endpoints[i] // copy for goroutine
		go func() {
			d := &models.WebhookDelivery{
				ID:         uuid.New(),
				EndpointID: ep.ID,
				AppID:      appID,
				EventType:  eventType,
				Payload:    string(payloadBytes),
				Attempt:    1,
			}
			s.deliver(ep, d, payloadBytes)
		}()
	}
}

// deliver performs the actual HTTP POST and persists the delivery record.
// If delivery fails and retries remain, it schedules the next attempt.
func (s *Service) deliver(ep models.WebhookEndpoint, d *models.WebhookDelivery, payloadBytes []byte) {
	log.Printf("[webhook] delivering endpoint=%s url=%s attempt=%d", ep.ID, ep.URL, d.Attempt)
	sig := signPayload(ep.Secret, payloadBytes)

	ctx, cancel := context.WithTimeout(context.Background(), deliveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		d.ErrorMessage = fmt.Sprintf("failed to build request: %v", err)
		s.scheduleRetryOrSave(d, ep)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "sha256="+sig)
	req.Header.Set("X-Webhook-Event", ep.EventType)
	req.Header.Set("X-Webhook-App-ID", ep.AppID.String())

	start := time.Now()
	// #nosec G107 -- URL is user-supplied but validated at endpoint creation
	resp, err := http.DefaultClient.Do(req)
	d.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		d.ErrorMessage = fmt.Sprintf("delivery error: %v", err)
		s.scheduleRetryOrSave(d, ep)
		return
	}
	defer resp.Body.Close()

	d.StatusCode = resp.StatusCode
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	d.ResponseBody = string(body)
	d.Success = resp.StatusCode >= 200 && resp.StatusCode < 300

	if !d.Success && d.Attempt < maxAttempts {
		d.NextRetryAt = retryAt(d.Attempt)
	}

	if err := s.repo.CreateDelivery(d); err != nil {
		log.Printf("[webhook] failed to persist delivery record for endpoint %s (attempt %d, success=%v): %v", ep.ID, d.Attempt, d.Success, err)
	} else {
		log.Printf("[webhook] saved delivery %s for endpoint %s (attempt %d, status=%d, success=%v)", d.ID, ep.ID, d.Attempt, d.StatusCode, d.Success)
	}
}

// scheduleRetryOrSave persists a failed delivery and sets retry schedule if attempts remain.
func (s *Service) scheduleRetryOrSave(d *models.WebhookDelivery, ep models.WebhookEndpoint) {
	if d.Attempt < maxAttempts {
		d.NextRetryAt = retryAt(d.Attempt)
	}
	if err := s.repo.CreateDelivery(d); err != nil {
		log.Printf("[webhook] failed to persist failed delivery record for endpoint %s (attempt %d): %v", ep.ID, d.Attempt, err)
	} else {
		log.Printf("[webhook] saved failed delivery %s for endpoint %s (attempt %d, error=%q)", d.ID, ep.ID, d.Attempt, d.ErrorMessage)
	}
}

// ============================================================================
// Background retry worker
// ============================================================================

// retryWorker polls the DB every 30s for deliveries that need retrying.
func (s *Service) retryWorker() {
	ticker := time.NewTicker(retryWorkerPoll)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processRetries()
		}
	}
}

func (s *Service) processRetries() {
	pending, err := s.repo.GetPendingRetries(time.Now().UTC(), 100)
	if err != nil {
		log.Printf("[webhook] retry worker: failed to fetch pending retries: %v", err)
		return
	}

	for _, d := range pending {
		ep, err := s.repo.GetEndpointByID(d.EndpointID)
		if err != nil || ep == nil || ep.DeletedAt != nil || !ep.IsActive {
			// Endpoint gone or disabled — clear retry so we don't poll it again
			if clearErr := s.repo.ClearRetrySchedule(d.ID); clearErr != nil {
				log.Printf("[webhook] retry worker: failed to clear retry for delivery %s: %v", d.ID, clearErr)
			}
			continue
		}

		// Clear the old retry schedule before re-attempting
		if clearErr := s.repo.ClearRetrySchedule(d.ID); clearErr != nil {
			log.Printf("[webhook] retry worker: failed to clear retry schedule for delivery %s: %v", d.ID, clearErr)
		}

		payloadBytes := []byte(d.Payload)
		next := &models.WebhookDelivery{
			ID:         uuid.New(),
			EndpointID: d.EndpointID,
			AppID:      d.AppID,
			EventType:  d.EventType,
			Payload:    d.Payload,
			Attempt:    d.Attempt + 1,
		}
		go s.deliver(*ep, next, payloadBytes)
	}
}

// ============================================================================
// Helpers
// ============================================================================

// signPayload returns the HMAC-SHA256 hex signature of payload using the endpoint secret.
func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// retryAt computes the next retry timestamp using exponential backoff:
// delay = 1s * 2^attempt, capped at 1 hour.
func retryAt(attempt int) *time.Time {
	delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	if delay > time.Hour {
		delay = time.Hour
	}
	t := time.Now().UTC().Add(delay)
	return &t
}

// isValidEventType checks whether the given event type is in the supported list.
func isValidEventType(eventType string) bool {
	for _, v := range models.ValidEventTypes {
		if v == eventType {
			return true
		}
	}
	return false
}
