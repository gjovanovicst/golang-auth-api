package sso

import (
	"log"
	"net/http"
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

	// Find the user in the target app by email.
	targetUser, err := h.UserRepo.GetUserByEmail(targetAppID, sourceUser.Email)
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
	accessToken, refreshToken, sessionID, appErr := h.SessionService.CreateSession(
		targetAppID, targetUser.ID.String(), ip, userAgent, roles, accessTTL, refreshTTL,
	)
	if appErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	log.Printf("[SSO] Exchange: user %s (source app %s) -> target app %s, session %s",
		sourceUser.Email, sourceAppID, targetAppID, sessionID)

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
