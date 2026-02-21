package util

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetClientIP extracts the real client IP address from the request
func GetClientIP(c *gin.Context) string {
	// Check for X-Forwarded-For header (most common)
	forwarded := c.GetHeader("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, use the first one
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" && ip != "unknown" {
				return ip
			}
		}
	}

	// Check for X-Real-IP header
	realIP := c.GetHeader("X-Real-IP")
	if realIP != "" && realIP != "unknown" {
		return realIP
	}

	// Check for CF-Connecting-IP header (Cloudflare)
	cfIP := c.GetHeader("CF-Connecting-IP")
	if cfIP != "" && cfIP != "unknown" {
		return cfIP
	}

	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr // Return as is if parsing fails
	}

	return ip
}

// GetUserAgent extracts the User-Agent from the request
func GetUserAgent(c *gin.Context) string {
	userAgent := c.GetHeader("User-Agent")
	if userAgent == "" {
		return "Unknown"
	}
	return userAgent
}

// GetClientInfo returns both IP address and User-Agent
func GetClientInfo(c *gin.Context) (string, string) {
	return GetClientIP(c), GetUserAgent(c)
}
