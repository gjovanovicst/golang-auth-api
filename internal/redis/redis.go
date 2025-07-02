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
func SetRefreshToken(userID, token string) error {
	key := fmt.Sprintf("refresh_token:%s", userID)
	expiration := time.Hour * time.Duration(viper.GetInt("REFRESH_TOKEN_EXPIRATION_HOURS"))
	return Rdb.Set(ctx, key, token, expiration).Err()
}

// GetRefreshToken retrieves a refresh token
func GetRefreshToken(userID string) (string, error) {
	key := fmt.Sprintf("refresh_token:%s", userID)
	return Rdb.Get(ctx, key).Result()
}

// RevokeRefreshToken deletes a refresh token (effectively blacklisting it)
func RevokeRefreshToken(userID, token string) error {
	// For simplicity, we'll just delete the token associated with the user ID.
	// A more robust solution might involve a blacklist set for specific tokens.
	key := fmt.Sprintf("refresh_token:%s", userID)
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
func IsRefreshTokenRevoked(userID, token string) (bool, error) {
	key := fmt.Sprintf("refresh_token:%s", userID)
	val, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return true, nil // Token not found, so it's considered revoked or expired
	} else if err != nil {
		return false, err
	}
	return val != token, nil // If value doesn't match, it means a new token was issued, old one is implicitly revoked
}

// SetEmailVerificationToken stores an email verification token
func SetEmailVerificationToken(userID, token string, expiration time.Duration) error {
	key := fmt.Sprintf("email_verify:%s", token)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// GetEmailVerificationToken retrieves an email verification token
func GetEmailVerificationToken(token string) (string, error) {
	key := fmt.Sprintf("email_verify:%s", token)
	return Rdb.Get(ctx, key).Result()
}

// DeleteEmailVerificationToken deletes an email verification token
func DeleteEmailVerificationToken(token string) error {
	key := fmt.Sprintf("email_verify:%s", token)
	return Rdb.Del(ctx, key).Err()
}

// SetPasswordResetToken stores a password reset token
func SetPasswordResetToken(userID, token string, expiration time.Duration) error {
	key := fmt.Sprintf("password_reset:%s", token)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// GetPasswordResetToken retrieves a password reset token
func GetPasswordResetToken(token string) (string, error) {
	key := fmt.Sprintf("password_reset:%s", token)
	return Rdb.Get(ctx, key).Result()
}

// DeletePasswordResetToken deletes a password reset token
func DeletePasswordResetToken(token string) error {
	key := fmt.Sprintf("password_reset:%s", token)
	return Rdb.Del(ctx, key).Err()
}

// 2FA related functions

// SetTempTwoFASecret stores a temporary 2FA secret during setup
func SetTempTwoFASecret(userID, secret string, expiration time.Duration) error {
	key := fmt.Sprintf("temp_2fa_secret:%s", userID)
	return Rdb.Set(ctx, key, secret, expiration).Err()
}

// GetTempTwoFASecret retrieves a temporary 2FA secret
func GetTempTwoFASecret(userID string) (string, error) {
	key := fmt.Sprintf("temp_2fa_secret:%s", userID)
	return Rdb.Get(ctx, key).Result()
}

// DeleteTempTwoFASecret deletes a temporary 2FA secret
func DeleteTempTwoFASecret(userID string) error {
	key := fmt.Sprintf("temp_2fa_secret:%s", userID)
	return Rdb.Del(ctx, key).Err()
}

// SetTempUserSession stores a temporary user session for 2FA login
func SetTempUserSession(tempToken, userID string, expiration time.Duration) error {
	key := fmt.Sprintf("temp_session:%s", tempToken)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// GetTempUserSession retrieves a temporary user session
func GetTempUserSession(tempToken string) (string, error) {
	key := fmt.Sprintf("temp_session:%s", tempToken)
	return Rdb.Get(ctx, key).Result()
}

// DeleteTempUserSession deletes a temporary user session
func DeleteTempUserSession(tempToken string) error {
	key := fmt.Sprintf("temp_session:%s", tempToken)
	return Rdb.Del(ctx, key).Err()
}

// Access Token Blacklisting Functions

// BlacklistAccessToken adds an access token to the blacklist with its remaining TTL
func BlacklistAccessToken(tokenString string, userID string, expiration time.Duration) error {
	key := fmt.Sprintf("blacklist_token:%s", tokenString)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// IsAccessTokenBlacklisted checks if an access token is blacklisted
func IsAccessTokenBlacklisted(tokenString string) (bool, error) {
	key := fmt.Sprintf("blacklist_token:%s", tokenString)
	_, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // Token not found in blacklist
	} else if err != nil {
		return false, err // Redis error
	}
	return true, nil // Token found in blacklist
}

// BlacklistAllUserTokens blacklists all tokens for a specific user (useful for password changes, account compromise)
func BlacklistAllUserTokens(userID string, expiration time.Duration) error {
	key := fmt.Sprintf("blacklist_user:%s", userID)
	return Rdb.Set(ctx, key, "all_tokens_revoked", expiration).Err()
}

// IsUserTokensBlacklisted checks if all tokens for a user are blacklisted
func IsUserTokensBlacklisted(userID string) (bool, error) {
	key := fmt.Sprintf("blacklist_user:%s", userID)
	_, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // User tokens not blacklisted
	} else if err != nil {
		return false, err // Redis error
	}
	return true, nil // All user tokens are blacklisted
}
