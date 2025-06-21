## Phase 4: Social Authentication Integration Plan

This phase outlines the integration of social network authentication for Google, Facebook, and GitHub. Social login enhances user experience by allowing quick registration and sign-in without requiring users to create new credentials. The implementation will follow the OAuth2 authorization code flow, handling redirects, token exchanges, and user profile retrieval.

### 4.1 OAuth2 Flow Overview

The OAuth2 authorization code grant type is the most common and secure method for web applications. The general flow for social authentication is as follows:

1.  **Client Initiates Authorization:** The user clicks a social login button (e.g., "Sign in with Google").
2.  **Redirect to Authorization Server:** The client redirects the user\"s browser to the social provider\"s (authorization server\"s) authorization endpoint.
3.  **User Grants Authorization:** The user is prompted to log in to the social provider and grant permission to the application.
4.  **Authorization Code Granted:** If the user approves, the social provider redirects the user\"s browser back to the application\"s predefined redirect URI, including an authorization code.
5.  **Exchange Authorization Code for Tokens:** The application\"s backend receives the authorization code and exchanges it for an Access Token (and sometimes a Refresh Token) at the social provider\"s token endpoint. This exchange happens directly between the backend and the social provider, not via the user\"s browser.
6.  **Retrieve User Profile:** The application uses the Access Token to call the social provider\"s API to retrieve the user\"s profile information (e.g., email, name, unique ID).
7.  **Authentication/Registration:** The application checks if a `User` or `SocialAccount` already exists for the `ProviderUserID`. If not, a new `User` and `SocialAccount` are created. If yes, the existing `User` is authenticated.
8.  **Issue Internal JWT:** The application issues its own internal JWTs (Access and Refresh Tokens) to the user, similar to traditional email/password login.

### 4.2 Google OAuth2 Integration

Google OAuth2 will be integrated using the `golang.org/x/oauth2` and `golang.org/x/oauth2/google` packages. This involves configuring OAuth2 credentials in the Google Developer Console and setting up the necessary endpoints in our Go application.

**Configuration (Environment Variables):**
-   `GOOGLE_CLIENT_ID`
-   `GOOGLE_CLIENT_SECRET`
-   `GOOGLE_REDIRECT_URL` (e.g., `http://localhost:8080/auth/google/callback`)

**Process Flow:**
1.  **Initiate Google Login (`GET /auth/google/login`):**
    -   Construct the Google OAuth2 URL using `oauth2.Config`.
    -   Redirect the user\"s browser to this URL.

2.  **Google Callback (`GET /auth/google/callback`):**
    -   Receive the authorization code from Google in the query parameters.
    -   Exchange the authorization code for an Access Token and ID Token using `oauth2.Config.Exchange`.
    -   Use the Access Token to fetch user information from Google\"s `userinfo` endpoint (e.g., `https://www.googleapis.com/oauth2/v2/userinfo`).
    -   Extract `email`, `name`, and `id` (Google\"s unique user ID) from the user info.
    -   **Check for existing `SocialAccount`:** Query the database for a `SocialAccount` with `Provider=\"google\"` and `ProviderUserID` matching the Google ID.
        -   If found: Authenticate the associated `User` and issue internal JWTs.
        -   If not found:
            -   **Check for existing `User` by email:** If the Google profile provides an email, check if a `User` with that email already exists.
                -   If found: Link the new `SocialAccount` to this existing `User`.
                -   If not found: Create a new `User` record and then link the new `SocialAccount` to this newly created `User`.
    -   Issue internal JWTs (Access and Refresh Tokens) to the client.
    -   Redirect the client to a success page or return tokens in the response.

**Example Code Snippets (Conceptual):**

**`internal/social/handler.go` (Google Handlers):**

```go
package social

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"github.com/spf13/viper"
	"github.com/your_username/your_project/pkg/errors"
)

type Handler struct {
	Service *Service
	GoogleOauthConfig *oauth2.Config
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
	}
}

func (h *Handler) GoogleLogin(c *gin.Context) {
	url := h.GoogleOauthConfig.AuthCodeURL("randomstate") // \"randomstate\" should be a securely generated state token
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
```

**`internal/social/service.go` (Google Logic):**

```go
package social

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/your_username/your_project/pkg/errors"
	"github.com/your_username/your_project/pkg/jwt"
	"github.com/your_username/your_project/pkg/models"
	"github.com/your_username/your_project/internal/user"
	"github.com/your_username/your_project/internal/redis"
)

type Service struct {
	UserRepo *user.Repository
	SocialRepo *Repository
}

func NewService(ur *user.Repository, sr *Repository) *Service {
	return &Service{UserRepo: ur, SocialRepo: sr}
}

func (s *Service) HandleGoogleCallback(googleAccessToken string) (string, string, *errors.AppError) {
	// Fetch user info from Google
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + googleAccessToken)
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to get user info from Google")
	}
	defer resp.Body.Close()

	userData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to read Google user info response")
	}

	var googleUser struct {
		ID    string `json:\"id\"`
		Email string `json:\"email\"`
		Name  string `json:\"name\"`
	}
	if err := json.Unmarshal(userData, &googleUser); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to parse Google user info")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("google", googleUser.ID)
	if err == nil { // Social account found, user exists
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
		}
		if err := redis.SetRefreshToken(socialAccount.UserID.String(), refreshToken); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
		}
		return accessToken, refreshToken, nil
	}

	// If social account not found, check if user with this email exists
	user, err := s.UserRepo.GetUserByEmail(googleUser.Email)
	if err == nil { // User with this email exists, link social account
		socialAccount := &models.SocialAccount{
			UserID:         user.ID,
			Provider:       "google",
			ProviderUserID: googleUser.ID,
			AccessToken:    googleAccessToken,
			ExpiresAt:      nil, // Google access tokens have short expiry, might not be needed to store
		}
		if err := s.SocialRepo.CreateSocialAccount(socialAccount); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to link social account")
		}
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(user.ID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(user.ID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
		}
		if err := redis.SetRefreshToken(user.ID.String(), refreshToken); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
		}
		return accessToken, refreshToken, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		Email:         googleUser.Email,
		EmailVerified: true, // Assuming email from Google is verified
		// PasswordHash is not set for social logins
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to create new user")
	}

	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "google",
		ProviderUserID: googleUser.ID,
		AccessToken:    googleAccessToken,
		ExpiresAt:      nil,
	}
	if err := s.SocialRepo.CreateSocialAccount(newSocialAccount); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to create social account")
	}

	// Authenticate new user
	accessToken, err := jwt.GenerateAccessToken(newUser.ID.String())
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
	}
	refreshToken, err := jwt.GenerateRefreshToken(newUser.ID.String())
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
	}
	if err := redis.SetRefreshToken(newUser.ID.String(), refreshToken); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
	}
	return accessToken, refreshToken, nil
}
```

### 4.3 Facebook OAuth2 Integration

Facebook OAuth2 integration will follow a similar pattern to Google, using the `golang.org/x/oauth2` package. We will need to configure an app in the Facebook Developer Dashboard.

**Configuration (Environment Variables):**
-   `FACEBOOK_CLIENT_ID`
-   `FACEBOOK_CLIENT_SECRET`
-   `FACEBOOK_REDIRECT_URL` (e.g., `http://localhost:8080/auth/facebook/callback`)

**Process Flow:**
1.  **Initiate Facebook Login (`GET /auth/facebook/login`):**
    -   Construct the Facebook OAuth2 URL.
    -   Redirect the user\"s browser to this URL.

2.  **Facebook Callback (`GET /auth/facebook/callback`):**
    -   Receive the authorization code from Facebook.
    -   Exchange the authorization code for an Access Token.
    -   Use the Access Token to fetch user information from Facebook\"s Graph API (e.g., `https://graph.facebook.com/v18.0/me?fields=id,name,email`).
    -   Extract `id` (Facebook\"s unique user ID), `name`, and `email`.
    -   **Check for existing `SocialAccount`:** Query the database for a `SocialAccount` with `Provider=\"facebook\"` and `ProviderUserID` matching the Facebook ID.
        -   If found: Authenticate the associated `User` and issue internal JWTs.
        -   If not found:
            -   **Check for existing `User` by email:** If the Facebook profile provides an email, check if a `User` with that email already exists.
                -   If found: Link the new `SocialAccount` to this existing `User`.
                -   If not found: Create a new `User` record and then link the new `SocialAccount` to this newly created `User`.
    -   Issue internal JWTs (Access and Refresh Tokens) to the client.
    -   Redirect the client to a success page or return tokens in the response.

**Example Code Snippets (Conceptual):**

**`internal/social/handler.go` (Facebook Handlers - add to existing Handler struct):**

```go
// ... (inside Handler struct)
	FacebookOauthConfig *oauth2.Config
}

// ... (inside NewHandler function)
		FacebookOauthConfig: &oauth2.Config{
			RedirectURL:  viper.GetString("FACEBOOK_REDIRECT_URL"),
			ClientID:     viper.GetString("FACEBOOK_CLIENT_ID"),
			ClientSecret: viper.GetString("FACEBOOK_CLIENT_SECRET"),
			Scopes:       []string{"email", "public_profile"},
			Endpoint:     facebook.Endpoint,
		},
	}
}

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
```

**`internal/social/service.go` (Facebook Logic - add to existing Service struct):**

```go
// ... (inside Service struct)

func (s *Service) HandleFacebookCallback(facebookAccessToken string) (string, string, *errors.AppError) {
	// Fetch user info from Facebook Graph API
	resp, err := http.Get("https://graph.facebook.com/v18.0/me?fields=id,name,email&access_token=" + facebookAccessToken)
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to get user info from Facebook")
	}
	defer resp.Body.Close()

	userData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to read Facebook user info response")
	}

	var facebookUser struct {
		ID    string `json:\"id\"`
		Email string `json:\"email\"`
		Name  string `json:\"name\"`
	}
	if err := json.Unmarshal(userData, &facebookUser); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to parse Facebook user info")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("facebook", facebookUser.ID)
	if err == nil { // Social account found, user exists
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
		}
		if err := redis.SetRefreshToken(socialAccount.UserID.String(), refreshToken); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
		}
		return accessToken, refreshToken, nil
	}

	// If social account not found, check if user with this email exists
	user, err := s.UserRepo.GetUserByEmail(facebookUser.Email)
	if err == nil { // User with this email exists, link social account
		socialAccount := &models.SocialAccount{
			UserID:         user.ID,
			Provider:       "facebook",
			ProviderUserID: facebookUser.ID,
			AccessToken:    facebookAccessToken,
			ExpiresAt:      nil,
		}
		if err := s.SocialRepo.CreateSocialAccount(socialAccount); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to link social account")
		}
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(user.ID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(user.ID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
		}
		if err := redis.SetRefreshToken(user.ID.String(), refreshToken); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
		}
		return accessToken, refreshToken, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		Email:         facebookUser.Email,
		EmailVerified: true, // Assuming email from Facebook is verified
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to create new user")
	}

	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "facebook",
		ProviderUserID: facebookUser.ID,
		AccessToken:    facebookAccessToken,
		ExpiresAt:      nil,
	}
	if err := s.SocialRepo.CreateSocialAccount(newSocialAccount); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to create social account")
	}

	// Authenticate new user
	accessToken, err := jwt.GenerateAccessToken(newUser.ID.String())
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
	}
	refreshToken, err := jwt.GenerateRefreshToken(newUser.ID.String())
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
	}
	if err := redis.SetRefreshToken(newUser.ID.String(), refreshToken); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
	}
	return accessToken, refreshToken, nil
}
```

### 4.4 GitHub OAuth2 Integration

GitHub OAuth2 integration will also use the `golang.org/x/oauth2` package. We will need to register an OAuth App in GitHub.

**Configuration (Environment Variables):**
-   `GITHUB_CLIENT_ID`
-   `GITHUB_CLIENT_SECRET`
-   `GITHUB_REDIRECT_URL` (e.g., `http://localhost:8080/auth/github/callback`)

**Process Flow:**
1.  **Initiate GitHub Login (`GET /auth/github/login`):**
    -   Construct the GitHub OAuth2 URL.
    -   Redirect the user\"s browser to this URL.

2.  **GitHub Callback (`GET /auth/github/callback`):**
    -   Receive the authorization code from GitHub.
    -   Exchange the authorization code for an Access Token.
    -   Use the Access Token to fetch user information from GitHub\"s API (e.g., `https://api.github.com/user`).
    -   Extract `id` (GitHub\"s unique user ID), `login` (username), and `email` (if public or granted).
    -   **Check for existing `SocialAccount`:** Query the database for a `SocialAccount` with `Provider=\"github\"` and `ProviderUserID` matching the GitHub ID.
        -   If found: Authenticate the associated `User` and issue internal JWTs.
        -   If not found:
            -   **Check for existing `User` by email:** If the GitHub profile provides an email, check if a `User` with that email already exists.
                -   If found: Link the new `SocialAccount` to this existing `User`.
                -   If not found: Create a new `User` record and then link the new `SocialAccount` to this newly created `User`.
    -   Issue internal JWTs (Access and Refresh Tokens) to the client.
    -   Redirect the client to a success page or return tokens in the response.

**Example Code Snippets (Conceptual):**

**`internal/social/handler.go` (GitHub Handlers - add to existing Handler struct):**

```go
// ... (inside Handler struct)
	GithubOauthConfig *oauth2.Config
}

// ... (inside NewHandler function)
		GithubOauthConfig: &oauth2.Config{
			RedirectURL:  viper.GetString("GITHUB_REDIRECT_URL"),
			ClientID:     viper.GetString("GITHUB_CLIENT_ID"),
			ClientSecret: viper.GetString("GITHUB_CLIENT_SECRET"),
			Scopes:       []string{"user:email"}, // Request email scope
			Endpoint:     github.Endpoint,
		},
	}
}

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
```

**`internal/social/service.go` (GitHub Logic - add to existing Service struct):**

```go
// ... (inside Service struct)

func (s *Service) HandleGithubCallback(githubAccessToken string) (string, string, *errors.AppError) {
	// Fetch user info from GitHub API
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to create GitHub request")
	}
	req.Header.Set("Authorization", "token "+githubAccessToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to get user info from GitHub")
	}
	defer resp.Body.Close()

	userData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to read GitHub user info response")
	}

	var githubUser struct {
		ID    int64  `json:\"id\"`
		Login string `json:\"login\"`
		Email string `json:\"email\"`
	}
	if err := json.Unmarshal(userData, &githubUser); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to parse GitHub user info")
	}

	// GitHub\"s user endpoint might not always return email if it\"s private. Fetch public emails separately.
	if githubUser.Email == "" {
		req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to create GitHub emails request")
		}
		req.Header.Set("Authorization", "token "+githubAccessToken)
		resp, err := client.Do(req)
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to get user emails from GitHub")
		}
		defer resp.Body.Close()

		emailData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to read GitHub emails response")
		}

		var emails []struct {
			Email    string `json:\"email\"`
			Primary  bool   `json:\"primary\"`
			Verified bool   `json:\"verified\"`
		}
		if err := json.Unmarshal(emailData, &emails); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to parse GitHub emails")
		}

		for _, email := range emails {
			if email.Primary && email.Verified {
				githubUser.Email = email.Email
				break
			}
		}
	}

	if githubUser.Email == "" {
		// Handle case where no public or primary verified email is available
		return "", "", errors.NewAppError(http.StatusBadRequest, "No public or primary verified email found for GitHub account. Please ensure your primary email is public and verified on GitHub.")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("github", fmt.Sprintf(\"%d\", githubUser.ID))
	if err == nil { // Social account found, user exists
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
		}
		if err := redis.SetRefreshToken(socialAccount.UserID.String(), refreshToken); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
		}
		return accessToken, refreshToken, nil
	}

	// If social account not found, check if user with this email exists
	user, err := s.UserRepo.GetUserByEmail(githubUser.Email)
	if err == nil { // User with this email exists, link social account
		socialAccount := &models.SocialAccount{
			UserID:         user.ID,
			Provider:       "github",
			ProviderUserID: fmt.Sprintf(\"%d\", githubUser.ID),
			AccessToken:    githubAccessToken,
			ExpiresAt:      nil,
		}
		if err := s.SocialRepo.CreateSocialAccount(socialAccount); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to link social account")
		}
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(user.ID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(user.ID.String())
		if err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
		}
		if err := redis.SetRefreshToken(user.ID.String(), refreshToken); err != nil {
			return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
		}
		return accessToken, refreshToken, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		Email:         githubUser.Email,
		EmailVerified: true, // Assuming email from GitHub is verified if primary and verified
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to create new user")
	}

	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "github",
		ProviderUserID: fmt.Sprintf(\"%d\", githubUser.ID),
		AccessToken:    githubAccessToken,
		ExpiresAt:      nil,
	}
	if err := s.SocialRepo.CreateSocialAccount(newSocialAccount); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to create social account")
	}

	// Authenticate new user
	accessToken, err := jwt.GenerateAccessToken(newUser.ID.String())
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate access token")
	}
	refreshToken, err := jwt.GenerateRefreshToken(newUser.ID.String())
	if err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to generate refresh token")
	}
	if err := redis.SetRefreshToken(newUser.ID.String(), refreshToken); err != nil {
		return "", "", errors.NewAppError(http.StatusInternalServerError, "Failed to store refresh token")
	}
	return accessToken, refreshToken, nil
}
```

### 4.5 Handling Social Account Linking and New User Creation

The `HandleGoogleCallback`, `HandleFacebookCallback`, and `HandleGithubCallback` functions in the `social/service.go` will implement the core logic for managing social logins:

1.  **Check for existing `SocialAccount`:** The first step is to query the `SocialAccount` table using the `Provider` and `ProviderUserID` obtained from the social provider. If a matching record is found, it means the user has previously logged in with this social account. The system will then authenticate the associated `User` and issue internal JWTs.

2.  **Check for existing `User` by email:** If no `SocialAccount` is found for the given `Provider` and `ProviderUserID`, the system will attempt to find an existing `User` record using the email address provided by the social provider. This handles scenarios where a user might have registered with email/password and later tries to link a social account with the same email.
    -   If a `User` with the same email exists, a new `SocialAccount` record will be created and linked to this existing `User`.
    -   The `EmailVerified` flag for the `User` can be set to `true` if the social provider confirms the email is verified.

3.  **Create new `User` and `SocialAccount`:** If neither a matching `SocialAccount` nor an existing `User` with the same email is found, it indicates a new user. In this case, a new `User` record will be created (with `EmailVerified` set to `true` if the social provider verifies the email), and a new `SocialAccount` record will be created and linked to this new `User`.

4.  **Issue Internal JWTs:** Regardless of whether it\"s an existing user, a linked account, or a new registration, the system will always issue its own internal Access and Refresh Tokens to the client upon successful social authentication. This ensures a consistent authentication mechanism across all login methods.

This concludes the social authentication integration plan.

