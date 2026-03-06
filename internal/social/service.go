package social

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/session"
	"github.com/gjovanovicst/auth_api/internal/user"
	"github.com/gjovanovicst/auth_api/internal/webhook"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	UserRepo          *user.Repository
	SocialRepo        *Repository
	SessionService    *session.Service           // Session management for creating sessions on social login
	LookupRoles       user.RoleLookupFunc        // Optional: if nil, tokens are generated without roles
	AssignDefaultRole user.AssignDefaultRoleFunc // Optional: if nil, no default role on social signup
	WebhookService    *webhook.Service           // Optional: if nil, webhook dispatch is skipped
}

func NewService(ur *user.Repository, sr *Repository) *Service {
	return &Service{UserRepo: ur, SocialRepo: sr}
}

// getUserRoles fetches roles for JWT embedding. Returns nil on error (non-fatal).
// Self-healing: if the user has no roles and AssignDefaultRole is available,
// assigns the "member" role automatically (covers pre-RBAC users).
func (s *Service) getUserRoles(appID, userID string) []string {
	if s.LookupRoles == nil {
		return nil
	}
	roles, err := s.LookupRoles(appID, userID)
	if err != nil {
		log.Printf("Warning: failed to lookup roles for user %s in app %s: %v", userID, appID, err)
		return nil
	}

	// Self-healing: assign default role if user has none (pre-RBAC users)
	if len(roles) == 0 && s.AssignDefaultRole != nil {
		if err := s.AssignDefaultRole(appID, userID); err != nil {
			log.Printf("Warning: self-healing role assignment failed for user %s in app %s: %v", userID, appID, err)
			return nil
		}
		// Re-fetch roles after assignment
		roles, err = s.LookupRoles(appID, userID)
		if err != nil {
			log.Printf("Warning: failed to re-lookup roles after self-healing for user %s in app %s: %v", userID, appID, err)
			return nil
		}
		log.Printf("Info: self-healing assigned default role to user %s in app %s, roles: %v", userID, appID, roles)
	}

	return roles
}

// assignDefaultRole assigns the default role to a newly created social user (non-fatal).
func (s *Service) assignDefaultRole(appID, userID string) {
	if s.AssignDefaultRole == nil {
		return
	}
	if err := s.AssignDefaultRole(appID, userID); err != nil {
		log.Printf("Warning: failed to assign default role to social user %s in app %s: %v", userID, appID, err)
	}
}

// CreateSessionOrTokens creates a session via the session service if available,
// otherwise falls back to legacy token generation.
func (s *Service) CreateSessionOrTokens(appID, userID, ip, userAgent string) (accessToken, refreshToken string, appErr *errors.AppError) {
	roles := s.getUserRoles(appID, userID)

	if s.SessionService != nil {
		at, rt, _, sErr := s.SessionService.CreateSession(appID, userID, ip, userAgent, roles)
		if sErr != nil {
			return "", "", sErr
		}
		return at, rt, nil
	}

	// Legacy fallback
	var err error
	accessToken, err = jwt.GenerateAccessToken(appID, userID, "", roles)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}
	refreshToken, err = jwt.GenerateRefreshToken(appID, userID, "", roles)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}
	if err := redis.SetRefreshToken(appID, userID, refreshToken); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}
	return accessToken, refreshToken, nil
}

func (s *Service) HandleGoogleCallback(appID uuid.UUID, googleAccessToken string) (uuid.UUID, *errors.AppError) {
	// Fetch user info from Google
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + googleAccessToken)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user info from Google")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read Google user info response")
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
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse Google user info")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID(appID.String(), "google", googleUser.ID)
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
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		user, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			// Check if account is active
			if !user.IsActive {
				return uuid.UUID{}, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
			}

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

		return socialAccount.UserID, nil
	}

	// If social account not found, check if user with this email exists
	user, err := s.UserRepo.GetUserByEmail(appID.String(), googleUser.Email)
	if err == nil { // User with this email exists, link social account
		// Check if account is active
		if !user.IsActive {
			return uuid.UUID{}, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
		}

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
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update user profile")
		}

		rawDataJSON, _ := json.Marshal(googleUser)
		socialAccount := &models.SocialAccount{
			AppID:          appID,
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
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to link social account")
		}
		return user.ID, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		AppID:          appID,
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
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	// Assign default 'member' role to new social user
	s.assignDefaultRole(appID.String(), newUser.ID.String())

	rawDataJSON, _ := json.Marshal(googleUser)
	newSocialAccount := &models.SocialAccount{
		AppID:          appID,
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
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	return newUser.ID, nil
}

func (s *Service) HandleFacebookCallback(appID uuid.UUID, facebookAccessToken string) (uuid.UUID, *errors.AppError) {
	// Fetch user info from Facebook Graph API with extended fields
	resp, err := http.Get("https://graph.facebook.com/v18.0/me?fields=id,name,email,first_name,last_name,picture.type(large),locale&access_token=" + facebookAccessToken)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user info from Facebook")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read Facebook user info response")
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
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse Facebook user info")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID(appID.String(), "facebook", facebookUser.ID)
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
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		user, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			// Check if account is active
			if !user.IsActive {
				return uuid.UUID{}, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
			}

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

		return socialAccount.UserID, nil
	}

	// If social account not found, check if user with this email exists
	user, err := s.UserRepo.GetUserByEmail(appID.String(), facebookUser.Email)
	if err == nil { // User with this email exists, link social account
		// Check if account is active
		if !user.IsActive {
			return uuid.UUID{}, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
		}

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
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update user profile")
		}

		rawDataJSON, _ := json.Marshal(facebookUser)
		socialAccount := &models.SocialAccount{
			AppID:          appID,
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
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to link social account")
		}
		return user.ID, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		AppID:          appID,
		Email:          facebookUser.Email,
		EmailVerified:  true, // Assuming email from Facebook is verified
		Name:           facebookUser.Name,
		FirstName:      facebookUser.FirstName,
		LastName:       facebookUser.LastName,
		ProfilePicture: facebookUser.Picture.Data.URL,
		Locale:         facebookUser.Locale,
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	// Assign default 'member' role to new social user
	s.assignDefaultRole(appID.String(), newUser.ID.String())

	rawDataJSON, _ := json.Marshal(facebookUser)
	newSocialAccount := &models.SocialAccount{
		AppID:          appID,
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
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	return newUser.ID, nil
}

func (s *Service) HandleGithubCallback(appID uuid.UUID, githubAccessToken string) (uuid.UUID, *errors.AppError) {
	// Fetch user info from GitHub API
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create GitHub request")
	}
	req.Header.Set("Authorization", "token "+githubAccessToken)
	// #nosec G107,G704 -- This is a legitimate GitHub API call with a hardcoded trusted URL
	resp, err := client.Do(req)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user info from GitHub")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read GitHub user info response")
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
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse GitHub user info")
	}

	// GitHub's user endpoint might not always return email if it's private. Fetch public emails separately.
	if githubUser.Email == "" {
		req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		if err != nil {
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create GitHub emails request")
		}
		req.Header.Set("Authorization", "token "+githubAccessToken)
		// #nosec G107,G704 -- This is a legitimate GitHub API call with a hardcoded trusted URL
		resp, err := client.Do(req)
		if err != nil {
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to get user emails from GitHub")
		}
		defer resp.Body.Close()

		emailData, err := io.ReadAll(resp.Body)
		if err != nil {
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to read GitHub emails response")
		}

		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := json.Unmarshal(emailData, &emails); err != nil {
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to parse GitHub emails")
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
		return uuid.UUID{}, errors.NewAppError(errors.ErrBadRequest, "No public or primary verified email found for GitHub account. Please ensure your primary email is public and verified on GitHub.")
	}

	// Check if social account already exists
	socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID(appID.String(), "github", strconv.FormatInt(githubUser.ID, 10))
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
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		user, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			// Check if account is active
			if !user.IsActive {
				return uuid.UUID{}, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
			}

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

		return socialAccount.UserID, nil
	}

	// If social account not found, check if user with this email exists
	user, err := s.UserRepo.GetUserByEmail(appID.String(), githubUser.Email)
	if err == nil { // User with this email exists, link social account
		// Check if account is active
		if !user.IsActive {
			return uuid.UUID{}, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
		}

		// Update user profile with GitHub data if not already set
		if user.Name == "" && githubUser.Name != "" {
			user.Name = githubUser.Name
		}
		if user.ProfilePicture == "" && githubUser.AvatarURL != "" {
			user.ProfilePicture = githubUser.AvatarURL
		}
		if err := s.UserRepo.UpdateUser(user); err != nil {
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update user profile")
		}

		rawDataJSON, _ := json.Marshal(githubUser)
		socialAccount := &models.SocialAccount{
			AppID:          appID,
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
			return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to link social account")
		}
		return user.ID, nil
	}

	// No existing user or social account, create new user and social account
	newUser := &models.User{
		AppID:          appID,
		Email:          githubUser.Email,
		EmailVerified:  true, // Assuming email from GitHub is verified if primary and verified
		Name:           githubUser.Name,
		ProfilePicture: githubUser.AvatarURL,
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
	}

	// Assign default 'member' role to new social user
	s.assignDefaultRole(appID.String(), newUser.ID.String())

	rawDataJSON, _ := json.Marshal(githubUser)
	newSocialAccount := &models.SocialAccount{
		AppID:          appID,
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
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	return newUser.ID, nil
}

// GetLinkedAccounts returns all social accounts linked to a user
func (s *Service) GetLinkedAccounts(userID string) ([]models.SocialAccount, *errors.AppError) {
	accounts, err := s.SocialRepo.GetSocialAccountsByUserID(userID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to retrieve linked social accounts")
	}
	return accounts, nil
}

// UnlinkSocialAccount removes a social account link from a user's profile.
// Prevents unlinking the last auth method (no password + only 1 social account).
func (s *Service) UnlinkSocialAccount(appID, userID, socialAccountID string) *errors.AppError {
	// Fetch the social account
	socialAccount, err := s.SocialRepo.GetSocialAccountByID(socialAccountID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.NewAppError(errors.ErrNotFound, "Social account not found")
		}
		return errors.NewAppError(errors.ErrInternal, "Failed to retrieve social account")
	}

	// Verify ownership: the social account must belong to the requesting user and app
	if socialAccount.UserID.String() != userID || socialAccount.AppID.String() != appID {
		return errors.NewAppError(errors.ErrNotFound, "Social account not found")
	}

	// Lockout prevention: check if user has a password or other social accounts
	foundUser, err := s.UserRepo.GetUserByID(userID)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to retrieve user")
	}

	hasPassword := foundUser.PasswordHash != ""

	if !hasPassword {
		// Count social accounts — if this is the only one, prevent unlinking
		count, err := s.SocialRepo.CountSocialAccountsByUserID(userID)
		if err != nil {
			return errors.NewAppError(errors.ErrInternal, "Failed to count linked accounts")
		}
		if count <= 1 {
			return errors.NewAppError(errors.ErrBadRequest, "Cannot unlink the only social account. Please set a password first to maintain account access.")
		}
	}

	// Delete the social account
	if err := s.SocialRepo.DeleteSocialAccount(socialAccountID); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to unlink social account")
	}

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		parsedAppID, _ := uuid.Parse(appID)
		s.WebhookService.Dispatch(parsedAppID, "social.unlinked", map[string]interface{}{
			"user_id":           userID,
			"social_account_id": socialAccountID,
			"provider":          socialAccount.Provider,
		})
	}

	return nil
}

// HandleGoogleLinkCallback links a Google account to an existing authenticated user
func (s *Service) HandleGoogleLinkCallback(appID uuid.UUID, userID string, googleAccessToken string) (*models.SocialAccount, *errors.AppError) {
	// Fetch user info from Google
	// #nosec G107 -- URL is constructed from a trusted base with a user-provided token parameter
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + googleAccessToken)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to get user info from Google")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to read Google user info response")
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
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to parse Google user info")
	}

	// Check if this Google account is already linked to ANY user in this app
	existingAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID(appID.String(), "google", googleUser.ID)
	if err == nil {
		if existingAccount.UserID.String() == userID {
			return nil, errors.NewAppError(errors.ErrConflict, "This Google account is already linked to your profile")
		}
		return nil, errors.NewAppError(errors.ErrConflict, "This Google account is already linked to another user")
	}

	// Create the social account link
	rawDataJSON, _ := json.Marshal(googleUser)
	parsedUserID, _ := uuid.Parse(userID)
	newLinkAccount := &models.SocialAccount{
		AppID:          appID,
		UserID:         parsedUserID,
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
	}
	if err := s.SocialRepo.CreateSocialAccount(newLinkAccount); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to link Google account")
	}

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "social.linked", map[string]interface{}{
			"user_id":  userID,
			"provider": "google",
		})
	}

	return newLinkAccount, nil
}

// HandleFacebookLinkCallback links a Facebook account to an existing authenticated user
func (s *Service) HandleFacebookLinkCallback(appID uuid.UUID, userID string, facebookAccessToken string) (*models.SocialAccount, *errors.AppError) {
	// Fetch user info from Facebook Graph API
	// #nosec G107 -- URL is constructed from a trusted base with a user-provided token parameter
	resp, err := http.Get("https://graph.facebook.com/v18.0/me?fields=id,name,email,first_name,last_name,picture.type(large),locale&access_token=" + facebookAccessToken)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to get user info from Facebook")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to read Facebook user info response")
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
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to parse Facebook user info")
	}

	// Check if this Facebook account is already linked to ANY user in this app
	existingAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID(appID.String(), "facebook", facebookUser.ID)
	if err == nil {
		if existingAccount.UserID.String() == userID {
			return nil, errors.NewAppError(errors.ErrConflict, "This Facebook account is already linked to your profile")
		}
		return nil, errors.NewAppError(errors.ErrConflict, "This Facebook account is already linked to another user")
	}

	// Create the social account link
	rawDataJSON, _ := json.Marshal(facebookUser)
	parsedUserID, _ := uuid.Parse(userID)
	newLinkAccount := &models.SocialAccount{
		AppID:          appID,
		UserID:         parsedUserID,
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
	}
	if err := s.SocialRepo.CreateSocialAccount(newLinkAccount); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to link Facebook account")
	}

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "social.linked", map[string]interface{}{
			"user_id":  userID,
			"provider": "facebook",
		})
	}

	return newLinkAccount, nil
}

// HandleGithubLinkCallback links a GitHub account to an existing authenticated user
func (s *Service) HandleGithubLinkCallback(appID uuid.UUID, userID string, githubAccessToken string) (*models.SocialAccount, *errors.AppError) {
	// Fetch user info from GitHub API
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to create GitHub request")
	}
	req.Header.Set("Authorization", "token "+githubAccessToken)
	// #nosec G107,G704 -- This is a legitimate GitHub API call with a hardcoded trusted URL
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to get user info from GitHub")
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to read GitHub user info response")
	}

	var githubUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(userData, &githubUser); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to parse GitHub user info")
	}

	// GitHub's user endpoint might not always return email if it's private
	if githubUser.Email == "" {
		emailReq, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		if err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to create GitHub emails request")
		}
		emailReq.Header.Set("Authorization", "token "+githubAccessToken)
		// #nosec G107,G704 -- This is a legitimate GitHub API call with a hardcoded trusted URL
		emailResp, err := client.Do(emailReq)
		if err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to get user emails from GitHub")
		}
		defer emailResp.Body.Close()

		emailData, err := io.ReadAll(emailResp.Body)
		if err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to read GitHub emails response")
		}

		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := json.Unmarshal(emailData, &emails); err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to parse GitHub emails")
		}

		for _, email := range emails {
			if email.Primary && email.Verified {
				githubUser.Email = email.Email
				break
			}
		}
	}

	providerUserID := strconv.FormatInt(githubUser.ID, 10)

	// Check if this GitHub account is already linked to ANY user in this app
	existingAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID(appID.String(), "github", providerUserID)
	if err == nil {
		if existingAccount.UserID.String() == userID {
			return nil, errors.NewAppError(errors.ErrConflict, "This GitHub account is already linked to your profile")
		}
		return nil, errors.NewAppError(errors.ErrConflict, "This GitHub account is already linked to another user")
	}

	// Create the social account link
	rawDataJSON, _ := json.Marshal(githubUser)
	parsedUserID, _ := uuid.Parse(userID)
	newLinkAccount := &models.SocialAccount{
		AppID:          appID,
		UserID:         parsedUserID,
		Provider:       "github",
		ProviderUserID: providerUserID,
		Email:          githubUser.Email,
		Name:           githubUser.Name,
		ProfilePicture: githubUser.AvatarURL,
		Username:       githubUser.Login,
		RawData:        rawDataJSON,
		AccessToken:    githubAccessToken,
	}
	if err := s.SocialRepo.CreateSocialAccount(newLinkAccount); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to link GitHub account")
	}

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "social.linked", map[string]interface{}{
			"user_id":  userID,
			"provider": "github",
		})
	}

	return newLinkAccount, nil
}
