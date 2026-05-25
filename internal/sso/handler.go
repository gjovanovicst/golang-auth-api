package sso

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/session"
	"github.com/gjovanovicst/auth_api/internal/user"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RoleLookupFunc returns role names for (appID, userID). Matches the same
// type used in user.Service so callers can pass the same rbacService.GetUserRoleNames.
type RoleLookupFunc func(appID, userID string) ([]string, error)

// SSOPeerInfo describes a peer application in the same session group.
type SSOPeerInfo struct {
	AppID  string `json:"app_id"`
	Origin string `json:"origin"`
}

// AdminRepository is a minimal subset of admin.Repository that the SSO handler
// needs. Using an interface here avoids importing the admin package (import cycle).
type AdminRepository interface {
	// GetSessionGroupForApp returns the session group that the given appID belongs to.
	// Returns (nil, nil) when the app is not in any group.
	GetSessionGroupForApp(appID string) (*models.SessionGroup, error)
	// GetAppsInSessionGroup returns all app IDs that belong to the given group.
	GetAppsInSessionGroup(groupID string) ([]string, error)
	// GetPeersForApp returns all peer apps (excluding the requesting app) in the
	// same session group, with their app_id and frontend_url as origin.
	GetPeersForApp(appID string) ([]SSOPeerInfo, error)
}

// Handler serves the SSO token issuance and exchange endpoints.
type Handler struct {
	AdminRepo      AdminRepository
	UserRepo       *user.Repository
	SessionService *session.Service
	LookupRoles    RoleLookupFunc
	// DB is used for reading per-app token TTL overrides.
	DB *gorm.DB
}

// groupIDCache caches appID → groupID lookups so every SSE reconnect does not
// issue a DB query.  Entries are valid for 5 minutes; the TTL is short enough
// that a reconfigured session group is picked up quickly.
type groupIDEntry struct {
	groupID  string // empty string means "app not in any group"
	cachedAt time.Time
}

var (
	groupIDCacheMu sync.RWMutex
	groupIDCache   = map[string]groupIDEntry{}
	groupIDCacheTTL = 5 * time.Minute
)

func cachedGroupID(appID string, repo AdminRepository) (string, error) {
	groupIDCacheMu.RLock()
	entry, ok := groupIDCache[appID]
	groupIDCacheMu.RUnlock()
	if ok && time.Since(entry.cachedAt) < groupIDCacheTTL {
		return entry.groupID, nil
	}
	group, err := repo.GetSessionGroupForApp(appID)
	if err != nil {
		return "", err
	}
	groupID := ""
	if group != nil {
		groupID = group.ID.String()
	}
	groupIDCacheMu.Lock()
	groupIDCache[appID] = groupIDEntry{groupID: groupID, cachedAt: time.Now()}
	groupIDCacheMu.Unlock()
	return groupID, nil
}

// NewHandler creates a new SSO Handler.
func NewHandler(adminRepo AdminRepository, userRepo *user.Repository, sessionService *session.Service, db *gorm.DB) *Handler {
	return &Handler{
		AdminRepo:      adminRepo,
		UserRepo:       userRepo,
		SessionService: sessionService,
		DB:             db,
	}
}

// issueTokenRequest is the body for POST /sso/token.
type issueTokenRequest struct {
	TargetAppID string `json:"target_app_id" binding:"required"`
}

// IssueToken issues a short-lived (60 s), single-use SSO exchange token.
// The caller must be authenticated (JWT). The source app is taken from the JWT
// claims. The token encodes groupID|sourceAppID|userID and is stored in Redis.
//
// @Summary Issue SSO exchange token
// @Description Issue a 60-second single-use SSO token for cross-app login.
// @Tags sso
// @Accept json
// @Produce json
// @Param request body issueTokenRequest true "Target application ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Security ApiKeyAuth
// @Router /sso/token [post]
func (h *Handler) IssueToken(c *gin.Context) {
	// Extract user context set by AuthMiddleware.
	sourceAppIDRaw, _ := c.Get("app_id")
	userIDRaw, _ := c.Get("userID")
	sourceAppIDUUID, ok := sourceAppIDRaw.(uuid.UUID)
	var sourceAppID string
	if ok {
		sourceAppID = sourceAppIDUUID.String()
	} else {
		sourceAppID, _ = sourceAppIDRaw.(string)
	}
	userID, _ := userIDRaw.(string)

	if sourceAppID == "" || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing auth context"})
		return
	}

	var req issueTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_app_id is required"})
		return
	}

	// Validate that source app belongs to a session group.
	group, err := h.AdminRepo.GetSessionGroupForApp(sourceAppID)
	if err != nil || group == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "source app is not part of any session group"})
		return
	}

	// Validate that target app is in the same session group.
	groupApps, err := h.AdminRepo.GetAppsInSessionGroup(group.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate session group"})
		return
	}
	targetInGroup := false
	for _, appID := range groupApps {
		if appID == req.TargetAppID {
			targetInGroup = true
			break
		}
	}
	if !targetInGroup {
		c.JSON(http.StatusForbidden, gin.H{"error": "target app is not in the same session group"})
		return
	}

	// Generate a new opaque token and store it in Redis.
	token := uuid.New().String()
	if err := redis.SetSSOToken(token, group.ID.String(), sourceAppID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create SSO token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sso_token": token})
}

// exchangeRequest is the body for POST /sso/exchange.
type exchangeRequest struct {
	SSOToken string `json:"sso_token" binding:"required"`
}

// Exchange consumes a single-use SSO token and mints app-scoped access/refresh
// tokens for the user in the target app (identified by the X-App-ID header).
//
// @Summary Exchange SSO token for app-scoped tokens
// @Description Consume a single-use SSO token and receive access/refresh tokens for the target app.
// @Tags sso
// @Accept json
// @Produce json
// @Param X-App-ID header string true "Target application ID"
// @Param request body exchangeRequest true "SSO token"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /sso/exchange [post]
func (h *Handler) Exchange(c *gin.Context) {
	// Target app comes from X-App-ID header (set by AppIDMiddleware).
	targetAppIDRaw, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-App-ID header is required"})
		return
	}
	targetAppIDUUID, ok := targetAppIDRaw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-App-ID header is required"})
		return
	}
	targetAppID := targetAppIDUUID.String()

	var req exchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sso_token is required"})
		return
	}

	// Retrieve and immediately delete the SSO token (single-use).
	groupID, sourceAppID, userID, err := redis.GetSSOToken(req.SSOToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired SSO token"})
		return
	}
	if delErr := redis.DeleteSSOToken(req.SSOToken); delErr != nil {
		log.Printf("[SSO] Warning: failed to delete consumed SSO token: %v", delErr)
	}

	// Validate that the target app is in the same session group.
	groupApps, err := h.AdminRepo.GetAppsInSessionGroup(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate session group"})
		return
	}
	targetInGroup := false
	for _, appID := range groupApps {
		if appID == targetAppID {
			targetInGroup = true
			break
		}
	}
	if !targetInGroup {
		c.JSON(http.StatusForbidden, gin.H{"error": "target app is not in the same session group"})
		return
	}

	// Look up the user in the source app to get their email.
	sourceUser, err := h.UserRepo.GetUserByID(userID)
	if err != nil || sourceUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "source user not found"})
		return
	}

	// Find the user by email globally — a single user record is shared across all
	// apps in the session group (identified by a stable UUID). Per-app access is
	// controlled by user_apps rows rather than separate user records.
	targetUser, err := h.UserRepo.GetUserByEmailGlobal(sourceUser.Email)
	if err != nil || targetUser == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":       "user_not_found_in_target_app",
			"description": "the user does not have an account in the target application",
		})
		return
	}

	// Ensure the target-app user is active.
	if !targetUser.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "user account is deactivated in the target application"})
		return
	}

	// Resolve per-app token TTL overrides.
	var app models.Application
	var appPtr *models.Application
	if h.DB != nil {
		if h.DB.Select("access_token_ttl_minutes, refresh_token_ttl_hours").First(&app, "id = ?", targetAppID).Error == nil {
			appPtr = &app
		}
	}
	accessTTL, refreshTTL := resolveTokenTTLs(appPtr)

	// Look up roles for the target-app user.
	var roles []string
	if h.LookupRoles != nil {
		roles, _ = h.LookupRoles(targetAppID, targetUser.ID.String())
	}

	// Revoke any existing sessions for this user in the target app before
	// creating a new one. This prevents duplicate sessions accumulating when
	// the SSO exchange fires more than once (e.g. SSE reconnect, direct login
	// to a peer app that is already reached via SSO).
	if appErr := h.SessionService.RevokeAllUserSessions(targetAppID, targetUser.ID.String()); appErr != nil {
		log.Printf("[SSO] Exchange: warning — failed to revoke existing sessions for user %s in app %s: %v",
			targetUser.ID, targetAppID, appErr.Message)
	}

	// Create a new session in the target app.
	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	accessToken, refreshToken, sessionID, appErr := h.SessionService.CreateSession(
		targetAppID, targetUser.ID.String(), ip, userAgent, roles, accessTTL, refreshTTL,
	)
	if appErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	log.Printf("[SSO] Exchange: user %s (source app %s) -> target app %s, session %s",
		sourceUser.Email, sourceAppID, targetAppID, sessionID)

	// Record that the user is now logged in to the target app so that further
	// peer apps opened later can receive an on-demand peer_login event via the
	// presence-based fallback in StreamEvents.
	if presenceErr := redis.SetLoginPresence(targetAppID, targetUser.ID.String(), groupID); presenceErr != nil {
		log.Printf("[SSO] Exchange: failed to set login presence for target app %s: %v", targetAppID, presenceErr)
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"session_id":    sessionID,
		"user_id":       targetUser.ID.String(),
	})
}

// GetPeers returns the list of peer applications in the same session group as
// the requesting app (identified by X-App-ID header). This is a public endpoint
// used by frontends for dynamic peer discovery so they don't need to hardcode
// peer origins/app-IDs in their environment config.
//
// @Summary Get SSO peer apps
// @Description Return peer apps in the same session group (no auth required).
// @Tags sso
// @Produce json
// @Param X-App-ID header string true "Requesting application ID"
// @Success 200 {object} map[string]interface{}
// @Router /sso/peers [get]
func (h *Handler) GetPeers(c *gin.Context) {
	appIDRaw, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-App-ID header is required"})
		return
	}
	appIDUUID, ok := appIDRaw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-App-ID header is required"})
		return
	}
	appID := appIDUUID.String()

	peers, err := h.AdminRepo.GetPeersForApp(appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch peer apps"})
		return
	}
	if peers == nil {
		peers = []SSOPeerInfo{}
	}

	c.JSON(http.StatusOK, gin.H{"peers": peers})
}

// resolveTokenTTLs returns the effective access/refresh TTLs for an app, falling
// back to env-var defaults when the per-app override is 0.
func resolveTokenTTLs(app *models.Application) (accessTTL, refreshTTL time.Duration) {
	if app != nil && app.AccessTokenTTLMinutes > 0 {
		accessTTL = time.Duration(app.AccessTokenTTLMinutes) * time.Minute
	}
	if app != nil && app.RefreshTokenTTLHours > 0 {
		refreshTTL = time.Duration(app.RefreshTokenTTLHours) * time.Hour
	}
	return accessTTL, refreshTTL
}

// ============================================================================
// SSE Event Streaming
// ============================================================================

// ssoEvent is the shape published to Redis and forwarded to SSE clients.
type ssoEvent struct {
	Type      string `json:"type"`             // "peer_login" | "peer_logout" | "ping"
	SSOToken  string `json:"sso_token,omitempty"`
	TargetApp string `json:"target_app,omitempty"`
	Reason    string `json:"reason,omitempty"` // "voluntary" | "revoked" (peer_logout only)
}

// PublishLoginToGroup issues individual SSO exchange tokens for every peer app
// of sourceAppID and publishes a peer_login event to the Redis pub/sub channel
// for the session group.  It is safe to call in a goroutine.
func (h *Handler) PublishLoginToGroup(sourceAppID, userID string) {
	groupID, err := cachedGroupID(sourceAppID, h.AdminRepo)
	if err != nil || groupID == "" {
		return
	}

	// Record that this user is currently logged in to the source app so that
	// peer apps opening an SSE connection long after the login (past the 90 s
	// pending-event window) can still receive an on-demand peer_login event.
	if err := redis.SetLoginPresence(sourceAppID, userID, groupID); err != nil {
		log.Printf("[SSO] PublishLoginToGroup: failed to set login presence for app %s: %v", sourceAppID, err)
	}

	peers, err := h.AdminRepo.GetPeersForApp(sourceAppID)
	if err != nil {
		log.Printf("[SSO] PublishLoginToGroup: failed to get peers for app %s: %v", sourceAppID, err)
		return
	}

	for _, peer := range peers {
		token := uuid.New().String()
		if err := redis.SetSSOToken(token, groupID, sourceAppID, userID); err != nil {
			log.Printf("[SSO] PublishLoginToGroup: failed to set SSO token for peer %s: %v", peer.AppID, err)
			continue
		}
		evt := ssoEvent{
			Type:      "peer_login",
			SSOToken:  token,
			TargetApp: peer.AppID,
		}
		payload, _ := json.Marshal(evt)
		// Store for reconnecting clients BEFORE publishing so there is no window
		// where a client reconnects, misses the pub/sub message, and finds no pending entry.
		if err := redis.StorePendingLoginEvent(peer.AppID, string(payload)); err != nil {
			log.Printf("[SSO] PublishLoginToGroup: failed to store pending login for peer %s: %v", peer.AppID, err)
		}
		if err := redis.PublishSSOEvent(groupID, string(payload)); err != nil {
			log.Printf("[SSO] PublishLoginToGroup: failed to publish event for peer %s: %v", peer.AppID, err)
		}
	}
}

// PublishLogoutToGroup publishes a peer_logout event so all peer-app SSE
// clients can clear their tokens and redirect to the login page.
// reason should be "voluntary" for user-initiated logouts, "revoked" for
// admin/forced revocations.  Frontends use this to decide whether to show
// a "Session Revoked" modal or just redirect silently to the login page.
func (h *Handler) PublishLogoutToGroup(sourceAppID, userEmail, reason string) {
	groupID, err := cachedGroupID(sourceAppID, h.AdminRepo)
	if err != nil || groupID == "" {
		return
	}

	// Store a pending logout per peer app BEFORE publishing so that clients
	// which reconnect after the pub/sub message has already been delivered
	// still receive the logout signal on their next SSE connect.
	peers, err := h.AdminRepo.GetPeersForApp(sourceAppID)
	if err != nil {
		log.Printf("[SSO] PublishLogoutToGroup: failed to get peers for app %s: %v", sourceAppID, err)
	} else {
		for _, peer := range peers {
			if err := redis.StorePendingLogoutEvent(peer.AppID, reason); err != nil {
				log.Printf("[SSO] PublishLogoutToGroup: failed to store pending logout for peer %s: %v", peer.AppID, err)
			}
		}
	}

	// Also store a pending logout for the source app itself. When the logout is
	// triggered by an admin action or session expiry (not a user-initiated
	// logout from within the source app), the source app is excluded from
	// GetPeersForApp but still needs to receive the signal on its next SSE
	// reconnect so it can redirect to the login page.
	if err := redis.StorePendingLogoutEvent(sourceAppID, reason); err != nil {
		log.Printf("[SSO] PublishLogoutToGroup: failed to store pending logout for source app %s: %v", sourceAppID, err)
	}

	evt := ssoEvent{Type: "peer_logout", Reason: reason}
	payload, _ := json.Marshal(evt)
	if err := redis.PublishSSOEvent(groupID, string(payload)); err != nil {
		log.Printf("[SSO] PublishLogoutToGroup: failed to publish logout event: %v", err)
	}
}

// StreamEvents opens a Server-Sent Events stream for the app identified by the
// app_id query parameter (set by AppIDMiddleware).  The client receives
// peer_login and peer_logout events published by other apps in the same session
// group, as well as periodic ping frames to keep the connection alive.
//
// @Summary SSO Server-Sent Events stream
// @Description Subscribe to cross-app SSO login/logout events via SSE.
// @Tags sso
// @Produce text/event-stream
// @Param app_id query string true "Requesting application ID"
// @Success 200
// @Router /sso/events [get]
func (h *Handler) StreamEvents(c *gin.Context) {
	appIDRaw, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "app_id is required"})
		return
	}
	appIDUUID, ok := appIDRaw.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "app_id is required"})
		return
	}
	appID := appIDUUID.String()

	groupID, err := cachedGroupID(appID, h.AdminRepo)
	if err != nil || groupID == "" {
		// App not in a session group — return empty stream that closes immediately.
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		return
	}

	pubsub := redis.SubscribeSSOEvents(groupID)
	defer pubsub.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	redisCh := pubsub.Channel()
	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	flusher, canFlush := c.Writer.(http.Flusher)
	clientGone := c.Request.Context().Done()

	writeAndFlush := func(data string) bool {
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
			return false
		}
		if canFlush {
			flusher.Flush()
		}
		return true
	}

	// Replay any pending login event that was published while this client was
	// disconnected (e.g. killed by a proxy idle timeout between events).
	if pending, err := redis.PopPendingLoginEvent(appID); err == nil && pending != "" {
		writeAndFlush(pending)
	} else {
		// Fallback: the short-lived pending-login key (90 s) has expired, but the
		// user may still have an active session in a peer app.  Check every peer
		// app's login-presence record (24 h rolling TTL) and, if found, issue a
		// fresh on-demand SSO token so this newly-opened tab is auto-logged-in.
		peers, peersErr := h.AdminRepo.GetPeersForApp(appID)
		if peersErr == nil {
			for _, peer := range peers {
				presenceUserID, presenceGroupID, presenceErr := redis.GetLoginPresence(peer.AppID)
				if presenceErr != nil || presenceUserID == "" {
					continue
				}
				// Mint a fresh single-use exchange token for this app.
				token := uuid.New().String()
				if tokenErr := redis.SetSSOToken(token, presenceGroupID, peer.AppID, presenceUserID); tokenErr != nil {
					log.Printf("[SSO] StreamEvents: failed to set on-demand SSO token for app %s: %v", appID, tokenErr)
					continue
				}
				evt := ssoEvent{
					Type:      "peer_login",
					SSOToken:  token,
					TargetApp: appID,
				}
				payload, _ := json.Marshal(evt)
				writeAndFlush(string(payload))
				break // one peer with an active session is enough
			}
		}
	}

	// Replay any pending logout event — takes priority over a stale login so
	// check after login to let login write first, but logout will overwrite the
	// client state correctly on the frontend.
	if pending, err := redis.PopPendingLogoutEvent(appID); err == nil && pending != "" {
		writeAndFlush(pending)
	}
	for {
		select {
		case <-clientGone:
			return

		case <-pingTicker.C:
			// SSE comment line — keeps the connection alive without dispatching
			// a message event on the client. Browsers reset their idle-close
			// timer on any received bytes, so this prevents ~30s disconnects.
			if _, err := fmt.Fprintf(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			if canFlush {
				flusher.Flush()
			}

		case msg, ok := <-redisCh:
			if !ok {
				return
			}
			// Only forward events that target this app or have no target.
			var evt ssoEvent
			if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
				continue
			}
			if evt.TargetApp != "" && evt.TargetApp != appID {
				continue
			}
			if !writeAndFlush(msg.Payload) {
				return
			}
		}
	}
}
