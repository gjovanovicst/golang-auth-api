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
	GroupLogoutFunc func(sourceAppID, userEmail string)
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
// when GlobalLogout is enabled. This is called when a session expires or when a user logs out.
func (r *Revoker) RevokeAllUserSessionsInGroup(appID, userEmail string) {
	log.Printf("[SessionGroup] RevokeAllUserSessionsInGroup called: appID=%s userEmail=%s", appID, userEmail)

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
		if appErr := r.SessionService.RevokeAllUserSessions(targetAppID, targetUser.ID.String()); appErr != nil {
			log.Printf("[SessionGroup] Warning: failed to revoke sessions for user %s in app %s: %v",
				userEmail, targetAppID, appErr.Message)
		} else {
			log.Printf("[SessionGroup] Revoked sessions for user %s in app %s (session group: %s)",
				userEmail, targetAppID, group.Name)
		}

		// Remove the SSO login-presence record so that peer apps opened after this
		// group logout do not receive a stale on-demand peer_login event.
		if presenceErr := redis.DeleteLoginPresence(targetAppID); presenceErr != nil {
			log.Printf("[SessionGroup] Warning: failed to delete login presence for app %s: %v", targetAppID, presenceErr)
		}
	}

	// Notify all SSE-connected clients that they should log out.
	if r.GroupLogoutFunc != nil {
		r.GroupLogoutFunc(appID, userEmail)
	}
}

// RevokeAllUserSessionsInGroupByUserID revokes all sessions for a user across all apps in the same session group
// using the user ID instead of email. This is useful when you have the user ID but not the email.
func (r *Revoker) RevokeAllUserSessionsInGroupByUserID(appID, userID string) {
	log.Printf("[SessionGroup] RevokeAllUserSessionsInGroupByUserID called: appID=%s userID=%s", appID, userID)
	// First get the user to get their email — basic fetch, no social account preload needed
	userObj, err := r.UserRepo.GetUserByIDBasic(userID)
	if err != nil || userObj == nil {
		log.Printf("[SessionGroup] ERROR: could not fetch user for userID=%s err=%v obj=%v", userID, err, userObj)
		return
	}
	log.Printf("[SessionGroup] Resolved userID=%s → email=%s", userID, userObj.Email)

	r.RevokeAllUserSessionsInGroup(appID, userObj.Email)
}

// ShouldRevokeGroupSessions checks if a session group has GlobalLogout enabled
func (r *Revoker) ShouldRevokeGroupSessions(appID string) (bool, *models.SessionGroup) {
	group, err := r.AdminRepo.GetSessionGroupForApp(appID)
	if err != nil || group == nil {
		return false, nil
	}
	return group.GlobalLogout, group
}

// GetUserByID gets a user by ID (implements ExpiryHandlerInterface)
func (r *Revoker) GetUserByID(userID string) (*models.User, error) {
	return r.UserRepo.GetUserByIDBasic(userID)
}
