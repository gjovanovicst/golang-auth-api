package middleware

import (
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// CORSMiddleware creates and configures CORS middleware
func CORSMiddleware() gin.HandlerFunc {
	// OAuth provider origins are always allowed (they initiate callbacks)
	oauthOrigins := []string{
		"https://accounts.google.com",
		"https://www.facebook.com",
		"https://github.com",
	}

	config := cors.Config{
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

	if viper.GetString("GIN_MODE") == "release" {
		// Production: only allow explicitly configured frontend + OAuth origins
		frontendURL := viper.GetString("FRONTEND_URL")
		if frontendURL != "" {
			config.AllowOrigins = append([]string{frontendURL}, oauthOrigins...)
		} else {
			log.Println("WARNING: GIN_MODE=release but FRONTEND_URL is not set — CORS will only allow OAuth provider origins. Set FRONTEND_URL to your frontend domain.")
			config.AllowOrigins = oauthOrigins
		}
	} else {
		// Development: allow localhost dev servers + OAuth origins
		config.AllowOrigins = append([]string{
			"http://localhost:3000", // React dev server
			"http://localhost:5173", // Vite dev server
			"http://localhost:5174", // Vite dev server
			"http://localhost:8080", // API server itself
		}, oauthOrigins...)
	}

	return cors.New(config)
}
