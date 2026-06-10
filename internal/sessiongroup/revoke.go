package sessiongroup

import (
	"log"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/session"
	"github.com/gjovanovicst/auth_api/internal/user"
	"github.com/gjovanovicst/auth_api/pkg/models"
)

// AdminRepositoryInterface defines the subset of admin.Repository methods needed for session group revocation
type AdminRepositoryInterface interface {
	GetSessionGroupForApp(appID string) (*models.SessionGroup, error)
	GetAppsInSessionGroup(groupID string) ([]string, error)
}

// Revoker provides methods to revoke sessions across session groups
type Revoker struct {
	AdminRepo      AdminRepositoryInterface
	UserRepo       *user.Repository
	SessionService *session.Service
	// GroupLogoutFunc, if set, is called after all peer sessions are revoked so
	// the SSE layer can push a peer_logout event to all connected clients.
	// Signature matches sso.Handler.PublishLogoutToGroup.
	GroupLogoutFunc func(sourceAppID, userID, reason, deviceID string)
}

// NewRevoker creates a new session group revoker
func NewRevoker(adminRepo AdminRepositoryInterface, userRepo *user.Repository, sessionService *session.Service) *Revoker {
	return &Revoker{
		AdminRepo:      adminRepo,
		UserRepo:       userRepo,
		SessionService: sessionService,
	}
}

// RevokeAllUserSessionsInGroup revokes all sessions for a user across all apps in the same session group
// when GlobalLogout is enabled. reason is forwarded to the SSE peer_logout event so clients can
// distinguish between "revoked" (admin/explicit) and "expired" (natural TTL expiry).
func (r *Revoker) RevokeAllUserSessionsInGroup(appID, userEmail, reason, deviceID string) {
	log.Printf("[SessionGroup] RevokeAllUserSessionsInGroup called: appID=%s userEmail=%s reason=%s deviceID=%s", appID, userEmail, reason, deviceID)

	group, err := r.AdminRepo.GetSessionGroupForApp(appID)
	if err != nil {
		log.Printf("[SessionGroup] ERROR fetching session group for appID=%s: %v", appID, err)
		return
	}
	if group == nil {
		log.Printf("[SessionGroup] No session group found for appID=%s — skipping group revocation", appID)
		return
	}
	if !group.GlobalLogout {
		log.Printf("[SessionGroup] GlobalLogout=false for group %s (%s) — skipping group revocation", group.Name, group.ID)
		return
	}
	log.Printf("[SessionGroup] Found group %s (%s) GlobalLogout=true", group.Name, group.ID)

	appIDs, err := r.AdminRepo.GetAppsInSessionGroup(group.ID.String())
	if err != nil {
		log.Printf("[SessionGroup] ERROR fetching apps in group %s: %v", group.ID, err)
		return
	}
	log.Printf("[SessionGroup] Apps in group: %v", appIDs)

	// Look up the user once by email globally — user records are shared across
	// all apps in the session group (one UUID per person, access controlled by
	// user_apps rows). Per-app GetUserByEmail would miss users who only joined
	// via SSO exchange (they have no separate per-app user row).
	targetUser, err := r.UserRepo.GetUserByEmailGlobal(userEmail)
	if err != nil || targetUser == nil {
		log.Printf("[SessionGroup] Warning: user %s not found globally (err=%v) — cannot revoke peer sessions", userEmail, err)
		return
	}
	log.Printf("[SessionGroup] Resolved user %s → userID=%s", userEmail, targetUser.ID)

	for _, targetAppID := range appIDs {
		// When reason is "expired", the triggering app's session has already expired
		// naturally — there is nothing to revoke there, and revoking it would delete
		// any new session the user may have just created on that same app (race window
		// between login and the keyspace expiry notification firing).
		if reason == "expired" && targetAppID == appID {
			log.Printf("[SessionGroup] Skipping source app %s for expired-triggered revocation (session already gone)", targetAppID)
			continue
		}
		if deviceID != "" {
			if appErr := r.SessionService.RevokeUserSessionsByDevice(targetAppID, targetUser.ID.String(), deviceID); appErr != nil {
				log.Printf("[SessionGroup] Warning: failed to revoke device sessions for user %s in app %s: %v",
					userEmail, targetAppID, appErr.Message)
			} else {
				log.Printf("[SessionGroup] Revoked device sessions for user %s in app %s (session group: %s)",
					userEmail, targetAppID, group.Name)
			}
		} else {
			if appErr := r.SessionService.RevokeAllUserSessions(targetAppID, targetUser.ID.String()); appErr != nil {
				log.Printf("[SessionGroup] Warning: failed to revoke sessions for user %s in app %s: %v",
					userEmail, targetAppID, appErr.Message)
			} else {
				log.Printf("[SessionGroup] Revoked sessions for user %s in app %s (session group: %s)",
					userEmail, targetAppID, group.Name)
			}
		}

		// Remove the SSO login-presence record so that peer apps opened after this
		// group logout do not receive a stale on-demand peer_login event.
		if presenceErr := redis.DeleteLoginPresence(targetAppID); presenceErr != nil {
			log.Printf("[SessionGroup] Warning: failed to delete login presence for app %s: %v", targetAppID, presenceErr)
		}
	}

	// Notify all SSE-connected clients that their sessions have been revoked/expired.
	if r.GroupLogoutFunc != nil {
		r.GroupLogoutFunc(appID, targetUser.ID.String(), reason, deviceID)
	}
}

// RevokeAllUserSessionsInGroupByUserID revokes all sessions for a user across all apps in the same session group
// using the user ID instead of email. This is useful when you have the user ID but not the email.
func (r *Revoker) RevokeAllUserSessionsInGroupByUserID(appID, userID, deviceID string) {
	log.Printf("[SessionGroup] RevokeAllUserSessionsInGroupByUserID called: appID=%s userID=%s deviceID=%s", appID, userID, deviceID)
	// First get the user to get their email — basic fetch, no social account preload needed
	userObj, err := r.UserRepo.GetUserByIDBasic(userID)
	if err != nil || userObj == nil {
		log.Printf("[SessionGroup] ERROR: could not fetch user for userID=%s err=%v obj=%v", userID, err, userObj)
		return
	}
	log.Printf("[SessionGroup] Resolved userID=%s → email=%s", userID, userObj.Email)

	r.RevokeAllUserSessionsInGroup(appID, userObj.Email, "revoked", deviceID)
}

// ShouldRevokeGroupSessions checks if a session group has GlobalLogout enabled
func (r *Revoker) ShouldRevokeGroupSessions(appID string) (bool, *models.SessionGroup) {
	group, err := r.AdminRepo.GetSessionGroupForApp(appID)
	if err != nil || group == nil {
		return false, nil
	}
	return group.GlobalLogout, group
}

// ClearGroupUserBlacklist clears the user-wide token blacklist for a user across
// ALL applications — not just the session group peers. This is called after a
// successful login to guarantee that stale revocation entries left over from
// previous force-logouts, session-group expiries, or container restarts cannot
// block the newly authenticated user regardless of current group membership.
//
// Uses a Redis SCAN sweep (ScanAndClearAllUserBlacklists) which is safe and cheap
// in typical deployments. Falls back to per-app clear on error.
func (r *Revoker) ClearGroupUserBlacklist(appID, userID string) {
	if err := redis.ScanAndClearAllUserBlacklists(userID); err != nil {
		// Scan failed — fall back to clearing at least the login app and its group peers
		log.Printf("[SessionGroup] ClearGroupUserBlacklist: SCAN failed for user %s, falling back to per-app clear: %v", userID, err)
		if clearErr := redis.ClearUserTokenBlacklist(appID, userID); clearErr != nil {
			log.Printf("[SessionGroup] Warning: failed to clear token blacklist for user %s in app %s: %v", userID, appID, clearErr)
		}
		group, gerr := r.AdminRepo.GetSessionGroupForApp(appID)
		if gerr != nil || group == nil {
			return
		}
		appIDs, gerr := r.AdminRepo.GetAppsInSessionGroup(group.ID.String())
		if gerr != nil {
			return
		}
		for _, targetAppID := range appIDs {
			if clearErr := redis.ClearUserTokenBlacklist(targetAppID, userID); clearErr != nil {
				log.Printf("[SessionGroup] Warning: failed to clear token blacklist for user %s in app %s: %v", userID, targetAppID, clearErr)
			}
		}
		return
	}
	log.Printf("[SessionGroup] Cleared all blacklist entries for user %s across all apps", userID)
}

// GetUserByID gets a user by ID (implements ExpiryHandlerInterface)
func (r *Revoker) GetUserByID(userID string) (*models.User, error) {
	return r.UserRepo.GetUserByIDBasic(userID)
}
