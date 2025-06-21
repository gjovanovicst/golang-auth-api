package social

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
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

// Google OAuth2 Handlers
func (h *Handler) GoogleLogin(c *gin.Context) {
	url := h.GoogleOauthConfig.AuthCodeURL("randomstate") // "randomstate" should be a securely generated state token
	c.Redirect(http.StatusTemporaryRedirect, url)
}

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

	accessToken, refreshToken, appErr := h.Service.HandleGoogleCallback(token.AccessToken)
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// Facebook OAuth2 Handlers
func (h *Handler) FacebookLogin(c *gin.Context) {
	url := h.FacebookOauthConfig.AuthCodeURL("randomstate") // TODO: Securely generate state
	c.Redirect(http.StatusTemporaryRedirect, url)
}

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

	accessToken, refreshToken, appErr := h.Service.HandleFacebookCallback(token.AccessToken)
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// GitHub OAuth2 Handlers
func (h *Handler) GithubLogin(c *gin.Context) {
	url := h.GithubOauthConfig.AuthCodeURL("randomstate") // TODO: Securely generate state
	c.Redirect(http.StatusTemporaryRedirect, url)
}

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

	accessToken, refreshToken, appErr := h.Service.HandleGithubCallback(token.AccessToken)
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}