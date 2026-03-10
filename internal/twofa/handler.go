package twofa

import (
	"fmt"
	stdlog "log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/config"
	emailpkg "github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/geoip"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/session"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/google/uuid"
)

// RoleLookupFunc is a function that returns role names for a user in an app.
type RoleLookupFunc func(appID, userID string) ([]string, error)

// AssignDefaultRoleFunc is called to assign the default role to a user.
type AssignDefaultRoleFunc func(appID, userID string) error

type Handler struct {
	Service           *Service
	SessionService    *session.Service         // Session management for creating sessions on 2FA login completion
	LookupRoles       RoleLookupFunc           // Optional: if nil, tokens are generated without roles
	AssignDefaultRole AssignDefaultRoleFunc    // Optional: if nil, no self-healing role assignment
	IPRuleEvaluator   *geoip.IPRuleEvaluator   // IP access control evaluator (nil = no IP rules)
	AnomalyDetector   *log.AnomalyDetector     // Anomaly detector for login monitoring (nil = disabled)
	TrustedDeviceRepo *TrustedDeviceRepository // nil = trusted device feature disabled
}

func NewHandler(s *Service) *Handler {
	return &Handler{Service: s}
}

// getUserRoles fetches roles for JWT embedding. Returns nil on error (non-fatal).
// Self-healing: if the user has no roles and AssignDefaultRole is available,
// assigns the "member" role automatically (covers pre-RBAC users).
func (h *Handler) getUserRoles(appID, userID string) []string {
	if h.LookupRoles == nil {
		return nil
	}
	roles, err := h.LookupRoles(appID, userID)
	if err != nil {
		stdlog.Printf("Warning: failed to lookup roles for user %s in app %s: %v", userID, appID, err)
		return nil
	}

	// Self-healing: assign default role if user has none (pre-RBAC users)
	if len(roles) == 0 && h.AssignDefaultRole != nil {
		if err := h.AssignDefaultRole(appID, userID); err != nil {
			stdlog.Printf("Warning: self-healing role assignment failed for user %s in app %s: %v", userID, appID, err)
			return nil
		}
		// Re-fetch roles after assignment
		roles, err = h.LookupRoles(appID, userID)
		if err != nil {
			stdlog.Printf("Warning: failed to re-lookup roles after self-healing for user %s in app %s: %v", userID, appID, err)
			return nil
		}
		stdlog.Printf("Info: self-healing assigned default role to user %s in app %s, roles: %v", userID, appID, roles)
	}

	return roles
}

// checkIPAccess evaluates IP rules for the given app and IP address.
// Returns true if access is allowed, false if blocked.
func (h *Handler) checkIPAccess(c *gin.Context, appID uuid.UUID, ipAddress, userAgent string) bool {
	if h.IPRuleEvaluator == nil {
		return true
	}
	result := h.IPRuleEvaluator.EvaluateAccess(appID, ipAddress)
	if !result.Allowed {
		log.LogIPBlocked(appID, ipAddress, userAgent, map[string]interface{}{
			"reason":  result.Reason,
			"country": result.Country,
		})
		c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Access denied from your location"})
		return false
	}
	return true
}

// runLoginAnomalyDetection runs anomaly detection for a successful 2FA login and logs with the result.
func (h *Handler) runLoginAnomalyDetection(appID, userID uuid.UUID, email, ipAddress, userAgent, method string) {
	if h.AnomalyDetector == nil {
		// Fall back to standard logging
		log.Log2FALogin(appID, userID, ipAddress, userAgent, method)
		return
	}

	cfg := config.GetLoggingConfig()
	ctx := log.UserContext{
		UserID:    userID,
		AppID:     appID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Timestamp: time.Now().UTC(),
	}
	anomalyCfg := log.AnomalyConfig{
		Enabled:                cfg.AnomalyDetection.Enabled,
		LogOnNewIP:             cfg.AnomalyDetection.LogOnNewIP,
		LogOnNewUserAgent:      cfg.AnomalyDetection.LogOnNewUserAgent,
		LogOnGeographicChange:  cfg.AnomalyDetection.LogOnGeographicChange,
		LogOnUnusualTimeAccess: cfg.AnomalyDetection.LogOnUnusualTimeAccess,
		SessionWindow:          cfg.AnomalyDetection.SessionWindow,
		BruteForceEnabled:      cfg.AnomalyDetection.BruteForceEnabled,
		BruteForceThreshold:    cfg.AnomalyDetection.BruteForceThreshold,
		BruteForceWindow:       cfg.AnomalyDetection.BruteForceWindow,
		NotifyOnBruteForce:     cfg.AnomalyDetection.NotifyOnBruteForce,
		NotifyOnNewDevice:      cfg.AnomalyDetection.NotifyOnNewDevice,
		NotifyOnGeoChange:      cfg.AnomalyDetection.NotifyOnGeoChange,
		NotificationCooldown:   cfg.AnomalyDetection.NotificationCooldown,
	}
	anomalyResult := h.AnomalyDetector.DetectAnomaly(ctx, anomalyCfg)
	log.GetLogService().LogActivityWithAnomalyResult(appID, userID, email, log.Event2FALogin, ipAddress, userAgent, map[string]interface{}{
		"method": method,
	}, &anomalyResult)
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

	if err := h.Service.Disable2FA(appID, userID.(string)); err != nil {
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

	// Get client info early for IP blocking check
	ipAddress, userAgent := util.GetClientInfo(c)

	// Check IP-based access rules before processing 2FA verification
	if !h.checkIPAccess(c, appID, ipAddress, userAgent) {
		return
	}

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

		if userMethod == emailpkg.TwoFAMethodPasskey {
			// Passkey 2FA uses a two-step challenge-response flow via separate endpoints:
			// POST /2fa/passkey/begin → POST /2fa/passkey/finish
			// It cannot be verified with a simple code through this endpoint.
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error: "Passkey 2FA requires the /2fa/passkey/begin and /2fa/passkey/finish endpoints",
			})
			return
		} else if userMethod == emailpkg.TwoFAMethodEmail {
			// Email 2FA code verification
			method = "email"
			verificationErr = h.Service.VerifyEmail2FACode(appID, userID, req.Code)
		} else if userMethod == emailpkg.TwoFAMethodSMS {
			// SMS 2FA code verification
			method = "sms"
			verificationErr = h.Service.VerifySMS2FACode(appID, userID, req.Code)
		} else if userMethod == emailpkg.TwoFAMethodBackupEmail {
			// Backup email 2FA code verification
			method = "backup_email"
			verificationErr = h.Service.VerifyBackupEmail2FACode(appID, userID, req.Code)
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

	// Generate final tokens (via session if available, else legacy)
	roles := h.getUserRoles(appID.String(), userID)
	accessToken, refreshToken, tokenErr := h.createSessionOrTokens(appID.String(), userID, ipAddress, userAgent, roles)
	if tokenErr != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	// Look up user email for anomaly detection notifications
	userEmail := ""
	userUUID, parseErr := uuid.Parse(userID)
	if parseErr == nil {
		if user, lookupErr := h.Service.UserRepo.GetUserByID(userID); lookupErr == nil {
			userEmail = user.Email
		}
	}

	// Log successful 2FA login with anomaly detection
	if parseErr == nil {
		h.runLoginAnomalyDetection(appID, userUUID, userEmail, ipAddress, userAgent, method)
	}

	// Clear temporary session
	clearTempSession(appID.String(), req.TempToken)

	// If the user opted in to trusting this device, create a trusted device record and set the cookie.
	if req.RememberDevice && h.TrustedDeviceRepo != nil {
		if enabled, maxDays := h.Service.IsTrustedDeviceEnabled(appID); enabled {
			userUUIDForDevice, parseErrDevice := uuid.Parse(userID)
			if parseErrDevice == nil {
				deviceName := req.DeviceName
				if deviceName == "" {
					deviceName = "Unknown Device"
				}
				if plainToken, tdErr := h.Service.CreateTrustedDevice(appID, userUUIDForDevice, deviceName, userAgent, ipAddress, maxDays); tdErr == nil {
					secureCookie := gin.Mode() == gin.ReleaseMode
					sameSite := http.SameSiteLaxMode
					if secureCookie {
						sameSite = http.SameSiteStrictMode
					}
					http.SetCookie(c.Writer, &http.Cookie{
						Name:     "trusted_device",
						Value:    plainToken,
						Path:     "/",
						MaxAge:   maxDays * 86400,
						HttpOnly: true,
						Secure:   secureCookie,
						SameSite: sameSite,
					})
				}
			}
		}
	}

	// Dispatch webhook event (non-fatal)
	if h.Service.WebhookService != nil {
		h.Service.WebhookService.Dispatch(appID, "user.login", map[string]interface{}{
			"user_id": userID,
			"email":   userEmail,
			"ip":      ipAddress,
			"method":  "2fa_" + method,
		})
	}

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
	hasPasskey := false
	for _, m := range methods {
		if m == emailpkg.TwoFAMethodTOTP {
			hasTOTP = true
		}
		if m == emailpkg.TwoFAMethodEmail {
			hasEmail = true
		}
		if m == emailpkg.TwoFAMethodPasskey {
			hasPasskey = true
		}
	}

	c.JSON(http.StatusOK, dto.TwoFAMethodsResponse{
		AvailableMethods: methods,
		TOTPEnabled:      hasTOTP,
		Email2FAEnabled:  hasEmail,
		PasskeyEnabled:   hasPasskey,
	})
}

// Helper functions

// getUserIDFromTempSession retrieves userID from temporary 2FA session
func getUserIDFromTempSession(appID, tempToken string) (string, error) {
	return redis.GetTempUserSession(appID, tempToken)
}

// generateTokensForUser generates access and refresh tokens for a user (legacy, no session)
func generateTokensForUser(appID string, userID string, roles []string) (string, string, error) {
	accessToken, err := jwt.GenerateAccessToken(appID, userID, "", roles)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := jwt.GenerateRefreshToken(appID, userID, "", roles)
	if err != nil {
		return "", "", err
	}

	// Store refresh token in Redis
	if err := redis.SetRefreshToken(appID, userID, refreshToken); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// createSessionOrTokens creates a session via the session service if available,
// otherwise falls back to legacy token generation.
func (h *Handler) createSessionOrTokens(appID, userID, ip, userAgent string, roles []string) (string, string, error) {
	if h.SessionService != nil {
		accessToken, refreshToken, _, appErr := h.SessionService.CreateSession(appID, userID, ip, userAgent, roles)
		if appErr != nil {
			return "", "", fmt.Errorf("%s", appErr.Message)
		}
		return accessToken, refreshToken, nil
	}
	return generateTokensForUser(appID, userID, roles)
}

// clearTempSession clears the temporary 2FA session
func clearTempSession(appID, tempToken string) {
	if err := redis.DeleteTempUserSession(appID, tempToken); err != nil {
		// Log the error but don't fail the operation since the user is already authenticated
		fmt.Printf("Warning: Failed to delete temporary user session %s: %v\n", tempToken, err)
	}
}

// ============================================================================
// SMS 2FA Endpoints
// ============================================================================

// @Summary Enable SMS-based 2FA
// @Description Enable SMS-based 2FA for the user (phone number must be verified first)
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.TwoFAEnableResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/sms/enable [post]
func (h *Handler) EnableSMS2FA(c *gin.Context) {
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

	recoveryCodes, err := h.Service.EnableSMS2FA(appID, userID.(string))
	if err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.Log2FAEnable(appID, userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.TwoFAEnableResponse{
		Message:       "SMS 2FA enabled successfully",
		RecoveryCodes: recoveryCodes,
	})
}

// @Summary Resend SMS 2FA code
// @Description Resend a new SMS 2FA verification code to the user's phone during login
// @Tags 2FA
// @Accept json
// @Produce json
// @Param   resend  body  object{temp_token=string}  true  "Temporary login token"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/sms/resend [post]
func (h *Handler) ResendSMS2FACode(c *gin.Context) {
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

	userID, err := getUserIDFromTempSession(appID.String(), req.TempToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid or expired temporary token"})
		return
	}

	if appErr := h.Service.GenerateSMS2FACode(appID, userID); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "SMS 2FA code resent successfully"})
}

// ============================================================================
// Backup Email Endpoints
// ============================================================================

// @Summary Enable backup email 2FA
// @Description Enable backup-email-based 2FA for the authenticated user. The backup email must already be verified.
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.TwoFAEnableResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/backup-email/enable [post]
func (h *Handler) EnableBackupEmail2FA(c *gin.Context) {
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

	recoveryCodes, err := h.Service.EnableBackupEmail2FA(appID, userID.(string))
	if err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.Log2FAEnable(appID, userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.TwoFAEnableResponse{
		Message:       "Backup email 2FA enabled successfully",
		RecoveryCodes: recoveryCodes,
	})
}

// @Summary Disable backup email 2FA
// @Description Disable backup-email-based 2FA for the authenticated user. No verification code required — authentication alone is sufficient.
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/backup-email/disable [post]
func (h *Handler) DisableBackupEmail2FA(c *gin.Context) {
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

	// Verify the user's current 2FA method is backup_email before disabling
	method, methodErr := h.Service.GetUserTwoFAMethod(userID.(string))
	if methodErr != nil {
		c.JSON(methodErr.Code, dto.ErrorResponse{Error: methodErr.Message})
		return
	}
	if method != emailpkg.TwoFAMethodBackupEmail {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Backup email is not the current 2FA method"})
		return
	}

	if err := h.Service.DisableBackupEmail2FAMethod(appID, userID.(string)); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.Log2FADisable(appID, userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Backup email 2FA disabled successfully"})
}

// @Summary Resend backup email 2FA code
// @Description Resend a new 2FA verification code to the user's backup email during login
// @Tags 2FA
// @Accept json
// @Produce json
// @Param   resend  body  object{temp_token=string}  true  "Temporary login token"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/backup-email/resend [post]
func (h *Handler) ResendBackupEmail2FACode(c *gin.Context) {
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

	userID, err := getUserIDFromTempSession(appID.String(), req.TempToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid or expired temporary token"})
		return
	}

	if appErr := h.Service.ResendBackupEmail2FACode(appID, userID); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Backup email 2FA code resent successfully"})
}

// @Summary Add backup email
// @Description Register a secondary email address for 2FA recovery (sends verification email)
// @Tags 2FA
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   request  body  dto.AddBackupEmailRequest  true  "Backup email"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/backup-email [post]
func (h *Handler) AddBackupEmail(c *gin.Context) {
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

	var req dto.AddBackupEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if appErr := h.Service.AddBackupEmail(appID, userID.(string), req.BackupEmail); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Verification email sent to backup address"})
}

// @Summary Verify backup email
// @Description Confirm a backup email using the token from the verification email
// @Tags 2FA
// @Produce json
// @Param   token  query  string  true  "Verification token"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Router /2fa/backup-email/verify [get]
func (h *Handler) VerifyBackupEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "token query parameter is required"})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	if appErr := h.Service.VerifyBackupEmail(appID, token); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Backup email verified successfully"})
}

// @Summary Remove backup email
// @Description Remove the backup email address from the user account
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.MessageResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/backup-email [delete]
func (h *Handler) RemoveBackupEmail(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	if appErr := h.Service.RemoveBackupEmail(userID.(string)); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Backup email removed successfully"})
}

// @Summary Get backup email status
// @Description Return the current backup email address and whether it is verified
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.BackupEmailStatusResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /2fa/backup-email/status [get]
func (h *Handler) BackupEmailStatus(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	usr, err := h.Service.UserRepo.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "User not found"})
		return
	}

	c.JSON(http.StatusOK, dto.BackupEmailStatusResponse{
		BackupEmail: usr.BackupEmail,
		Verified:    usr.BackupEmailVerified,
	})
}

// ============================================================================
// Phone / SMS Endpoints
// ============================================================================

// @Summary Add phone number
// @Description Register a phone number for SMS 2FA (sends verification SMS)
// @Tags 2FA
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   request  body  dto.AddPhoneRequest  true  "Phone number"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /phone [post]
func (h *Handler) AddPhone(c *gin.Context) {
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

	var req dto.AddPhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if appErr := h.Service.AddPhone(appID, userID.(string), req.PhoneNumber); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Verification SMS sent to phone number"})
}

// @Summary Verify phone number
// @Description Confirm a phone number using the code from the verification SMS
// @Tags 2FA
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   request  body  dto.VerifyPhoneRequest  true  "Verification code"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Router /phone/verify [post]
func (h *Handler) VerifyPhone(c *gin.Context) {
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

	var req dto.VerifyPhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if appErr := h.Service.VerifyPhone(appID, userID.(string), req.Code); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Phone number verified successfully"})
}

// @Summary Remove phone number
// @Description Remove the phone number from the user account
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.MessageResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /phone [delete]
func (h *Handler) RemovePhone(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	if appErr := h.Service.RemovePhone(userID.(string)); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Phone number removed successfully"})
}

// @Summary Get phone number status
// @Description Return the current phone number and whether it is verified
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.PhoneStatusResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /phone/status [get]
func (h *Handler) PhoneStatus(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	usr, err := h.Service.UserRepo.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "User not found"})
		return
	}

	c.JSON(http.StatusOK, dto.PhoneStatusResponse{
		PhoneNumber: usr.PhoneNumber,
		Verified:    usr.PhoneVerified,
	})
}

// ============================================================================
// Trusted Device Endpoints
// ============================================================================

// @Summary List trusted devices
// @Description List all trusted devices for the authenticated user in the current app
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.TrustedDevicesListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/trusted-devices [get]
func (h *Handler) ListTrustedDevices(c *gin.Context) {
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

	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	devices, appErr := h.Service.ListTrustedDevices(userUUID, appID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	resp := dto.TrustedDevicesListResponse{Devices: make([]dto.TrustedDeviceResponse, 0, len(devices))}
	for _, d := range devices {
		resp.Devices = append(resp.Devices, dto.TrustedDeviceResponse{
			ID:         d.ID.String(),
			Name:       d.Name,
			UserAgent:  d.UserAgent,
			IPAddress:  d.IPAddress,
			LastUsedAt: d.LastUsedAt.Format(time.RFC3339),
			ExpiresAt:  d.ExpiresAt.Format(time.RFC3339),
			CreatedAt:  d.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Revoke a trusted device
// @Description Revoke a specific trusted device by ID
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Param   id  path  string  true  "Trusted device UUID"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /2fa/trusted-devices/{id} [delete]
func (h *Handler) RevokeTrustedDevice(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid device ID"})
		return
	}

	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	if appErr := h.Service.RevokeTrustedDevice(deviceID, userUUID); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Trusted device revoked successfully"})
}

// @Summary Revoke all trusted devices
// @Description Revoke all trusted devices for the authenticated user in the current app
// @Tags 2FA
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.MessageResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /2fa/trusted-devices [delete]
func (h *Handler) RevokeAllTrustedDevices(c *gin.Context) {
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

	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	if appErr := h.Service.RevokeAllTrustedDevices(userUUID, appID); appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "All trusted devices revoked successfully"})
}
