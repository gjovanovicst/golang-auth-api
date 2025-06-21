package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

		claims, err := jwt.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
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