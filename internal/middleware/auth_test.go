package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	redisLib "github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	// Enable viper to read from environment variables
	viper.AutomaticEnv()

	// Setup test configuration with defaults (will be overridden by env vars if present)
	// Secret must be >= 32 bytes
	viper.SetDefault("JWT_SECRET", "test-jwt-secret-that-is-at-least-32-bytes-long!")
	viper.SetDefault("ACCESS_TOKEN_EXPIRATION_MINUTES", 15)
	viper.SetDefault("REFRESH_TOKEN_EXPIRATION_HOURS", 720)
	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("REDIS_PASSWORD", "")
	viper.SetDefault("REDIS_DB", 1) // Use DB 1 for testing by default

	// Try to connect to Redis for testing, if it fails, use a mock
	setupTestRedis()

	m.Run()
}

func setupTestRedis() {
	// Try to setup real Redis connection for integration tests
	redis.Rdb = redisLib.NewClient(&redisLib.Options{
		Addr:     viper.GetString("REDIS_ADDR"),
		Password: viper.GetString("REDIS_PASSWORD"),
		DB:       viper.GetInt("REDIS_DB"),
	})

	// Test if Redis is available
	ctx := redis.Rdb.Context()
	if _, err := redis.Rdb.Ping(ctx).Result(); err != nil {
		// Redis not available, create a mock for basic functionality
		redis.Rdb = redisLib.NewClient(&redisLib.Options{
			Addr: "localhost:0", // Invalid address to ensure it fails gracefully
		})
	}
}

// Test basic JWT validation without Redis dependency
func TestAuthMiddlewareValidTokenNoRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Generate a valid token
	userID := "test-user-id"
	token, err := jwt.GenerateAccessToken("test-app-id", userID)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Create a basic JWT-only middleware for testing
	basicAuthMiddleware := func() gin.HandlerFunc {
		return func(c *gin.Context) {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
				return
			}

			tokenString := authHeader
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				tokenString = authHeader[7:]
			}

			// Parse and validate JWT only (no Redis checks)
			claims, err := jwt.ParseToken(tokenString)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
				return
			}

			c.Set("userID", claims.UserID)
			c.Next()
		}
	}

	// Setup router with basic middleware
	router := gin.New()
	router.Use(basicAuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		// Get userID from context
		contextUserID, exists := c.Get("userID")
		if !exists {
			t.Fatal("Expected userID to be set in context")
		}

		if contextUserID != userID {
			t.Fatalf("Expected userID %s, got %s", userID, contextUserID)
		}

		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request with valid token
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should succeed with basic JWT validation
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code 200 for valid JWT, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddlewareRedisUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Temporarily set Redis to nil to simulate unavailable Redis
	originalRdb := redis.Rdb
	redis.Rdb = nil
	defer func() { redis.Rdb = originalRdb }()

	// Generate a valid token
	userID := "test-user-id"
	token, err := jwt.GenerateAccessToken("test-app-id", userID)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Setup router with full middleware
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request with valid token
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail with 500 due to Redis being unavailable
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500 when Redis is unavailable, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Check for the correct error message
	if !contains(w.Body.String(), "Token validation service unavailable") {
		t.Fatalf("Expected service unavailable error message, got: %s", w.Body.String())
	}
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Skip if Redis is not available
	if redis.Rdb == nil {
		t.Skip("Redis not available for testing")
	}

	// Test Redis connection
	ctx := redis.Rdb.Context()
	if _, err := redis.Rdb.Ping(ctx).Result(); err != nil {
		t.Skip("Redis connection failed, skipping test")
	}

	// Generate a valid token
	userID := "test-user-id"
	token, err := jwt.GenerateAccessToken("test-app-id", userID)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Setup router with middleware
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		// Get userID from context
		contextUserID, exists := c.Get("userID")
		if !exists {
			t.Fatal("Expected userID to be set in context")
		}

		if contextUserID != userID {
			t.Fatalf("Expected userID %s, got %s", userID, contextUserID)
		}

		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request with valid token
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddlewareBlacklistedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Skip if Redis is not available
	if redis.Rdb == nil {
		t.Skip("Redis not available for testing")
	}

	// Test Redis connection
	ctx := redis.Rdb.Context()
	if _, err := redis.Rdb.Ping(ctx).Result(); err != nil {
		t.Skip("Redis connection failed, skipping test")
	}

	// Generate a valid token
	userID := "test-user-id"
	token, err := jwt.GenerateAccessToken("test-app-id", userID)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Blacklist the token
	if err := redis.BlacklistAccessToken("test-app-id", token, userID, time.Hour); err != nil {
		t.Fatalf("Failed to blacklist token: %v", err)
	}

	// Setup router with middleware
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request with blacklisted token
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status code 401 for blacklisted token, got %d", w.Code)
	}

	// Verify the response contains the correct error message
	if !contains(w.Body.String(), "Token has been revoked") {
		t.Fatalf("Expected revoked token error message, got: %s", w.Body.String())
	}

	// Cleanup: remove the blacklisted token
	redis.Rdb.Del(ctx, "app:test-app-id:blacklist_token:"+token)
}

func TestAuthMiddlewareUserTokensBlacklisted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Skip if Redis is not available
	if redis.Rdb == nil {
		t.Skip("Redis not available for testing")
	}

	// Test Redis connection
	ctx := redis.Rdb.Context()
	if _, err := redis.Rdb.Ping(ctx).Result(); err != nil {
		t.Skip("Redis connection failed, skipping test")
	}

	// Generate a valid token
	userID := "test-user-id-" + time.Now().Format("20060102150405") // Unique userID for this test
	token, err := jwt.GenerateAccessToken("test-app-id", userID)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Blacklist all tokens for this user
	if err := redis.BlacklistAllUserTokens("test-app-id", userID, time.Hour); err != nil {
		t.Fatalf("Failed to blacklist user tokens: %v", err)
	}

	// Setup router with middleware
	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request with token from blacklisted user
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status code 401 for blacklisted user, got %d", w.Code)
	}

	// Verify the response contains the correct error message
	if !contains(w.Body.String(), "All user tokens have been revoked") {
		t.Fatalf("Expected user tokens revoked error message, got: %s", w.Body.String())
	}

	// Cleanup: remove the blacklisted user
	redis.Rdb.Del(ctx, "app:test-app-id:blacklist_user:"+userID)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAuthMiddlewareMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request without Authorization header
	req, _ := http.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status code 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request with invalid token
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status code 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareTokenWithoutBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Generate a valid token
	userID := "test-user-id"
	token, err := jwt.GenerateAccessToken("test-app-id", userID)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		// Get userID from context
		contextUserID, exists := c.Get("userID")
		if !exists {
			t.Fatal("Expected userID to be set in context")
		}

		if contextUserID != userID {
			t.Fatalf("Expected userID %s, got %s", userID, contextUserID)
		}

		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request with token but without "Bearer " prefix
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddlewareEmptyBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(AuthMiddleware())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request with "Bearer " but no token
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer ")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status code 401, got %d", w.Code)
	}
}

func TestAuthorizeRoleWithValidUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Middleware that sets userID in context (simulating AuthMiddleware)
	router.Use(func(c *gin.Context) {
		c.Set("userID", "test-user-id")
		c.Next()
	})

	router.Use(AuthorizeRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin access granted"})
	})

	req, _ := http.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}
}

func TestAuthorizeRoleWithoutUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(AuthorizeRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin access granted"})
	})

	req, _ := http.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500, got %d", w.Code)
	}
}

func TestAuthMiddlewareAndAuthorizeRoleTogether(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Generate a valid token
	userID := "test-user-id"
	token, err := jwt.GenerateAccessToken("test-app-id", userID)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	router := gin.New()
	router.Use(AuthMiddleware())
	router.Use(AuthorizeRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		// Verify userID is still accessible
		contextUserID, exists := c.Get("userID")
		if !exists {
			t.Fatal("Expected userID to be set in context")
		}

		if contextUserID != userID {
			t.Fatalf("Expected userID %s, got %s", userID, contextUserID)
		}

		c.JSON(http.StatusOK, gin.H{"message": "admin access granted"})
	})

	req, _ := http.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}
