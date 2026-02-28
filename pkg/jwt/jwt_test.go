package jwt

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	// Setup test configuration — secret must be >= 32 bytes
	viper.Set("JWT_SECRET", "test-jwt-secret-that-is-at-least-32-bytes-long!")
	viper.Set("ACCESS_TOKEN_EXPIRATION_MINUTES", 15)
	viper.Set("REFRESH_TOKEN_EXPIRATION_HOURS", 720)

	m.Run()
}

func TestGenerateAccessToken(t *testing.T) {
	appID := "00000000-0000-0000-0000-000000000001"
	userID := "test-user-id"

	token, err := GenerateAccessToken(appID, userID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if token == "" {
		t.Fatal("Expected token to be generated, got empty string")
	}

	// Verify token can be parsed
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("Expected token to be parseable, got error: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("Expected user ID %s, got %s", userID, claims.UserID)
	}

	if claims.AppID != appID {
		t.Fatalf("Expected app ID %s, got %s", appID, claims.AppID)
	}

	// Check that token has been issued recently (within last minute)
	if time.Since(claims.IssuedAt.Time) > time.Minute {
		t.Fatal("Token seems to have been issued too long ago")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	appID := "00000000-0000-0000-0000-000000000001"
	userID := "test-user-id"

	token, err := GenerateRefreshToken(appID, userID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if token == "" {
		t.Fatal("Expected token to be generated, got empty string")
	}

	// Verify token can be parsed
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("Expected token to be parseable, got error: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("Expected user ID %s, got %s", userID, claims.UserID)
	}

	if claims.AppID != appID {
		t.Fatalf("Expected app ID %s, got %s", appID, claims.AppID)
	}

	// Check that token has been issued recently (within last minute)
	if time.Since(claims.IssuedAt.Time) > time.Minute {
		t.Fatal("Token seems to have been issued too long ago")
	}
}

func TestParseTokenValid(t *testing.T) {
	appID := "00000000-0000-0000-0000-000000000001"
	userID := "test-user-id"

	// Generate a token first
	token, err := GenerateAccessToken(appID, userID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Parse the token
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("Expected no error parsing valid token, got %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("Expected user ID %s, got %s", userID, claims.UserID)
	}

	if claims.AppID != appID {
		t.Fatalf("Expected app ID %s, got %s", appID, claims.AppID)
	}

	// Check that token has been issued recently (within last minute)
	if time.Since(claims.IssuedAt.Time) > time.Minute {
		t.Fatal("Token seems to have been issued too long ago")
	}
}

func TestParseTokenInvalid(t *testing.T) {
	invalidToken := "invalid.token.here"

	_, err := ParseToken(invalidToken)
	if err == nil {
		t.Fatal("Expected error parsing invalid token, got nil")
	}
}

func TestParseTokenEmpty(t *testing.T) {
	_, err := ParseToken("")
	if err == nil {
		t.Fatal("Expected error parsing empty token, got nil")
	}
}

func TestParseTokenExpired(t *testing.T) {
	// Set very short expiration for test
	viper.Set("ACCESS_TOKEN_EXPIRATION_MINUTES", 0) // This should create an already expired token

	appID := "00000000-0000-0000-0000-000000000001"
	userID := "test-user-id"
	token, err := GenerateAccessToken(appID, userID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Wait a moment to ensure expiration
	time.Sleep(time.Second)

	_, err = ParseToken(token)
	if err == nil {
		t.Fatal("Expected error parsing expired token, got nil")
	}

	// Reset for other tests
	viper.Set("ACCESS_TOKEN_EXPIRATION_MINUTES", 15)
}

func TestGenerateTokenWithEmptyUserID(t *testing.T) {
	appID := "00000000-0000-0000-0000-000000000001"
	// Our implementation allows empty user IDs, so this should succeed
	token, err := GenerateAccessToken(appID, "")
	if err != nil {
		t.Fatalf("Expected no error generating token with empty user ID, got %v", err)
	}

	if token == "" {
		t.Fatal("Expected token to be generated even with empty user ID")
	}

	// Verify we can parse it back
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("Expected to be able to parse token with empty user ID, got error: %v", err)
	}

	if claims.UserID != "" {
		t.Fatalf("Expected empty user ID in claims, got %s", claims.UserID)
	}
}

func TestTokenTypeDifferentiation(t *testing.T) {
	appID := "00000000-0000-0000-0000-000000000001"
	userID := "test-user-id"

	accessToken, err := GenerateAccessToken(appID, userID)
	if err != nil {
		t.Fatalf("Failed to generate access token: %v", err)
	}

	refreshToken, err := GenerateRefreshToken(appID, userID)
	if err != nil {
		t.Fatalf("Failed to generate refresh token: %v", err)
	}

	accessClaims, err := ParseToken(accessToken)
	if err != nil {
		t.Fatalf("Failed to parse access token: %v", err)
	}

	refreshClaims, err := ParseToken(refreshToken)
	if err != nil {
		t.Fatalf("Failed to parse refresh token: %v", err)
	}

	// Check that both tokens have been issued recently
	if time.Since(accessClaims.IssuedAt.Time) > time.Minute {
		t.Fatal("Access token seems to have been issued too long ago")
	}

	if time.Since(refreshClaims.IssuedAt.Time) > time.Minute {
		t.Fatal("Refresh token seems to have been issued too long ago")
	}

	// Tokens should be different
	if accessToken == refreshToken {
		t.Fatal("Access and refresh tokens should be different")
	}
}
