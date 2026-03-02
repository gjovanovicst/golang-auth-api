package webauthn

import (
	"fmt"
	stdlog "log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

// RoleLookupFunc is a function that returns role names for a user in an app.
type RoleLookupFunc func(appID, userID string) ([]string, error)

// Handler handles HTTP requests for WebAuthn/Passkey operations.
type Handler struct {
	Service     *Service
	LookupRoles RoleLookupFunc
}

// NewHandler creates a new WebAuthn handler.
func NewHandler(s *Service) *Handler {
	return &Handler{Service: s}
}

// getUserRoles fetches roles for JWT embedding. Returns nil on error (non-fatal).
func (h *Handler) getUserRoles(appID, userID string) []string {
	if h.LookupRoles == nil {
		return nil
	}
	roles, err := h.LookupRoles(appID, userID)
	if err != nil {
		stdlog.Printf("Warning: failed to lookup roles for user %s in app %s: %v", userID, appID, err)
		return nil
	}
	return roles
}

// ============================================================================
// Passkey Registration (Protected — requires JWT auth)
// ============================================================================

// @Summary Begin passkey registration
// @Description Start the WebAuthn registration ceremony to add a new passkey
// @Tags Passkeys
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.PasskeyRegisterBeginResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /passkey/register/begin [post]
func (h *Handler) BeginRegistration(c *gin.Context) {
	userID, appID, err := extractUserAndApp(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	options, appErr := h.Service.BeginRegistration(appID, userID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.PasskeyRegisterBeginResponse{Options: options})
}

// @Summary Finish passkey registration
// @Description Complete the WebAuthn registration ceremony with the client's attestation response
// @Tags Passkeys
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.PasskeyRegisterFinishRequest true "Registration response"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /passkey/register/finish [post]
func (h *Handler) FinishRegistration(c *gin.Context) {
	userID, appID, err := extractUserAndApp(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	var req dto.PasskeyRegisterFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if appErr := h.Service.FinishRegistration(appID, userID, req.Name, req.Credential); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	// Log passkey registration
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogPasskeyRegister(appID, userID, ipAddress, userAgent, req.Name)

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Passkey registered successfully"})
}

// ============================================================================
// Passkey Management (Protected — requires JWT auth)
// ============================================================================

// @Summary List passkeys
// @Description List all registered passkeys for the authenticated user
// @Tags Passkeys
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.PasskeyListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /passkeys [get]
func (h *Handler) ListCredentials(c *gin.Context) {
	userID, appID, err := extractUserAndApp(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	creds, appErr := h.Service.ListCredentials(userID, appID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	passkeys := make([]dto.PasskeyResponse, len(creds))
	for i, cred := range creds {
		passkeys[i] = toPasskeyResponse(cred)
	}

	c.JSON(http.StatusOK, dto.PasskeyListResponse{Passkeys: passkeys})
}

// @Summary Rename a passkey
// @Description Update the friendly name of a registered passkey
// @Tags Passkeys
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Passkey ID"
// @Param request body dto.PasskeyRenameRequest true "New name"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /passkeys/{id} [put]
func (h *Handler) RenameCredential(c *gin.Context) {
	userID, _, err := extractUserAndApp(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	credID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid passkey ID"})
		return
	}

	var req dto.PasskeyRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if appErr := h.Service.RenameCredential(userID, credID, req.Name); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Passkey renamed successfully"})
}

// @Summary Delete a passkey
// @Description Remove a registered passkey
// @Tags Passkeys
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Passkey ID"
// @Success 200 {object} dto.MessageResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /passkeys/{id} [delete]
func (h *Handler) DeleteCredential(c *gin.Context) {
	userID, appID, err := extractUserAndApp(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	credID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid passkey ID"})
		return
	}

	if appErr := h.Service.DeleteCredential(userID, credID); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	// Log passkey deletion
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogPasskeyDelete(appID, userID, ipAddress, userAgent)

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Passkey deleted successfully"})
}

// ============================================================================
// Passkey 2FA Login (Public — uses temp token)
// ============================================================================

// @Summary Begin passkey 2FA verification
// @Description Start the WebAuthn assertion ceremony for 2FA verification during login
// @Tags 2FA
// @Accept json
// @Produce json
// @Param request body dto.Passkey2FABeginRequest true "Temp token"
// @Success 200 {object} dto.Passkey2FABeginResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/passkey/begin [post]
func (h *Handler) BeginPasskey2FA(c *gin.Context) {
	var req dto.Passkey2FABeginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	// Get userID from temporary session
	userIDStr, err := redis.GetTempUserSession(appID.String(), req.TempToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid or expired temporary token"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	options, appErr := h.Service.BeginLogin(appID, userID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.Passkey2FABeginResponse{Options: options})
}

// @Summary Finish passkey 2FA verification
// @Description Complete the WebAuthn assertion ceremony for 2FA verification during login
// @Tags 2FA
// @Accept json
// @Produce json
// @Param request body dto.Passkey2FAFinishRequest true "Assertion response"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/passkey/finish [post]
func (h *Handler) FinishPasskey2FA(c *gin.Context) {
	var req dto.Passkey2FAFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	// Get userID from temporary session
	userIDStr, err := redis.GetTempUserSession(appID.String(), req.TempToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid or expired temporary token"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	// Verify the passkey assertion
	if appErr := h.Service.FinishLogin(appID, userID, req.Credential); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	// Generate final tokens
	roles := h.getUserRoles(appID.String(), userIDStr)
	accessToken, refreshToken, tokenErr := generateTokensForUser(appID.String(), userIDStr, roles)
	if tokenErr != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	// Log successful 2FA login via passkey
	ipAddress, userAgent := util.GetClientInfo(c)
	log.Log2FALogin(appID, userID, ipAddress, userAgent, "passkey")

	// Clear temporary session
	clearTempSession(appID.String(), req.TempToken)

	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// ============================================================================
// Passwordless Login (Public)
// ============================================================================

// @Summary Begin passwordless login
// @Description Start a passwordless login ceremony using discoverable credentials (passkeys)
// @Tags Passkeys
// @Produce json
// @Success 200 {object} dto.PasskeyLoginBeginResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /passkey/login/begin [post]
func (h *Handler) BeginPasswordlessLogin(c *gin.Context) {
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	options, sessionID, appErr := h.Service.BeginPasswordlessLogin(appID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.PasskeyLoginBeginResponse{
		Options:   options,
		SessionID: sessionID,
	})
}

// @Summary Finish passwordless login
// @Description Complete the passwordless login ceremony with the client's assertion response
// @Tags Passkeys
// @Accept json
// @Produce json
// @Param request body dto.PasskeyLoginFinishRequest true "Assertion response"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /passkey/login/finish [post]
func (h *Handler) FinishPasswordlessLogin(c *gin.Context) {
	var req dto.PasskeyLoginFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	userIDStr, appErr := h.Service.FinishPasswordlessLogin(appID, req.SessionID, req.Credential)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	// Generate tokens
	roles := h.getUserRoles(appID.String(), userIDStr)
	accessToken, refreshToken, tokenErr := generateTokensForUser(appID.String(), userIDStr, roles)
	if tokenErr != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	// Log passwordless login
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogPasskeyLogin(appID, userID, ipAddress, userAgent)

	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// ============================================================================
// Helper functions
// ============================================================================

// extractUserAndApp extracts userID and appID from the Gin context (set by auth middleware).
func extractUserAndApp(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("User ID not found in context")
	}

	userID, err := uuid.Parse(userIDVal.(string))
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("Invalid user ID")
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("App ID missing from context")
	}
	appID := appIDVal.(uuid.UUID)

	return userID, appID, nil
}

// generateTokensForUser generates access and refresh tokens for a user.
func generateTokensForUser(appID string, userID string, roles []string) (string, string, error) {
	accessToken, err := jwt.GenerateAccessToken(appID, userID, roles)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := jwt.GenerateRefreshToken(appID, userID, roles)
	if err != nil {
		return "", "", err
	}

	if err := redis.SetRefreshToken(appID, userID, refreshToken); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// clearTempSession clears the temporary 2FA session.
func clearTempSession(appID, tempToken string) {
	if err := redis.DeleteTempUserSession(appID, tempToken); err != nil {
		stdlog.Printf("Warning: Failed to delete temporary user session %s: %v\n", tempToken, err)
	}
}

// toPasskeyResponse converts a WebAuthnCredential model to a PasskeyResponse DTO.
func toPasskeyResponse(cred models.WebAuthnCredential) dto.PasskeyResponse {
	resp := dto.PasskeyResponse{
		ID:             cred.ID.String(),
		Name:           cred.Name,
		CreatedAt:      cred.CreatedAt.Format("2006-01-02T15:04:05Z"),
		BackupEligible: cred.BackupEligible,
		BackupState:    cred.BackupState,
	}

	if cred.LastUsedAt != nil {
		t := cred.LastUsedAt.Format("2006-01-02T15:04:05Z")
		resp.LastUsedAt = &t
	}

	if cred.Transports != "" {
		resp.Transports = strings.Split(cred.Transports, ",")
	} else {
		resp.Transports = []string{}
	}

	return resp
}
