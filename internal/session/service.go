package session

import (
	"log"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// Service handles session lifecycle management backed by Redis.
type Service struct{}

// NewService creates a new session service.
func NewService() *Service {
	return &Service{}
}

// refreshTokenTTL returns the configured refresh token TTL (used as session TTL).
func refreshTokenTTL() time.Duration {
	return time.Hour * time.Duration(viper.GetInt("REFRESH_TOKEN_EXPIRATION_HOURS"))
}

// CreateSession creates a new session in Redis and returns the session ID.
// It generates tokens and stores the refresh token inside the session hash.
func (s *Service) CreateSession(appID, userID, ip, userAgent string, roles []string) (accessToken, refreshToken, sessionID string, appErr *errors.AppError) {
	sessionID = uuid.New().String()

	accessToken, err := jwt.GenerateAccessToken(appID, userID, sessionID, roles)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}

	refreshToken, err = jwt.GenerateRefreshToken(appID, userID, sessionID, roles)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}

	if err := redis.CreateSession(appID, sessionID, userID, refreshToken, ip, userAgent, refreshTokenTTL()); err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to create session")
	}

	return accessToken, refreshToken, sessionID, nil
}

// RefreshSession validates the old refresh token against the session, rotates tokens,
// and updates the session metadata. Returns new access token, new refresh token, and userID.
func (s *Service) RefreshSession(oldRefreshToken string) (string, string, string, *errors.AppError) {
	claims, err := jwt.ParseToken(oldRefreshToken)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid refresh token")
	}

	// Reject access tokens used as refresh tokens
	if claims.TokenType != "" && claims.TokenType != jwt.TokenTypeRefresh {
		return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid token type")
	}

	// Legacy tokens without session_id: fall back to old single-token flow
	if claims.SessionID == "" {
		return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Session expired, please log in again")
	}

	// Verify session exists and refresh token matches
	storedToken, err := redis.GetSessionRefreshToken(claims.AppID, claims.SessionID)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Session expired or revoked")
	}
	if storedToken != oldRefreshToken {
		return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Refresh token revoked or invalid")
	}

	// Generate new token pair (same session ID)
	newAccessToken, tokenErr := jwt.GenerateAccessToken(claims.AppID, claims.UserID, claims.SessionID, claims.Roles)
	if tokenErr != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new access token")
	}
	newRefreshToken, tokenErr := jwt.GenerateRefreshToken(claims.AppID, claims.UserID, claims.SessionID, claims.Roles)
	if tokenErr != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new refresh token")
	}

	// Update session with new refresh token and touch last_active
	if err := redis.UpdateSessionRefreshToken(claims.AppID, claims.SessionID, newRefreshToken); err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to update session")
	}
	redis.TouchSession(claims.AppID, claims.SessionID)

	return newAccessToken, newRefreshToken, claims.UserID, nil
}

// RevokeSession deletes a specific session. Also blacklists any access tokens
// from that session by relying on the middleware session-existence check.
func (s *Service) RevokeSession(appID, userID, sessionID string) *errors.AppError {
	if err := redis.DeleteSession(appID, sessionID, userID); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to revoke session")
	}
	return nil
}

// RevokeAllSessions revokes all sessions for a user except the one specified.
// If exceptSessionID is empty, all sessions are revoked.
func (s *Service) RevokeAllSessions(appID, userID, exceptSessionID string) *errors.AppError {
	if err := redis.DeleteAllUserSessions(appID, userID, exceptSessionID); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to revoke sessions")
	}
	return nil
}

// ListSessions returns all active sessions for a user.
func (s *Service) ListSessions(appID, userID, currentSessionID string) (*dto.SessionListResponse, *errors.AppError) {
	sessionIDs, err := redis.GetUserSessionIDs(appID, userID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to list sessions")
	}

	sessions := make([]dto.SessionResponse, 0, len(sessionIDs))
	for _, sid := range sessionIDs {
		data, err := redis.GetSession(appID, sid)
		if err != nil {
			continue // Session may have expired between listing and fetching
		}

		sessions = append(sessions, dto.SessionResponse{
			ID:         sid,
			IPAddress:  data["ip"],
			UserAgent:  data["user_agent"],
			CreatedAt:  data["created_at"],
			LastActive: data["last_active"],
			IsCurrent:  sid == currentSessionID,
		})
	}

	return &dto.SessionListResponse{Sessions: sessions}, nil
}

// LogoutSession handles the logout flow for a specific session.
// It revokes the session and blacklists the access token for defense-in-depth.
func (s *Service) LogoutSession(appID, userID, sessionID, accessToken string) *errors.AppError {
	// Delete the session from Redis
	if err := redis.DeleteSession(appID, sessionID, userID); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to revoke session")
	}

	// Blacklist the access token (defense in depth)
	if accessToken != "" {
		claims, err := jwt.ParseToken(accessToken)
		if err == nil {
			remainingTime := time.Until(claims.ExpiresAt.Time)
			if remainingTime > 0 {
				if err := redis.BlacklistAccessToken(appID, accessToken, userID, remainingTime); err != nil {
					log.Printf("Warning: Failed to blacklist access token: %v\n", err)
				}
			}
		}
	}

	return nil
}

// RevokeAllUserSessions revokes all sessions and blacklists all user tokens.
// Used for security events like password changes.
func (s *Service) RevokeAllUserSessions(appID, userID string) *errors.AppError {
	// Delete all sessions from Redis
	if err := redis.DeleteAllUserSessions(appID, userID, ""); err != nil {
		log.Printf("Warning: Failed to delete all sessions for user %s: %v\n", userID, err)
	}

	// Blacklist all tokens as a safety net
	maxTokenLifetime := time.Hour * time.Duration(24*30) // 30 days
	if err := redis.BlacklistAllUserTokens(appID, userID, maxTokenLifetime); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to blacklist user tokens")
	}

	return nil
}
