package social

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/user"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

type Service struct {
	UserRepo   *user.Repository
	SocialRepo *Repository
}

func NewService(ur *user.Repository, sr *Repository) *Service {
	return &Service{UserRepo: ur, SocialRepo: sr}
}

func (s *Service) HandleGoogleCallback(googleAccessToken string) (string, string, uuid.UUID, *errors.AppError) {
	// Fetch user info from Google
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + googleAccessToken)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user info from Google")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read Google user info response")
	}

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(userData, &googleUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse Google user info")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("google", googleUser.ID)
	if err == nil { // Social account found, user exists
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
		}
		// Store refresh token in Redis
		if err := redis.SetRefreshToken(socialAccount.UserID.String(), refreshToken); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
		}
		return accessToken, refreshToken, socialAccount.UserID, nil
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
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to link social account")
		}
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(user.ID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(user.ID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
		}
		// Store refresh token in Redis
		if err := redis.SetRefreshToken(user.ID.String(), refreshToken); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
		}
		return accessToken, refreshToken, user.ID, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		Email:         googleUser.Email,
		EmailVerified: true, // Assuming email from Google is verified
		// PasswordHash is not set for social logins
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "google",
		ProviderUserID: googleUser.ID,
		AccessToken:    googleAccessToken,
		ExpiresAt:      nil,
	}
	if err := s.SocialRepo.CreateSocialAccount(newSocialAccount); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	// Authenticate new user
	accessToken, err := jwt.GenerateAccessToken(newUser.ID.String())
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}
	refreshToken, err := jwt.GenerateRefreshToken(newUser.ID.String())
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}
	// Store refresh token in Redis
	if err := redis.SetRefreshToken(newUser.ID.String(), refreshToken); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}
	return accessToken, refreshToken, newUser.ID, nil
}

func (s *Service) HandleFacebookCallback(facebookAccessToken string) (string, string, uuid.UUID, *errors.AppError) {
	// Fetch user info from Facebook Graph API
	resp, err := http.Get("https://graph.facebook.com/v18.0/me?fields=id,name,email&access_token=" + facebookAccessToken)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user info from Facebook")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read Facebook user info response")
	}

	var facebookUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(userData, &facebookUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse Facebook user info")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("facebook", facebookUser.ID)
	if err == nil { // Social account found, user exists
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
		}
		// Store refresh token in Redis
		if err := redis.SetRefreshToken(socialAccount.UserID.String(), refreshToken); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
		}
		return accessToken, refreshToken, socialAccount.UserID, nil
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
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to link social account")
		}
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(user.ID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(user.ID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
		}
		// Store refresh token in Redis
		if err := redis.SetRefreshToken(user.ID.String(), refreshToken); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
		}
		return accessToken, refreshToken, user.ID, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		Email:         facebookUser.Email,
		EmailVerified: true, // Assuming email from Facebook is verified
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "facebook",
		ProviderUserID: facebookUser.ID,
		AccessToken:    facebookAccessToken,
		ExpiresAt:      nil,
	}
	if err := s.SocialRepo.CreateSocialAccount(newSocialAccount); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	// Authenticate new user
	accessToken, err := jwt.GenerateAccessToken(newUser.ID.String())
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}
	refreshToken, err := jwt.GenerateRefreshToken(newUser.ID.String())
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}
	// Store refresh token in Redis
	if err := redis.SetRefreshToken(newUser.ID.String(), refreshToken); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}
	return accessToken, refreshToken, newUser.ID, nil
}

func (s *Service) HandleGithubCallback(githubAccessToken string) (string, string, uuid.UUID, *errors.AppError) {
	// Fetch user info from GitHub API
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create GitHub request")
	}
	req.Header.Set("Authorization", "token "+githubAccessToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user info from GitHub")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read GitHub user info response")
	}

	var githubUser struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(userData, &githubUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse GitHub user info")
	}

	// GitHub's user endpoint might not always return email if it's private. Fetch public emails separately.
	if githubUser.Email == "" {
		req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create GitHub emails request")
		}
		req.Header.Set("Authorization", "token "+githubAccessToken)
		resp, err := client.Do(req)
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user emails from GitHub")
		}
		defer resp.Body.Close()

		emailData, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read GitHub emails response")
		}

		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := json.Unmarshal(emailData, &emails); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse GitHub emails")
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
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrBadRequest, "No public or primary verified email found for GitHub account. Please ensure your primary email is public and verified on GitHub.")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("github", strconv.FormatInt(githubUser.ID, 10))
	if err == nil { // Social account found, user exists
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(socialAccount.UserID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
		}
		// Store refresh token in Redis
		if err := redis.SetRefreshToken(socialAccount.UserID.String(), refreshToken); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
		}
		return accessToken, refreshToken, socialAccount.UserID, nil
	}

	// If social account not found, check if user with this email exists
	user, err := s.UserRepo.GetUserByEmail(githubUser.Email)
	if err == nil { // User with this email exists, link social account
		socialAccount := &models.SocialAccount{
			UserID:         user.ID,
			Provider:       "github",
			ProviderUserID: strconv.FormatInt(githubUser.ID, 10),
			AccessToken:    githubAccessToken,
			ExpiresAt:      nil,
		}
		if err := s.SocialRepo.CreateSocialAccount(socialAccount); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to link social account")
		}
		// Authenticate existing user
		accessToken, err := jwt.GenerateAccessToken(user.ID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
		}
		refreshToken, err := jwt.GenerateRefreshToken(user.ID.String())
		if err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
		}
		// Store refresh token in Redis
		if err := redis.SetRefreshToken(user.ID.String(), refreshToken); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
		}
		return accessToken, refreshToken, user.ID, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		Email:         githubUser.Email,
		EmailVerified: true, // Assuming email from GitHub is verified if primary and verified
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "github",
		ProviderUserID: strconv.FormatInt(githubUser.ID, 10),
		AccessToken:    githubAccessToken,
		ExpiresAt:      nil,
	}
	if err := s.SocialRepo.CreateSocialAccount(newSocialAccount); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	// Authenticate new user
	accessToken, err := jwt.GenerateAccessToken(newUser.ID.String())
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}
	refreshToken, err := jwt.GenerateRefreshToken(newUser.ID.String())
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}
	// Store refresh token in Redis
	if err := redis.SetRefreshToken(newUser.ID.String(), refreshToken); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}
	return accessToken, refreshToken, newUser.ID, nil
}
