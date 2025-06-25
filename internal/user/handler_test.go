package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/spf13/viper"
)

func setupTestHandler() *Handler {
	// Setup test configuration
	viper.Set("JWT_SECRET", "testsecret")
	viper.Set("ACCESS_TOKEN_EXPIRATION_MINUTES", 15)
	viper.Set("REFRESH_TOKEN_EXPIRATION_HOURS", 720)

	// Use a simple in-memory mock instead of SQLite
	// For production, these would be integration tests with real DB
	repo := &Repository{} // Empty repo for basic testing
	emailService := email.NewService()
	service := NewService(repo, emailService)
	handler := NewHandler(service)

	return handler
}

func TestRegisterHandlerJSONParsing(t *testing.T) {
	handler := setupTestHandler()

	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", handler.Register)

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for invalid JSON, got %d", w.Code)
	}
}

func TestRegisterHandlerValidation(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", handler.Register)

	// Test missing email
	reqBody := dto.RegisterRequest{
		Password: "password123",
	}
	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for missing email, got %d", w.Code)
	}
}

func TestRegisterHandlerInvalidEmail(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", handler.Register)

	// Test invalid email format
	reqBody := dto.RegisterRequest{
		Email:    "invalid-email",
		Password: "password123",
	}
	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for invalid email, got %d", w.Code)
	}
}

func TestRegisterHandlerWeakPassword(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", handler.Register)

	// Test weak password
	reqBody := dto.RegisterRequest{
		Email:    "test@example.com",
		Password: "123", // Too short
	}
	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for weak password, got %d", w.Code)
	}
}

func TestLoginHandlerJSONParsing(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", handler.Login)

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for invalid JSON, got %d", w.Code)
	}
}

func TestLoginHandlerValidation(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", handler.Login)

	// Test missing email
	reqBody := dto.LoginRequest{
		Password: "password123",
	}
	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for missing email, got %d", w.Code)
	}
}

func TestRefreshTokenHandlerJSONParsing(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/refresh-token", handler.RefreshToken)

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/refresh-token", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for invalid JSON, got %d", w.Code)
	}
}

func TestForgotPasswordHandlerJSONParsing(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/forgot-password", handler.ForgotPassword)

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/forgot-password", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for invalid JSON, got %d", w.Code)
	}
}

func TestForgotPasswordHandlerValidation(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/forgot-password", handler.ForgotPassword)

	// Test invalid email format
	reqBody := dto.ForgotPasswordRequest{
		Email: "invalid-email",
	}
	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/forgot-password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for invalid email, got %d", w.Code)
	}
}

func TestResetPasswordHandlerJSONParsing(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/reset-password", handler.ResetPassword)

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/reset-password", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for invalid JSON, got %d", w.Code)
	}
}

func TestVerifyEmailHandlerMissingToken(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/verify-email", handler.VerifyEmail)

	// Test with missing token
	req, _ := http.NewRequest("GET", "/verify-email", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for missing token, got %d", w.Code)
	}
}

func TestGetProfileHandlerMissingUserID(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/profile", handler.GetProfile)

	// Test without setting userID in context (should fail)
	req, _ := http.NewRequest("GET", "/profile", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500 for missing user context, got %d", w.Code)
	}
}

func TestLogoutHandlerJSONParsing(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Mock middleware that sets userID
		c.Set("userID", "test-user-id")
		c.Next()
	})
	router.POST("/logout", handler.Logout)

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/logout", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for invalid JSON, got %d", w.Code)
	}
}

func TestLogoutHandlerValidation(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Mock middleware that sets userID
		c.Set("userID", "test-user-id")
		c.Next()
	})
	router.POST("/logout", handler.Logout)

	// Test missing refresh token
	logoutData := map[string]interface{}{}
	jsonData, _ := json.Marshal(logoutData)
	req, _ := http.NewRequest("POST", "/logout", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400 for missing refresh token, got %d", w.Code)
	}
}

func TestLogoutHandlerMissingUserID(t *testing.T) {
	handler := setupTestHandler()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Don't set userID to simulate missing authentication
	router.POST("/logout", handler.Logout)

	// Test valid logout request but missing userID
	logoutData := map[string]interface{}{
		"refresh_token": "valid-refresh-token",
	}
	jsonData, _ := json.Marshal(logoutData)
	req, _ := http.NewRequest("POST", "/logout", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500 for missing user context, got %d", w.Code)
	}
}
