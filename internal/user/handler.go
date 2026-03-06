package user

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/bruteforce"
	"github.com/gjovanovicst/auth_api/internal/config"
	"github.com/gjovanovicst/auth_api/internal/geoip"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type Handler struct {
	Service           *Service
	IPRuleEvaluator   *geoip.IPRuleEvaluator // IP access control evaluator (nil = no IP rules)
	AnomalyDetector   *log.AnomalyDetector   // Anomaly detector for login monitoring (nil = disabled)
	BruteForceService *bruteforce.Service    // Brute-force protection service (lockout, delays, CAPTCHA)
}

func NewHandler(s *Service) *Handler {
	return &Handler{Service: s}
}

// checkIPAccess evaluates IP rules for the given app and IP address.
// Returns true if access is allowed, false if blocked.
// When blocked, it sends the appropriate JSON error response and logs the event.
func (h *Handler) checkIPAccess(c *gin.Context, appID uuid.UUID, ipAddress, userAgent string) bool {
	if h.IPRuleEvaluator == nil {
		return true // No evaluator configured, allow by default
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

// runLoginAnomalyDetection runs anomaly detection for a successful login and logs with the result.
// It replaces the plain LogLogin call to enable anomaly-based notifications.
func (h *Handler) runLoginAnomalyDetection(appID, userID uuid.UUID, email, ipAddress, userAgent string, details map[string]interface{}) {
	if h.AnomalyDetector == nil {
		// Fall back to standard logging
		log.LogLogin(appID, userID, ipAddress, userAgent, details)
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
	log.GetLogService().LogActivityWithAnomalyResult(appID, userID, email, log.EventLogin, ipAddress, userAgent, details, &anomalyResult)
}

// handleFailedLogin tracks a failed login attempt for brute-force detection.
// Returns wasLocked and lockExpiresAt if the account was locked as a result.
func (h *Handler) handleFailedLogin(appID uuid.UUID, email, ipAddress, userAgent string, bfCfg bruteforce.BruteForceConfig) (bool, *time.Time) {
	// Log the failed attempt
	log.LogLoginFailed(appID, ipAddress, userAgent, map[string]interface{}{
		"email": email,
	})

	var wasLocked bool
	var lockExpiresAt *time.Time
	var failCount int64

	// Brute-force protection: account lockout + counter increment.
	// When BruteForceService is present, it owns the Redis counter increment
	// to avoid double-counting with the anomaly detector.
	if h.BruteForceService != nil {
		var lockErr error
		wasLocked, lockExpiresAt, failCount, lockErr = h.BruteForceService.HandleFailedLogin(appID, email, bfCfg)
		if lockErr == nil && wasLocked {
			log.LogAccountLocked(appID, uuid.Nil, ipAddress, userAgent, map[string]interface{}{
				"email":        email,
				"locked_until": lockExpiresAt.Format(time.RFC3339),
			})
		}

		// Increment delay tiers for both email and IP
		h.BruteForceService.IncrementDelayTier(appID, email, ipAddress, bfCfg)
	}

	// Anomaly detection for brute-force notifications
	if h.AnomalyDetector != nil {
		cfg := config.GetLoggingConfig()
		if cfg.AnomalyDetection.BruteForceEnabled {
			// If BruteForceService already incremented the counter, use its count.
			// Otherwise, increment via the anomaly detector (legacy path).
			count := failCount
			if h.BruteForceService == nil {
				var err error
				count, err = h.AnomalyDetector.IncrementAndCheckBruteForce(appID, email, cfg.AnomalyDetection.BruteForceWindow)
				if err != nil {
					count = 0
				}
			}

			if count >= int64(cfg.AnomalyDetection.BruteForceThreshold) {
				ctx := log.UserContext{
					AppID:     appID,
					IPAddress: ipAddress,
					UserAgent: userAgent,
					Timestamp: time.Now().UTC(),
				}
				bruteResult := h.AnomalyDetector.DetectBruteForce(ctx, log.AnomalyConfig{
					BruteForceEnabled:    cfg.AnomalyDetection.BruteForceEnabled,
					BruteForceThreshold:  cfg.AnomalyDetection.BruteForceThreshold,
					BruteForceWindow:     cfg.AnomalyDetection.BruteForceWindow,
					NotifyOnBruteForce:   cfg.AnomalyDetection.NotifyOnBruteForce,
					NotificationCooldown: cfg.AnomalyDetection.NotificationCooldown,
				}, count)

				log.LogBruteForceDetected(appID, uuid.Nil, ipAddress, userAgent, map[string]interface{}{
					"email":        email,
					"failed_count": count,
				})

				if bruteResult.NotifyUser {
					log.GetLogService().LogActivityWithAnomalyResult(appID, uuid.Nil, email, log.EventBruteForceDetected, ipAddress, userAgent, map[string]interface{}{
						"email":        email,
						"failed_count": count,
					}, &bruteResult)
				}
			}
		}
	}

	return wasLocked, lockExpiresAt
}

// @Summary Register a new user
// @Description Register a new user with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Param   registration  body      dto.RegisterRequest  true  "User Registration Data"
// @Success 201 {object}  dto.UserResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 409 {object}  dto.ErrorResponse
// @Failure 429 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /register [post]
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	userID, err := h.Service.RegisterUser(appID, req.Email, req.Password)
	if err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log registration activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogRegister(appID, userID, ipAddress, userAgent, req.Email)

	c.JSON(http.StatusCreated, dto.MessageResponse{Message: "User registered successfully. Please check your email for verification."})
}

// @Summary User login
// @Description Authenticate user and issue JWTs
// @Tags Auth
// @Accept json
// @Produce json
// @Param   login  body      dto.LoginRequest  true  "User Login Data"
// @Success 200 {object}  dto.LoginResponse
// @Success 202 {object}  dto.TwoFARequiredResponse "2FA verification or setup required"
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse "May include retry_after (seconds) advisory field"
// @Failure 403 {object}  dto.CaptchaRequiredResponse "CAPTCHA verification required"
// @Failure 423 {object}  dto.AccountLockedResponse "Account is locked"
// @Failure 500 {object}  dto.ErrorResponse
// @Router /login [post]
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	// Get client info for logging and session tracking
	ipAddress, userAgent := util.GetClientInfo(c)

	// Check IP-based access rules before processing login
	if !h.checkIPAccess(c, appID, ipAddress, userAgent) {
		return
	}

	// --- Resolve per-app brute-force configuration ---
	// Load the Application from DB to get per-app overrides; fallback to global defaults.
	var bfCfg bruteforce.BruteForceConfig
	if h.BruteForceService != nil {
		var app models.Application
		if dbErr := h.Service.DB.Select(
			"bf_lockout_enabled, bf_lockout_threshold, bf_lockout_durations, bf_lockout_window, bf_lockout_tier_ttl, "+
				"bf_delay_enabled, bf_delay_start_after, bf_delay_max_seconds, bf_delay_tier_ttl, "+
				"bf_captcha_enabled, bf_captcha_site_key, bf_captcha_secret_key, bf_captcha_threshold",
		).First(&app, "id = ?", appID).Error; dbErr != nil {
			// App not found or DB error — use global defaults
			bfCfg = bruteforce.ResolveConfig(nil)
		} else {
			bfCfg = bruteforce.ResolveConfig(&app)
		}
	}

	// --- Brute-force pre-auth checks ---
	if h.BruteForceService != nil {
		// Check CAPTCHA requirement (before any processing).
		// CAPTCHA acts as a gate — the request is rejected until a valid token is provided.
		captchaRequired, captchaErr := h.BruteForceService.IsCaptchaRequired(appID, req.Email, bfCfg)
		if captchaErr == nil && captchaRequired {
			// Compute advisory delay for the response
			advisoryDelay, _ := h.BruteForceService.GetDelay(appID, req.Email, ipAddress, bfCfg)

			if req.CaptchaToken == "" {
				// CAPTCHA is required but no token provided — tell client to solve CAPTCHA
				c.JSON(http.StatusForbidden, dto.CaptchaRequiredResponse{
					Error:           "CAPTCHA verification required",
					CaptchaRequired: true,
					SiteKey:         bfCfg.CaptchaSiteKey,
					RetryAfter:      advisoryDelay,
				})
				return
			}
			// Verify the provided CAPTCHA token
			if err := bruteforce.VerifyCaptcha(req.CaptchaToken, ipAddress, bfCfg); err != nil {
				c.JSON(http.StatusForbidden, dto.CaptchaRequiredResponse{
					Error:           "CAPTCHA verification failed",
					CaptchaRequired: true,
					SiteKey:         bfCfg.CaptchaSiteKey,
					RetryAfter:      advisoryDelay,
				})
				return
			}
		}
	}

	loginResult, err := h.Service.LoginUser(appID, req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		// Only track as failed login if it was an authentication failure (401),
		// not if the account is locked/deactivated (403) or other errors.
		if err.Code == http.StatusUnauthorized {
			wasLocked, lockExpiresAt := h.handleFailedLogin(appID, req.Email, ipAddress, userAgent, bfCfg)

			if wasLocked && lockExpiresAt != nil {
				retryAfter := int(time.Until(*lockExpiresAt).Seconds())
				if retryAfter < 0 {
					retryAfter = 0
				}
				c.JSON(http.StatusLocked, dto.AccountLockedResponse{
					Error:      "Account has been locked due to too many failed login attempts",
					LockedUtil: lockExpiresAt.Format(time.RFC3339),
					RetryAfter: retryAfter,
				})
				return
			}

			// Include advisory delay info in the 401 response so the client
			// knows how long to wait, without blocking the failure counter.
			if h.BruteForceService != nil {
				advisoryDelay, _ := h.BruteForceService.GetDelay(appID, req.Email, ipAddress, bfCfg)
				if advisoryDelay > 0 {
					c.JSON(err.Code, gin.H{
						"error":       err.Message,
						"retry_after": advisoryDelay,
					})
					return
				}
			}
		}

		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	// Successful credential verification — reset brute-force counters
	if h.AnomalyDetector != nil {
		_ = h.AnomalyDetector.ResetBruteForceCounter(appID, req.Email)
	}
	if h.BruteForceService != nil {
		h.BruteForceService.ResetOnSuccess(appID, req.Email, ipAddress)
	}

	// Check if 2FA is required
	if loginResult.RequiresTwoFA {
		// Log partial login (2FA required) — anomaly detection will run after 2FA completion
		details := map[string]interface{}{
			"requires_2fa": true,
		}
		log.LogLogin(appID, loginResult.UserID, ipAddress, userAgent, details)
		c.JSON(http.StatusAccepted, loginResult.TwoFAResponse)
		return
	}

	// Check if 2FA setup is mandatory for this app
	if loginResult.RequiresTwoFASetup {
		details := map[string]interface{}{
			"requires_2fa_setup": true,
		}
		log.LogLogin(appID, loginResult.UserID, ipAddress, userAgent, details)
		c.JSON(http.StatusAccepted, loginResult.TwoFASetupResponse)
		return
	}

	// Log successful login with anomaly detection
	details := map[string]interface{}{
		"requires_2fa": false,
	}
	h.runLoginAnomalyDetection(appID, loginResult.UserID, req.Email, ipAddress, userAgent, details)

	// Standard login response
	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken:  loginResult.AccessToken,
		RefreshToken: loginResult.RefreshToken,
	})
}

// @Summary Refresh access token
// @Description Get new access token using refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Param   refresh  body      dto.RefreshTokenRequest  true  "Refresh Token"
// @Success 200 {object}  dto.LoginResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 429 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /refresh-token [post]
func (h *Handler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newAccessToken, newRefreshToken, userID, err := h.Service.RefreshUserToken(req.RefreshToken)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	// Log token refresh activity
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID)
	if parseErr == nil {
		appIDVal, appIDExists := c.Get("app_id")
		if appIDExists {
			log.LogTokenRefresh(appIDVal.(uuid.UUID), userUUID, ipAddress, userAgent)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
	})
}

// @Summary Request password reset
// @Description Initiate password reset process
// @Tags Auth
// @Accept json
// @Produce json
// @Param   email  body      dto.ForgotPasswordRequest  true  "User Email"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 429 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /forgot-password [post]
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	// Note: We don't log password reset requests for security reasons
	// as it could be used to enumerate valid email addresses
	if err := h.Service.RequestPasswordReset(appID, req.Email); err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists, a password reset link has been sent."})
}

// @Summary Reset password
// @Description Complete password reset process
// @Tags Auth
// @Accept json
// @Produce json
// @Param   reset  body      dto.ResetPasswordRequest  true  "Reset Token and New Password"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 429 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /reset-password [post]
func (h *Handler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	userID, err := h.Service.ConfirmPasswordReset(appID, req.Token, req.NewPassword)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	// Log password reset completion
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogPasswordReset(appID, userID, ipAddress, userAgent)

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully."})
}

// @Summary Verify email
// @Description Verify user's email address
// @Tags Auth
// @Produce json
// @Param   token  query     string  true  "Verification Token"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /verify-email [get]
func (h *Handler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token is missing"})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	userID, err := h.Service.VerifyEmail(appID, token)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	// Log email verification
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogEmailVerify(appID, userID, ipAddress, userAgent)

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully!"})
}

// @Summary Resend email verification
// @Description Resend verification email to user. Returns a generic success message regardless of whether the email exists or is already verified (to prevent email enumeration).
// @Tags Auth
// @Accept json
// @Produce json
// @Param   request  body      dto.ResendVerificationRequest  true  "Email address"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 429 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /resend-verification [post]
func (h *Handler) ResendVerification(c *gin.Context) {
	var req dto.ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	if err := h.Service.ResendVerificationEmail(appID, req.Email); err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists and is not yet verified, a verification email has been sent."})
}

// @Summary Get user profile
// @Description Retrieve authenticated user's profile information
// @Tags User
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object}  dto.UserResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /profile [get]
func (h *Handler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	user, err := h.Service.Repo.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "User not found"})
		return
	}

	// Log profile access (now controlled by configuration and anomaly detection)
	// This will only log if enabled in config or if an anomaly is detected
	ipAddress, userAgent := util.GetClientInfo(c)
	appIDVal, appIDExists := c.Get("app_id")
	if appIDExists {
		log.LogProfileAccess(appIDVal.(uuid.UUID), user.ID, ipAddress, userAgent)
	}

	// Convert social accounts to DTO
	socialAccounts := make([]dto.SocialAccountResponse, len(user.SocialAccounts))
	for i, sa := range user.SocialAccounts {
		socialAccounts[i] = dto.SocialAccountResponse{
			ID:             sa.ID.String(),
			Provider:       sa.Provider,
			ProviderUserID: sa.ProviderUserID,
			Email:          sa.Email,
			Name:           sa.Name,
			FirstName:      sa.FirstName,
			LastName:       sa.LastName,
			ProfilePicture: sa.ProfilePicture,
			Username:       sa.Username,
			Locale:         sa.Locale,
			CreatedAt:      sa.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:      sa.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	// Get user roles from context (set by AuthMiddleware from JWT claims)
	var userRoles []string
	if rolesVal, rolesExist := c.Get("roles"); rolesExist {
		if r, ok := rolesVal.([]string); ok {
			userRoles = r
		}
	}

	// Return user profile without sensitive information
	c.JSON(http.StatusOK, dto.UserResponse{
		ID:             user.ID.String(),
		Email:          user.Email,
		EmailVerified:  user.EmailVerified,
		Name:           user.Name,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		ProfilePicture: user.ProfilePicture,
		Locale:         user.Locale,
		TwoFAEnabled:   user.TwoFAEnabled,
		Roles:          userRoles,
		CreatedAt:      user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		SocialAccounts: socialAccounts,
	})
}

// @Summary User logout
// @Description Logout user and revoke refresh token
// @Tags Auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   logout  body      dto.LogoutRequest  true  "Logout Data"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /logout [post]
func (h *Handler) Logout(c *gin.Context) {
	// Get userID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	// Get session ID from context (set by AuthMiddleware from JWT claims)
	sessionID := ""
	if sid, exists := c.Get("sessionID"); exists {
		sessionID = sid.(string)
	}

	if err := h.Service.LogoutUser(appID.String(), userID.(string), sessionID, req.RefreshToken, req.AccessToken); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log logout activity
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.LogLogout(appID, userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Successfully logged out"})
}

// ValidateToken godoc
// @Summary      Validate JWT Token
// @Description  Validates a JWT token and returns basic user info for external services
// @Tags         auth
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /auth/validate [get]
func (h *Handler) ValidateToken(c *gin.Context) {
	// Get user ID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "User ID not found in context",
		})
		return
	}

	// Get user basic info
	user, err := h.Service.Repo.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
			Error: "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":  true,
		"userID": user.ID,
		"email":  user.Email,
	})
}

// @Summary Update user profile
// @Description Update authenticated user's profile information (name, first_name, last_name, profile_picture, locale)
// @Tags User
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   profile  body      dto.UpdateProfileRequest  true  "Profile Update Data"
// @Success 200 {object}  dto.UserResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /profile [put]
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.Service.UpdateUserProfile(userID.(string), req); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Get updated user profile
	user, err := h.Service.Repo.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "User not found"})
		return
	}

	// Log profile update
	ipAddress, userAgent := util.GetClientInfo(c)
	details := map[string]interface{}{
		"updated_fields": req,
	}
	appIDVal, appIDExists := c.Get("app_id")
	if appIDExists {
		log.LogProfileUpdate(appIDVal.(uuid.UUID), user.ID, ipAddress, userAgent, details)
	}

	// Convert social accounts to DTO
	socialAccounts := make([]dto.SocialAccountResponse, len(user.SocialAccounts))
	for i, sa := range user.SocialAccounts {
		socialAccounts[i] = dto.SocialAccountResponse{
			ID:             sa.ID.String(),
			Provider:       sa.Provider,
			ProviderUserID: sa.ProviderUserID,
			Email:          sa.Email,
			Name:           sa.Name,
			FirstName:      sa.FirstName,
			LastName:       sa.LastName,
			ProfilePicture: sa.ProfilePicture,
			Username:       sa.Username,
			Locale:         sa.Locale,
			CreatedAt:      sa.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:      sa.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	// Get user roles from context (set by AuthMiddleware from JWT claims)
	var profileRoles []string
	if rolesVal, rolesExist := c.Get("roles"); rolesExist {
		if r, ok := rolesVal.([]string); ok {
			profileRoles = r
		}
	}

	c.JSON(http.StatusOK, dto.UserResponse{
		ID:             user.ID.String(),
		Email:          user.Email,
		EmailVerified:  user.EmailVerified,
		Name:           user.Name,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		ProfilePicture: user.ProfilePicture,
		Locale:         user.Locale,
		TwoFAEnabled:   user.TwoFAEnabled,
		Roles:          profileRoles,
		CreatedAt:      user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		SocialAccounts: socialAccounts,
	})
}

// @Summary Update user email
// @Description Update authenticated user's email address (requires password verification and email re-verification)
// @Tags User
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   email  body      dto.UpdateEmailRequest  true  "Email Update Data"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 409 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /profile/email [put]
func (h *Handler) UpdateEmail(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	if err := h.Service.UpdateUserEmail(appID, userID.(string), req); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log email update
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		details := map[string]interface{}{
			"new_email": req.Email,
		}
		log.LogEmailChange(appID, userUUID, ipAddress, userAgent, details)
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Email updated successfully. Please check your new email for verification."})
}

// @Summary Update user password
// @Description Update authenticated user's password (requires current password verification)
// @Tags User
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   password  body      dto.UpdatePasswordRequest  true  "Password Update Data"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /profile/password [put]
func (h *Handler) UpdatePassword(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	if err := h.Service.UpdateUserPassword(appID, userID.(string), req); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log password change
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.LogPasswordChange(appID, userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Password updated successfully. All sessions have been logged out for security."})
}

// @Summary Delete user account
// @Description Delete authenticated user's account permanently (requires password verification and confirmation)
// @Tags User
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   delete  body      dto.DeleteAccountRequest  true  "Account Deletion Data"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /profile [delete]
func (h *Handler) DeleteAccount(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	// Log account deletion before actually deleting
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	if parseErr == nil {
		log.LogAccountDeletion(appID, userUUID, ipAddress, userAgent)
	}

	if err := h.Service.DeleteUserAccount(appID, userID.(string), req); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Account deleted successfully. We're sorry to see you go."})
}

// @Summary Request a magic link login email
// @Description Send a magic link to the user's email for passwordless authentication. Always returns 200 regardless of whether the email exists (to prevent enumeration).
// @Tags Auth
// @Accept json
// @Produce json
// @Param   request  body      dto.MagicLinkRequest  true  "Magic Link Request Data"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 429 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /magic-link/request [post]
func (h *Handler) RequestMagicLink(c *gin.Context) {
	var req dto.MagicLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	if err := h.Service.RequestMagicLink(appID, req.Email); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogMagicLinkRequested(appID, ipAddress, userAgent)

	// Always return success to prevent email enumeration
	c.JSON(http.StatusOK, dto.MessageResponse{Message: "If an account exists with that email, a magic link has been sent."})
}

// @Summary Verify a magic link token and log in
// @Description Verify the magic link token from the email and return access/refresh tokens. The token is single-use and expires after 10 minutes.
// @Tags Auth
// @Accept json
// @Produce json
// @Param   request  body      dto.MagicLinkVerifyRequest  true  "Magic Link Verification Data"
// @Success 200 {object}  dto.LoginResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 403 {object}  dto.ErrorResponse
// @Failure 429 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /magic-link/verify [post]
func (h *Handler) VerifyMagicLink(c *gin.Context) {
	var req dto.MagicLinkVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	ipAddress, userAgent := util.GetClientInfo(c)

	// Check IP-based access rules before processing magic link
	if !h.checkIPAccess(c, appID, ipAddress, userAgent) {
		return
	}

	result, err := h.Service.VerifyMagicLink(appID, req.Token, ipAddress, userAgent)
	if err != nil {
		// Log failed attempt
		log.LogMagicLinkFailed(appID, ipAddress, userAgent, err.Message)
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Look up user email for anomaly detection notifications
	userEmail := ""
	if user, lookupErr := h.Service.Repo.GetUserByID(result.UserID.String()); lookupErr == nil {
		userEmail = user.Email
	}

	// Log successful magic link login with anomaly detection
	h.runLoginAnomalyDetection(appID, result.UserID, userEmail, ipAddress, userAgent, map[string]interface{}{
		"login_method": "magic_link",
	})

	// Dispatch webhook event (non-fatal)
	if h.Service.WebhookService != nil {
		h.Service.WebhookService.Dispatch(appID, "user.login", map[string]interface{}{
			"user_id": result.UserID.String(),
			"email":   userEmail,
			"ip":      ipAddress,
			"method":  "magic_link",
		})
	}

	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}
