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
var ctx = context.Background()

func ConnectRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     viper.GetString("REDIS_ADDR"),
		Password: viper.GetString("REDIS_PASSWORD"),
		DB:       viper.GetInt("REDIS_DB"),
	})

	_, err := Rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
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

// ==================== Session Management Functions ====================

// CreateSession stores a new session as a Redis Hash with metadata.
// Key pattern: app:{appID}:session:{sessionID}
// Also adds the sessionID to the user's session index set.
func CreateSession(appID, sessionID, userID, refreshToken, ip, userAgent string, ttl time.Duration) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	fields := map[string]interface{}{
		"user_id":       userID,
		"refresh_token": refreshToken,
		"ip":            ip,
		"user_agent":    userAgent,
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

// TouchSession updates the last_active timestamp of a session.
func TouchSession(appID, sessionID string) error {
	key := fmt.Sprintf("app:%s:session:%s", appID, sessionID)
	return Rdb.HSet(ctx, key, "last_active", time.Now().UTC().Format(time.RFC3339)).Err()
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
