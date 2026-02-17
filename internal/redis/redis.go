package redis

import (
	"context"
	"fmt"
	"log"
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

// SetEmailVerificationToken stores an email verification token
func SetEmailVerificationToken(appID, userID, token string, expiration time.Duration) error {
	key := fmt.Sprintf("app:%s:email_verify:%s", appID, token)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// GetEmailVerificationToken retrieves an email verification token
func GetEmailVerificationToken(appID, token string) (string, error) {
	key := fmt.Sprintf("app:%s:email_verify:%s", appID, token)
	return Rdb.Get(ctx, key).Result()
}

// DeleteEmailVerificationToken deletes an email verification token
func DeleteEmailVerificationToken(appID, token string) error {
	key := fmt.Sprintf("app:%s:email_verify:%s", appID, token)
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
