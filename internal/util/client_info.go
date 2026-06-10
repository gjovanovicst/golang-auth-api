package util

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetClientIP extracts the real client IP address from the request.
func GetClientIP(c *gin.Context) string {
	forwarded := c.GetHeader("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" && ip != "unknown" {
				return ip
			}
		}
	}
	realIP := c.GetHeader("X-Real-IP")
	if realIP != "" && realIP != "unknown" {
		return realIP
	}
	cfIP := c.GetHeader("CF-Connecting-IP")
	if cfIP != "" && cfIP != "unknown" {
		return cfIP
	}
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return ip
}

// GetUserAgent extracts the User-Agent from the request.
func GetUserAgent(c *gin.Context) string {
	userAgent := c.GetHeader("User-Agent")
	if userAgent == "" {
		return "Unknown"
	}
	return userAgent
}

// GetClientInfo returns both IP address and User-Agent.
func GetClientInfo(c *gin.Context) (string, string) {
	return GetClientIP(c), GetUserAgent(c)
}

// DeviceFingerprint returns a SHA-256 hash of the client's IP + User-Agent,
// truncated to 8 hex chars. Same browser → same fingerprint; different
// browser/device → different fingerprint. Used for device-scoped SSO
// without cookies or frontend changes.
func DeviceFingerprint(c *gin.Context) string {
	ip := GetClientIP(c)
	ua := GetUserAgent(c)
	h := sha256.Sum256([]byte(ip + "|" + ua))
	return hex.EncodeToString(h[:4])
}
