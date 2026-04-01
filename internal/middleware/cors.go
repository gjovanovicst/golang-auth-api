package middleware

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// splitTrimmed splits a comma-separated string and trims whitespace from each item,
// filtering out any empty entries.
func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// CORSMiddleware creates and configures CORS middleware from Viper settings.
// All values are read once at startup; a server restart is required to apply changes.
func CORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins:     splitTrimmed(viper.GetString("CORS_ALLOWED_ORIGINS")),
		AllowMethods:     splitTrimmed(viper.GetString("CORS_ALLOWED_METHODS")),
		AllowHeaders:     splitTrimmed(viper.GetString("CORS_ALLOWED_HEADERS")),
		ExposeHeaders:    splitTrimmed(viper.GetString("CORS_EXPOSE_HEADERS")),
		AllowCredentials: viper.GetBool("CORS_ALLOW_CREDENTIALS"),
		MaxAge:           time.Duration(viper.GetInt("CORS_MAX_AGE_HOURS")) * time.Hour,
	}

	return cors.New(config)
}
