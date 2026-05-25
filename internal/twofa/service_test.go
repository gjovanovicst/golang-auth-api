package twofa

import (
	"encoding/base32"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// generateSecretKey
// ---------------------------------------------------------------------------

func TestGenerateSecretKey_Length(t *testing.T) {
	key := generateSecretKey()
	// 32 random bytes base32-encoded → 56 characters (with padding)
	require.NotEmpty(t, key)
	decoded, err := base32.StdEncoding.DecodeString(key)
	require.NoError(t, err, "generateSecretKey must produce valid base32")
	assert.Equal(t, 32, len(decoded), "decoded secret must be 32 bytes")
}

func TestGenerateSecretKey_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		k := generateSecretKey()
		_, dup := seen[k]
		assert.False(t, dup, "duplicate secret key generated on iteration %d", i)
		seen[k] = struct{}{}
	}
}

// ---------------------------------------------------------------------------
// generate6DigitCode
// ---------------------------------------------------------------------------

var sixDigitRe = regexp.MustCompile(`^\d{6}$`)

func TestGenerate6DigitCode_Format(t *testing.T) {
	for i := 0; i < 200; i++ {
		code := generate6DigitCode()
		assert.Regexp(t, sixDigitRe, code, "6-digit code must match \\d{6}")
	}
}

func TestGenerate6DigitCode_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 50; i++ {
		seen[generate6DigitCode()] = struct{}{}
	}
	// With 50 draws over 10^6 space the probability of all being the same is negligible
	assert.Greater(t, len(seen), 1, "all generated 6-digit codes were identical — RNG likely broken")
}

// ---------------------------------------------------------------------------
// generateRecoveryCodes
// ---------------------------------------------------------------------------

var hexCodeRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

func TestGenerateRecoveryCodes_Count(t *testing.T) {
	for _, n := range []int{1, 5, 10, 16} {
		codes := generateRecoveryCodes(n)
		assert.Len(t, codes, n, "expected %d codes", n)
	}
}

func TestGenerateRecoveryCodes_Format(t *testing.T) {
	codes := generateRecoveryCodes(10)
	for _, c := range codes {
		assert.Regexp(t, hexCodeRe, c, "recovery code must be 16 hex chars, got %q", c)
	}
}

func TestGenerateRecoveryCodes_Uniqueness(t *testing.T) {
	codes := generateRecoveryCodes(10)
	seen := make(map[string]struct{})
	for _, c := range codes {
		_, dup := seen[c]
		assert.False(t, dup, "duplicate recovery code: %s", c)
		seen[c] = struct{}{}
	}
}

// ---------------------------------------------------------------------------
// VerifyTOTP — invalid userID returns ErrBadRequest without touching the DB
// ---------------------------------------------------------------------------

func TestVerifyTOTP_InvalidUserID(t *testing.T) {
	svc := &Service{} // nil repos — must not be reached for bad UUID
	appID := uuid.New()
	appErr := svc.VerifyTOTP(appID, "not-a-uuid", "123456")
	require.NotNil(t, appErr)
	assert.Contains(t, appErr.Message, "Invalid user ID")
}

// ---------------------------------------------------------------------------
// VerifyRecoveryCode — invalid userID returns ErrBadRequest without touching the DB
// ---------------------------------------------------------------------------

func TestVerifyRecoveryCode_InvalidUserID(t *testing.T) {
	svc := &Service{}
	appID := uuid.New()
	appErr := svc.VerifyRecoveryCode(appID, "bad-uuid", "abc123")
	require.NotNil(t, appErr)
	assert.Contains(t, appErr.Message, "Invalid user ID")
}

// ---------------------------------------------------------------------------
// GenerateNewRecoveryCodes — invalid userID returns ErrBadRequest without touching the DB
// ---------------------------------------------------------------------------

func TestGenerateNewRecoveryCodes_InvalidUserID(t *testing.T) {
	svc := &Service{}
	appID := uuid.New()
	codes, appErr := svc.GenerateNewRecoveryCodes(appID, "bad-uuid")
	require.NotNil(t, appErr)
	assert.Nil(t, codes)
	assert.Contains(t, appErr.Message, "Invalid user ID")
}

// ---------------------------------------------------------------------------
// GetUserTwoFAMethod — invalid userID returns ErrBadRequest without touching the DB
// ---------------------------------------------------------------------------

func TestGetUserTwoFAMethod_InvalidUserID(t *testing.T) {
	svc := &Service{}
	appID := uuid.New()
	method, appErr := svc.GetUserTwoFAMethod(appID, "bad-uuid")
	require.NotNil(t, appErr)
	assert.Empty(t, method)
	assert.Contains(t, appErr.Message, "Invalid user ID")
}
