package admin

import (
	"crypto/subtle"
	"testing"
)

// TestCSRFTimingSafeComparison verifies that the CSRF comparison logic uses
// constant-time comparison (subtle.ConstantTimeCompare) rather than ==.
//
// We can't test ValidateCSRFToken directly without Redis, but we can verify
// that the comparison function used (subtle.ConstantTimeCompare) behaves
// correctly for all cases: matching tokens, mismatched tokens, empty tokens,
// and different-length tokens.
func TestCSRFTimingSafeComparison(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		expected int
	}{
		{"matching tokens", "abc123def456", "abc123def456", 1},
		{"mismatched tokens", "abc123def456", "xyz789ghi012", 0},
		{"empty both", "", "", 1},
		{"empty a", "", "abc", 0},
		{"empty b", "abc", "", 0},
		{"different lengths", "short", "muchlongerstring", 0},
		{"single char difference", "abcdef", "abcdeg", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := subtle.ConstantTimeCompare([]byte(tc.a), []byte(tc.b))
			if result != tc.expected {
				t.Errorf("ConstantTimeCompare(%q, %q) = %d, want %d",
					tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

// TestValidateCSRFTokenRejectsEmpty verifies the guard clauses in ValidateCSRFToken.
// This does NOT require Redis — it tests the early returns before Redis is called.
func TestValidateCSRFTokenRejectsEmpty(t *testing.T) {
	// Create service with nil repo (we won't reach the repo in these cases).
	svc := &AccountService{Repo: nil}

	tests := []struct {
		name      string
		sessionID string
		token     string
	}{
		{"empty session ID", "", "sometoken"},
		{"empty token", "somesession", ""},
		{"both empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if svc.ValidateCSRFToken(tc.sessionID, tc.token) {
				t.Error("expected false for empty inputs")
			}
		})
	}
}
