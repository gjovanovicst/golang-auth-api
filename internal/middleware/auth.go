package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/rbac"
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
		c.Set("appID", claims.AppID)
		c.Set("roles", claims.Roles)
		c.Next()
	}
}

// AuthorizeRole checks if the authenticated user has at least one of the required roles.
// It first checks JWT claims (fast path), then falls back to the RBAC service (Redis/DB).
// If rbacService is nil, only JWT claims are checked.
func AuthorizeRole(rbacService *rbac.Service, requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in context"})
			return
		}

		appID, _ := c.Get("appID")

		// Fast path: check roles from JWT claims
		if rolesVal, ok := c.Get("roles"); ok && rolesVal != nil {
			if jwtRoles, ok := rolesVal.([]string); ok && len(jwtRoles) > 0 {
				for _, required := range requiredRoles {
					for _, role := range jwtRoles {
						if role == required {
							c.Next()
							return
						}
					}
				}
			}
		}

		// Fallback: check RBAC service (Redis cache → DB)
		if rbacService != nil && appID != nil {
			hasRole, err := rbacService.HasRole(appID.(string), userID.(string), requiredRoles...)
			if err == nil && hasRole {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient role permissions"})
	}
}

// AuthorizePermission checks if the authenticated user has a specific permission
// (resource:action format, e.g. "user:read", "log:delete").
// Always uses the RBAC service for resolution (Redis cache → DB).
func AuthorizePermission(rbacService *rbac.Service, resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in context"})
			return
		}

		appID, _ := c.Get("appID")

		if rbacService == nil || appID == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Authorization service not available"})
			return
		}

		hasPerm, err := rbacService.HasPermission(appID.(string), userID.(string), resource, action)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Authorization check failed"})
			return
		}

		if !hasPerm {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			return
		}

		c.Next()
	}
}
