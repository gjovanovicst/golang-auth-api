## Phase 6: API Endpoints and Middleware Implementation Plan

This phase focuses on defining the RESTful API endpoints for authentication and authorization, and implementing middleware for JWT authentication and authorization. A well-structured API and robust middleware are essential for secure and efficient communication between the client and the server.

### 6.1 API Endpoints

We will define a set of RESTful API endpoints to handle user registration, login, token refresh, social authentication, email verification, and password reset. The API will follow REST principles, using standard HTTP methods and status codes.

**Authentication Endpoints:**

| HTTP Method | Path                        | Description                                     | Handler Function (Conceptual)           |
|-------------|-----------------------------|-------------------------------------------------|-----------------------------------------|
| `POST`      | `/register`                 | Register a new user.                            | `user.Register`                         |
| `POST`      | `/login`                    | Authenticate user and issue JWTs.               | `user.Login`                            |
| `POST`      | `/refresh-token`            | Refresh access token using refresh token.       | `user.RefreshToken`                     |
| `GET`       | `/verify-email`             | Verify user email with token.                   | `user.VerifyEmail`                      |
| `POST`      | `/forgot-password`          | Request password reset link.                    | `user.ForgotPassword`                   |
| `POST`      | `/reset-password`           | Reset password with token.                      | `user.ResetPassword`                    |

**Social Authentication Endpoints:**

| HTTP Method | Path                        | Description                                     | Handler Function (Conceptual)           |
|-------------|-----------------------------|-------------------------------------------------|-----------------------------------------|
| `GET`       | `/auth/google/login`        | Initiate Google OAuth2 login.                   | `social.GoogleLogin`                    |
| `GET`       | `/auth/google/callback`     | Google OAuth2 callback.                         | `social.GoogleCallback`                 |
| `GET`       | `/auth/facebook/login`      | Initiate Facebook OAuth2 login.                 | `social.FacebookLogin`                  |
| `GET`       | `/auth/facebook/callback`   | Facebook OAuth2 callback.                       | `social.FacebookCallback`               |
| `GET`       | `/auth/github/login`        | Initiate GitHub OAuth2 login.                   | `social.GithubLogin`                    |
| `GET`       | `/auth/github/callback`     | GitHub OAuth2 callback.                         | `social.GithubCallback`                 |

**Protected Endpoints (Example):**

| HTTP Method | Path                        | Description                                     | Handler Function (Conceptual)           |
|-------------|-----------------------------|-------------------------------------------------|-----------------------------------------|
| `GET`       | `/profile`                  | Get user profile (requires authentication).     | `user.GetProfile`                       |

### 6.2 Middleware Implementation

Middleware functions will be used to handle cross-cutting concerns such as authentication, authorization, and error handling. Gin framework allows easy integration of middleware.

**JWT Authentication Middleware (`internal/middleware/auth.go`):**

This middleware will intercept incoming requests, extract the JWT from the `Authorization` header, validate it, and if valid, set the user information in the Gin context for subsequent handlers.

**Process Flow:**
1.  **Extract Token:** Get the JWT from the `Authorization` header (e.g., `Bearer <token>`).
2.  **Validate Token:** Use the `jwt.ParseToken` function to validate the token.
3.  **Extract Claims:** If valid, extract the `user_id` from the token claims.
4.  **Set Context:** Set the `user_id` in the Gin context (e.g., `c.Set("userID", claims.UserID)`). This makes the user ID accessible to subsequent handlers.
5.  **Call Next:** Call `c.Next()` to pass the request to the next handler in the chain.
6.  **Error Handling:** If the token is missing, invalid, or expired, return an appropriate HTTP error response (e.g., 401 Unauthorized, 403 Forbidden).

**Example Code Snippet (Conceptual):**

```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/your_username/your_project/pkg/errors"
	"github.com/your_username/your_project/pkg/jwt"
)

// AuthMiddleware authenticates requests using JWT
func AuthMiddleware() gin.HandlerFunc {
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

		claims, err := jwt.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("userID", claims.UserID)
		c.Next()
	}
}
```

**Authorization Middleware (Conceptual):**

This middleware can be used to check user roles or permissions if a more granular authorization system is required. For this initial plan, we will focus on authentication, but this is a placeholder for future expansion.

```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthorizeRole checks if the user has the required role
func AuthorizeRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Assuming userID is already set by AuthMiddleware
		userID, exists := c.Get("userID")
		if !exists {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in context"})
			return
		}

		// TODO: Fetch user roles from database or claims
		// For demonstration, let's assume a simple check
		// if userHasRole(userID.(string), requiredRole) {
		// 	c.Next()
		// } else {
		// 	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		// }

		// For now, just proceed if authenticated
		c.Next()
	}
}
```

### 6.3 Router Setup

The Gin router will be configured in `cmd/api/main.go` to define the API routes and apply middleware.

**Example `cmd/api/main.go` (Router Setup):**

```go
package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"github.com/your_username/your_project/internal/email"
	"github.com/your_username/your_project/internal/middleware"
	"github.com/your_username/your_project/internal/social"
	"github.com/your_username/your_project/internal/user"
	"github.com/your_username/your_project/internal/database"
	"github.com/your_username/your_project/internal/redis"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	// Initialize Viper for configuration management
	vip.AutomaticEnv() // Read environment variables
	vip.SetDefault("PORT", "8080")
	vip.SetDefault("ACCESS_TOKEN_EXPIRATION_MINUTES", 15)
	vip.SetDefault("REFRESH_TOKEN_EXPIRATION_HOURS", 720)

	// Connect to Database
	database.ConnectDatabase()
	database.MigrateDatabase()

	// Connect to Redis
	redis.ConnectRedis()

	// Initialize Services and Handlers
	userRepo := user.NewRepository(database.DB)
	emailService := email.NewService()
	userService := user.NewService(userRepo, emailService)
	userHandler := user.NewHandler(userService)

	socialRepo := social.NewRepository(database.DB)
	socialService := social.NewService(userRepo, socialRepo)
	socialHandler := social.NewHandler(socialService)

	// Setup Gin Router
	r := gin.Default()

	// Public routes
	public := r.Group("/")
	{
		public.POST("/register", userHandler.Register)
		public.POST("/login", userHandler.Login)
		public.POST("/refresh-token", userHandler.RefreshToken)
		public.GET("/verify-email", userHandler.VerifyEmail)
		public.POST("/forgot-password", userHandler.ForgotPassword)
		public.POST("/reset-password", userHandler.ResetPassword)

		// Social Auth Routes
		public.GET("/auth/google/login", socialHandler.GoogleLogin)
		public.GET("/auth/google/callback", socialHandler.GoogleCallback)
		public.GET("/auth/facebook/login", socialHandler.FacebookLogin)
		public.GET("/auth/facebook/callback", socialHandler.FacebookCallback)
		public.GET("/auth/github/login", socialHandler.GithubLogin)
		public.GET("/auth/github/callback", socialHandler.GithubCallback)
	}

	// Protected routes (require JWT authentication)
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/profile", userHandler.GetProfile) // Example protected route
		// Add other protected routes here
	}

	// Start the server
	port := viper.GetString("PORT")
	log.Printf("Server starting on port %s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
```

This phase establishes the API endpoints and integrates the necessary middleware for authentication and authorization, making the application functional and secure.

