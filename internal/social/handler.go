package social

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Handler struct {
	Service             *Service
	GoogleOauthConfig   *oauth2.Config
	FacebookOauthConfig *oauth2.Config
	GithubOauthConfig   *oauth2.Config
}

func NewHandler(s *Service) *Handler {
	return &Handler{
		Service: s,
		GoogleOauthConfig: &oauth2.Config{
			RedirectURL:  viper.GetString("GOOGLE_REDIRECT_URL"),
			ClientID:     viper.GetString("GOOGLE_CLIENT_ID"),
			ClientSecret: viper.GetString("GOOGLE_CLIENT_SECRET"),
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
			Endpoint:     google.Endpoint,
		},
		FacebookOauthConfig: &oauth2.Config{
			RedirectURL:  viper.GetString("FACEBOOK_REDIRECT_URL"),
			ClientID:     viper.GetString("FACEBOOK_CLIENT_ID"),
			ClientSecret: viper.GetString("FACEBOOK_CLIENT_SECRET"),
			Scopes:       []string{"email", "public_profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://www.facebook.com/v18.0/dialog/oauth",
				TokenURL: "https://graph.facebook.com/v18.0/oauth/access_token",
			},
		},
		GithubOauthConfig: &oauth2.Config{
			RedirectURL:  viper.GetString("GITHUB_REDIRECT_URL"),
			ClientID:     viper.GetString("GITHUB_CLIENT_ID"),
			ClientSecret: viper.GetString("GITHUB_CLIENT_SECRET"),
			Scopes:       []string{"user:email"}, // Request email scope
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://github.com/login/oauth/authorize",
				TokenURL: "https://github.com/login/oauth/access_token",
			},
		},
	}
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
	// Get redirect URI from query parameter or use default
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	// Create secure state with redirect URI
	state, err := CreateOAuthState(redirectURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid redirect URI",
			"details": err.Error(),
		})
		return
	}

	// Generate OAuth URL with secure state
	url := h.GoogleOauthConfig.AuthCodeURL(state)
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

	token, err := h.GoogleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	accessToken, refreshToken, userID, appErr := h.Service.HandleGoogleCallback(token.AccessToken)
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
		err := redis.SetTempUserSession(tempToken, user.ID.String(), 10*time.Minute)
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
	log.LogSocialLogin(userID, ipAddress, userAgent, "google")

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
	// Get redirect URI from query parameter or use default
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	// Create secure state with redirect URI
	state, err := CreateOAuthState(redirectURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid redirect URI",
			"details": err.Error(),
		})
		return
	}

	// Generate OAuth URL with secure state
	url := h.FacebookOauthConfig.AuthCodeURL(state)
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

	token, err := h.FacebookOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	accessToken, refreshToken, userID, appErr := h.Service.HandleFacebookCallback(token.AccessToken)
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
		err := redis.SetTempUserSession(tempToken, user.ID.String(), 10*time.Minute)
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
	log.LogSocialLogin(userID, ipAddress, userAgent, "facebook")

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
	// Get redirect URI from query parameter or use default
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		redirectURI = GetDefaultRedirectURI()
	}

	// Create secure state with redirect URI
	state, err := CreateOAuthState(redirectURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid redirect URI",
			"details": err.Error(),
		})
		return
	}

	// Generate OAuth URL with secure state
	url := h.GithubOauthConfig.AuthCodeURL(state)
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

	token, err := h.GithubOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		// Redirect to frontend with error
		errorMsg := url.QueryEscape(fmt.Sprintf("Could not retrieve token: %v", err))
		frontendURL := fmt.Sprintf("%s?error=%s", redirectURI, errorMsg)
		c.Redirect(http.StatusFound, frontendURL)
		return
	}

	accessToken, refreshToken, userID, appErr := h.Service.HandleGithubCallback(token.AccessToken)
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
		err := redis.SetTempUserSession(tempToken, user.ID.String(), 10*time.Minute)
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
	log.LogSocialLogin(userID, ipAddress, userAgent, "github")

	// Redirect to frontend with tokens in URL parameters
	frontendURL := fmt.Sprintf("%s?access_token=%s&refresh_token=%s&provider=github",
		redirectURI,
		url.QueryEscape(accessToken),
		url.QueryEscape(refreshToken))

	c.Redirect(http.StatusFound, frontendURL)
}
