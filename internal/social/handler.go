package social

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/util"
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
// @Success      307 {string} string "Redirect"
// @Router       /auth/google/login [get]
func (h *Handler) GoogleLogin(c *gin.Context) {
	url := h.GoogleOauthConfig.AuthCodeURL("randomstate") // "randomstate" should be a securely generated state token
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
	state := c.Query("state")
	if state != "randomstate" { // TODO: Validate state token securely
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid state"})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code not provided"})
		return
	}

	token, err := h.GoogleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Could not retrieve token: %v", err)})
		return
	}

	accessToken, refreshToken, userID, appErr := h.Service.HandleGoogleCallback(token.AccessToken)
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	// Log social login activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogSocialLogin(userID, ipAddress, userAgent, "google")

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// FacebookLogin godoc
// @Summary      Facebook OAuth2 Login
// @Description  Redirects user to Facebook OAuth2 login page
// @Tags         social
// @Produce      json
// @Success      307 {string} string "Redirect"
// @Router       /auth/facebook/login [get]
func (h *Handler) FacebookLogin(c *gin.Context) {
	url := h.FacebookOauthConfig.AuthCodeURL("randomstate") // TODO: Securely generate state
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
	state := c.Query("state")
	if state != "randomstate" { // TODO: Validate state token securely
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid state"})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code not provided"})
		return
	}

	token, err := h.FacebookOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Could not retrieve token: %v", err)})
		return
	}

	accessToken, refreshToken, userID, appErr := h.Service.HandleFacebookCallback(token.AccessToken)
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	// Log social login activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogSocialLogin(userID, ipAddress, userAgent, "facebook")

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// GithubLogin godoc
// @Summary      GitHub OAuth2 Login
// @Description  Redirects user to GitHub OAuth2 login page
// @Tags         social
// @Produce      json
// @Success      307 {string} string "Redirect"
// @Router       /auth/github/login [get]
func (h *Handler) GithubLogin(c *gin.Context) {
	url := h.GithubOauthConfig.AuthCodeURL("randomstate") // TODO: Securely generate state
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
	state := c.Query("state")
	if state != "randomstate" { // TODO: Validate state token securely
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid state"})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code not provided"})
		return
	}

	token, err := h.GithubOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Could not retrieve token: %v", err)})
		return
	}

	accessToken, refreshToken, userID, appErr := h.Service.HandleGithubCallback(token.AccessToken)
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	// Log social login activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogSocialLogin(userID, ipAddress, userAgent, "github")

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}
