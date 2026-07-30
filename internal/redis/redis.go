package redis

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

var Rdb *redis.Client

// pubsubRdb is a dedicated Redis client used exclusively for pub/sub subscriptions.
// go-redis pub/sub connections are NOT returned to the shared pool — they hold
// their connection for the entire subscription lifetime.  Using a separate client
// prevents SSE subscribers from exhausting the command pool used by every other
// auth operation (session lookups, token blacklisting, etc.).
var pubsubRdb *redis.Client

var ctx = context.Background()

func ConnectRedis() {
	opts := &redis.Options{
		Addr:     viper.GetString("REDIS_ADDR"),
		Password: viper.GetString("REDIS_PASSWORD"),
		DB:       viper.GetInt("REDIS_DB"),
		// Generous pool for the command client: handles all non-SSE operations.
		PoolSize:    20,
		MinIdleConns: 5,
	}
	Rdb = redis.NewClient(opts)

	_, err := Rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	// Separate client for pub/sub only — pool size = max expected concurrent SSE
	// connections across all apps.  Each subscriber holds exactly one connection.
	pubsubRdb = redis.NewClient(&redis.Options{
		Addr:     viper.GetString("REDIS_ADDR"),
		Password: viper.GetString("REDIS_PASSWORD"),
		DB:       viper.GetInt("REDIS_DB"),
		PoolSize: 50, // allow up to 50 simultaneous SSE connections
	})

	if _, err := pubsubRdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("Could not connect to Redis (pubsub client): %v", err)
	}

	log.Println("Connected to Redis!")
}

// SetRefreshToken stores a refresh token with its expiration
func SetRefreshToken(appID, userID, token string) error {
	key := fmt.Sprintf("app:%s:refresh_token:%s", appID, userID)
	expiration := time.Hour * time.Duration(viper.GetInt("REFRESH_TOKEN_EXPIRATION_HOURS"))
	return Rdb.Set(ctx, key, token, expiration).Err()
}

// GetRefreshToken retrieves a refresh token
func GetRefreshToken(appID, userID string) (string, error) {
	key := fmt.Sprintf("app:%s:refresh_token:%s", appID, userID)
	return Rdb.Get(ctx, key).Result()
}

// RevokeRefreshToken deletes a refresh token (effectively blacklisting it)
func RevokeRefreshToken(appID, userID, token string) error {
	// For simplicity, we'll just delete the token associated with the user ID.
	// A more robust solution might involve a blacklist set for specific tokens.
	key := fmt.Sprintf("app:%s:refresh_token:%s", appID, userID)
	val, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil // Token already gone or never existed
	} else if err != nil {
		return err
	}

	if val == token {
		return Rdb.Del(ctx, key).Err()
	}
	return nil // Token found but doesn't match, might be an older token
}

// IsRefreshTokenRevoked checks if a refresh token is revoked (by checking if it exists)
func IsRefreshTokenRevoked(appID, userID, token string) (bool, error) {
	key := fmt.Sprintf("app:%s:refresh_token:%s", appID, userID)
	val, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return true, nil // Token not found, so it's considered revoked or expired
	} else if err != nil {
		return false, err
	}
	return val != token, nil // If value doesn't match, it means a new token was issued, old one is implicitly revoked
}

// SetEmailVerificationToken stores an email verification token and a reverse lookup key (userID → token).
// The reverse lookup allows invalidating old tokens when a new one is issued.
func SetEmailVerificationToken(appID, userID, token string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:email_verify:%s", appID, token)
	if err := Rdb.Set(ctx, key, userID, expiration).Err(); err != nil {
		return err
	}
	// Store reverse lookup: userID → token (so we can find and invalidate old tokens)
	reverseKey := fmt.Sprintf("app:%s:email_verify_user:%s", appID, userID)
	return Rdb.Set(ctx, reverseKey, token, expiration).Err()
}

// GetEmailVerificationToken retrieves an email verification token
func GetEmailVerificationToken(appID, token string) (string, error) {
	key := fmt.Sprintf("app:%s:email_verify:%s", appID, token)
	return Rdb.Get(ctx, key).Result()
}

// GetEmailVerificationTokenByUserID retrieves the current verification token for a user (reverse lookup).
func GetEmailVerificationTokenByUserID(appID, userID string) (string, error) {
	key := fmt.Sprintf("app:%s:email_verify_user:%s", appID, userID)
	return Rdb.Get(ctx, key).Result()
}

// DeleteEmailVerificationToken deletes an email verification token and its reverse lookup key.
func DeleteEmailVerificationToken(appID, token string) error {
	key := fmt.Sprintf("app:%s:email_verify:%s", appID, token)
	// Look up the userID so we can also clean up the reverse key
	userID, err := Rdb.Get(ctx, key).Result()
	if err == nil && userID != "" {
		reverseKey := fmt.Sprintf("app:%s:email_verify_user:%s", appID, userID)
		Rdb.Del(ctx, reverseKey) // Best-effort cleanup
	}
	return Rdb.Del(ctx, key).Err()
}

// SetPasswordResetToken stores a password reset token
func SetPasswordResetToken(appID, userID, token string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:password_reset:%s", appID, token)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// GetPasswordResetToken retrieves a password reset token
func GetPasswordResetToken(appID, token string) (string, error) {
	key := fmt.Sprintf("app:%s:password_reset:%s", appID, token)
	return Rdb.Get(ctx, key).Result()
}

// DeletePasswordResetToken deletes a password reset token
func DeletePasswordResetToken(appID, token string) error {
	key := fmt.Sprintf("app:%s:password_reset:%s", appID, token)
	return Rdb.Del(ctx, key).Err()
}

// Magic Link related functions

// SetMagicLinkToken stores a magic link token and a reverse lookup key (userID → token).
// The reverse lookup allows invalidating old tokens when a new one is issued.
func SetMagicLinkToken(appID, userID, token string, expiration time.Duration) error {
	// Invalidate any existing magic link token for this user (only one active at a time)
	reverseKey := fmt.Sprintf("app:%s:magic_link_user:%s", appID, userID)
	oldToken, err := Rdb.Get(ctx, reverseKey).Result()
	if err == nil && oldToken != "" {
		oldKey := fmt.Sprintf("app:%s:magic_link:%s", appID, oldToken)
		Rdb.Del(ctx, oldKey) // Best-effort cleanup of old token
	}

	// Store token → userID mapping
	key := fmt.Sprintf("app:%s:magic_link:%s", appID, token)
	if err := Rdb.Set(ctx, key, userID, expiration).Err(); err != nil {
		return err
	}
	// Store reverse lookup: userID → token
	return Rdb.Set(ctx, reverseKey, token, expiration).Err()
}

// GetMagicLinkToken retrieves the userID associated with a magic link token
func GetMagicLinkToken(appID, token string) (string, error) {
	key := fmt.Sprintf("app:%s:magic_link:%s", appID, token)
	return Rdb.Get(ctx, key).Result()
}

// DeleteMagicLinkToken deletes a magic link token and its reverse lookup key (single-use).
func DeleteMagicLinkToken(appID, token string) error {
	key := fmt.Sprintf("app:%s:magic_link:%s", appID, token)
	// Look up the userID so we can also clean up the reverse key
	userID, err := Rdb.Get(ctx, key).Result()
	if err == nil && userID != "" {
		reverseKey := fmt.Sprintf("app:%s:magic_link_user:%s", appID, userID)
		Rdb.Del(ctx, reverseKey) // Best-effort cleanup
	}
	return Rdb.Del(ctx, key).Err()
}

// 2FA related functions

// SetTempTwoFASecret stores a temporary 2FA secret during setup
func SetTempTwoFASecret(appID, userID, secret string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:temp_2fa_secret:%s", appID, userID)
	return Rdb.Set(ctx, key, secret, expiration).Err()
}

// GetTempTwoFASecret retrieves a temporary 2FA secret
func GetTempTwoFASecret(appID, userID string) (string, error) {
	key := fmt.Sprintf("app:%s:temp_2fa_secret:%s", appID, userID)
	return Rdb.Get(ctx, key).Result()
}

// DeleteTempTwoFASecret deletes a temporary 2FA secret
func DeleteTempTwoFASecret(appID, userID string) error {
	key := fmt.Sprintf("app:%s:temp_2fa_secret:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// SetTempUserSession stores a temporary user session for 2FA login
func SetTempUserSession(appID, tempToken, userID string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:temp_session:%s", appID, tempToken)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// GetTempUserSession retrieves a temporary user session
func GetTempUserSession(appID, tempToken string) (string, error) {
	key := fmt.Sprintf("app:%s:temp_session:%s", appID, tempToken)
	return Rdb.Get(ctx, key).Result()
}

// DeleteTempUserSession deletes a temporary user session
func DeleteTempUserSession(appID, tempToken string) error {
	key := fmt.Sprintf("app:%s:temp_session:%s", appID, tempToken)
	return Rdb.Del(ctx, key).Err()
}

// Access Token Blacklisting Functions

// BlacklistAccessToken adds an access token to the blacklist with its remaining TTL
func BlacklistAccessToken(appID, tokenString string, userID string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:blacklist_token:%s", appID, tokenString)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// IsAccessTokenBlacklisted checks if an access token is blacklisted
func IsAccessTokenBlacklisted(appID, tokenString string) (bool, error) {
	key := fmt.Sprintf("app:%s:blacklist_token:%s", appID, tokenString)
	_, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // Token not found in blacklist
	} else if err != nil {
		return false, err // Redis error
	}
	return true, nil // Token found in blacklist
}

// BlacklistAllUserTokens blacklists all tokens for a specific user (useful for password changes, account compromise)
func BlacklistAllUserTokens(appID, userID string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:blacklist_user:%s", appID, userID)
	return Rdb.Set(ctx, key, "all_tokens_revoked", expiration).Err()
}

// IsUserTokensBlacklisted checks if all tokens for a user are blacklisted
func IsUserTokensBlacklisted(appID, userID string) (bool, error) {
	key := fmt.Sprintf("app:%s:blacklist_user:%s", appID, userID)
	_, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // User tokens not blacklisted
	} else if err != nil {
		return false, err // Redis error
	}
	return true, nil // All user tokens are blacklisted
}

// ClearUserTokenBlacklist removes the user-wide token blacklist entry.
// Called when a user successfully authenticates with fresh credentials (e.g. new login
// after a password reset) so that newly issued tokens are not blocked by the stale
// post-reset blacklist.
func ClearUserTokenBlacklist(appID, userID string) error {
	key := fmt.Sprintf("app:%s:blacklist_user:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// ScanAndClearAllUserBlacklists removes every app-scoped blacklist_user key for the
// given userID across ALL applications. This is a SCAN-based sweep used on login to
// guarantee that stale revocation entries left over from previous force-logouts,
// session-group expiries, or container restarts cannot block the newly authenticated
// user — regardless of which apps are currently in a session group.
//
// The scan uses the pattern "app:*:blacklist_user:{userID}" and deletes in batches.
// Cost: one SCAN round-trip per 100 keys in the keyspace (typically a single round-trip
// in dev/small environments). Safe to call asynchronously.
func ScanAndClearAllUserBlacklists(userID string) error {
	pattern := fmt.Sprintf("app:*:blacklist_user:%s", userID)
	var cursor uint64
	for {
		keys, nextCursor, err := Rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := Rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// ==================== Session Management Functions ====================

// CreateSession stores a new session as a Redis Hash with metadata.
// Key pattern: app:{appID}:session:{sessionID}
// Also adds the sessionID to the user's session index set.
func CreateSession(appID, sessionID, userID, refreshToken, ip, userAgent, deviceID string, ttl time.Duration) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	fields := map[string]interface{}{
		"user_id":       userID,
		"refresh_token": refreshToken,
		"ip":            ip,
		"user_agent":    userAgent,
		"device_id":     deviceID,
		"created_at":    time.Now().UTC().Format(time.RFC3339),
		"last_active":   time.Now().UTC().Format(time.RFC3339),
	}
	if err := Rdb.HSet(ctx, key, fields).Err(); err != nil {
		return err
	}
	if err := Rdb.Expire(ctx, key, ttl).Err(); err != nil {
		return err
	}
	// Add to user session index
	indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
	if err := Rdb.SAdd(ctx, indexKey, sessionID).Err(); err != nil {
		return err
	}
	// Set a generous TTL on the index (longer than any single session) to prevent stale keys
	Rdb.Expire(ctx, indexKey, ttl+24*time.Hour)

	// Add to app-level session index (for admin dashboard enumeration)
	appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
	Rdb.SAdd(ctx, appIndexKey, sessionID)
	Rdb.Expire(ctx, appIndexKey, ttl+24*time.Hour)

	// Store session metadata for expiration detection
	metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sessionID)
	if err := Rdb.Set(ctx, metaKey, "1", ttl).Err(); err != nil {
		// Log but don't fail session creation
		log.Printf("Warning: Failed to create session metadata key: %v", err)
	}

	return nil
}

// GetSession retrieves all fields of a session hash.
func GetSession(appID, sessionID string) (map[string]string, error) {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	result, err := Rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, redis.Nil
	}
	return result, nil
}

// GetSessionField retrieves a single field from a session hash.
func GetSessionField(appID, sessionID, field string) (string, error) {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	return Rdb.HGet(ctx, key, field).Result()
}

// GetSessionRefreshToken retrieves only the refresh_token field from a session.
func GetSessionRefreshToken(appID, sessionID string) (string, error) {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	return Rdb.HGet(ctx, key, "refresh_token").Result()
}

// UpdateSessionRefreshToken updates the refresh token stored in a session hash.
func UpdateSessionRefreshToken(appID, sessionID, newRefreshToken string) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	return Rdb.HSet(ctx, key, "refresh_token", newRefreshToken).Err()
}

// ResetSessionTTL resets the TTL on a session hash key.
// Call this on every token rotation so the session lifetime slides forward
// with the newly issued refresh token instead of expiring at the original login time.
func ResetSessionTTL(appID, sessionID string, ttl time.Duration) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	return Rdb.Expire(ctx, key, ttl).Err()
}

// ResetSessionMetaTTL slides the session_meta key TTL forward on every token rotation.
// The session_meta key is used by keyspace-notification-based session group expiry
// revocation (SESSION_GROUP_EXPIRY_REVOCATION_ENABLED). Without this reset, the key
// expires at the original login time even if the user continues to actively refresh
// their tokens, which can cause premature cross-app session revocation.
func ResetSessionMetaTTL(appID, userID, sessionID string, ttl time.Duration) error {
	metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sessionID)
	// Use SET with KEEPTTL-compatible approach: only reset if the key exists.
	// If the key was already deleted (session revoked), this is a no-op.
	result, err := Rdb.Expire(ctx, metaKey, ttl).Result()
	if err != nil {
		return err
	}
	if !result {
		// Key does not exist — session may have been revoked; nothing to reset.
		return nil
	}
	return nil
}

// TouchSession updates the last_active timestamp of a session.
func TouchSession(appID, sessionID string) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	return Rdb.HSet(ctx, key, "last_active", time.Now().UTC().Format(time.RFC3339)).Err()
}

// TouchSessionThrottled updates the last_active timestamp only if it has not been
// updated within minInterval. This is safe to call on every authenticated request
// without flooding Redis with writes — at most one write per session per minInterval.
// Returns (true, nil) if the timestamp was updated, (false, nil) if skipped (too soon),
// and (false, err) if a Redis error occurred.
func TouchSessionThrottled(appID, sessionID string, minInterval time.Duration) (bool, error) {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	lastActiveStr, err := Rdb.HGet(ctx, key, "last_active").Result()
	if err != nil {
		// Key missing (session deleted) or Redis error — skip touch, let middleware
		// handle the missing session on its own SessionExists check.
		return false, nil
	}
	if lastActive, parseErr := time.Parse(time.RFC3339, lastActiveStr); parseErr == nil {
		if time.Since(lastActive) < minInterval {
			return false, nil // updated recently enough — skip
		}
	}
	if err := Rdb.HSet(ctx, key, "last_active", time.Now().UTC().Format(time.RFC3339)).Err(); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteSession removes a session hash and removes it from the user and app session indexes.
func DeleteSession(appID, sessionID, userID string) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	if err := Rdb.Del(ctx, key).Err(); err != nil {
		return err
	}
	// Remove from user session index
	indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
	Rdb.SRem(ctx, indexKey, sessionID)
	// Remove from app-level session index
	appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
	Rdb.SRem(ctx, appIndexKey, sessionID)
	// Delete session metadata key
	metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sessionID)
	Rdb.Del(ctx, metaKey)
	return nil
}

// GetUserSessionIDs returns all session IDs for a user from the session index set.
// It performs lazy cleanup: any session ID in the set that no longer exists in Redis is removed.
func GetUserSessionIDs(appID, userID string) ([]string, error) {
	indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
	sessionIDs, err := Rdb.SMembers(ctx, indexKey).Result()
	if err != nil {
		return nil, err
	}

	// Lazy cleanup: verify each session still exists
	var validIDs []string
	for _, sid := range sessionIDs {
		sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sid)
		exists, err := Rdb.Exists(ctx, sessionKey).Result()
		if err != nil {
			continue // Skip on error, don't remove
		}
		if exists == 0 {
			// Session expired, remove from index
			Rdb.SRem(ctx, indexKey, sid)
			continue
		}
		validIDs = append(validIDs, sid)
	}

	return validIDs, nil
}

// CleanupStaleUserSessions removes all stale session artifacts for a user in an app.
// A session is stale when its session hash no longer exists in Redis (expired naturally
// or explicitly deleted) but its ID still lingers in the user_sessions index set or an
// orphaned session_meta key remains. This function:
//   - Removes stale session IDs from the user_sessions index set
//   - Removes stale session IDs from the all_sessions index set
//   - Deletes orphaned session_meta keys
//
// Called at login time before creating a new session, so that orphaned session_meta
// keys from prior expired sessions cannot later fire (via keyspace notification or
// periodic scanner) and trigger cascading group-wide revocation via handleExpiredKey.
func CleanupStaleUserSessions(appID, userID string) (cleaned int, err error) {
	indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
	sessionIDs, smErr := Rdb.SMembers(ctx, indexKey).Result()
	if smErr != nil {
		// Redis returns an empty set (not an error) when the key doesn't exist,
		// but if the key is not a set, SMembers returns WRONGTYPE. In that case
		// or any other error, treat as "nothing to clean".
		if strings.Contains(smErr.Error(), "nil") || strings.Contains(smErr.Error(), "WRONGTYPE") {
			return 0, nil
		}
		return 0, smErr
	}

	appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)

	for _, sid := range sessionIDs {
		sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sid)
		exists, _ := Rdb.Exists(ctx, sessionKey).Result()
		if exists == 0 {
			// Session hash is gone — clean up all related artifacts
			Rdb.SRem(ctx, indexKey, sid)
			Rdb.SRem(ctx, appIndexKey, sid)
			metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sid)
			Rdb.Del(ctx, metaKey)
			cleaned++
		}
	}
	return cleaned, nil
}

// DeleteAllUserSessions removes all sessions for a user except the one specified by exceptSessionID.
// If exceptSessionID is empty, all sessions are removed.
func DeleteAllUserSessions(appID, userID, exceptSessionID string) error {
	sessionIDs, err := GetUserSessionIDs(appID, userID)
	if err != nil {
		return err
	}

	for _, sid := range sessionIDs {
		if sid == exceptSessionID {
			continue
		}
		sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sid)
		Rdb.Del(ctx, sessionKey)
		// Remove from app-level session index
		appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
		Rdb.SRem(ctx, appIndexKey, sid)
		// Delete the session_meta key so it does not become an orphan that
		// later expires via keyspace notification and triggers a cascading
		// group-wide revocation of the user's brand-new replacement session.
		metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sid)
		Rdb.Del(ctx, metaKey)
	}

	// Clean up the index
	indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
	if exceptSessionID == "" {
		Rdb.Del(ctx, indexKey)
	} else {
		// Rebuild the set with only the excepted session
		Rdb.Del(ctx, indexKey)
		Rdb.SAdd(ctx, indexKey, exceptSessionID)
	}

	return nil
}

// DeleteUserSessionsByDeviceExcept deletes sessions for a user in an app whose
// device_id matches the given deviceID, EXCEPT the session identified by
// exceptSessionID. Use this after creating a new session to clean up any prior
// session on the same device without deleting the one just created.
func DeleteUserSessionsByDeviceExcept(appID, userID, deviceID, exceptSessionID string) error {
	if deviceID == "" {
		return nil
	}
	sessionIDs, err := GetUserSessionIDs(appID, userID)
	if err != nil {
		return err
	}
	for _, sid := range sessionIDs {
		if sid == exceptSessionID {
			continue
		}
		sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sid)
		did, err := Rdb.HGet(ctx, sessionKey, "device_id").Result()
		if err != nil {
			continue
		}
		if did == deviceID {
			Rdb.Del(ctx, sessionKey)
			appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
			Rdb.SRem(ctx, appIndexKey, sid)
			metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sid)
			Rdb.Del(ctx, metaKey)
			// Also remove from the user_sessions index set
			indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
			Rdb.SRem(ctx, indexKey, sid)
		}
	}
	return nil
}

// DeleteUserSessionsByDevice deletes sessions for a user in an app, but only
// those whose device_id field matches the given deviceID. Sessions with an
// empty device_id are only deleted during broadcast revocation (deviceID="").
func DeleteUserSessionsByDevice(appID, userID, deviceID string) error {
	sessionIDs, err := GetUserSessionIDs(appID, userID)
	if err != nil {
		return err
	}
	for _, sid := range sessionIDs {
		sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sid)
		did, err := Rdb.HGet(ctx, sessionKey, "device_id").Result()
		if err != nil || did == "" {
			// Pre-migration session or missing field: only delete when
			// broadcast-revoking (deviceID is empty).
			if deviceID == "" {
				Rdb.Del(ctx, sessionKey)
				appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
				Rdb.SRem(ctx, appIndexKey, sid)
				indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
				Rdb.SRem(ctx, indexKey, sid)
				// Clean up session_meta to prevent orphaned keys from
				// triggering cascading group revocations on expiry.
				metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sid)
				Rdb.Del(ctx, metaKey)
			}
			continue
		}
		if did == deviceID {
			Rdb.Del(ctx, sessionKey)
			appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
			Rdb.SRem(ctx, appIndexKey, sid)
			indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
			Rdb.SRem(ctx, indexKey, sid)
			// Clean up session_meta to prevent orphaned keys.
			metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sid)
			Rdb.Del(ctx, metaKey)
		}
	}
	if deviceID == "" {
		indexKey := fmt.Sprintf("app:%s:user_sessions:%s", appID, userID)
		Rdb.Del(ctx, indexKey)
	}
	return nil
}

// SetSessionOrgContext stores the active org_id and org_role in the session hash.
// Called by the /auth/reissue handler after a context-switch so that subsequent
// token refreshes can re-embed the same org context.
func SetSessionOrgContext(appID, sessionID, orgID, orgRole string) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	return Rdb.HSet(ctx, key, map[string]interface{}{
		"org_id":   orgID,
		"org_role": orgRole,
	}).Err()
}

// GetSessionOrgContext reads the org_id and org_role stored in the session hash.
// Returns empty strings when no org context has been set.
func GetSessionOrgContext(appID, sessionID string) (orgID, orgRole string, err error) {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	vals, err := Rdb.HMGet(ctx, key, "org_id", "org_role").Result()
	if err != nil {
		return "", "", err
	}
	if vals[0] != nil {
		orgID, _ = vals[0].(string)
	}
	if vals[1] != nil {
		orgRole, _ = vals[1].(string)
	}
	return orgID, orgRole, nil
}

// ClearSessionOrgContext removes org context from the session hash (e.g. on explicit context-clear).
func ClearSessionOrgContext(appID, sessionID string) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	return Rdb.HDel(ctx, key, "org_id", "org_role").Err()
}

// SessionExists checks whether a session hash key exists in Redis.
func SessionExists(appID, sessionID string) (bool, error) {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	exists, err := Rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// GetAppSessionIDs returns all session IDs for an app from the app-level session index.
// Performs lazy cleanup: removes IDs whose session hash has expired.
func GetAppSessionIDs(appID string) ([]string, error) {
	indexKey := fmt.Sprintf("app:%s:all_sessions", appID)
	sessionIDs, err := Rdb.SMembers(ctx, indexKey).Result()
	if err != nil {
		return nil, err
	}

	var validIDs []string
	for _, sid := range sessionIDs {
		sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sid)
		exists, err := Rdb.Exists(ctx, sessionKey).Result()
		if err != nil {
			continue
		}
		if exists == 0 {
			Rdb.SRem(ctx, indexKey, sid)
			continue
		}
		validIDs = append(validIDs, sid)
	}
	return validIDs, nil
}

// CountAppSessions returns the count of entries in the app-level session index.
// Note: may include stale entries until lazy cleanup runs via GetAppSessionIDs.
func CountAppSessions(appID string) (int64, error) {
	indexKey := fmt.Sprintf("app:%s:all_sessions", appID)
	return Rdb.SCard(ctx, indexKey).Result()
}

// GetAllSessionsForApp returns full session metadata for all active sessions in an app.
// Each returned map contains: session_id, user_id, ip, user_agent, created_at, last_active.
// The refresh_token field is intentionally excluded for security.
func GetAllSessionsForApp(appID string) ([]map[string]string, error) {
	sessionIDs, err := GetAppSessionIDs(appID)
	if err != nil {
		return nil, err
	}

	var sessions []map[string]string
	for _, sid := range sessionIDs {
		data, err := GetSession(appID, sid)
		if err != nil {
			continue
		}
		data["session_id"] = sid
		// Remove refresh_token from admin-visible data
		delete(data, "refresh_token")
		sessions = append(sessions, data)
	}
	return sessions, nil
}

// Admin Session Functions

// SetAdminSession stores an admin session in Redis
func SetAdminSession(sessionID, adminID string, expiration time.Duration) error {
	key := fmt.Sprintf("admin:session:%s", sessionID)
	return Rdb.Set(ctx, key, adminID, expiration).Err()
}

// GetAdminSession retrieves an admin session from Redis, returning the admin ID
func GetAdminSession(sessionID string) (string, error) {
	key := fmt.Sprintf("admin:session:%s", sessionID)
	return Rdb.Get(ctx, key).Result()
}

// DeleteAdminSession removes an admin session from Redis
func DeleteAdminSession(sessionID string) error {
	key := fmt.Sprintf("admin:session:%s", sessionID)
	return Rdb.Del(ctx, key).Err()
}

// Admin CSRF Functions

// SetCSRFToken stores a CSRF token for an admin session
func SetCSRFToken(sessionID, token string, expiration time.Duration) error {
	key := fmt.Sprintf("admin:csrf:%s", sessionID)
	return Rdb.Set(ctx, key, token, expiration).Err()
}

// GetCSRFToken retrieves the CSRF token for an admin session
func GetCSRFToken(sessionID string) (string, error) {
	key := fmt.Sprintf("admin:csrf:%s", sessionID)
	return Rdb.Get(ctx, key).Result()
}

// Admin Login Rate Limiting Functions

// IncrLoginAttempts increments the login attempt counter for an IP and sets a 60-second TTL.
// Returns the new count after increment.
func IncrLoginAttempts(ip string) (int64, error) {
	key := fmt.Sprintf("admin:login_attempts:%s", ip)
	count, err := Rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Set TTL only on first attempt (when count == 1)
	if count == 1 {
		Rdb.Expire(ctx, key, 60*time.Second)
	}
	return count, nil
}

// GetLoginAttempts returns the current login attempt count for an IP
func GetLoginAttempts(ip string) (int64, error) {
	key := fmt.Sprintf("admin:login_attempts:%s", ip)
	count, err := Rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// SetLoginLockout sets a lockout flag for an IP with the given expiration
func SetLoginLockout(ip string, expiration time.Duration) error {
	key := fmt.Sprintf("admin:login_lockout:%s", ip)
	return Rdb.Set(ctx, key, "locked", expiration).Err()
}

// IsLoginLocked checks if an IP is currently locked out
func IsLoginLocked(ip string) (bool, error) {
	key := fmt.Sprintf("admin:login_lockout:%s", ip)
	_, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// ClearLoginAttempts removes the attempt counter and lockout for an IP (called on successful login)
func ClearLoginAttempts(ip string) error {
	attemptsKey := fmt.Sprintf("admin:login_attempts:%s", ip)
	lockoutKey := fmt.Sprintf("admin:login_lockout:%s", ip)
	return Rdb.Del(ctx, attemptsKey, lockoutKey).Err()
}

// Email 2FA Code Functions

// Set2FAEmailCode stores a 2FA email verification code with a 5-minute expiration.
func Set2FAEmailCode(appID, userID, code string) error {
	key := fmt.Sprintf("app:%s:2fa_email:%s", appID, userID)
	return Rdb.Set(ctx, key, code, 5*time.Minute).Err()
}

// Get2FAEmailCode retrieves a stored 2FA email verification code.
func Get2FAEmailCode(appID, userID string) (string, error) {
	key := fmt.Sprintf("app:%s:2fa_email:%s", appID, userID)
	return Rdb.Get(ctx, key).Result()
}

// Delete2FAEmailCode removes a 2FA email verification code after successful verification.
func Delete2FAEmailCode(appID, userID string) error {
	key := fmt.Sprintf("app:%s:2fa_email:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// ClearRateLimitKeys removes the generic rate-limit attempt counter and lockout
// for a given prefix + identifier. Used by the generic RateLimitMiddleware.
func ClearRateLimitKeys(keyPrefix, identifier string) error {
	attemptsKey := fmt.Sprintf("rl:%s:attempts:%s", keyPrefix, identifier)
	lockoutKey := fmt.Sprintf("rl:%s:lockout:%s", keyPrefix, identifier)
	return Rdb.Del(ctx, attemptsKey, lockoutKey).Err()
}

// WebAuthn Challenge Functions

// SetWebAuthnRegistrationChallenge stores a WebAuthn registration challenge session in Redis.
func SetWebAuthnRegistrationChallenge(appID, userID, sessionJSON string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:webauthn_reg:%s", appID, userID)
	return Rdb.Set(ctx, key, sessionJSON, expiration).Err()
}

// GetWebAuthnRegistrationChallenge retrieves a WebAuthn registration challenge session from Redis.
func GetWebAuthnRegistrationChallenge(appID, userID string) (string, error) {
	key := fmt.Sprintf("app:%s:webauthn_reg:%s", appID, userID)
	return Rdb.Get(ctx, key).Result()
}

// DeleteWebAuthnRegistrationChallenge removes a WebAuthn registration challenge session from Redis.
func DeleteWebAuthnRegistrationChallenge(appID, userID string) error {
	key := fmt.Sprintf("app:%s:webauthn_reg:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// SetWebAuthnLoginChallenge stores a WebAuthn login/assertion challenge session in Redis.
// The identifier can be a userID (for 2FA) or a sessionID (for passwordless).
func SetWebAuthnLoginChallenge(appID, identifier, sessionJSON string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:webauthn_login:%s", appID, identifier)
	return Rdb.Set(ctx, key, sessionJSON, expiration).Err()
}

// GetWebAuthnLoginChallenge retrieves a WebAuthn login/assertion challenge session from Redis.
func GetWebAuthnLoginChallenge(appID, identifier string) (string, error) {
	key := fmt.Sprintf("app:%s:webauthn_login:%s", appID, identifier)
	return Rdb.Get(ctx, key).Result()
}

// DeleteWebAuthnLoginChallenge removes a WebAuthn login/assertion challenge session from Redis.
func DeleteWebAuthnLoginChallenge(appID, identifier string) error {
	key := fmt.Sprintf("app:%s:webauthn_login:%s", appID, identifier)
	return Rdb.Del(ctx, key).Err()
}

// Admin 2FA Functions

// SetAdmin2FATempSecret stores a temporary TOTP secret during admin 2FA setup (10-minute TTL).
func SetAdmin2FATempSecret(adminID, secret string) error {
	key := fmt.Sprintf("admin:2fa_temp_secret:%s", adminID)
	return Rdb.Set(ctx, key, secret, 10*time.Minute).Err()
}

// GetAdmin2FATempSecret retrieves a temporary TOTP secret during admin 2FA setup.
func GetAdmin2FATempSecret(adminID string) (string, error) {
	key := fmt.Sprintf("admin:2fa_temp_secret:%s", adminID)
	return Rdb.Get(ctx, key).Result()
}

// DeleteAdmin2FATempSecret removes the temporary TOTP secret after setup is complete.
func DeleteAdmin2FATempSecret(adminID string) error {
	key := fmt.Sprintf("admin:2fa_temp_secret:%s", adminID)
	return Rdb.Del(ctx, key).Err()
}

// SetAdmin2FATempSession stores a partial login session awaiting 2FA verification (10-minute TTL).
// The value is the admin account ID.
func SetAdmin2FATempSession(tempToken, adminID string) error {
	key := fmt.Sprintf("admin:2fa_temp_session:%s", tempToken)
	return Rdb.Set(ctx, key, adminID, 10*time.Minute).Err()
}

// GetAdmin2FATempSession retrieves the admin ID from a temporary 2FA login session.
func GetAdmin2FATempSession(tempToken string) (string, error) {
	key := fmt.Sprintf("admin:2fa_temp_session:%s", tempToken)
	return Rdb.Get(ctx, key).Result()
}

// DeleteAdmin2FATempSession removes a temporary 2FA login session after verification.
func DeleteAdmin2FATempSession(tempToken string) error {
	key := fmt.Sprintf("admin:2fa_temp_session:%s", tempToken)
	return Rdb.Del(ctx, key).Err()
}

// SetAdmin2FAEmailCode stores a 2FA email verification code for an admin (5-minute TTL).
func SetAdmin2FAEmailCode(adminID, code string) error {
	key := fmt.Sprintf("admin:2fa_email:%s", adminID)
	return Rdb.Set(ctx, key, code, 5*time.Minute).Err()
}

// GetAdmin2FAEmailCode retrieves a stored 2FA email verification code for an admin.
func GetAdmin2FAEmailCode(adminID string) (string, error) {
	key := fmt.Sprintf("admin:2fa_email:%s", adminID)
	return Rdb.Get(ctx, key).Result()
}

// DeleteAdmin2FAEmailCode removes a 2FA email verification code after successful verification.
func DeleteAdmin2FAEmailCode(adminID string) error {
	key := fmt.Sprintf("admin:2fa_email:%s", adminID)
	return Rdb.Del(ctx, key).Err()
}

// Admin Magic Link Functions

// SetAdminMagicLinkToken stores a magic link token and a reverse lookup key (adminID → token).
// The reverse lookup allows invalidating old tokens when a new one is issued.
func SetAdminMagicLinkToken(adminID, token string, expiration time.Duration) error {
	// Invalidate any existing magic link token for this admin (only one active at a time)
	reverseKey := fmt.Sprintf("admin:magic_link_user:%s", adminID)
	oldToken, err := Rdb.Get(ctx, reverseKey).Result()
	if err == nil && oldToken != "" {
		oldKey := fmt.Sprintf("admin:magic_link:%s", oldToken)
		Rdb.Del(ctx, oldKey) // Best-effort cleanup of old token
	}

	// Store token → adminID mapping
	key := fmt.Sprintf("admin:magic_link:%s", token)
	if err := Rdb.Set(ctx, key, adminID, expiration).Err(); err != nil {
		return err
	}
	// Store reverse lookup: adminID → token
	return Rdb.Set(ctx, reverseKey, token, expiration).Err()
}

// GetAdminMagicLinkToken retrieves the adminID associated with a magic link token.
func GetAdminMagicLinkToken(token string) (string, error) {
	key := fmt.Sprintf("admin:magic_link:%s", token)
	return Rdb.Get(ctx, key).Result()
}

// DeleteAdminMagicLinkToken deletes a magic link token and its reverse lookup key (single-use).
func DeleteAdminMagicLinkToken(token string) error {
	key := fmt.Sprintf("admin:magic_link:%s", token)
	// Look up the adminID so we can also clean up the reverse key
	adminID, err := Rdb.Get(ctx, key).Result()
	if err == nil && adminID != "" {
		reverseKey := fmt.Sprintf("admin:magic_link_user:%s", adminID)
		Rdb.Del(ctx, reverseKey) // Best-effort cleanup
	}
	return Rdb.Del(ctx, key).Err()
}

// ==================== Failed Login Tracking (Brute-Force Detection) ====================

// IncrFailedLogin increments the failed login counter for a given app + identifier (email or IP).
// The counter auto-expires after the given window duration.
// Returns the new count after increment.
func IncrFailedLogin(appID, identifier string, window time.Duration) (int64, error) {
	key := fmt.Sprintf("app:%s:failed_login:%s", appID, identifier)
	count, err := Rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Set TTL only on first attempt (when count == 1)
	if count == 1 {
		Rdb.Expire(ctx, key, window)
	}
	return count, nil
}

// GetFailedLoginCount returns the current failed login count for a given app + identifier.
func GetFailedLoginCount(appID, identifier string) (int64, error) {
	key := fmt.Sprintf("app:%s:failed_login:%s", appID, identifier)
	count, err := Rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// ResetFailedLogins clears the failed login counter for a given app + identifier.
// Call this on successful login.
func ResetFailedLogins(appID, identifier string) error {
	key := fmt.Sprintf("app:%s:failed_login:%s", appID, identifier)
	return Rdb.Del(ctx, key).Err()
}

// ==================== Notification Cooldown ====================

// SetNotificationCooldown sets a cooldown flag to prevent spamming notification emails.
// Key pattern: notify_cooldown:{appID}:{userID}:{notificationType}
func SetNotificationCooldown(appID, userID, notificationType string, cooldown time.Duration) error {
	key := fmt.Sprintf("notify_cooldown:%s:%s:%s", appID, userID, notificationType)
	return Rdb.Set(ctx, key, "1", cooldown).Err()
}

// IsNotificationOnCooldown checks whether a notification cooldown is active for a user.
func IsNotificationOnCooldown(appID, userID, notificationType string) (bool, error) {
	key := fmt.Sprintf("notify_cooldown:%s:%s:%s", appID, userID, notificationType)
	_, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// ==================== Account Lockout Tier Tracking ====================

// IncrLockoutTier increments the lockout tier for a given app + email and sets the TTL.
// The tier determines which escalating lockout duration to use (e.g., tier 0 = 15m, tier 1 = 30m, etc.).
// Returns the new tier value (1-based after increment).
func IncrLockoutTier(appID, email string, ttl time.Duration) (int64, error) {
	key := fmt.Sprintf("app:%s:lockout_tier:%s", appID, email)
	tier, err := Rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Always refresh TTL on each lockout so the tier escalation window resets
	Rdb.Expire(ctx, key, ttl)
	return tier, nil
}

// GetLockoutTier returns the current lockout tier for a given app + email.
// Returns 0 if no tier is set (user has not been locked out recently).
func GetLockoutTier(appID, email string) (int64, error) {
	key := fmt.Sprintf("app:%s:lockout_tier:%s", appID, email)
	tier, err := Rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return tier, err
}

// ResetLockoutTier clears the lockout tier for a given app + email.
// Called by admin when manually unlocking an account.
func ResetLockoutTier(appID, email string) error {
	key := fmt.Sprintf("app:%s:lockout_tier:%s", appID, email)
	return Rdb.Del(ctx, key).Err()
}

// ==================== Progressive Delay Tier Tracking ====================

// IncrDelayTier increments the delay tier for a given app + identifier (email or IP).
// The tier determines the exponential backoff delay applied before login processing.
// Returns the new tier value (1-based after increment).
func IncrDelayTier(appID, identifier string, ttl time.Duration) (int64, error) {
	key := fmt.Sprintf("app:%s:delay_tier:%s", appID, identifier)
	tier, err := Rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if tier == 1 {
		Rdb.Expire(ctx, key, ttl)
	}
	return tier, nil
}

// GetDelayTier returns the current delay tier for a given app + identifier.
// Returns 0 if no tier is set (no recent failed attempts).
func GetDelayTier(appID, identifier string) (int64, error) {
	key := fmt.Sprintf("app:%s:delay_tier:%s", appID, identifier)
	tier, err := Rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return tier, err
}

// ResetDelayTier clears the delay tier for a given app + identifier.
// Called on successful login to reset progressive delays.
func ResetDelayTier(appID, identifier string) error {
	key := fmt.Sprintf("app:%s:delay_tier:%s", appID, identifier)
	return Rdb.Del(ctx, key).Err()
}

// ─── OIDC browser session (login cookie) ───────────────────────────────────────

// SetOIDCBrowserSession stores an opaque session token → userID mapping used by
// the OIDC login cookie. The token is a random value, never the user UUID.
func SetOIDCBrowserSession(appID, sessionToken, userID string, ttl time.Duration) error {
	key := fmt.Sprintf("app:%s:oidc_browser:%s", appID, sessionToken)
	return Rdb.Set(ctx, key, userID, ttl).Err()
}

// GetOIDCBrowserSession resolves an opaque OIDC browser session token to a userID.
// Returns ("", nil) when the session does not exist.
func GetOIDCBrowserSession(appID, sessionToken string) (string, error) {
	key := fmt.Sprintf("app:%s:oidc_browser:%s", appID, sessionToken)
	val, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// DeleteOIDCBrowserSession removes the OIDC browser session (e.g. on logout).
func DeleteOIDCBrowserSession(appID, sessionToken string) error {
	key := fmt.Sprintf("app:%s:oidc_browser:%s", appID, sessionToken)
	return Rdb.Del(ctx, key).Err()
}

// ==================== Backup Email Verification ====================

// SetBackupEmailVerificationToken stores a token → (userID, pendingEmail) mapping used during
// backup email verification. The token is a random URL-safe value emailed to the backup address.
func SetBackupEmailVerificationToken(appID, userID, token, pendingEmail string, expiration time.Duration) error {
	// token → "userID|pendingEmail"
	key := fmt.Sprintf("app:%s:backup_email_verify:%s", appID, token)
	value := userID + "|" + pendingEmail
	return Rdb.Set(ctx, key, value, expiration).Err()
}

// GetBackupEmailVerificationToken retrieves the userID and pending email for a backup email verification token.
func GetBackupEmailVerificationToken(appID, token string) (userID, pendingEmail string, err error) {
	key := fmt.Sprintf("app:%s:backup_email_verify:%s", appID, token)
	val, err := Rdb.Get(ctx, key).Result()
	if err != nil {
		return "", "", err
	}
	// Split on first "|" only
	idx := strings.Index(val, "|")
	if idx < 0 {
		return val, "", nil
	}
	return val[:idx], val[idx+1:], nil
}

// DeleteBackupEmailVerificationToken removes a backup email verification token after use.
func DeleteBackupEmailVerificationToken(appID, token string) error {
	key := fmt.Sprintf("app:%s:backup_email_verify:%s", appID, token)
	return Rdb.Del(ctx, key).Err()
}

// ==================== SMS / Phone Verification Codes ====================

// SetPhoneVerificationCode stores a 6-digit code used to verify a new phone number.
func SetPhoneVerificationCode(appID, userID, code string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:phone_verify:%s", appID, userID)
	return Rdb.Set(ctx, key, code, expiration).Err()
}

// GetPhoneVerificationCode retrieves a phone verification code.
func GetPhoneVerificationCode(appID, userID string) (string, error) {
	key := fmt.Sprintf("app:%s:phone_verify:%s", appID, userID)
	return Rdb.Get(ctx, key).Result()
}

// DeletePhoneVerificationCode removes a phone verification code after successful use.
func DeletePhoneVerificationCode(appID, userID string) error {
	key := fmt.Sprintf("app:%s:phone_verify:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// Set2FASMSCode stores a 6-digit SMS 2FA / recovery code during login (5-minute TTL).
func Set2FASMSCode(appID, userID, code string) error {
	key := fmt.Sprintf("app:%s:2fa_sms:%s", appID, userID)
	return Rdb.Set(ctx, key, code, 5*time.Minute).Err()
}

// Get2FASMSCode retrieves a stored SMS 2FA code.
func Get2FASMSCode(appID, userID string) (string, error) {
	key := fmt.Sprintf("app:%s:2fa_sms:%s", appID, userID)
	return Rdb.Get(ctx, key).Result()
}

// Delete2FASMSCode removes an SMS 2FA code after successful verification (one-time use).
func Delete2FASMSCode(appID, userID string) error {
	key := fmt.Sprintf("app:%s:2fa_sms:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// SetBackupEmail2FACode stores a 6-digit code sent to the backup email during login (5-minute TTL).
func SetBackupEmail2FACode(appID, userID, code string) error {
	key := fmt.Sprintf("app:%s:2fa_backup_email:%s", appID, userID)
	return Rdb.Set(ctx, key, code, 5*time.Minute).Err()
}

// GetBackupEmail2FACode retrieves a stored backup-email 2FA code.
func GetBackupEmail2FACode(appID, userID string) (string, error) {
	key := fmt.Sprintf("app:%s:2fa_backup_email:%s", appID, userID)
	return Rdb.Get(ctx, key).Result()
}

// DeleteBackupEmail2FACode removes a backup-email 2FA code after successful verification.
func DeleteBackupEmail2FACode(appID, userID string) error {
	key := fmt.Sprintf("app:%s:2fa_backup_email:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// ─── OIDC granted scopes (per session) ─────────────────────────────────────────

// SetOIDCGrantedScopes stores the space-separated scopes that were granted for
// a given OIDC session. Used by the UserInfo endpoint to gate which claims are
// returned without embedding scopes in the JWT itself.
func SetOIDCGrantedScopes(appID, sessionID, scopes string, ttl time.Duration) error {
	key := fmt.Sprintf("app:%s:oidc_scopes:%s", appID, sessionID)
	return Rdb.Set(ctx, key, scopes, ttl).Err()
}

// GetOIDCGrantedScopes retrieves the space-separated scopes for an OIDC session.
// Returns ("", nil) when not found (e.g. token issued before this feature).
func GetOIDCGrantedScopes(appID, sessionID string) (string, error) {
	key := fmt.Sprintf("app:%s:oidc_scopes:%s", appID, sessionID)
	val, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// ============================================================================
// Account Merge Token helpers
//
// A merge token is a short-lived (15 min) Redis entry that stores all the
// information needed to link a social provider account to an existing user
// account.  It is created when a social-login callback detects that the
// provider email matches an existing user who does not yet have that social
// account linked, so the frontend can prompt the user to confirm the merge
// by supplying their existing password.
//
// Key layout: app:{appID}:merge_token:{mergeToken}  →  JSON-encoded payload
// ============================================================================

// SetMergeToken stores a merge token with a JSON-encoded payload and the given TTL.
func SetMergeToken(appID, mergeToken, payload string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:merge_token:%s", appID, mergeToken)
	return Rdb.Set(ctx, key, payload, expiration).Err()
}

// GetMergeToken retrieves the JSON payload for a merge token.
// Returns ("", redis.Nil) when the token does not exist or has expired.
func GetMergeToken(appID, mergeToken string) (string, error) {
	key := fmt.Sprintf("app:%s:merge_token:%s", appID, mergeToken)
	val, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", err
	}
	return val, err
}

// DeleteMergeToken removes a merge token after it has been consumed.
func DeleteMergeToken(appID, mergeToken string) error {
	key := fmt.Sprintf("app:%s:merge_token:%s", appID, mergeToken)
	return Rdb.Del(ctx, key).Err()
}

// ============================================================================
// SSO (Shared Session) Token helpers
//
// An SSO token is a short-lived (60 s), single-use opaque token that encodes
// the source app, the session group, and the authenticated user.  It is issued
// by POST /sso/token and consumed exactly once by POST /sso/exchange on the
// target application to mint a new app-scoped token pair without re-auth.
//
// Key layout: sso:token:{token}  →  "{groupID}|{sourceAppID}|{userID}"
// ============================================================================

const ssoTokenTTL = 60 * time.Second

// SetSSOToken stores a new SSO exchange token with a 60-second TTL.
func SetSSOToken(token, groupID, sourceAppID, userID string) error {
	key := fmt.Sprintf("sso:token:%s", token)
	value := groupID + "|" + sourceAppID + "|" + userID
	return Rdb.Set(ctx, key, value, ssoTokenTTL).Err()
}

// GetSSOToken retrieves the group, source app, and user encoded in an SSO token.
// Returns redis.Nil error when the token does not exist or has expired.
func GetSSOToken(token string) (groupID, sourceAppID, userID string, err error) {
	key := fmt.Sprintf("sso:token:%s", token)
	val, err := Rdb.Get(ctx, key).Result()
	if err != nil {
		return "", "", "", err
	}
	parts := strings.SplitN(val, "|", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("malformed SSO token value")
	}
	return parts[0], parts[1], parts[2], nil
}

// DeleteSSOToken removes an SSO token after it has been consumed (single-use).
func DeleteSSOToken(token string) error {
	key := fmt.Sprintf("sso:token:%s", token)
	return Rdb.Del(ctx, key).Err()
}

// ============================================================================
// SSO Pub/Sub Functions
// ============================================================================

// ssoEventChannel returns the Redis pub/sub channel name for an SSO session group.
func ssoEventChannel(groupID string) string {
	return fmt.Sprintf("sso_events:%s", groupID)
}

// PublishSSOEvent publishes a JSON-encoded payload to the SSO pub/sub channel
// for the given session group. The payload must be a valid JSON string.
func PublishSSOEvent(groupID, payload string) error {
	return Rdb.Publish(ctx, ssoEventChannel(groupID), payload).Err()
}

// StorePendingLoginEvent stores a per-app-per-user peer_login payload in Redis
// so that a client that was disconnected when the event was published can receive
// it on reconnect.  The entry expires after 90 seconds — long enough to survive a
// proxy-forced reconnect cycle but short enough not to replay stale logins.
// The key includes userID so that a login for one user is never replayed to a
// different user sharing the same browser/device.
func StorePendingLoginEvent(appID, userID, payload string) error {
	key := fmt.Sprintf("sso:pending_login:%s:%s", appID, userID)
	return Rdb.Set(ctx, key, payload, 90*time.Second).Err()
}

// PopPendingLoginEvent returns and deletes the pending login payload for the
// given appID+userID, or ("", nil) if there is none.
func PopPendingLoginEvent(appID, userID string) (string, error) {
	key := fmt.Sprintf("sso:pending_login:%s:%s", appID, userID)
	val, err := Rdb.GetDel(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// StorePendingLoginDeviceMapping maps a deviceID to the userID so that the SSE
// StreamEvents handler can find pending login events for a device without knowing
// the userID in advance (the client may not have a cached_user_profile).
func StorePendingLoginDeviceMapping(appID, deviceID, userID string) error {
	key := fmt.Sprintf("sso:pending_login_d:%s:%s", appID, deviceID)
	return Rdb.Set(ctx, key, userID, 90*time.Second).Err()
}

// PopPendingLoginDeviceMapping returns and deletes the userID for the given
// appID+deviceID, or ("", nil) if there is none.
func PopPendingLoginDeviceMapping(appID, deviceID string) (string, error) {
	key := fmt.Sprintf("sso:pending_login_d:%s:%s", appID, deviceID)
	val, err := Rdb.GetDel(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// ============================================================================
// SSO Login Presence
//
// A login presence record is written whenever a user successfully logs in to
// an app that belongs to a session group.  Unlike the short-lived
// sso:pending_login key (90 s), the presence key uses a rolling 24-hour TTL
// that is refreshed on every token refresh.  This allows peer apps opened long
// after the initial login to still receive an on-demand peer_login SSE event
// on SSE connect.
//
// Key layout: sso:presence:{appID}:{userID}  →  "{groupID}"
// ============================================================================

const loginPresenceTTL = 1 * time.Hour

// SetLoginPresence records that userID is currently logged in to appID as part
// of groupID, together with the device fingerprint.  The key is user-scoped so
// that multiple users can have active presence records for the same app
// simultaneously.  The key is refreshed (rolling TTL) on every call so it
// stays alive for as long as the session is actively used.
func SetLoginPresence(appID, userID, groupID, deviceID string) error {
	key := fmt.Sprintf("sso:presence:%s:%s", appID, userID)
	value := fmt.Sprintf("%s|%s", groupID, deviceID)
	return Rdb.Set(ctx, key, value, loginPresenceTTL).Err()
}

// GetLoginPresence returns the groupID and deviceID stored by SetLoginPresence
// for the given appID+userID, or ("", "", nil) when no presence record exists.
// Backward-compatible: old records stored without a deviceID (before the device
// scoping change) return the groupID with an empty deviceID.
func GetLoginPresence(appID, userID string) (groupID, deviceID string, err error) {
	key := fmt.Sprintf("sso:presence:%s:%s", appID, userID)
	val, redisErr := Rdb.Get(ctx, key).Result()
	if redisErr != nil {
		if redisErr.Error() == "redis: nil" {
			return "", "", nil
		}
		return "", "", redisErr
	}
	parts := strings.SplitN(val, "|", 2)
	groupID = parts[0]
	if len(parts) > 1 {
		deviceID = parts[1]
	}
	return groupID, deviceID, nil
}

// DeleteLoginPresence removes the login presence record for appID+userID.
// Called on logout and group-wide session revocation.
func DeleteLoginPresence(appID, userID string) error {
	key := fmt.Sprintf("sso:presence:%s:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// StorePresenceDeviceMapping maps a deviceID to the userID within a session group
// so that the SSE StreamEvents handler can discover which user is logged in on this
// device even without a user_id query parameter (e.g. when the client has no
// cached_user_profile on the current origin).
func StorePresenceDeviceMapping(groupID, deviceID, userID string) error {
	key := fmt.Sprintf("sso:presence_d:%s:%s", groupID, deviceID)
	return Rdb.Set(ctx, key, userID, loginPresenceTTL).Err()
}

// GetPresenceUserByDevice returns the userID mapped to the given
// groupID+deviceID, or ("", nil) if there is none.
func GetPresenceUserByDevice(groupID, deviceID string) (string, error) {
	key := fmt.Sprintf("sso:presence_d:%s:%s", groupID, deviceID)
	val, err := Rdb.Get(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// StorePendingLogoutEvent stores a per-app-per-user peer_logout signal in Redis
// so that a client reconnecting after missing the pub/sub broadcast still gets
// logged out.  TTL is 90 seconds — same reasoning as pending login.
// reason should be "voluntary" (user-initiated) or "revoked" (admin/forced).
// The key includes userID so that a logout for one user does not accidentally
// trigger a "session expired" dialog for a different user on the same app.
func StorePendingLogoutEvent(appID, userID, reason, deviceID string) error {
	key := fmt.Sprintf("sso:pending_logout:%s:%s", appID, userID)
	payload := fmt.Sprintf(`{"type":"peer_logout","reason":%q,"device_id":%q}`, reason, deviceID)
	return Rdb.Set(ctx, key, payload, 90*time.Second).Err()
}

// PopPendingLogoutEvent returns and deletes the pending logout signal for the
// given appID+userID, or ("", nil) if there is none.
func PopPendingLogoutEvent(appID, userID string) (string, error) {
	key := fmt.Sprintf("sso:pending_logout:%s:%s", appID, userID)
	val, err := Rdb.GetDel(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// DeletePendingLogoutEvent deletes any pending logout signal for the given
// appID+userID. Called after a successful login so a stale logout event from
// a previous session is not replayed to the newly authenticated client.
func DeletePendingLogoutEvent(appID, userID string) error {
	key := fmt.Sprintf("sso:pending_logout:%s:%s", appID, userID)
	return Rdb.Del(ctx, key).Err()
}

// DeleteAllPendingLogoutEvents removes every sso:pending_logout:* key for the
// given userID across ALL applications. This is called synchronously during
// login (via SyncLoginFunc) so that stale peer_logout events from a previous
// session are guaranteed to be cleared BEFORE the browser receives the login
// redirect and opens an SSE connection — avoiding a race where the SSE replays
// a stale logout and immediately kills the brand-new session.
func DeleteAllPendingLogoutEvents(userID string) error {
	pattern := fmt.Sprintf("sso:pending_logout:*:%s", userID)
	var cursor uint64
	for {
		keys, nextCursor, err := Rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := Rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// SubscribeSSOEvents subscribes to the SSO event channel for a session group
// and returns the *redis.PubSub handle. Callers are responsible for closing it.
// Uses the dedicated pubsub client so subscriptions never contend with the
// command pool used by session lookups, token ops, etc.
func SubscribeSSOEvents(groupID string) *redis.PubSub {
	return pubsubRdb.Subscribe(ctx, ssoEventChannel(groupID))
}

// PSubscribeKeyExpiry subscribes to Redis keyspace expiry notifications using the
// dedicated pubsub client so it never contends with the command pool.
func PSubscribeKeyExpiry(subCtx context.Context, pattern string) *redis.PubSub {
	return pubsubRdb.PSubscribe(subCtx, pattern)
}

// ============================================================================
// Session Metadata Functions for Expiration Detection
// ============================================================================

// ParseSessionMetaKey extracts appID, userID, and sessionID from a session_meta key
func ParseSessionMetaKey(metaKey string) (appID, userID, sessionID string, err error) {
	// Remove the "session_meta:" prefix
	if !strings.HasPrefix(metaKey, "session_meta:") {
		return "", "", "", fmt.Errorf("not a session_meta key")
	}

	parts := strings.Split(metaKey[len("session_meta:"):], ":")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("malformed session_meta key")
	}

	return parts[0], parts[1], parts[2], nil
}

// CleanOrphanedSessionMetaKeys scans all session_meta:* keys and deletes any whose
// corresponding session hash (app:{appID}:session:{sessionID}) no longer exists.
// This prevents orphaned keys from expiring via keyspace notification and triggering
// cascading group-wide session revocations long after the original session was deleted.
// Called once at startup and periodically by the expiry service.
func CleanOrphanedSessionMetaKeys() (cleaned int, err error) {
	var cursor uint64
	for {
		keys, nextCursor, scanErr := Rdb.Scan(ctx, cursor, "session_meta:*", 100).Result()
		if scanErr != nil {
			return cleaned, scanErr
		}
		for _, metaKey := range keys {
			appID, _, sessionID, parseErr := ParseSessionMetaKey(metaKey)
			if parseErr != nil {
				continue
			}
			sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
			exists, _ := Rdb.Exists(ctx, sessionKey).Result()
			if exists == 0 {
				Rdb.Del(ctx, metaKey)
				cleaned++
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return cleaned, nil
}

// GetExpiredSessionMetaKeys returns all session_meta keys that have expired (TTL <= 0).
// As a side effect, it also proactively cleans up orphaned session_meta keys whose
// corresponding session hash no longer exists, preventing them from accumulating and
// later triggering spurious group-wide revocation.
func GetExpiredSessionMetaKeys() ([]string, error) {
	var expiredKeys []string

	// Use SCAN to find all session_meta keys
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = Rdb.Scan(ctx, cursor, "session_meta:*", 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			// Check TTL
			ttl, err := Rdb.TTL(ctx, key).Result()
			if err != nil {
				continue
			}
			// Redis TTL semantics:
			//   > 0  → key is alive with remaining TTL (skip)
			//  == -1 → key exists but has NO expiry (persistent/orphaned key — skip,
			//           do NOT treat as expired). Proactively check if the session hash
			//           is gone and clean up the orphan silently.
			//  == -2 → key does not exist (SCAN won't return these)
			// We deliberately exclude -1 (no-TTL) keys from the expired list because
			// treating them as expired was causing all sessions to be revoked immediately
			// after server restart or after any session_meta key was created without an
			// expiry.
			if ttl > 0 {
				continue // still alive
			}
			if ttl == -1 {
				// Persistent key without expiry — check if the session hash is still
				// alive. If the hash is gone, clean up this orphan silently without
				// triggering group revocation.
				appID, _, sessionID, parseErr := ParseSessionMetaKey(key)
				if parseErr == nil {
					sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
					exists, _ := Rdb.Exists(ctx, sessionKey).Result()
					if exists == 0 {
						Rdb.Del(ctx, key)
					}
				}
				continue // never treat no-TTL keys as expired
			}
			// ttl == 0: genuinely expiring right now
			expiredKeys = append(expiredKeys, key)
		}

		if cursor == 0 {
			break
		}
	}

	return expiredKeys, nil
}

// SetTrustedDeviceActivation stores a short-lived activation payload keyed by a
// random HMAC-signed ID.  The payload contains the plaintext trusted-device token,
// user ID, and app ID.  TTL is fixed at 60 seconds (single login session).
func SetTrustedDeviceActivation(id, payload string) error {
	key := fmt.Sprintf("trusted_device_activation:%s", id)
	return Rdb.Set(ctx, key, payload, 60*time.Second).Err()
}

// GetAndDeleteTrustedDeviceActivation atomically reads and deletes an activation
// entry.  Returns redis.Nil when the key does not exist or has already expired.
func GetAndDeleteTrustedDeviceActivation(id string) (string, error) {
	key := fmt.Sprintf("trusted_device_activation:%s", id)
	val, err := Rdb.GetDel(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}
