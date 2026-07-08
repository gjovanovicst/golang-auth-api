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
	"github.com/gjovanovicst/auth_api/internal/util"
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

	// Create a new session in the target app.
	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	deviceID := util.DeviceFingerprint(c)
	accessToken, refreshToken, sessionID, appErr := h.SessionService.CreateSession(
		targetAppID, targetUser.ID.String(), ip, userAgent, deviceID, roles, accessTTL, refreshTTL,
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
	exchangeDeviceID := util.DeviceFingerprint(c)
	if presenceErr := redis.SetLoginPresence(targetAppID, targetUser.ID.String(), groupID, exchangeDeviceID); presenceErr != nil {
		log.Printf("[SSO] Exchange: failed to set login presence for target app %s: %v", targetAppID, presenceErr)
	} else {
		_ = redis.StorePresenceDeviceMapping(groupID, exchangeDeviceID, targetUser.ID.String())
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
	DeviceID  string `json:"device_id,omitempty"`
	UserID    string `json:"user_id,omitempty"` // scopes the event to a specific user
}

// PublishLoginToGroup issues individual SSO exchange tokens for every peer app
// of sourceAppID and publishes a peer_login event to the Redis pub/sub channel
// for the session group. It is safe to call in a goroutine.
// deviceID is the device fingerprint (IP+UA hash) scoping the SSO token.
func (h *Handler) PublishLoginToGroup(sourceAppID, userID, deviceID string) {
	groupID, err := cachedGroupID(sourceAppID, h.AdminRepo)
	if err != nil || groupID == "" {
		return
	}

	// Clear any stale pending logout event for the source app so a previous
	// session's revocation is not replayed to the newly logged-in client.
	_ = redis.DeletePendingLogoutEvent(sourceAppID, userID)

	// Record that this user is currently logged in to the source app so that
	// peer apps opening an SSE connection long after the login (past the 90 s
	// pending-event window) can still receive an on-demand peer_login event.
	if err := redis.SetLoginPresence(sourceAppID, userID, groupID, deviceID); err != nil {
		log.Printf("[SSO] PublishLoginToGroup: failed to set login presence for app %s: %v", sourceAppID, err)
	} else {
		_ = redis.StorePresenceDeviceMapping(groupID, deviceID, userID)
	}

	peers, err := h.AdminRepo.GetPeersForApp(sourceAppID)
	if err != nil {
		log.Printf("[SSO] PublishLoginToGroup: failed to get peers for app %s: %v", sourceAppID, err)
		return
	}

	for _, peer := range peers {
		// Clear any stale pending logout for each peer app as well.
		_ = redis.DeletePendingLogoutEvent(peer.AppID, userID)

		token := uuid.New().String()
		if err := redis.SetSSOToken(token, groupID, sourceAppID, userID); err != nil {
			log.Printf("[SSO] PublishLoginToGroup: failed to set SSO token for peer %s: %v", peer.AppID, err)
			continue
		}
		evt := ssoEvent{
			Type:      "peer_login",
			SSOToken:  token,
			TargetApp: peer.AppID,
			DeviceID:  deviceID,
			UserID:    userID,
		}
		payload, _ := json.Marshal(evt)
		// Store for reconnecting clients BEFORE publishing so there is no window
		// where a client reconnects, misses the pub/sub message, and finds no pending entry.
		if err := redis.StorePendingLoginEvent(peer.AppID, userID, string(payload)); err != nil {
		log.Printf("[SSO] PublishLoginToGroup: failed to store pending login for peer %s: %v", peer.AppID, err)
		}
		// Also store a deviceID→userID mapping so the SSE StreamEvents handler
			// can find this pending event by device fingerprint when the client does
			// not provide a user_id (e.g. no cached_user_profile on that origin).
			if err := redis.StorePendingLoginDeviceMapping(peer.AppID, deviceID, userID); err != nil {
				log.Printf("[SSO] PublishLoginToGroup: failed to store pending login device mapping for peer %s: %v", peer.AppID, err)
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
func (h *Handler) PublishLogoutToGroup(sourceAppID, userID, reason, deviceID string) {
	log.Printf("[SSO] PublishLogoutToGroup: source=%s user=%s reason=%s device=%s", sourceAppID, userID, reason, deviceID)
	groupID, err := cachedGroupID(sourceAppID, h.AdminRepo)
	if err != nil || groupID == "" {
		log.Printf("[SSO] PublishLogoutToGroup: group not found for app %s (err=%v) — skipping", sourceAppID, err)
		return
	}

	peers, err := h.AdminRepo.GetPeersForApp(sourceAppID)
	if err != nil {
		log.Printf("[SSO] PublishLogoutToGroup: failed to get peers for app %s: %v", sourceAppID, err)
	} else {
		for _, peer := range peers {
			if err := redis.StorePendingLogoutEvent(peer.AppID, userID, reason, deviceID); err != nil {
				log.Printf("[SSO] PublishLogoutToGroup: failed to store pending logout for peer %s: %v", peer.AppID, err)
			}
			evt := ssoEvent{Type: "peer_logout", Reason: reason, DeviceID: deviceID, UserID: userID, TargetApp: peer.AppID}
			payload, _ := json.Marshal(evt)
			if err := redis.PublishSSOEvent(groupID, string(payload)); err != nil {
				log.Printf("[SSO] PublishLogoutToGroup: failed to publish logout event for peer %s: %v", peer.AppID, err)
			}
		}
	}

	// Also notify the source app — needed when logout is triggered by an admin
	// action or session expiry where the source app didn't initiate the logout.
	if err := redis.StorePendingLogoutEvent(sourceAppID, userID, reason, deviceID); err != nil {
		log.Printf("[SSO] PublishLogoutToGroup: failed to store pending logout for source app %s: %v", sourceAppID, err)
	}
	evt := ssoEvent{Type: "peer_logout", Reason: reason, DeviceID: deviceID, UserID: userID, TargetApp: sourceAppID}
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

	// Compute the device fingerprint for this SSE connection. Used to
	// filter peer_login and peer_logout events to the same browser/device.
	sseDeviceID := util.DeviceFingerprint(c)

	// user_id is passed as a query param so that SSO events are scoped to
	// the connecting user. Without it, events from a different user sharing
	// the same browser/deviceID would leak across SSE connections.
	userID := c.Query("user_id")

	// If the client did not provide a user_id (e.g. no cached_user_profile on
	// this origin), try to discover the user by device fingerprint.  This
	// allows cross-app SSO to work when the user opens a peer app for the
	// first time and has no stored identity on that origin.
	if userID == "" && sseDeviceID != "" {
		// 1. Try pending login device mapping (90 s TTL, set on recent login).
		if deviceUserID, err := redis.PopPendingLoginDeviceMapping(appID, sseDeviceID); err == nil && deviceUserID != "" {
			userID = deviceUserID
		}
		// 2. Fall back to presence device mapping (1 h TTL, refreshed on token refresh).
		if userID == "" {
			if deviceUserID, err := redis.GetPresenceUserByDevice(groupID, sseDeviceID); err == nil && deviceUserID != "" {
				userID = deviceUserID
			}
		}
	}

	// Replay any pending login event that was published while this client was
	// disconnected. Only replay if the device fingerprint matches AND the
	// userID matches (when provided).
	replayedLogin := false
	if userID != "" {
		if pending, err := redis.PopPendingLoginEvent(appID, userID); err == nil && pending != "" {
			var pendingEvt ssoEvent
			if json.Unmarshal([]byte(pending), &pendingEvt) == nil {
				if pendingEvt.DeviceID == "" || pendingEvt.DeviceID == sseDeviceID {
					writeAndFlush(pending)
					replayedLogin = true
				}
			}
		}
	}

	// Replay any pending logout event — takes priority over a stale login so
	// check after login to let login write first, but logout will overwrite the
	// client state correctly on the frontend.
	// user_id is passed as a query param so we only replay the logout for the
	// specific user connecting, not for any other user who may have logged out
	// of the same app previously.
	replayedLogout := false
	if userID != "" {
		if pending, err := redis.PopPendingLogoutEvent(appID, userID); err == nil && pending != "" {
			var pendingEvt ssoEvent
			if json.Unmarshal([]byte(pending), &pendingEvt) == nil {
				if pendingEvt.DeviceID == "" || pendingEvt.DeviceID == sseDeviceID {
					writeAndFlush(pending)
					replayedLogout = true
				}
			}
		}
	}

	// Presence-based fallback: when no pending login or logout was replayed
	// (e.g. a new tab opened long after the initial login, past the 90 s
	// pending-event window), scan peer apps in the same session group for an
	// active login-presence record.  If one is found, generate an on-demand
	// SSO exchange token and emit a synthetic peer_login event so the client
	// can auto-login without re-entering credentials.
	if !replayedLogin && !replayedLogout && userID != "" {
		peerApps, peerErr := h.AdminRepo.GetAppsInSessionGroup(groupID)
		if peerErr == nil {
			for _, peerAppID := range peerApps {
				if peerAppID == appID {
					continue
				}
				peerGroupID, presenceDeviceID, presErr := redis.GetLoginPresence(peerAppID, userID)
				if presErr != nil || peerGroupID != groupID {
				 continue
				}
				// Only auto-login if the presence was set by the same device.
				// Without this check, a stale cached_user_profile from browser data
				// import in a different browser (different deviceID) would cause
				// auto-login as the wrong user.
				if presenceDeviceID != "" && presenceDeviceID != sseDeviceID {
				log.Printf("[SSO] StreamEvents: presence-based fallback skipped — device mismatch for user %s (presence=%s, sse=%s)",
				userID, presenceDeviceID, sseDeviceID)
				continue
				}
				// Found a peer with an active login presence for this user on this device —
				// generate a single-use SSO token scoped to the requesting app.
				token := uuid.New().String()
				if tokErr := redis.SetSSOToken(token, groupID, peerAppID, userID); tokErr != nil {
					log.Printf("[SSO] StreamEvents: failed to create on-demand SSO token for app %s (peer %s): %v",
						appID, peerAppID, tokErr)
					continue
				}
				evt := ssoEvent{
					Type:      "peer_login",
					SSOToken:  token,
					TargetApp: appID,
					UserID:    userID,
				}
				payload, _ := json.Marshal(evt)
				log.Printf("[SSO] StreamEvents: emitted on-demand peer_login for app %s (peer %s user %s)",
					appID, peerAppID, userID)
				writeAndFlush(string(payload))
				break // one on-demand token is enough
			}
		}
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
			// Filter peer_login and peer_logout by device fingerprint.
			// Events without a device_id (old clients) broadcast to all.
			if evt.DeviceID != "" && evt.DeviceID != sseDeviceID {
				continue
			}
			// peer_login and peer_logout events are user-scoped when the SSE
			// connection provides a user_id. This prevents session leakage
			// between different users sharing the same browser/device.
			// When the client is unauthenticated (no user_id), events are
			// still delivered; the SSO token exchange provides its own
			// security by verifying the source user's identity.
			if userID != "" && evt.UserID != "" && evt.UserID != userID {
				continue
			}
			if !writeAndFlush(msg.Payload) {
				return
			}
		}
	}
}
