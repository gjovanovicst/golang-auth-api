package social

import (
	"context"
	"fmt"
	stdlog "log"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Handler struct {
	Service *Service
}

func NewHandler(s *Service) *Handler {
	return &Handler{
		Service: s,
	}
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

	accessToken, refreshToken, userID, appErr := h.Service.HandleGoogleCallback(appID, token.AccessToken)
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

	if user.TwoFAEnabled {
		tempToken := uuid.New().String()
		err := redis.SetTempUserSession(appID.String(), tempToken, user.ID.String(), 10*time.Minute)
		if err != nil {
			// Redirect to frontend with error
			errorMsg := url.QueryEscape("Failed to create temporary session for 2FA")
			frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		// Redirect with 2FA requirement
		redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true&provider=google", redirectURI, tempToken)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Log social login activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogSocialLogin(appID, userID, ipAddress, userAgent, "google")

	// Redirect to frontend with tokens in URL parameters
	frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=google",
		redirectURI,
		url.QueryEscape(accessToken),
		url.QueryEscape(refreshToken))

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

	accessToken, refreshToken, userID, appErr := h.Service.HandleFacebookCallback(appID, token.AccessToken)
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

	if user.TwoFAEnabled {
		tempToken := uuid.New().String()
		err := redis.SetTempUserSession(appID.String(), tempToken, user.ID.String(), 10*time.Minute)
		if err != nil {
			// Redirect to frontend with error
			errorMsg := url.QueryEscape("Failed to create temporary session for 2FA")
			frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		// Redirect with 2FA requirement
		redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true&provider=facebook", redirectURI, tempToken)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Log social login activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogSocialLogin(appID, userID, ipAddress, userAgent, "facebook")

	// Redirect to frontend with tokens in URL parameters
	frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=facebook",
		redirectURI,
		url.QueryEscape(accessToken),
		url.QueryEscape(refreshToken))

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

	accessToken, refreshToken, userID, appErr := h.Service.HandleGithubCallback(appID, token.AccessToken)
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

	if user.TwoFAEnabled {
		tempToken := uuid.New().String()
		err := redis.SetTempUserSession(appID.String(), tempToken, user.ID.String(), 10*time.Minute)
		if err != nil {
			// Redirect to frontend with error
			errorMsg := url.QueryEscape("Failed to create temporary session for 2FA")
			frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
			c.Redirect(http.StatusFound, frontendURL)
			return
		}
		// Redirect with 2FA requirement
		redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true&provider=github", redirectURI, tempToken)
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Log social login activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogSocialLogin(appID, userID, ipAddress, userAgent, "github")

	// Redirect to frontend with tokens in URL parameters
	frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=github",
		redirectURI,
		url.QueryEscape(accessToken),
		url.QueryEscape(refreshToken))

	c.Redirect(http.StatusFound, frontendURL)
}
