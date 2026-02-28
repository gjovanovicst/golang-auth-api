package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	// KeyTypeAdmin is the key type for admin API keys (/admin/* routes).
	KeyTypeAdmin = "admin"
	// KeyTypeApp is the key type for per-application API keys.
	KeyTypeApp = "app"

	// adminKeyPrefix is prepended to admin keys for visual identification.
	adminKeyPrefix = "ak_"
	// appKeyPrefix is prepended to app keys for visual identification.
	appKeyPrefix = "apk_"

	// keyRandomBytes is the number of random bytes (24 bytes = 48 hex chars = 192 bits entropy).
	keyRandomBytes = 24
)

// GenerateApiKey creates a new random API key for the given type.
// Returns (rawKey, hash, prefix, suffix).
// The rawKey should be shown to the user once and never stored.
func GenerateApiKey(keyType string) (rawKey, keyHash, keyPrefix, keySuffix string, err error) {
	// Generate random bytes
	randomBytes := make([]byte, keyRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Build the raw key with prefix
	prefix := adminKeyPrefix
	if keyType == KeyTypeApp {
		prefix = appKeyPrefix
	}
	rawKey = prefix + hex.EncodeToString(randomBytes)

	// SHA-256 hash for storage
	keyHash = HashApiKey(rawKey)

	// Extract display prefix (first 12 chars including the type prefix)
	keyPrefix = rawKey[:12]

	// Extract display suffix (last 4 chars)
	keySuffix = rawKey[len(rawKey)-4:]

	return rawKey, keyHash, keyPrefix, keySuffix, nil
}

// HashApiKey computes the SHA-256 hex digest of a raw API key.
func HashApiKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}
