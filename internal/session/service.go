package session

import (
	"log"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/google/uuid"
)

// InactivityResolver resolves the effective inactivity timeout (in minutes) for a given app.
// Returns 0 if the feature is disabled for that app.
type InactivityResolver interface {
	GetInactivityTimeoutMinutes(appID string) int
}

// Service handles session lifecycle management backed by Redis.
type Service struct {
	// InactivityResolver resolves per-app inactivity timeout. Optional; if nil, inactivity
	// enforcement is skipped.
	InactivityResolver InactivityResolver
	// GroupLogoutFunc is called when a session is expired due to inactivity so that peer
	// apps in the same SSO session group receive a peer_logout SSE event.
	GroupLogoutFunc func(sourceAppID, userID, reason, deviceID string)
}

// NewService creates a new session service.
func NewService() *Service {
	return &Service{}
}

// CreateSession creates a new session in Redis and returns the session ID.
// It generates tokens and stores the refresh token inside the session hash.
// Any prior user-wide token blacklist (set on password reset / account compromise)
// is cleared here because a fresh credential check already proved identity.
//
// accessTTL and refreshTTL control token lifetimes. Pass 0 to use the global
// defaults configured via environment variables.
func (s *Service) CreateSession(appID, userID, ip, userAgent, deviceID string, roles []string, accessTTL, refreshTTL time.Duration) (accessToken, refreshToken, sessionID string, appErr *errors.AppError) {
	sessionID = uuid.New().String()

	// Resolve effective refresh TTL for Redis session expiry
	effectiveRefreshTTL := refreshTTL
	if effectiveRefreshTTL <= 0 {
		effectiveRefreshTTL = jwt.DefaultRefreshTokenTTL()
	}

	accessToken, err := jwt.GenerateAccessToken(appID, userID, sessionID, roles, accessTTL)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}

	refreshToken, err = jwt.GenerateRefreshToken(appID, userID, sessionID, roles, refreshTTL)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}

	if err := redis.CreateSession(appID, sessionID, userID, refreshToken, ip, userAgent, deviceID, effectiveRefreshTTL); err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to create session")
	}

	// Clear any user-wide token blacklist so the newly issued tokens are not immediately
	// rejected by AuthMiddleware. The blacklist was set to invalidate pre-reset tokens;
	// a successful new login proves identity and a fresh token pair is now in use.
	if clearErr := redis.ClearUserTokenBlacklist(appID, userID); clearErr != nil {
		log.Printf("Warning: Failed to clear user token blacklist for user %s: %v\n", userID, clearErr)
	}

	return accessToken, refreshToken, sessionID, nil
}

// RefreshSession validates the old refresh token against the session, rotates tokens,
// and updates the session metadata. Returns new access token, new refresh token, and userID.
//
// accessTTL and refreshTTL control the new token lifetimes. Pass 0 to use the global defaults.
func (s *Service) RefreshSession(oldRefreshToken string, accessTTL, refreshTTL time.Duration) (string, string, string, *errors.AppError) {
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

	// --- Inactivity timeout enforcement ---
	// Check whether the session has been idle for longer than the per-app (or global)
	// inactivity window. The last_active field is updated on every token refresh so it
	// accurately tracks real user activity.
	if s.InactivityResolver != nil {
		timeoutMinutes := s.InactivityResolver.GetInactivityTimeoutMinutes(claims.AppID)
		if timeoutMinutes > 0 {
			lastActiveStr, laErr := redis.GetSessionField(claims.AppID, claims.SessionID, "last_active")
			if laErr == nil && lastActiveStr != "" {
				if lastActive, parseErr := time.Parse(time.RFC3339, lastActiveStr); parseErr == nil {
					if time.Since(lastActive) > time.Duration(timeoutMinutes)*time.Minute {
						// Read device_id before deleting so the peer_logout is scoped.
						deviceID, _ := redis.GetSessionField(claims.AppID, claims.SessionID, "device_id")
						// Revoke the session immediately so subsequent requests also fail.
						_ = redis.DeleteSession(claims.AppID, claims.SessionID, claims.UserID)
						// Fire peer group logout asynchronously so the user is signed out on all
						// peer apps too. Reason "expired" → amber "Session Expired" modal on frontends.
						if s.GroupLogoutFunc != nil {
							go s.GroupLogoutFunc(claims.AppID, claims.UserID, "expired", deviceID)
						}
						return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Session expired due to inactivity")
					}
				}
			}
		}
	}

	// Generate new token pair (same session ID).
	// If the session has an active org context (stored by /auth/reissue after a
	// context-switch) re-embed org_id / org_role so the user does not silently
	// lose their active org scope when the access token is silently rotated.
	var (
		newAccessToken string
		tokenErr       error
	)
	sessionOrgID, sessionOrgRole, _ := redis.GetSessionOrgContext(claims.AppID, claims.SessionID)
	if sessionOrgID != "" && sessionOrgRole != "" {
		newAccessToken, tokenErr = jwt.GenerateOrgContextToken(claims.AppID, claims.UserID, claims.SessionID, sessionOrgID, sessionOrgRole, claims.Roles, accessTTL)
	} else {
		newAccessToken, tokenErr = jwt.GenerateAccessToken(claims.AppID, claims.UserID, claims.SessionID, claims.Roles, accessTTL)
	}
	if tokenErr != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new access token")
	}
	var newRefreshToken string
	newRefreshToken, tokenErr = jwt.GenerateRefreshToken(claims.AppID, claims.UserID, claims.SessionID, claims.Roles, refreshTTL)
	if tokenErr != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new refresh token")
	}

	// Resolve effective refresh TTL for Redis session expiry
	effectiveRefreshTTL := refreshTTL
	if effectiveRefreshTTL <= 0 {
		effectiveRefreshTTL = jwt.DefaultRefreshTokenTTL()
	}

	// Update session with new refresh token and touch last_active
	if err := redis.UpdateSessionRefreshToken(claims.AppID, claims.SessionID, newRefreshToken); err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to update session")
	}
	// Reset the Redis session TTL so it slides forward with the newly issued
	// refresh token. Without this, the session key expires at the original
	// login time regardless of per-app TTL overrides or token rotation.
	if err := redis.ResetSessionTTL(claims.AppID, claims.SessionID, effectiveRefreshTTL); err != nil {
		log.Printf("Warning: Failed to reset session TTL: %v\n", err)
	}
	// Also slide the session_meta key TTL forward so keyspace-notification-based
	// session group expiry revocation does not fire prematurely for long-running
	// sessions that are actively refreshing their tokens.
	if err := redis.ResetSessionMetaTTL(claims.AppID, claims.UserID, claims.SessionID, effectiveRefreshTTL); err != nil {
		log.Printf("Warning: Failed to reset session_meta TTL: %v\n", err)
	}
	if err := redis.TouchSession(claims.AppID, claims.SessionID); err != nil {
		log.Printf("Warning: Failed to touch session: %v\n", err)
	}

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
	maxTokenLifetime := jwt.DefaultRefreshTokenTTL()
	if maxTokenLifetime <= 0 {
		maxTokenLifetime = 720 * time.Hour
	}
	if err := redis.BlacklistAllUserTokens(appID, userID, maxTokenLifetime); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to blacklist user tokens")
	}

	return nil
}

// RevokeUserSessionsByDevice deletes sessions for a user in an app where the
// device_id matches. Sessions without device_id (pre-migration) are deleted too.
func (s *Service) RevokeUserSessionsByDevice(appID, userID, deviceID string) *errors.AppError {
	if err := redis.DeleteUserSessionsByDevice(appID, userID, deviceID); err != nil {
		log.Printf("Warning: Failed to delete device-scoped sessions for user %s: %v\n", userID, err)
	}
	return nil
}

