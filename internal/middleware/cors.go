package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// CORSMiddleware creates and configures CORS middleware
func CORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000",       // React dev server
			"http://localhost:5173",       // Vite dev server
			"http://localhost:5174",       // Vite dev server
			"http://localhost:8080",       // API server itself
			"https://accounts.google.com", // Google OAuth
			"https://www.facebook.com",    // Facebook OAuth
			"https://github.com",          // GitHub OAuth
		},
		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"DELETE",
			"OPTIONS",
			"HEAD",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"Authorization",
			"Accept",
			"Cache-Control",
			"X-Requested-With",
			"X-App-ID",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"Access-Control-Allow-Origin",
			"Access-Control-Allow-Headers",
			"Content-Type",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	// In production, restrict origins to specific domains
	if viper.GetString("GIN_MODE") == "release" {
		// Get frontend URLs from environment
		frontendURL := viper.GetString("FRONTEND_URL")
		if frontendURL != "" {
			config.AllowOrigins = []string{
				frontendURL,
				"https://accounts.google.com",
				"https://www.facebook.com",
				"https://github.com",
			}
		}
	}

	return cors.New(config)
}
