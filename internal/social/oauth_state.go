package social

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// OAuthState represents the data stored in OAuth state parameter
type OAuthState struct {
	RedirectURI string    `json:"redirect_uri"`
	Nonce       string    `json:"nonce"`
	Timestamp   time.Time `json:"timestamp"`
}

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// IsAllowedRedirectURI validates if the redirect URI is allowed
func IsAllowedRedirectURI(redirectURI string) bool {
	if redirectURI == "" {
		return false
	}

	// Parse the URL to validate it
	parsedURL, err := url.Parse(redirectURI)
	if err != nil {
		return false
	}

	// Get allowed domains from environment
	allowedDomains := viper.GetStringSlice("ALLOWED_REDIRECT_DOMAINS")
	if len(allowedDomains) == 0 {
		// Default allowed domains for development
		allowedDomains = []string{
			"localhost:3000",
			"localhost:5173",
			"localhost:8080",
			"127.0.0.1:3000",
			"127.0.0.1:5173",
			"127.0.0.1:8080",
		}
	}

	// Check if the host is in the allowed list
	host := parsedURL.Host
	for _, allowedDomain := range allowedDomains {
		if host == allowedDomain {
			return true
		}
		// Allow subdomains if domain starts with a dot (e.g., ".example.com")
		if strings.HasPrefix(allowedDomain, ".") && strings.HasSuffix(host, allowedDomain) {
			return true
		}
	}

	return false
}

// CreateOAuthState creates a secure state parameter with redirect URI
func CreateOAuthState(redirectURI string) (string, error) {
	// Validate redirect URI
	if !IsAllowedRedirectURI(redirectURI) {
		return "", fmt.Errorf("redirect URI not allowed: %s", redirectURI)
	}

	// Generate a random nonce
	nonce, err := generateRandomString(16)
	if err != nil {
		return "", err
	}

	// Create state object
	state := OAuthState{
		RedirectURI: redirectURI,
		Nonce:       nonce,
		Timestamp:   time.Now(),
	}

	// Encode state as JSON
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return "", err
	}

	// Base64 encode the state
	encodedState := base64.URLEncoding.EncodeToString(stateJSON)
	return encodedState, nil
}

// ParseOAuthState parses and validates the OAuth state parameter
func ParseOAuthState(encodedState string) (*OAuthState, error) {
	// Base64 decode
	stateJSON, err := base64.URLEncoding.DecodeString(encodedState)
	if err != nil {
		return nil, fmt.Errorf("invalid state encoding: %v", err)
	}

	// Parse JSON
	var state OAuthState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return nil, fmt.Errorf("invalid state format: %v", err)
	}

	// Validate timestamp (state should not be older than 1 hour)
	if time.Since(state.Timestamp) > time.Hour {
		return nil, fmt.Errorf("state has expired")
	}

	// Validate redirect URI again
	if !IsAllowedRedirectURI(state.RedirectURI) {
		return nil, fmt.Errorf("redirect URI not allowed: %s", state.RedirectURI)
	}

	return &state, nil
}

// GetDefaultRedirectURI returns the default redirect URI for fallback
func GetDefaultRedirectURI() string {
	defaultURI := viper.GetString("DEFAULT_REDIRECT_URI")
	if defaultURI == "" {
		// Default fallback for development
		defaultURI = "http://localhost:5173/auth/callback"
	}
	return defaultURI
}
