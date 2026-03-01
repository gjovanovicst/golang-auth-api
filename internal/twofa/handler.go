package twofa

import (
	"fmt"
	stdlog "log"
	"net/http"

	"github.com/gin-gonic/gin"
	emailpkg "github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/google/uuid"
)

// RoleLookupFunc is a function that returns role names for a user in an app.
type RoleLookupFunc func(appID, userID string) ([]string, error)

type Handler struct {
	Service     *Service
	LookupRoles RoleLookupFunc // Optional: if nil, tokens are generated without roles
}

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

// @Summary Generate 2FA setup
// @Description Generate a 2FA secret and QR code for user setup
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} twofa.TwoFASetupResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/generate [post]
func (h *Handler) Generate2FA(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	setup, err := h.Service.Generate2FASecret(appID, userID.(string))
	if err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	c.JSON(http.StatusOK, setup)
}

// @Summary Verify 2FA setup
// @Description Verify the initial 2FA setup with a TOTP code
// @Tags 2FA
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   setup  body      dto.TwoFAVerifyRequest  true  "TOTP Code"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/verify-setup [post]
func (h *Handler) VerifySetup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.TwoFAVerifyRequest
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

	if err := h.Service.VerifySetup(appID, userID.(string), req.Code); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "2FA setup verification successful"})
}

// @Summary Enable 2FA
// @Description Enable 2FA for the user after successful verification
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.TwoFAEnableResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/enable [post]
func (h *Handler) Enable2FA(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	recoveryCodes, err := h.Service.Enable2FA(appID, userID.(string))
	if err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log 2FA enable activity
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.Log2FAEnable(appID, userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.TwoFAEnableResponse{
		Message:       "2FA enabled successfully",
		RecoveryCodes: recoveryCodes,
	})
}

// @Summary Disable 2FA
// @Description Disable 2FA for the user (requires TOTP code or email 2FA code)
// @Tags 2FA
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   disable  body      dto.TwoFADisableRequest  true  "Disable 2FA Data"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/disable [post]
func (h *Handler) Disable2FA(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.TwoFADisableRequest
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

	// Determine the user's current 2FA method and verify accordingly
	method, methodErr := h.Service.GetUserTwoFAMethod(userID.(string))
	if methodErr != nil {
		c.JSON(methodErr.Code, dto.ErrorResponse{Error: methodErr.Message})
		return
	}

	if method == emailpkg.TwoFAMethodEmail {
		// For email 2FA, verify the email code
		if verifyErr := h.Service.VerifyEmail2FACode(appID, userID.(string), req.Code); verifyErr != nil {
			c.JSON(verifyErr.Code, dto.ErrorResponse{Error: verifyErr.Message})
			return
		}
	} else {
		// For TOTP, verify the TOTP code
		if verifyErr := h.Service.VerifyTOTP(userID.(string), req.Code); verifyErr != nil {
			c.JSON(verifyErr.Code, dto.ErrorResponse{Error: verifyErr.Message})
			return
		}
	}

	if err := h.Service.Disable2FA(userID.(string)); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log 2FA disable activity
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.Log2FADisable(appID, userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "2FA disabled successfully"})
}

// @Summary Verify 2FA during login
// @Description Verify the 2FA code during the login process (supports TOTP, email code, or recovery code)
// @Tags 2FA
// @Accept json
// @Produce json
// @Param   verify  body      dto.TwoFALoginRequest  true  "2FA Login Data"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/login-verify [post]
func (h *Handler) VerifyLogin(c *gin.Context) {
	var req dto.TwoFALoginRequest
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
	userID, err := getUserIDFromTempSession(appID.String(), req.TempToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid or expired temporary token"})
		return
	}

	// Determine verification method for logging
	var method string
	var verificationErr *errors.AppError

	if req.RecoveryCode != "" {
		// Recovery code verification — works for both TOTP and email 2FA
		method = "recovery_code"
		verificationErr = h.Service.VerifyRecoveryCode(userID, req.RecoveryCode)

		// Log recovery code usage
		ipAddress, userAgent := util.GetClientInfo(c)
		userUUID, parseErr := uuid.Parse(userID)
		if parseErr == nil {
			log.LogRecoveryCodeUsed(appID, userUUID, ipAddress, userAgent)
		}
	} else if req.Code != "" {
		// Determine the user's 2FA method to decide how to verify the code
		userMethod, methodErr := h.Service.GetUserTwoFAMethod(userID)
		if methodErr != nil {
			c.JSON(methodErr.Code, dto.ErrorResponse{Error: methodErr.Message})
			return
		}

		if userMethod == emailpkg.TwoFAMethodEmail {
			// Email 2FA code verification
			method = "email"
			verificationErr = h.Service.VerifyEmail2FACode(appID, userID, req.Code)
		} else {
			// TOTP code verification (default)
			method = "totp"
			verificationErr = h.Service.VerifyTOTP(userID, req.Code)
		}
	} else {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Either code or recovery code is required"})
		return
	}

	if verificationErr != nil {
		c.JSON(verificationErr.Code, dto.ErrorResponse{Error: verificationErr.Message})
		return
	}

	// Generate final tokens
	roles := h.getUserRoles(appID.String(), userID)
	accessToken, refreshToken, err := generateTokensForUser(appID.String(), userID, roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	// Log successful 2FA login
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID)
	if parseErr == nil {
		log.Log2FALogin(appID, userUUID, ipAddress, userAgent, method)
	}

	// Clear temporary session
	clearTempSession(appID.String(), req.TempToken)

	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// @Summary Generate new recovery codes
// @Description Generate and display new recovery codes (requires password and/or TOTP code)
// @Tags 2FA
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   verify  body      dto.TwoFAVerifyRequest  true  "TOTP Code"
// @Success 200 {object} dto.TwoFARecoveryCodesResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/recovery-codes [post]
func (h *Handler) GenerateRecoveryCodes(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.TwoFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	// Verify TOTP code before generating new recovery codes
	if err := h.Service.VerifyTOTP(userID.(string), req.Code); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	recoveryCodes, err := h.Service.GenerateNewRecoveryCodes(userID.(string))
	if err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log recovery code generation
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		appIDVal, appIDExists := c.Get("app_id")
		if appIDExists {
			log.LogRecoveryCodeGenerate(appIDVal.(uuid.UUID), userUUID, ipAddress, userAgent)
		}
	}

	c.JSON(http.StatusOK, dto.TwoFARecoveryCodesResponse{
		Message:       "New recovery codes generated successfully",
		RecoveryCodes: recoveryCodes,
	})
}

// ============================================================================
// Email 2FA Endpoints
// ============================================================================

// @Summary Enable email-based 2FA
// @Description Enable email-based 2FA for the user (sends codes to registered email)
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.TwoFAEnableResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/email/enable [post]
func (h *Handler) EnableEmail2FA(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	recoveryCodes, err := h.Service.EnableEmail2FA(appID, userID.(string))
	if err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log 2FA enable activity
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.Log2FAEnable(appID, userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.TwoFAEnableResponse{
		Message:       "Email 2FA enabled successfully",
		RecoveryCodes: recoveryCodes,
	})
}

// @Summary Resend email 2FA code
// @Description Resend a new 2FA verification code to the user's email during login
// @Tags 2FA
// @Accept json
// @Produce json
// @Param   resend  body  object{temp_token=string}  true  "Temporary login token"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/email/resend [post]
func (h *Handler) ResendEmail2FACode(c *gin.Context) {
	var req struct {
		TempToken string `json:"temp_token" validate:"required"`
	}
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
	userID, err := getUserIDFromTempSession(appID.String(), req.TempToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid or expired temporary token"})
		return
	}

	if appErr := h.Service.ResendEmail2FACode(appID, uuid.MustParse(userID).String()); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "2FA code resent successfully"})
}

// @Summary Get available 2FA methods
// @Description Get the 2FA methods available for the current application
// @Tags 2FA
// @Produce json
// @Success 200 {object} dto.TwoFAMethodsResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/methods [get]
func (h *Handler) GetAvailableMethods(c *gin.Context) {
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	methods := h.Service.GetAvailableMethods(appID)

	hasTOTP := false
	hasEmail := false
	for _, m := range methods {
		if m == emailpkg.TwoFAMethodTOTP {
			hasTOTP = true
		}
		if m == emailpkg.TwoFAMethodEmail {
			hasEmail = true
		}
	}

	c.JSON(http.StatusOK, dto.TwoFAMethodsResponse{
		AvailableMethods: methods,
		TOTPEnabled:      hasTOTP,
		Email2FAEnabled:  hasEmail,
	})
}

// Helper functions

// getUserIDFromTempSession retrieves userID from temporary 2FA session
func getUserIDFromTempSession(appID, tempToken string) (string, error) {
	return redis.GetTempUserSession(appID, tempToken)
}

// generateTokensForUser generates access and refresh tokens for a user
func generateTokensForUser(appID string, userID string, roles []string) (string, string, error) {
	accessToken, err := jwt.GenerateAccessToken(appID, userID, roles)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := jwt.GenerateRefreshToken(appID, userID, roles)
	if err != nil {
		return "", "", err
	}

	// Store refresh token in Redis
	if err := redis.SetRefreshToken(appID, userID, refreshToken); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// clearTempSession clears the temporary 2FA session
func clearTempSession(appID, tempToken string) {
	if err := redis.DeleteTempUserSession(appID, tempToken); err != nil {
		// Log the error but don't fail the operation since the user is already authenticated
		fmt.Printf("Warning: Failed to delete temporary user session %s: %v\n", tempToken, err)
	}
}
