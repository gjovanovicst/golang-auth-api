package social

import (
	"encoding/json"
	"io"
	"log"
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
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
		Locale        string `json:"locale"`
	}
	if err := json.Unmarshal(userData, &googleUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse Google user info")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("google", googleUser.ID)
	if err == nil { // Social account found, user exists
		// Update social account with latest data from provider
		rawDataJSON, _ := json.Marshal(googleUser)
		socialAccount.Email = googleUser.Email
		socialAccount.Name = googleUser.Name
		socialAccount.FirstName = googleUser.GivenName
		socialAccount.LastName = googleUser.FamilyName
		socialAccount.ProfilePicture = googleUser.Picture
		socialAccount.Locale = googleUser.Locale
		socialAccount.RawData = rawDataJSON
		socialAccount.AccessToken = googleAccessToken

		if err := s.SocialRepo.UpdateSocialAccount(socialAccount); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		user, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			updated := false
			if user.Name != googleUser.Name && googleUser.Name != "" {
				user.Name = googleUser.Name
				updated = true
			}
			if user.FirstName != googleUser.GivenName && googleUser.GivenName != "" {
				user.FirstName = googleUser.GivenName
				updated = true
			}
			if user.LastName != googleUser.FamilyName && googleUser.FamilyName != "" {
				user.LastName = googleUser.FamilyName
				updated = true
			}
			if user.ProfilePicture != googleUser.Picture && googleUser.Picture != "" {
				user.ProfilePicture = googleUser.Picture
				updated = true
			}
			if user.Locale != googleUser.Locale && googleUser.Locale != "" {
				user.Locale = googleUser.Locale
				updated = true
			}
			if updated {
				if err := s.UserRepo.UpdateUser(user); err != nil {
					// Log error but don't fail authentication
					log.Printf("Failed to update user profile: %v", err)
				}
			}
		}

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
		// Update user profile with Google data if not already set
		if user.Name == "" && googleUser.Name != "" {
			user.Name = googleUser.Name
		}
		if user.FirstName == "" && googleUser.GivenName != "" {
			user.FirstName = googleUser.GivenName
		}
		if user.LastName == "" && googleUser.FamilyName != "" {
			user.LastName = googleUser.FamilyName
		}
		if user.ProfilePicture == "" && googleUser.Picture != "" {
			user.ProfilePicture = googleUser.Picture
		}
		if user.Locale == "" && googleUser.Locale != "" {
			user.Locale = googleUser.Locale
		}
		if err := s.UserRepo.UpdateUser(user); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update user profile")
		}

		rawDataJSON, _ := json.Marshal(googleUser)
		socialAccount := &models.SocialAccount{
			UserID:         user.ID,
			Provider:       "google",
			ProviderUserID: googleUser.ID,
			Email:          googleUser.Email,
			Name:           googleUser.Name,
			FirstName:      googleUser.GivenName,
			LastName:       googleUser.FamilyName,
			ProfilePicture: googleUser.Picture,
			Locale:         googleUser.Locale,
			RawData:        rawDataJSON,
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
		Email:          googleUser.Email,
		EmailVerified:  googleUser.VerifiedEmail,
		Name:           googleUser.Name,
		FirstName:      googleUser.GivenName,
		LastName:       googleUser.FamilyName,
		ProfilePicture: googleUser.Picture,
		Locale:         googleUser.Locale,
		// PasswordHash is not set for social logins
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	rawDataJSON, _ := json.Marshal(googleUser)
	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "google",
		ProviderUserID: googleUser.ID,
		Email:          googleUser.Email,
		Name:           googleUser.Name,
		FirstName:      googleUser.GivenName,
		LastName:       googleUser.FamilyName,
		ProfilePicture: googleUser.Picture,
		Locale:         googleUser.Locale,
		RawData:        rawDataJSON,
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
	// Fetch user info from Facebook Graph API with extended fields
	resp, err := http.Get("https://graph.facebook.com/v18.0/me?fields=id,name,email,first_name,last_name,picture.type(large),locale&access_token=" + facebookAccessToken)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user info from Facebook")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read Facebook user info response")
	}

	var facebookUser struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Picture   struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
		Locale string `json:"locale"`
	}
	if err := json.Unmarshal(userData, &facebookUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse Facebook user info")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("facebook", facebookUser.ID)
	if err == nil { // Social account found, user exists
		// Update social account with latest data from provider
		rawDataJSON, _ := json.Marshal(facebookUser)
		socialAccount.Email = facebookUser.Email
		socialAccount.Name = facebookUser.Name
		socialAccount.FirstName = facebookUser.FirstName
		socialAccount.LastName = facebookUser.LastName
		socialAccount.ProfilePicture = facebookUser.Picture.Data.URL
		socialAccount.Locale = facebookUser.Locale
		socialAccount.RawData = rawDataJSON
		socialAccount.AccessToken = facebookAccessToken

		if err := s.SocialRepo.UpdateSocialAccount(socialAccount); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		user, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			updated := false
			if user.Name != facebookUser.Name && facebookUser.Name != "" {
				user.Name = facebookUser.Name
				updated = true
			}
			if user.FirstName != facebookUser.FirstName && facebookUser.FirstName != "" {
				user.FirstName = facebookUser.FirstName
				updated = true
			}
			if user.LastName != facebookUser.LastName && facebookUser.LastName != "" {
				user.LastName = facebookUser.LastName
				updated = true
			}
			if user.ProfilePicture != facebookUser.Picture.Data.URL && facebookUser.Picture.Data.URL != "" {
				user.ProfilePicture = facebookUser.Picture.Data.URL
				updated = true
			}
			if user.Locale != facebookUser.Locale && facebookUser.Locale != "" {
				user.Locale = facebookUser.Locale
				updated = true
			}
			if updated {
				if err := s.UserRepo.UpdateUser(user); err != nil {
					// Log error but don't fail authentication
					log.Printf("Failed to update user profile: %v", err)
				}
			}
		}

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
		// Update user profile with Facebook data if not already set
		if user.Name == "" && facebookUser.Name != "" {
			user.Name = facebookUser.Name
		}
		if user.FirstName == "" && facebookUser.FirstName != "" {
			user.FirstName = facebookUser.FirstName
		}
		if user.LastName == "" && facebookUser.LastName != "" {
			user.LastName = facebookUser.LastName
		}
		if user.ProfilePicture == "" && facebookUser.Picture.Data.URL != "" {
			user.ProfilePicture = facebookUser.Picture.Data.URL
		}
		if user.Locale == "" && facebookUser.Locale != "" {
			user.Locale = facebookUser.Locale
		}
		if err := s.UserRepo.UpdateUser(user); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update user profile")
		}

		rawDataJSON, _ := json.Marshal(facebookUser)
		socialAccount := &models.SocialAccount{
			UserID:         user.ID,
			Provider:       "facebook",
			ProviderUserID: facebookUser.ID,
			Email:          facebookUser.Email,
			Name:           facebookUser.Name,
			FirstName:      facebookUser.FirstName,
			LastName:       facebookUser.LastName,
			ProfilePicture: facebookUser.Picture.Data.URL,
			Locale:         facebookUser.Locale,
			RawData:        rawDataJSON,
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
		Email:          facebookUser.Email,
		EmailVerified:  true, // Assuming email from Facebook is verified
		Name:           facebookUser.Name,
		FirstName:      facebookUser.FirstName,
		LastName:       facebookUser.LastName,
		ProfilePicture: facebookUser.Picture.Data.URL,
		Locale:         facebookUser.Locale,
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	rawDataJSON, _ := json.Marshal(facebookUser)
	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "facebook",
		ProviderUserID: facebookUser.ID,
		Email:          facebookUser.Email,
		Name:           facebookUser.Name,
		FirstName:      facebookUser.FirstName,
		LastName:       facebookUser.LastName,
		ProfilePicture: facebookUser.Picture.Data.URL,
		Locale:         facebookUser.Locale,
		RawData:        rawDataJSON,
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
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Bio       string `json:"bio"`
		Location  string `json:"location"`
		Company   string `json:"company"`
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
		// Update social account with latest data from provider
		rawDataJSON, _ := json.Marshal(githubUser)
		socialAccount.Email = githubUser.Email
		socialAccount.Name = githubUser.Name
		socialAccount.ProfilePicture = githubUser.AvatarURL
		socialAccount.Username = githubUser.Login
		socialAccount.RawData = rawDataJSON
		socialAccount.AccessToken = githubAccessToken

		if err := s.SocialRepo.UpdateSocialAccount(socialAccount); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		user, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			updated := false
			if user.Name != githubUser.Name && githubUser.Name != "" {
				user.Name = githubUser.Name
				updated = true
			}
			if user.ProfilePicture != githubUser.AvatarURL && githubUser.AvatarURL != "" {
				user.ProfilePicture = githubUser.AvatarURL
				updated = true
			}
			if updated {
				if err := s.UserRepo.UpdateUser(user); err != nil {
					// Log error but don't fail authentication
					log.Printf("Failed to update user profile: %v", err)
				}
			}
		}

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
		// Update user profile with GitHub data if not already set
		if user.Name == "" && githubUser.Name != "" {
			user.Name = githubUser.Name
		}
		if user.ProfilePicture == "" && githubUser.AvatarURL != "" {
			user.ProfilePicture = githubUser.AvatarURL
		}
		if err := s.UserRepo.UpdateUser(user); err != nil {
			return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update user profile")
		}

		rawDataJSON, _ := json.Marshal(githubUser)
		socialAccount := &models.SocialAccount{
			UserID:         user.ID,
			Provider:       "github",
			ProviderUserID: strconv.FormatInt(githubUser.ID, 10),
			Email:          githubUser.Email,
			Name:           githubUser.Name,
			ProfilePicture: githubUser.AvatarURL,
			Username:       githubUser.Login,
			RawData:        rawDataJSON,
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
		Email:          githubUser.Email,
		EmailVerified:  true, // Assuming email from GitHub is verified if primary and verified
		Name:           githubUser.Name,
		ProfilePicture: githubUser.AvatarURL,
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return "", "", uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	rawDataJSON, _ := json.Marshal(githubUser)
	newSocialAccount := &models.SocialAccount{
		UserID:         newUser.ID,
		Provider:       "github",
		ProviderUserID: strconv.FormatInt(githubUser.ID, 10),
		Email:          githubUser.Email,
		Name:           githubUser.Name,
		ProfilePicture: githubUser.AvatarURL,
		Username:       githubUser.Login,
		RawData:        rawDataJSON,
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
