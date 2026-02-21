package user

import (
	"testing"

	"github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	// Setup test configuration — secret must be >= 32 bytes
	viper.Set("JWT_SECRET", "test-jwt-secret-that-is-at-least-32-bytes-long!")
	viper.Set("ACCESS_TOKEN_EXPIRATION_MINUTES", 15)
	viper.Set("REFRESH_TOKEN_EXPIRATION_HOURS", 720)

	m.Run()
}

func TestPasswordHashing(t *testing.T) {
	password := "testpassword123"

	// Test bcrypt hashing
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Expected no error hashing password, got %v", err)
	}

	// Verify password
	bcryptErr := bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if bcryptErr != nil {
		t.Fatalf("Expected password hash to match original password, got error: %v", bcryptErr)
	}

	// Test wrong password
	wrongPassword := "wrongpassword"
	bcryptErr = bcrypt.CompareHashAndPassword(hashedPassword, []byte(wrongPassword))
	if bcryptErr == nil {
		t.Fatal("Expected error for wrong password, got nil")
	}
}

func TestServiceCreation(t *testing.T) {
	// Test that service can be created without errors
	repo := &Repository{}
	emailService := email.NewService()
	service := NewService(repo, emailService)

	if service == nil {
		t.Fatal("Expected service to be created, got nil")
	}

	if service.Repo != repo {
		t.Fatal("Expected service to have correct repository")
	}

	if service.EmailService != emailService {
		t.Fatal("Expected service to have correct email service")
	}
}

func TestAppErrorCreation(t *testing.T) {
	// Test custom error creation
	err := errors.NewAppError(errors.ErrUnauthorized, "Test error message")

	if err == nil {
		t.Fatal("Expected error to be created, got nil")
	}

	// The Code field contains the HTTP status code, not the error type
	if err.Code != 401 { // http.StatusUnauthorized
		t.Fatalf("Expected HTTP status code 401, got %d", err.Code)
	}

	if err.Message != "Test error message" {
		t.Fatalf("Expected error message 'Test error message', got '%s'", err.Message)
	}
}

func TestPasswordStrengthRequirements(t *testing.T) {
	// Test various password scenarios
	testCases := []struct {
		password   string
		minLength  int
		shouldPass bool
	}{
		{"short", 8, false},
		{"password123", 8, true},
		{"verylongpasswordthatshoulddefinitelypass", 8, true},
		{"", 8, false},
		{"1234567", 8, false},
		{"12345678", 8, true},
	}

	for _, tc := range testCases {
		isValid := len(tc.password) >= tc.minLength
		if isValid != tc.shouldPass {
			t.Fatalf("Password '%s' with min length %d: expected %t, got %t",
				tc.password, tc.minLength, tc.shouldPass, isValid)
		}
	}
}

func TestEmailValidation(t *testing.T) {
	// Test basic email format validation (simplified)
	testCases := []struct {
		email   string
		isValid bool
	}{
		{"test@example.com", true},
		{"user@domain.org", true},
		{"invalid-email", false},
		{"@domain.com", false},
		{"user@", false},
		{"", false},
		{"user.name@domain.co.uk", true},
	}

	for _, tc := range testCases {
		// Improved email validation
		hasAt := false
		hasDot := false
		atIndex := -1

		for i, char := range tc.email {
			if char == '@' {
				if hasAt { // Multiple @ symbols
					hasAt = false
					break
				}
				hasAt = true
				atIndex = i
			}
			if char == '.' && hasAt && i > atIndex {
				hasDot = true
			}
		}

		// Must have @ and ., @ cannot be first or last, must have text after @
		isValid := hasAt && hasDot && len(tc.email) > 0 && atIndex > 0 && atIndex < len(tc.email)-1

		if isValid != tc.isValid {
			t.Fatalf("Email '%s': expected %t, got %t", tc.email, tc.isValid, isValid)
		}
	}
}
