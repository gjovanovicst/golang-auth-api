package social

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/spf13/viper"
)

const (
	// OAuthStateVersion prefixes every signed state token. Tokens without this
	// prefix (for example the legacy unsigned base64-JSON states) are rejected,
	// and the version is mixed into the HMAC so a future scheme change cannot be
	// confused with the current one.
	OAuthStateVersion = "v2"

	// OAuthStateTTL is the maximum age of an OAuth state token and the lifetime
	// of the server-side replay-protection entry for its nonce.
	OAuthStateTTL = 10 * time.Minute

	// oauthStateMaxClockSkew tolerates small clock differences between the
	// instance that minted a state token and the one that validates it.
	oauthStateMaxClockSkew = 60 * time.Second

	// minOAuthStateSecretLen mirrors the JWT secret policy; an HMAC key shorter
	// than this is not acceptable.
	minOAuthStateSecretLen = 32

	// oauthStateNonceKeyPrefix namespaces replay-protection keys in Redis.
	oauthStateNonceKeyPrefix = "oauth:state:"

	// oauthStateCookieName is the browser-binding cookie that is set when a
	// state token is minted and checked when the provider redirects back.
	oauthStateCookieName = "oauth_state_nonce"

	// FlowLogin and FlowLink are the supported OAuth state flows.
	FlowLogin = "login"
	FlowLink  = "link"
)

// OAuthState represents the data stored in the OAuth state parameter.
//
// The state is signed with HMAC-SHA256 (see NewOAuthState) so clients cannot
// alter any field. The nonce is additionally registered server-side and can be
// consumed exactly once, which prevents replay of a captured state token.
type OAuthState struct {
	RedirectURI string    `json:"redirect_uri"`
	AppID       string    `json:"app_id"`
	Nonce       string    `json:"nonce"`
	Timestamp   time.Time `json:"timestamp"`
	UserID      string    `json:"user_id,omitempty"` // Set when linking a social account to an authenticated user
	Flow        string    `json:"flow,omitempty"`    // "login" (default) or "link"
}

// oauthStateContext is used for the Redis calls that back replay protection.
var oauthStateContext = context.Background()

// generateRandomString returns n cryptographically secure random bytes encoded
// with URL-safe base64 (no padding), so the result is safe inside a URL query.
func generateRandomString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
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
			"localhost:5174",
			"localhost:5175",
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

// oauthStateSigningKey returns the key used to sign state tokens.
//
// OAUTH_STATE_SECRET is preferred; JWT_SECRET is used as a fallback so existing
// deployments keep working without an extra required variable. The key is
// domain-separated from the JWT signing key with HMAC-SHA256.
func oauthStateSigningKey() ([]byte, error) {
	secret := strings.TrimSpace(viper.GetString("OAUTH_STATE_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(viper.GetString("JWT_SECRET"))
	}
	if len(secret) < minOAuthStateSecretLen {
		return nil, fmt.Errorf(
			"OAuth state signing secret is missing or too short: set OAUTH_STATE_SECRET (or JWT_SECRET) to at least %d bytes",
			minOAuthStateSecretLen,
		)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("auth_api/oauth_state/" + OAuthStateVersion))
	return mac.Sum(nil), nil
}

// signOAuthStatePayload returns the base64url HMAC-SHA256 of the encoded payload.
func signOAuthStatePayload(payload string) (string, error) {
	key, err := oauthStateSigningKey()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(OAuthStateVersion + "." + payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// NewOAuthState builds a signed state token for the given flow and returns the
// encoded token together with the state it represents.
//
// Rules enforced here:
//   - the redirect URI must be allow-listed;
//   - the link flow must carry the authenticated user's ID, the login flow must not;
//   - the token is signed with HMAC-SHA256 and carries a random 32-byte nonce.
func NewOAuthState(redirectURI, appID, flow, userID string) (string, *OAuthState, error) {
	if flow == "" {
		flow = FlowLogin
	}
	switch flow {
	case FlowLogin:
		if userID != "" {
			return "", nil, fmt.Errorf("login flow must not carry a user ID")
		}
	case FlowLink:
		if userID == "" {
			return "", nil, fmt.Errorf("link flow requires a user ID")
		}
	default:
		return "", nil, fmt.Errorf("unsupported OAuth flow %q", flow)
	}

	// Validate redirect URI
	if !IsAllowedRedirectURI(redirectURI) {
		return "", nil, fmt.Errorf("redirect URI not allowed: %s", redirectURI)
	}

	nonce, err := generateRandomString(32)
	if err != nil {
		return "", nil, err
	}

	state := OAuthState{
		RedirectURI: redirectURI,
		AppID:       appID,
		Nonce:       nonce,
		Timestamp:   time.Now().UTC(),
		UserID:      userID,
		Flow:        flow,
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return "", nil, err
	}

	payload := base64.RawURLEncoding.EncodeToString(stateJSON)
	signature, err := signOAuthStatePayload(payload)
	if err != nil {
		return "", nil, err
	}

	return OAuthStateVersion + "." + payload + "." + signature, &state, nil
}

// CreateOAuthState creates a signed state parameter for the social login flow.
func CreateOAuthState(redirectURI string, appID string) (string, error) {
	token, _, err := NewOAuthState(redirectURI, appID, FlowLogin, "")
	return token, err
}

// CreateOAuthLinkState creates a signed state parameter for the account linking
// flow. The user ID travels inside the signed payload, so it cannot be changed
// by the client; it is additionally bound to the browser that started the flow
// and to a single-use server-side nonce.
func CreateOAuthLinkState(redirectURI string, appID string, userID string) (string, error) {
	token, _, err := NewOAuthState(redirectURI, appID, FlowLink, userID)
	return token, err
}

// ParseOAuthState verifies and decodes an OAuth state parameter.
//
// It rejects unsigned/legacy tokens, tokens with a bad or missing signature,
// tokens older than OAuthStateTTL, tokens dated in the future, and non
// allow-listed redirect URIs. It does not consume the nonce; callers that run
// the callback (see Handler.completeOAuthState) must do that as well.
func ParseOAuthState(encodedState string) (*OAuthState, error) {
	parts := strings.Split(encodedState, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid state format")
	}
	if parts[0] != OAuthStateVersion {
		return nil, fmt.Errorf("unsupported state version")
	}

	expectedSig, err := signOAuthStatePayload(parts[1])
	if err != nil {
		return nil, err
	}
	// constant-time comparison — never compare signatures with ==
	if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return nil, fmt.Errorf("invalid state signature")
	}

	stateJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid state encoding: %v", err)
	}

	var state OAuthState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return nil, fmt.Errorf("invalid state format: %v", err)
	}

	if state.Nonce == "" {
		return nil, fmt.Errorf("state is missing a nonce")
	}

	// Reject stale and future-dated states. The future check stops an attacker
	// from pre-computing a state with a far-future timestamp.
	age := time.Since(state.Timestamp)
	if age > OAuthStateTTL {
		return nil, fmt.Errorf("state has expired")
	}
	if age < -oauthStateMaxClockSkew {
		return nil, fmt.Errorf("state timestamp is in the future")
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

// ============================================================================
// Single-use nonce store (replay protection)
// ============================================================================

// oauthStateNonceStore records issued state nonces so each one can be redeemed
// exactly once.
type oauthStateNonceStore interface {
	Save(nonce string, ttl time.Duration) error
	// Consume atomically removes the nonce and reports whether it existed.
	Consume(nonce string) (bool, error)
}

// memoryNonceStore is a process-local fallback used when Redis is unavailable.
type memoryNonceStore struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

func newMemoryNonceStore() *memoryNonceStore {
	return &memoryNonceStore{entries: make(map[string]time.Time)}
}

func (m *memoryNonceStore) Save(nonce string, ttl time.Duration) error {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	// Opportunistic cleanup: keep the map from growing without bound if Redis
	// is down for a long time.
	if len(m.entries) > 10000 {
		for k, exp := range m.entries {
			if now.After(exp) {
				delete(m.entries, k)
			}
		}
	}
	m.entries[nonce] = now.Add(ttl)
	return nil
}

func (m *memoryNonceStore) Consume(nonce string) (bool, error) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.entries[nonce]
	if !ok {
		return false, nil
	}
	delete(m.entries, nonce)
	return now.Before(exp), nil
}

// hybridNonceStore stores nonces in Redis (shared across instances) and in a
// process-local map (so the flow keeps working if Redis is briefly unavailable).
// A nonce is consumed if either store had it, and it is deleted from both.
type hybridNonceStore struct {
	memory *memoryNonceStore
}

func newHybridNonceStore() *hybridNonceStore {
	return &hybridNonceStore{memory: newMemoryNonceStore()}
}

func (h *hybridNonceStore) Save(nonce string, ttl time.Duration) error {
	if err := h.memory.Save(nonce, ttl); err != nil {
		return err
	}
	if redis.Rdb != nil {
		if err := redis.Rdb.Set(oauthStateContext, oauthStateNonceKeyPrefix+nonce, "1", ttl).Err(); err != nil {
			// Degraded mode: replay protection falls back to this process only.
			log.Printf("[OAUTH] failed to persist state nonce in Redis: %v", err)
		}
	}
	return nil
}

func (h *hybridNonceStore) Consume(nonce string) (bool, error) {
	usedInMemory, err := h.memory.Consume(nonce)
	if err != nil {
		return false, err
	}
	usedInRedis := false
	if redis.Rdb != nil {
		deleted, redisErr := redis.Rdb.Del(oauthStateContext, oauthStateNonceKeyPrefix+nonce).Result()
		if redisErr != nil {
			log.Printf("[OAUTH] failed to consume state nonce in Redis: %v", redisErr)
		} else if deleted > 0 {
			usedInRedis = true
		}
	}
	return usedInMemory || usedInRedis, nil
}

// oauthStateNonces is the package-level store. Tests may replace it.
var oauthStateNonces oauthStateNonceStore = newHybridNonceStore()

// RegisterOAuthState records the nonce of a freshly minted state so the
// callback can redeem it exactly once.
func RegisterOAuthState(state *OAuthState) error {
	if state == nil || state.Nonce == "" {
		return fmt.Errorf("cannot register an empty OAuth state")
	}
	return oauthStateNonces.Save(state.Nonce, OAuthStateTTL)
}

// ConsumeOAuthStateNonce redeems a state nonce. It returns false when the nonce
// was never issued or has already been used.
func ConsumeOAuthStateNonce(nonce string) (bool, error) {
	if nonce == "" {
		return false, nil
	}
	return oauthStateNonces.Consume(nonce)
}
