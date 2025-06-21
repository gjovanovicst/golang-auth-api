package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	// Setup test configuration
	viper.Set("JWT_SECRET", "testsecret")
	viper.Set("ACCESS_TOKEN_EXPIRATION_MINUTES", 15)
	viper.Set("REFRESH_TOKEN_EXPIRATION_HOURS", 720)
	
	m.Run()
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Generate a valid token
	userID := "test-user-id"
	token, err := jwt.GenerateAccessToken(userID)
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
	token, err := jwt.GenerateAccessToken(userID)
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
	token, err := jwt.GenerateAccessToken(userID)
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