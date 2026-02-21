package errors

import (
	"net/http"
	"testing"
)

func TestAppErrorImplementsError(t *testing.T) {
	err := NewAppError(ErrBadRequest, "bad request")
	// Verify it implements the error interface.
	var _ error = err
	if err.Error() != "bad request" {
		t.Errorf("Error() = %q, want %q", err.Error(), "bad request")
	}
}

func TestNewAppErrorMapsHTTPCodes(t *testing.T) {
	tests := []struct {
		errType  int
		expected int
	}{
		{ErrInternal, http.StatusInternalServerError},
		{ErrUnauthorized, http.StatusUnauthorized},
		{ErrForbidden, http.StatusForbidden},
		{ErrNotFound, http.StatusNotFound},
		{ErrConflict, http.StatusConflict},
		{ErrBadRequest, http.StatusBadRequest},
	}

	for _, tc := range tests {
		err := NewAppError(tc.errType, "test")
		if err.Code != tc.expected {
			t.Errorf("errType %d: got HTTP code %d, want %d", tc.errType, err.Code, tc.expected)
		}
	}
}

func TestNewAppErrorUnknownType(t *testing.T) {
	err := NewAppError(999, "unknown error type")
	if err.Code != http.StatusInternalServerError {
		t.Errorf("unknown errType: got %d, want %d", err.Code, http.StatusInternalServerError)
	}
}

func TestAppErrorMessage(t *testing.T) {
	msg := "email already registered"
	err := NewAppError(ErrConflict, msg)
	if err.Message != msg {
		t.Errorf("Message = %q, want %q", err.Message, msg)
	}
}

func TestErrorConstants(t *testing.T) {
	// Verify constants are distinct (iota-based).
	consts := []int{ErrInternal, ErrUnauthorized, ErrForbidden, ErrNotFound, ErrConflict, ErrBadRequest}
	seen := make(map[int]bool)
	for _, c := range consts {
		if seen[c] {
			t.Errorf("duplicate error constant value: %d", c)
		}
		seen[c] = true
	}
}
