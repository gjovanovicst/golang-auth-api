package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

// AccountService handles admin authentication and session management
type AccountService struct {
	Repo *AccountRepository
}

// NewAccountService creates a new AccountService
func NewAccountService(repo *AccountRepository) *AccountService {
	return &AccountService{Repo: repo}
}

// Authenticate validates admin credentials and updates the last login timestamp.
// Returns the admin account on success, or an error on failure.
func (s *AccountService) Authenticate(username, password string) (*models.AdminAccount, error) {
	account, err := s.Repo.GetByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login timestamp (non-blocking, best effort)
	_ = s.Repo.UpdateLastLogin(account.ID.String())

	return account, nil
}

// CreateSession generates a secure session token and stores it in Redis.
// Returns the session ID that should be set as a cookie value.
func (s *AccountService) CreateSession(adminID string) (string, error) {
	sessionBytes := make([]byte, 32)
	if _, err := rand.Read(sessionBytes); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	sessionID := hex.EncodeToString(sessionBytes)

	expiration := s.sessionExpiration()
	if err := redis.SetAdminSession(sessionID, adminID, expiration); err != nil {
		return "", fmt.Errorf("failed to store session: %w", err)
	}

	return sessionID, nil
}

// ValidateSession checks if a session is valid and returns the associated admin account.
func (s *AccountService) ValidateSession(sessionID string) (*models.AdminAccount, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("empty session ID")
	}

	adminID, err := redis.GetAdminSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin account not found")
	}

	return account, nil
}

// Logout removes a session from Redis.
func (s *AccountService) Logout(sessionID string) error {
	return redis.DeleteAdminSession(sessionID)
}

// GenerateCSRFToken returns the existing CSRF token for the session if one
// exists, or creates and stores a new one. This ensures a stable token per
// session so that HTMX GET requests (which also pass through the middleware)
// don't invalidate the token held in the page's <meta> tag.
func (s *AccountService) GenerateCSRFToken(sessionID string) (string, error) {
	// Return existing token if still valid
	existing, err := redis.GetCSRFToken(sessionID)
	if err == nil && existing != "" {
		return existing, nil
	}

	// Generate a new token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiration := s.sessionExpiration()
	if err := redis.SetCSRFToken(sessionID, token, expiration); err != nil {
		return "", fmt.Errorf("failed to store CSRF token: %w", err)
	}

	return token, nil
}

// ValidateCSRFToken checks if the provided CSRF token matches the one stored for the session.
func (s *AccountService) ValidateCSRFToken(sessionID, token string) bool {
	if sessionID == "" || token == "" {
		return false
	}

	storedToken, err := redis.GetCSRFToken(sessionID)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(storedToken), []byte(token)) == 1
}

// sessionExpiration returns the configured session TTL
func (s *AccountService) sessionExpiration() time.Duration {
	hours := viper.GetInt("ADMIN_SESSION_EXPIRATION_HOURS")
	if hours <= 0 {
		hours = 8 // default: 8 hours
	}
	return time.Duration(hours) * time.Hour
}
