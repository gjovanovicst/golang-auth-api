package social

import (
	"context"
	"fmt"
	stdlog "log"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/config"
	"github.com/gjovanovicst/auth_api/internal/geoip"
	"github.com/gjovanovicst/auth_api/internal/health"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/redis"
	twofa "github.com/gjovanovicst/auth_api/internal/twofa"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Handler struct {
	Service               *Service
	IPRuleEvaluator       *geoip.IPRuleEvaluator                               // IP access control evaluator (nil = no IP rules)
	AnomalyDetector       *log.AnomalyDetector                                 // Anomaly detector for login monitoring (nil = disabled)
	TwoFAService          *twofa.Service                                       // Optional: if set, auto-sends SMS 2FA code on social login with SMS 2FA
	ValidateTrustedDevice func(plainToken string) (uuid.UUID, uuid.UUID, bool) // Optional: if set, trusted device bypass is checked before requiring 2FA
}

func NewHandler(s *Service) *Handler {
	return &Handler{
		Service: s,
	}
}

// trySendSMSCode auto-sends an SMS 2FA code when a social-login user has SMS 2FA enabled.
// Failures are non-fatal and logged as warnings — the user can still request a resend via /2fa/sms/resend.
func (h *Handler) trySendSMSCode(appID uuid.UUID, userID string) {
	if h.TwoFAService == nil {
		return
	}
	if err := h.TwoFAService.GenerateSMS2FACode(appID, userID); err != nil {
		stdlog.Printf("Warning: failed to auto-send SMS 2FA code for user %s in app %s: %v", userID, appID, err.Message)
	}
}

// trySendBackupEmailCode auto-sends a backup-email 2FA code when a social-login user has
// backup_email 2FA enabled. Failures are non-fatal — the user can still request a resend.
func (h *Handler) trySendBackupEmailCode(appID uuid.UUID, userID string) {
	if h.TwoFAService == nil {
		return
	}
	if err := h.TwoFAService.GenerateBackupEmail2FACode(appID, userID); err != nil {
		stdlog.Printf("Warning: failed to auto-send backup email 2FA code for user %s in app %s: %v", userID, appID, err.Message)
	}
}

// checkIPAccessRedirect evaluates IP rules for the given app and IP address.
// Returns true if access is allowed, false if blocked.
// When blocked, it redirects with an error parameter and logs the event.
func (h *Handler) checkIPAccessRedirect(c *gin.Context, appID uuid.UUID, ipAddress, userAgent, redirectURI string) bool {
	if h.IPRuleEvaluator == nil {
		return true // No evaluator configured, allow by default
	}
	result := h.IPRuleEvaluator.EvaluateAccess(appID, ipAddress)
	if !result.Allowed {
		log.LogIPBlocked(appID, ipAddress, userAgent, map[string]interface{}{
			"reason":  result.Reason,
			"country": result.Country,
		})
		frontendURL := fmt.Sprintf("%s?error=ip_blocked", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return false
	}
	return true
}

// runSocialLoginAnomalyDetection runs anomaly detection for a successful social login.
func (h *Handler) runSocialLoginAnomalyDetection(appID, userID uuid.UUID, email, ipAddress, userAgent, provider string) {
	if h.AnomalyDetector == nil {
		// Fall back to standard logging
		log.LogSocialLogin(appID, userID, ipAddress, userAgent, provider)
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
	log.GetLogService().LogActivityWithAnomalyResult(appID, userID, email, log.EventSocialLogin, ipAddress, userAgent, map[string]interface{}{
		"provider": provider,
	}, &anomalyResult)
}

func (h *Handler) getGoogleConfig(appID string) (*oauth2.Config, error) {
	config, err := h.Service.SocialRepo.GetOAuthProviderConfig(appID, "google")
	if err != nil {
		return nil, err
	}
	if !config.IsEnabled {
		return nil, fmt.Errorf("google login is disabled for this app")
	}
	return &oauth2.Config{
		RedirectURL:  config.RedirectURL,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}, nil
}

func (h *Handler) getFacebookConfig(appID string) (*oauth2.Config, error) {
	config, err := h.Service.SocialRepo.GetOAuthProviderConfig(appID, "facebook")
	if err != nil {
		return nil, err
	}
	if !config.IsEnabled {
		return nil, fmt.Errorf("facebook login is disabled for this app")
	}
	return &oauth2.Config{
		RedirectURL:  config.RedirectURL,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Scopes:       []string{"email", "public_profile"},
		// #nosec G101 -- These are public OAuth endpoint URLs, not credentials
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.facebook.com/v18.0/dialog/oauth",
			TokenURL: "https://graph.facebook.com/v18.0/oauth/access_token",
		},
	}, nil
}

func (h *Handler) getGithubConfig(appID string) (*oauth2.Config, error) {
	config, err := h.Service.SocialRepo.GetOAuthProviderConfig(appID, "github")
	if err != nil {
		return nil, err
	}
	if !config.IsEnabled {
		return nil, fmt.Errorf("github login is disabled for this app")
	}
	return &oauth2.Config{
		RedirectURL:  config.RedirectURL,
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Scopes:       []string{"user:email"},
		// #nosec G101 -- These are public OAuth endpoint URLs, not credentials
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}, nil
}

// GoogleLogin godoc
// @Summary      Google OAuth2 Login
// @Description  Redirects user to Google OAuth2 login page
// @Tags         social
// @Produce      json
// @Param        redirect_uri query string false "Frontend callback URL"
// @Success      307 {string} string "Redirect"
// @Router       /auth/google/login [get]
func (h *Handler) GoogleLogin(c *gin.Context) {
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	googleConfig, err := h.getGoogleConfig(appID.String())
	if err != nil {
		stdlog.Printf("Failed to get Google OAuth config for app %s: %v", appID.String(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth configuration error"})
		return
	}

	// Get redirect URI from query parameter or use default
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	// Create secure state with redirect URI
	state, err := CreateOAuthState(redirectURI, appID.String())
	if err != nil {
		stdlog.Printf("Invalid OAuth redirect URI for Google login: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid redirect URI",
		})
		return
	}

	// Generate OAuth URL with secure state
	url := googleConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback godoc
// @Summary      Google OAuth2 Callback
// @Description  Handles Google OAuth2 callback and returns JWT tokens
// @Tags         social
// @Produce      json
// @Param        state query string true "State token"
// @Param        code  query string true "Authorization code"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /auth/google/callback [get]
func (h *Handler) GoogleCallback(c *gin.Context) {
	encodedState := c.Query("state")
	if encodedState == "" {
		// Redirect to default with error
		frontendURL := fmt.Sprintf("%s?error=missing_state", GetDefaultRedirectURI())
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Parse and validate state
	state, err := ParseOAuthState(encodedState)
	if err != nil {
		// Redirect to default with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Invalid state: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", GetDefaultRedirectURI(), errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	code := c.Query("code")
	if code == "" {
		// Redirect to frontend with error
		frontendURL := fmt.Sprintf("%s?error=authorization_code_missing", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Use the validated redirect URI from state
	redirectURI := state.RedirectURI

	var appID uuid.UUID
	appIDVal, exists := c.Get("app_id")
	if exists {
		appID = appIDVal.(uuid.UUID)
	} else if state.AppID != "" {
		parsedAppID, err := uuid.Parse(state.AppID)
		if err != nil {
			frontendURL := fmt.Sprintf("%s?error=invalid_app_id_state", redirectURI)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		appID = parsedAppID
	} else {
		frontendURL := fmt.Sprintf("%s?error=app_id_missing", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	googleConfig, err := h.getGoogleConfig(appID.String())
	if err != nil {
		frontendURL := fmt.Sprintf("%s?error=config_error", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	token, err := googleConfig.Exchange(context.Background(), code)
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	userID, appErr := h.Service.HandleGoogleCallback(appID, token.AccessToken)
	if appErr != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(appErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Fetch user to check 2FA status
	user, err := h.Service.UserRepo.GetUserByID(userID.String())
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape("Failed to fetch user for 2FA check")
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Only create session when 2FA is NOT required
	ipAddress, userAgent := util.GetClientInfo(c)

	if user.TwoFAEnabled && h.Service.IsAppTwoFAEnabled(appID) {
		// Trusted device check: if the client presents a valid trusted-device cookie
		// matching this user + app, skip 2FA entirely and issue tokens immediately.
		if h.ValidateTrustedDevice != nil {
			if cookieToken, cookieErr := c.Cookie("trusted_device"); cookieErr == nil && cookieToken != "" {
				if tdUserID, tdAppID, ok := h.ValidateTrustedDevice(cookieToken); ok &&
					tdUserID == user.ID && tdAppID == appID {
					// Check IP-based access rules before completing login
					if !h.checkIPAccessRedirect(c, appID, ipAddress, userAgent, redirectURI) {
						return
					}
					accessToken, refreshToken, sessionErr := h.Service.CreateSessionOrTokens(appID.String(), userID.String(), ipAddress, userAgent)
					if sessionErr != nil {
						errorMsg := url.QueryEscape(sessionErr.Message)
						frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
						c.Redirect(http.StatusFound, frontendURL)
						return
					}
					h.runSocialLoginAnomalyDetection(appID, userID, user.Email, ipAddress, userAgent, "google")
					frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=google",
						redirectURI,
						url.QueryEscape(accessToken),
						url.QueryEscape(refreshToken))
					health.IncLoginSuccess(appID.String())
					c.Redirect(http.StatusFound, frontendURL)
					return
				}
			}
		}

		tempToken := uuid.New().String()
		err := redis.SetTempUserSession(appID.String(), tempToken, user.ID.String(), 10*time.Minute)
		if err != nil {
			// Redirect to frontend with error
			errorMsg := url.QueryEscape("Failed to create temporary session for 2FA")
			frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		// Redirect with 2FA requirement — NO session created yet
		// Include the user's configured 2FA method so the frontend can show the correct input
		twoFAMethod := user.TwoFAMethod
		if twoFAMethod == "" {
			twoFAMethod = "totp"
		}
		// Auto-send SMS code if the user's 2FA method is SMS
		if twoFAMethod == "sms" {
			h.trySendSMSCode(appID, user.ID.String())
		}
		// Auto-send backup email code if the user's 2FA method is backup_email
		if twoFAMethod == "backup_email" {
			h.trySendBackupEmailCode(appID, user.ID.String())
		}
		redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true&provider=google&method=%s", redirectURI, tempToken, twoFAMethod)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Check IP-based access rules before completing login
	if !h.checkIPAccessRedirect(c, appID, ipAddress, userAgent, redirectURI) {
		return
	}

	accessToken, refreshToken, sessionErr := h.Service.CreateSessionOrTokens(appID.String(), userID.String(), ipAddress, userAgent)
	if sessionErr != nil {
		errorMsg := url.QueryEscape(sessionErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Log social login activity with anomaly detection
	h.runSocialLoginAnomalyDetection(appID, userID, user.Email, ipAddress, userAgent, "google")

	// Redirect to frontend with tokens in URL parameters
	frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=google",
		redirectURI,
		url.QueryEscape(accessToken),
		url.QueryEscape(refreshToken))

	health.IncLoginSuccess(appID.String())
	c.Redirect(http.StatusFound, frontendURL)
}

// FacebookLogin godoc
// @Summary      Facebook OAuth2 Login
// @Description  Redirects user to Facebook OAuth2 login page
// @Tags         social
// @Produce      json
// @Param        redirect_uri query string false "Frontend callback URL"
// @Success      307 {string} string "Redirect"
// @Router       /auth/facebook/login [get]
func (h *Handler) FacebookLogin(c *gin.Context) {
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	facebookConfig, err := h.getFacebookConfig(appID.String())
	if err != nil {
		stdlog.Printf("Failed to get Facebook OAuth config for app %s: %v", appID.String(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth configuration error"})
		return
	}

	// Get redirect URI from query parameter or use default
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	// Create secure state with redirect URI
	state, err := CreateOAuthState(redirectURI, appID.String())
	if err != nil {
		stdlog.Printf("Invalid OAuth redirect URI for Facebook login: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid redirect URI",
		})
		return
	}

	// Generate OAuth URL with secure state
	url := facebookConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// FacebookCallback godoc
// @Summary      Facebook OAuth2 Callback
// @Description  Handles Facebook OAuth2 callback and returns JWT tokens
// @Tags         social
// @Produce      json
// @Param        state query string true "State token"
// @Param        code  query string true "Authorization code"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /auth/facebook/callback [get]
func (h *Handler) FacebookCallback(c *gin.Context) {
	encodedState := c.Query("state")
	if encodedState == "" {
		// Redirect to default with error
		frontendURL := fmt.Sprintf("%s?error=missing_state", GetDefaultRedirectURI())
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Parse and validate state
	state, err := ParseOAuthState(encodedState)
	if err != nil {
		// Redirect to default with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Invalid state: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", GetDefaultRedirectURI(), errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	code := c.Query("code")
	if code == "" {
		// Redirect to frontend with error
		frontendURL := fmt.Sprintf("%s?error=authorization_code_missing", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Use the validated redirect URI from state
	redirectURI := state.RedirectURI

	var appID uuid.UUID
	appIDVal, exists := c.Get("app_id")
	if exists {
		appID = appIDVal.(uuid.UUID)
	} else if state.AppID != "" {
		parsedAppID, err := uuid.Parse(state.AppID)
		if err != nil {
			frontendURL := fmt.Sprintf("%s?error=invalid_app_id_state", redirectURI)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		appID = parsedAppID
	} else {
		frontendURL := fmt.Sprintf("%s?error=app_id_missing", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	facebookConfig, err := h.getFacebookConfig(appID.String())
	if err != nil {
		frontendURL := fmt.Sprintf("%s?error=config_error", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	token, err := facebookConfig.Exchange(context.Background(), code)
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	userID, appErr := h.Service.HandleFacebookCallback(appID, token.AccessToken)
	if appErr != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(appErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Fetch user to check 2FA status
	user, err := h.Service.UserRepo.GetUserByID(userID.String())
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape("Failed to fetch user for 2FA check")
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	ipAddress, userAgent := util.GetClientInfo(c)

	if user.TwoFAEnabled && h.Service.IsAppTwoFAEnabled(appID) {
		// Trusted device check: if the client presents a valid trusted-device cookie
		// matching this user + app, skip 2FA entirely and issue tokens immediately.
		if h.ValidateTrustedDevice != nil {
			if cookieToken, cookieErr := c.Cookie("trusted_device"); cookieErr == nil && cookieToken != "" {
				if tdUserID, tdAppID, ok := h.ValidateTrustedDevice(cookieToken); ok &&
					tdUserID == user.ID && tdAppID == appID {
					// Check IP-based access rules before completing login
					if !h.checkIPAccessRedirect(c, appID, ipAddress, userAgent, redirectURI) {
						return
					}
					accessToken, refreshToken, sessionErr := h.Service.CreateSessionOrTokens(appID.String(), userID.String(), ipAddress, userAgent)
					if sessionErr != nil {
						errorMsg := url.QueryEscape(sessionErr.Message)
						frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
						c.Redirect(http.StatusFound, frontendURL)
						return
					}
					h.runSocialLoginAnomalyDetection(appID, userID, user.Email, ipAddress, userAgent, "facebook")
					frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=facebook",
						redirectURI,
						url.QueryEscape(accessToken),
						url.QueryEscape(refreshToken))
					health.IncLoginSuccess(appID.String())
					c.Redirect(http.StatusFound, frontendURL)
					return
				}
			}
		}

		tempToken := uuid.New().String()
		err := redis.SetTempUserSession(appID.String(), tempToken, user.ID.String(), 10*time.Minute)
		if err != nil {
			// Redirect to frontend with error
			errorMsg := url.QueryEscape("Failed to create temporary session for 2FA")
			frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		// Redirect with 2FA requirement — NO session created yet
		// Include the user's configured 2FA method so the frontend can show the correct input
		twoFAMethod := user.TwoFAMethod
		if twoFAMethod == "" {
			twoFAMethod = "totp"
		}
		// Auto-send SMS code if the user's 2FA method is SMS
		if twoFAMethod == "sms" {
			h.trySendSMSCode(appID, user.ID.String())
		}
		// Auto-send backup email code if the user's 2FA method is backup_email
		if twoFAMethod == "backup_email" {
			h.trySendBackupEmailCode(appID, user.ID.String())
		}
		redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true&provider=facebook&method=%s", redirectURI, tempToken, twoFAMethod)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Check IP-based access rules before completing login
	if !h.checkIPAccessRedirect(c, appID, ipAddress, userAgent, redirectURI) {
		return
	}

	accessToken, refreshToken, sessionErr := h.Service.CreateSessionOrTokens(appID.String(), userID.String(), ipAddress, userAgent)
	if sessionErr != nil {
		errorMsg := url.QueryEscape(sessionErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Log social login activity with anomaly detection
	h.runSocialLoginAnomalyDetection(appID, userID, user.Email, ipAddress, userAgent, "facebook")

	// Redirect to frontend with tokens in URL parameters
	frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=facebook",
		redirectURI,
		url.QueryEscape(accessToken),
		url.QueryEscape(refreshToken))

	health.IncLoginSuccess(appID.String())
	c.Redirect(http.StatusFound, frontendURL)
}

// GithubLogin godoc
// @Summary      GitHub OAuth2 Login
// @Description  Redirects user to GitHub OAuth2 login page
// @Tags         social
// @Produce      json
// @Param        redirect_uri query string false "Frontend callback URL"
// @Success      307 {string} string "Redirect"
// @Router       /auth/github/login [get]
func (h *Handler) GithubLogin(c *gin.Context) {
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	githubConfig, err := h.getGithubConfig(appID.String())
	if err != nil {
		stdlog.Printf("Failed to get GitHub OAuth config for app %s: %v", appID.String(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth configuration error"})
		return
	}

	// Get redirect URI from query parameter or use default
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	// Create secure state with redirect URI
	state, err := CreateOAuthState(redirectURI, appID.String())
	if err != nil {
		stdlog.Printf("Invalid OAuth redirect URI for GitHub login: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid redirect URI",
		})
		return
	}

	// Generate OAuth URL with secure state
	url := githubConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GithubCallback godoc
// @Summary      GitHub OAuth2 Callback
// @Description  Handles GitHub OAuth2 callback and returns JWT tokens
// @Tags         social
// @Produce      json
// @Param        state query string true "State token"
// @Param        code  query string true "Authorization code"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /auth/github/callback [get]
func (h *Handler) GithubCallback(c *gin.Context) {
	encodedState := c.Query("state")
	if encodedState == "" {
		// Redirect to default with error
		frontendURL := fmt.Sprintf("%s?error=missing_state", GetDefaultRedirectURI())
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Parse and validate state
	state, err := ParseOAuthState(encodedState)
	if err != nil {
		// Redirect to default with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Invalid state: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", GetDefaultRedirectURI(), errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	code := c.Query("code")
	if code == "" {
		// Redirect to frontend with error
		frontendURL := fmt.Sprintf("%s?error=authorization_code_missing", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Use the validated redirect URI from state
	redirectURI := state.RedirectURI

	var appID uuid.UUID
	appIDVal, exists := c.Get("app_id")
	if exists {
		appID = appIDVal.(uuid.UUID)
	} else if state.AppID != "" {
		parsedAppID, err := uuid.Parse(state.AppID)
		if err != nil {
			frontendURL := fmt.Sprintf("%s?error=invalid_app_id_state", redirectURI)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		appID = parsedAppID
	} else {
		frontendURL := fmt.Sprintf("%s?error=app_id_missing", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	githubConfig, err := h.getGithubConfig(appID.String())
	if err != nil {
		frontendURL := fmt.Sprintf("%s?error=config_error", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	token, err := githubConfig.Exchange(context.Background(), code)
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	userID, appErr := h.Service.HandleGithubCallback(appID, token.AccessToken)
	if appErr != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(appErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Fetch user to check 2FA status
	user, err := h.Service.UserRepo.GetUserByID(userID.String())
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape("Failed to fetch user for 2FA check")
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	if user.TwoFAEnabled && h.Service.IsAppTwoFAEnabled(appID) {
		// Trusted device check: if the client presents a valid trusted-device cookie
		// matching this user + app, skip 2FA entirely and issue tokens immediately.
		ipAddress, userAgent := util.GetClientInfo(c)
		if h.ValidateTrustedDevice != nil {
			if cookieToken, cookieErr := c.Cookie("trusted_device"); cookieErr == nil && cookieToken != "" {
				if tdUserID, tdAppID, ok := h.ValidateTrustedDevice(cookieToken); ok &&
					tdUserID == user.ID && tdAppID == appID {
					// Check IP-based access rules before completing login
					if !h.checkIPAccessRedirect(c, appID, ipAddress, userAgent, redirectURI) {
						return
					}
					accessToken, refreshToken, sessionErr := h.Service.CreateSessionOrTokens(appID.String(), userID.String(), ipAddress, userAgent)
					if sessionErr != nil {
						errorMsg := url.QueryEscape(sessionErr.Message)
						frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
						c.Redirect(http.StatusFound, frontendURL)
						return
					}
					h.runSocialLoginAnomalyDetection(appID, userID, user.Email, ipAddress, userAgent, "github")
					frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=github",
						redirectURI,
						url.QueryEscape(accessToken),
						url.QueryEscape(refreshToken))
					health.IncLoginSuccess(appID.String())
					c.Redirect(http.StatusFound, frontendURL)
					return
				}
			}
		}

		tempToken := uuid.New().String()
		err := redis.SetTempUserSession(appID.String(), tempToken, user.ID.String(), 10*time.Minute)
		if err != nil {
			// Redirect to frontend with error
			errorMsg := url.QueryEscape("Failed to create temporary session for 2FA")
			frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		// Redirect with 2FA requirement — NO session created yet
		// Include the user's configured 2FA method so the frontend can show the correct input
		twoFAMethod := user.TwoFAMethod
		if twoFAMethod == "" {
			twoFAMethod = "totp"
		}
		// Auto-send SMS code if the user's 2FA method is SMS
		if twoFAMethod == "sms" {
			h.trySendSMSCode(appID, user.ID.String())
		}
		// Auto-send backup email code if the user's 2FA method is backup_email
		if twoFAMethod == "backup_email" {
			h.trySendBackupEmailCode(appID, user.ID.String())
		}
		redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true&provider=github&method=%s", redirectURI, tempToken, twoFAMethod)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Only create session when 2FA is NOT required
	ipAddress, userAgent := util.GetClientInfo(c)

	// Check IP-based access rules before completing login
	if !h.checkIPAccessRedirect(c, appID, ipAddress, userAgent, redirectURI) {
		return
	}

	accessToken, refreshToken, sessionErr := h.Service.CreateSessionOrTokens(appID.String(), userID.String(), ipAddress, userAgent)
	if sessionErr != nil {
		errorMsg := url.QueryEscape(sessionErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Log social login activity with anomaly detection
	h.runSocialLoginAnomalyDetection(appID, userID, user.Email, ipAddress, userAgent, "github")

	// Redirect to frontend with tokens in URL parameters
	frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=github",
		redirectURI,
		url.QueryEscape(accessToken),
		url.QueryEscape(refreshToken))

	health.IncLoginSuccess(appID.String())
	c.Redirect(http.StatusFound, frontendURL)
}

// ListSocialAccounts godoc
// @Summary      List linked social accounts
// @Description  Returns all social accounts linked to the authenticated user
// @Tags         social
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200 {object} dto.SocialAccountListResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /profile/social-accounts [get]
func (h *Handler) ListSocialAccounts(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in context"})
		return
	}

	accounts, appErr := h.Service.GetLinkedAccounts(userID.(string))
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	socialAccounts := make([]dto.SocialAccountResponse, len(accounts))
	for i, sa := range accounts {
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
			CreatedAt:      sa.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      sa.UpdatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, dto.SocialAccountListResponse{
		SocialAccounts: socialAccounts,
	})
}

// UnlinkSocialAccount godoc
// @Summary      Unlink a social account
// @Description  Removes a linked social account from the authenticated user's profile
// @Tags         social
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id path string true "Social account ID"
// @Success      200 {object} dto.UnlinkSocialAccountResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /profile/social-accounts/{id} [delete]
func (h *Handler) UnlinkSocialAccount(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in context"})
		return
	}

	appIDVal, appIDExists := c.Get("app_id")
	if !appIDExists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	socialAccountID := c.Param("id")
	if socialAccountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Social account ID is required"})
		return
	}

	if appErr := h.Service.UnlinkSocialAccount(appID.String(), userID.(string), socialAccountID); appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	// Log the unlink activity
	ipAddress, userAgent := util.GetClientInfo(c)
	parsedUserID, _ := uuid.Parse(userID.(string))
	log.LogSocialAccountUnlinked(appID, parsedUserID, ipAddress, userAgent, socialAccountID)

	c.JSON(http.StatusOK, dto.UnlinkSocialAccountResponse{
		Message: "Social account unlinked successfully",
	})
}

// GoogleLink godoc
// @Summary      Link Google account
// @Description  Initiates OAuth flow to link a Google account to the authenticated user
// @Tags         social
// @Produce      json
// @Security     ApiKeyAuth
// @Param        redirect_uri query string false "Frontend callback URL"
// @Success      307 {string} string "Redirect"
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /auth/google/link [get]
func (h *Handler) GoogleLink(c *gin.Context) {
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	userID, userExists := c.Get("userID")
	if !userExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	googleConfig, err := h.getGoogleConfig(appID.String())
	if err != nil {
		stdlog.Printf("Failed to get Google OAuth config for app %s: %v", appID.String(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth configuration error"})
		return
	}

	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	state, err := CreateOAuthLinkState(redirectURI, appID.String(), userID.(string))
	if err != nil {
		stdlog.Printf("Invalid OAuth redirect URI for Google link: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid redirect URI"})
		return
	}

	oauthURL := googleConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, oauthURL)
}

// GoogleLinkCallback godoc
// @Summary      Google link callback
// @Description  Handles callback from Google OAuth to link account to existing user
// @Tags         social
// @Produce      json
// @Param        state query string true "State token"
// @Param        code  query string true "Authorization code"
// @Success      302 {string} string "Redirect with linked=true"
// @Failure      302 {string} string "Redirect with error"
// @Router       /auth/google/link/callback [get]
func (h *Handler) GoogleLinkCallback(c *gin.Context) {
	encodedState := c.Query("state")
	if encodedState == "" {
		frontendURL := fmt.Sprintf("%s?error=missing_state", GetDefaultRedirectURI())
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	state, err := ParseOAuthState(encodedState)
	if err != nil {
		errorMsg := url.QueryEscape(fmt.Sprintf("Invalid state: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", GetDefaultRedirectURI(), errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	if state.Flow != "link" || state.UserID == "" {
		frontendURL := fmt.Sprintf("%s?error=invalid_link_state", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	code := c.Query("code")
	if code == "" {
		frontendURL := fmt.Sprintf("%s?error=authorization_code_missing", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	redirectURI := state.RedirectURI

	var appID uuid.UUID
	if state.AppID != "" {
		parsedAppID, err := uuid.Parse(state.AppID)
		if err != nil {
			frontendURL := fmt.Sprintf("%s?error=invalid_app_id_state", redirectURI)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		appID = parsedAppID
	} else {
		frontendURL := fmt.Sprintf("%s?error=app_id_missing", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	googleConfig, err := h.getGoogleConfig(appID.String())
	if err != nil {
		frontendURL := fmt.Sprintf("%s?error=config_error", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	token, err := googleConfig.Exchange(context.Background(), code)
	if err != nil {
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	_, appErr := h.Service.HandleGoogleLinkCallback(appID, state.UserID, token.AccessToken)
	if appErr != nil {
		errorMsg := url.QueryEscape(appErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Log the link activity
	ipAddress, userAgent := util.GetClientInfo(c)
	parsedUserID, _ := uuid.Parse(state.UserID)
	log.LogSocialAccountLinked(appID, parsedUserID, ipAddress, userAgent, "google")

	successURL := fmt.Sprintf("%s?linked=true&provider=google", redirectURI)
	c.Redirect(http.StatusFound, successURL)
}

// FacebookLink godoc
// @Summary      Link Facebook account
// @Description  Initiates OAuth flow to link a Facebook account to the authenticated user
// @Tags         social
// @Produce      json
// @Security     ApiKeyAuth
// @Param        redirect_uri query string false "Frontend callback URL"
// @Success      307 {string} string "Redirect"
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /auth/facebook/link [get]
func (h *Handler) FacebookLink(c *gin.Context) {
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	userID, userExists := c.Get("userID")
	if !userExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	facebookConfig, err := h.getFacebookConfig(appID.String())
	if err != nil {
		stdlog.Printf("Failed to get Facebook OAuth config for app %s: %v", appID.String(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth configuration error"})
		return
	}

	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	state, err := CreateOAuthLinkState(redirectURI, appID.String(), userID.(string))
	if err != nil {
		stdlog.Printf("Invalid OAuth redirect URI for Facebook link: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid redirect URI"})
		return
	}

	oauthURL := facebookConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, oauthURL)
}

// FacebookLinkCallback godoc
// @Summary      Facebook link callback
// @Description  Handles callback from Facebook OAuth to link account to existing user
// @Tags         social
// @Produce      json
// @Param        state query string true "State token"
// @Param        code  query string true "Authorization code"
// @Success      302 {string} string "Redirect with linked=true"
// @Failure      302 {string} string "Redirect with error"
// @Router       /auth/facebook/link/callback [get]
func (h *Handler) FacebookLinkCallback(c *gin.Context) {
	encodedState := c.Query("state")
	if encodedState == "" {
		frontendURL := fmt.Sprintf("%s?error=missing_state", GetDefaultRedirectURI())
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	state, err := ParseOAuthState(encodedState)
	if err != nil {
		errorMsg := url.QueryEscape(fmt.Sprintf("Invalid state: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", GetDefaultRedirectURI(), errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	if state.Flow != "link" || state.UserID == "" {
		frontendURL := fmt.Sprintf("%s?error=invalid_link_state", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	code := c.Query("code")
	if code == "" {
		frontendURL := fmt.Sprintf("%s?error=authorization_code_missing", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	redirectURI := state.RedirectURI

	var appID uuid.UUID
	if state.AppID != "" {
		parsedAppID, err := uuid.Parse(state.AppID)
		if err != nil {
			frontendURL := fmt.Sprintf("%s?error=invalid_app_id_state", redirectURI)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		appID = parsedAppID
	} else {
		frontendURL := fmt.Sprintf("%s?error=app_id_missing", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	facebookConfig, err := h.getFacebookConfig(appID.String())
	if err != nil {
		frontendURL := fmt.Sprintf("%s?error=config_error", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	token, err := facebookConfig.Exchange(context.Background(), code)
	if err != nil {
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	_, appErr := h.Service.HandleFacebookLinkCallback(appID, state.UserID, token.AccessToken)
	if appErr != nil {
		errorMsg := url.QueryEscape(appErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Log the link activity
	ipAddress, userAgent := util.GetClientInfo(c)
	parsedUserID, _ := uuid.Parse(state.UserID)
	log.LogSocialAccountLinked(appID, parsedUserID, ipAddress, userAgent, "facebook")

	successURL := fmt.Sprintf("%s?linked=true&provider=facebook", redirectURI)
	c.Redirect(http.StatusFound, successURL)
}

// GithubLink godoc
// @Summary      Link GitHub account
// @Description  Initiates OAuth flow to link a GitHub account to the authenticated user
// @Tags         social
// @Produce      json
// @Security     ApiKeyAuth
// @Param        redirect_uri query string false "Frontend callback URL"
// @Success      307 {string} string "Redirect"
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /auth/github/link [get]
func (h *Handler) GithubLink(c *gin.Context) {
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "App ID missing from context"})
		return
	}
	appID := appIDVal.(uuid.UUID)

	userID, userExists := c.Get("userID")
	if !userExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	githubConfig, err := h.getGithubConfig(appID.String())
	if err != nil {
		stdlog.Printf("Failed to get GitHub OAuth config for app %s: %v", appID.String(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth configuration error"})
		return
	}

	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	state, err := CreateOAuthLinkState(redirectURI, appID.String(), userID.(string))
	if err != nil {
		stdlog.Printf("Invalid OAuth redirect URI for GitHub link: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid redirect URI"})
		return
	}

	oauthURL := githubConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, oauthURL)
}

// GithubLinkCallback godoc
// @Summary      GitHub link callback
// @Description  Handles callback from GitHub OAuth to link account to existing user
// @Tags         social
// @Produce      json
// @Param        state query string true "State token"
// @Param        code  query string true "Authorization code"
// @Success      302 {string} string "Redirect with linked=true"
// @Failure      302 {string} string "Redirect with error"
// @Router       /auth/github/link/callback [get]
func (h *Handler) GithubLinkCallback(c *gin.Context) {
	encodedState := c.Query("state")
	if encodedState == "" {
		frontendURL := fmt.Sprintf("%s?error=missing_state", GetDefaultRedirectURI())
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	state, err := ParseOAuthState(encodedState)
	if err != nil {
		errorMsg := url.QueryEscape(fmt.Sprintf("Invalid state: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", GetDefaultRedirectURI(), errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	if state.Flow != "link" || state.UserID == "" {
		frontendURL := fmt.Sprintf("%s?error=invalid_link_state", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	code := c.Query("code")
	if code == "" {
		frontendURL := fmt.Sprintf("%s?error=authorization_code_missing", state.RedirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	redirectURI := state.RedirectURI

	var appID uuid.UUID
	if state.AppID != "" {
		parsedAppID, err := uuid.Parse(state.AppID)
		if err != nil {
			frontendURL := fmt.Sprintf("%s?error=invalid_app_id_state", redirectURI)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		appID = parsedAppID
	} else {
		frontendURL := fmt.Sprintf("%s?error=app_id_missing", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	githubConfig, err := h.getGithubConfig(appID.String())
	if err != nil {
		frontendURL := fmt.Sprintf("%s?error=config_error", redirectURI)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	token, err := githubConfig.Exchange(context.Background(), code)
	if err != nil {
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	_, appErr := h.Service.HandleGithubLinkCallback(appID, state.UserID, token.AccessToken)
	if appErr != nil {
		errorMsg := url.QueryEscape(appErr.Message)
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	// Log the link activity
	ipAddress, userAgent := util.GetClientInfo(c)
	parsedUserID, _ := uuid.Parse(state.UserID)
	log.LogSocialAccountLinked(appID, parsedUserID, ipAddress, userAgent, "github")

	successURL := fmt.Sprintf("%s?linked=true&provider=github", redirectURI)
	c.Redirect(http.StatusFound, successURL)
}
