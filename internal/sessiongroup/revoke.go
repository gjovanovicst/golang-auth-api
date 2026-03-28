package sessiongroup

import (
	"log"

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
	group, err := r.AdminRepo.GetSessionGroupForApp(appID)
	if err != nil || group == nil || !group.GlobalLogout {
		return
	}

	appIDs, err := r.AdminRepo.GetAppsInSessionGroup(group.ID.String())
	if err != nil {
		return
	}

	for _, otherAppID := range appIDs {
		if otherAppID == appID {
			continue
		}

		targetUser, err := r.UserRepo.GetUserByEmail(otherAppID, userEmail)
		if err != nil || targetUser == nil {
			continue
		}

		if appErr := r.SessionService.RevokeAllUserSessions(otherAppID, targetUser.ID.String()); appErr != nil {
			log.Printf("[SessionGroup] Warning: failed to revoke sessions for user %s in app %s: %v",
				userEmail, otherAppID, appErr.Message)
		} else {
			log.Printf("[SessionGroup] Revoked sessions for user %s in app %s (session group: %s)",
				userEmail, otherAppID, group.Name)
		}
	}
}

// RevokeAllUserSessionsInGroupByUserID revokes all sessions for a user across all apps in the same session group
// using the user ID instead of email. This is useful when you have the user ID but not the email.
func (r *Revoker) RevokeAllUserSessionsInGroupByUserID(appID, userID string) {
	// First get the user to get their email
	userObj, err := r.UserRepo.GetUserByID(userID)
	if err != nil || userObj == nil {
		return
	}

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
	return r.UserRepo.GetUserByID(userID)
}
