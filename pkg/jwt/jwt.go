package jwt

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

const (
	// minJWTSecretLength is the minimum acceptable length for the JWT signing secret.
	minJWTSecretLength = 32

	// TokenTypeAccess identifies an access token.
	TokenTypeAccess = "access"

	// TokenTypeRefresh identifies a refresh token.
	TokenTypeRefresh = "refresh"
)

var (
	jwtSecret []byte
	secretMu  sync.Once
)

// loadSecret reads and validates the JWT signing secret from configuration.
// It runs exactly once (via sync.Once) on the first call to any JWT function.
// Using lazy initialization instead of init() allows test code to configure
// viper *before* the secret is read, while still failing fast in production
// (where GenerateAccessToken / ParseToken are called at startup).
func loadSecret() {
	secretMu.Do(func() {
		secret := viper.GetString("JWT_SECRET")

		if len(secret) == 0 {
			log.Fatalf("FATAL: JWT_SECRET environment variable is not set. An auth API cannot run without a signing secret.")
		}
		if len(secret) < minJWTSecretLength {
			log.Fatalf("FATAL: JWT_SECRET is too short (%d bytes). Minimum required: %d bytes.", len(secret), minJWTSecretLength)
		}

		jwtSecret = []byte(secret)
	})
}

// Claims struct that will be embedded in JWT
type Claims struct {
	UserID    string   `json:"user_id"`
	AppID     string   `json:"app_id"`
	SessionID string   `json:"session_id,omitempty"` // Session identifier for multi-device session management
	TokenType string   `json:"token_type,omitempty"` // "access" or "refresh"; empty for legacy tokens
	Roles     []string `json:"roles,omitempty"`      // User's role names in the application
	jwt.RegisteredClaims
}

// DefaultAccessTokenTTL returns the configured global access token TTL.
func DefaultAccessTokenTTL() time.Duration {
	return time.Minute * time.Duration(viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES"))
}

// DefaultRefreshTokenTTL returns the configured global refresh token TTL.
func DefaultRefreshTokenTTL() time.Duration {
	return time.Hour * time.Duration(viper.GetInt("REFRESH_TOKEN_EXPIRATION_HOURS"))
}

// GenerateAccessToken generates a new access token with an explicit TTL.
// Pass 0 (or DefaultAccessTokenTTL()) to use the global configured value.
func GenerateAccessToken(appID, userID, sessionID string, roles []string, ttl time.Duration) (string, error) {
	loadSecret()
	if ttl <= 0 {
		ttl = DefaultAccessTokenTTL()
	}
	expirationTime := time.Now().Add(ttl)
	claims := &Claims{
		UserID:    userID,
		AppID:     appID,
		SessionID: sessionID,
		TokenType: TokenTypeAccess,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// GenerateRefreshToken generates a new refresh token with an explicit TTL.
// Pass 0 (or DefaultRefreshTokenTTL()) to use the global configured value.
func GenerateRefreshToken(appID, userID, sessionID string, roles []string, ttl time.Duration) (string, error) {
	loadSecret()
	if ttl <= 0 {
		ttl = DefaultRefreshTokenTTL()
	}
	expirationTime := time.Now().Add(ttl)
	claims := &Claims{
		UserID:    userID,
		AppID:     appID,
		SessionID: sessionID,
		TokenType: TokenTypeRefresh,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken parses and validates a JWT token
func ParseToken(tokenString string) (*Claims, error) {
	loadSecret()
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}
