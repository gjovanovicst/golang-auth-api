package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
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

		// Parse and validate JWT
		claims, err := jwt.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Reject refresh tokens used as access tokens.
		// Empty TokenType is allowed for backward compatibility with pre-existing tokens.
		if claims.TokenType != "" && claims.TokenType != jwt.TokenTypeAccess {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token type"})
			return
		}

		// Check Redis blacklists only if Redis is available
		if redis.Rdb != nil {
			// Check if the specific access token is blacklisted
			blacklisted, err := redis.IsAccessTokenBlacklisted(claims.AppID, tokenString)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Token validation error"})
				return
			}
			if blacklisted {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has been revoked"})
				return
			}

			// Check if all tokens for this user are blacklisted (e.g., after password change)
			userBlacklisted, err := redis.IsUserTokensBlacklisted(claims.AppID, claims.UserID)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Token validation error"})
				return
			}
			if userBlacklisted {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "All user tokens have been revoked"})
				return
			}
		} else {
			// Redis not available - log warning in production, but allow for testing
			// In production, this should be treated as an error
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Token validation service unavailable"})
			return
		}

		c.Set("userID", claims.UserID)
		c.Next()
	}
}

// AuthorizeRole checks if the user has the required role
func AuthorizeRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Assuming userID is already set by AuthMiddleware
		_, exists := c.Get("userID")
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
