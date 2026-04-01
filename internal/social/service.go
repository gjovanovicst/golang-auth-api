package social

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/session"
	"github.com/gjovanovicst/auth_api/internal/user"
	"github.com/gjovanovicst/auth_api/internal/webhook"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MergeTokenTTL is the duration a pending merge token is valid.
const MergeTokenTTL = 15 * time.Minute

// SocialLoginResult is the outcome of a social-login callback.
// When RequiresMerge is true, the callback found an existing account with the
// same email but no social link yet — the frontend should redirect the user to a
// merge confirmation screen rather than issuing tokens immediately.
type SocialLoginResult struct {
	UserID        uuid.UUID // set when RequiresMerge == false
	RequiresMerge bool
	MergeToken    string // set when RequiresMerge == true; store in Redis
	MergeEmail    string // the email of the existing account (for display)
}

// mergeTokenPayload is the JSON body stored under a merge token in Redis.
type mergeTokenPayload struct {
	UserID         string          `json:"user_id"`
	Provider       string          `json:"provider"`
	ProviderUserID string          `json:"provider_user_id"`
	Email          string          `json:"email"`
	Name           string          `json:"name,omitempty"`
	FirstName      string          `json:"first_name,omitempty"`
	LastName       string          `json:"last_name,omitempty"`
	ProfilePicture string          `json:"profile_picture,omitempty"`
	Username       string          `json:"username,omitempty"`
	Locale         string          `json:"locale,omitempty"`
	RawData        json.RawMessage `json:"raw_data,omitempty"`
	AccessToken    string          `json:"access_token"` // #nosec G101 -- provider OAuth access token, not a credential
}

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
// Per-app token TTL overrides are resolved via ResolveTokenTTLs.
func (s *Service) CreateSessionOrTokens(appID, userID, ip, userAgent string) (accessToken, refreshToken string, appErr *errors.AppError) {
	roles := s.getUserRoles(appID, userID)

	// Load per-app token TTL overrides
	var app models.Application
	var appPtr *models.Application
	if s.SocialRepo.DB.Select("access_token_ttl_minutes, refresh_token_ttl_hours").First(&app, "id = ?", appID).Error == nil {
		appPtr = &app
	}
	accessTTL, refreshTTL := user.ResolveTokenTTLs(appPtr)

	if s.SessionService != nil {
		at, rt, _, sErr := s.SessionService.CreateSession(appID, userID, ip, userAgent, roles, accessTTL, refreshTTL)
		if sErr != nil {
			return "", "", sErr
		}
		return at, rt, nil
	}

	// Legacy fallback
	var err error
	accessToken, err = jwt.GenerateAccessToken(appID, userID, "", roles, accessTTL)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}
	refreshToken, err = jwt.GenerateRefreshToken(appID, userID, "", roles, refreshTTL)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}
	if err := redis.SetRefreshToken(appID, userID, refreshToken); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}
	return accessToken, refreshToken, nil
}

func (s *Service) HandleGoogleCallback(appID uuid.UUID, googleAccessToken string) (*SocialLoginResult, *errors.AppError) {
	// Fetch user info from Google
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
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		foundUser, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			// Check if account is active
			if !foundUser.IsActive {
				return nil, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
			}

			updated := false
			if foundUser.Name != googleUser.Name && googleUser.Name != "" {
				foundUser.Name = googleUser.Name
				updated = true
			}
			if foundUser.FirstName != googleUser.GivenName && googleUser.GivenName != "" {
				foundUser.FirstName = googleUser.GivenName
				updated = true
			}
			if foundUser.LastName != googleUser.FamilyName && googleUser.FamilyName != "" {
				foundUser.LastName = googleUser.FamilyName
				updated = true
			}
			if foundUser.ProfilePicture != googleUser.Picture && googleUser.Picture != "" {
				foundUser.ProfilePicture = googleUser.Picture
				updated = true
			}
			if foundUser.Locale != googleUser.Locale && googleUser.Locale != "" {
				foundUser.Locale = googleUser.Locale
				updated = true
			}
			// Sync email verification status from Google on every login
			if foundUser.EmailVerified != googleUser.VerifiedEmail {
				foundUser.EmailVerified = googleUser.VerifiedEmail
				updated = true
			}
			if updated {
				if err := s.UserRepo.UpdateUser(foundUser); err != nil {
					// Log error but don't fail authentication
					log.Printf("Failed to update user profile: %v", err)
				}
			}
		}

		return &SocialLoginResult{UserID: socialAccount.UserID}, nil
	}

	// Social account not found — check if a user with this email already exists.
	// If yes, we must not silently merge: issue a merge token so the frontend can
	// prompt the user to confirm ownership before linking the social account.
	existingUser, err := s.UserRepo.GetUserByEmail(appID.String(), googleUser.Email)
	if err == nil {
		if !existingUser.IsActive {
			return nil, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
		}
		rawDataJSON, _ := json.Marshal(googleUser)
		mergeToken, mergeErr := s.createMergeToken(appID.String(), existingUser.ID.String(), "google", googleUser.ID, googleUser.Email, googleUser.Name, googleUser.GivenName, googleUser.FamilyName, googleUser.Picture, "", googleUser.Locale, rawDataJSON, googleAccessToken)
		if mergeErr != nil {
			return nil, mergeErr
		}
		return &SocialLoginResult{
			RequiresMerge: true,
			MergeToken:    mergeToken,
			MergeEmail:    googleUser.Email,
		}, nil
	}

	// No existing user or social account — create new user and social account.
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
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
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
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	return &SocialLoginResult{UserID: newUser.ID}, nil
}

func (s *Service) HandleFacebookCallback(appID uuid.UUID, facebookAccessToken string) (*SocialLoginResult, *errors.AppError) {
	// Fetch user info from Facebook Graph API with extended fields
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
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		foundUser, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			// Check if account is active
			if !foundUser.IsActive {
				return nil, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
			}

			updated := false
			if foundUser.Name != facebookUser.Name && facebookUser.Name != "" {
				foundUser.Name = facebookUser.Name
				updated = true
			}
			if foundUser.FirstName != facebookUser.FirstName && facebookUser.FirstName != "" {
				foundUser.FirstName = facebookUser.FirstName
				updated = true
			}
			if foundUser.LastName != facebookUser.LastName && facebookUser.LastName != "" {
				foundUser.LastName = facebookUser.LastName
				updated = true
			}
			if foundUser.ProfilePicture != facebookUser.Picture.Data.URL && facebookUser.Picture.Data.URL != "" {
				foundUser.ProfilePicture = facebookUser.Picture.Data.URL
				updated = true
			}
			if foundUser.Locale != facebookUser.Locale && facebookUser.Locale != "" {
				foundUser.Locale = facebookUser.Locale
				updated = true
			}
			// Facebook-sourced emails are always considered verified
			if !foundUser.EmailVerified {
				foundUser.EmailVerified = true
				updated = true
			}
			if updated {
				if err := s.UserRepo.UpdateUser(foundUser); err != nil {
					// Log error but don't fail authentication
					log.Printf("Failed to update user profile: %v", err)
				}
			}
		}

		return &SocialLoginResult{UserID: socialAccount.UserID}, nil
	}

	// Social account not found — check if a user with this email already exists.
	// If yes, issue a merge token instead of silently auto-linking.
	existingUser, err := s.UserRepo.GetUserByEmail(appID.String(), facebookUser.Email)
	if err == nil {
		if !existingUser.IsActive {
			return nil, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
		}
		rawDataJSON, _ := json.Marshal(facebookUser)
		mergeToken, mergeErr := s.createMergeToken(appID.String(), existingUser.ID.String(), "facebook", facebookUser.ID, facebookUser.Email, facebookUser.Name, facebookUser.FirstName, facebookUser.LastName, facebookUser.Picture.Data.URL, "", facebookUser.Locale, rawDataJSON, facebookAccessToken)
		if mergeErr != nil {
			return nil, mergeErr
		}
		return &SocialLoginResult{
			RequiresMerge: true,
			MergeToken:    mergeToken,
			MergeEmail:    facebookUser.Email,
		}, nil
	}

	// No existing user or social account — create new user and social account.
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
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
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
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	return &SocialLoginResult{UserID: newUser.ID}, nil
}

func (s *Service) HandleGithubCallback(appID uuid.UUID, githubAccessToken string) (*SocialLoginResult, *errors.AppError) {
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
		Bio       string `json:"bio"`
		Location  string `json:"location"`
		Company   string `json:"company"`
	}
	if err := json.Unmarshal(userData, &githubUser); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to parse GitHub user info")
	}

	// GitHub's user endpoint might not always return email if it's private. Fetch public emails separately.
	if githubUser.Email == "" {
		req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		if err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to create GitHub emails request")
		}
		req.Header.Set("Authorization", "token "+githubAccessToken)
		// #nosec G107,G704 -- This is a legitimate GitHub API call with a hardcoded trusted URL
		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to get user emails from GitHub")
		}
		defer resp.Body.Close()

		emailData, err := io.ReadAll(resp.Body)
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

	if githubUser.Email == "" {
		// Handle case where no public or primary verified email is available
		return nil, errors.NewAppError(errors.ErrBadRequest, "No public or primary verified email found for GitHub account. Please ensure your primary email is public and verified on GitHub.")
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
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to update social account")
		}

		// Also update user profile with latest data
		foundUser, err := s.UserRepo.GetUserByID(socialAccount.UserID.String())
		if err == nil {
			// Check if account is active
			if !foundUser.IsActive {
				return nil, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
			}

			updated := false
			if foundUser.Name != githubUser.Name && githubUser.Name != "" {
				foundUser.Name = githubUser.Name
				updated = true
			}
			if foundUser.ProfilePicture != githubUser.AvatarURL && githubUser.AvatarURL != "" {
				foundUser.ProfilePicture = githubUser.AvatarURL
				updated = true
			}
			// GitHub emails are pre-filtered to primary+verified, so always heal if unverified
			if !foundUser.EmailVerified {
				foundUser.EmailVerified = true
				updated = true
			}
			if updated {
				if err := s.UserRepo.UpdateUser(foundUser); err != nil {
					// Log error but don't fail authentication
					log.Printf("Failed to update user profile: %v", err)
				}
			}
		}

		return &SocialLoginResult{UserID: socialAccount.UserID}, nil
	}

	// Social account not found — check if a user with this email already exists.
	// If yes, issue a merge token instead of silently auto-linking.
	existingUser, err := s.UserRepo.GetUserByEmail(appID.String(), githubUser.Email)
	if err == nil {
		if !existingUser.IsActive {
			return nil, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
		}
		rawDataJSON, _ := json.Marshal(githubUser)
		mergeToken, mergeErr := s.createMergeToken(appID.String(), existingUser.ID.String(), "github", strconv.FormatInt(githubUser.ID, 10), githubUser.Email, githubUser.Name, "", "", githubUser.AvatarURL, githubUser.Login, "", rawDataJSON, githubAccessToken)
		if mergeErr != nil {
			return nil, mergeErr
		}
		return &SocialLoginResult{
			RequiresMerge: true,
			MergeToken:    mergeToken,
			MergeEmail:    githubUser.Email,
		}, nil
	}

	// No existing user or social account — create new user and social account.
	newUser := &models.User{
		AppID:          appID,
		Email:          githubUser.Email,
		EmailVerified:  true, // Assuming email from GitHub is verified if primary and verified
		Name:           githubUser.Name,
		ProfilePicture: githubUser.AvatarURL,
	}
	if err := s.UserRepo.CreateUser(newUser); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to create new user")
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
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to create social account")
	}

	return &SocialLoginResult{UserID: newUser.ID}, nil
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

// createMergeToken generates a UUID merge token, marshals the social-account
// details into JSON, stores them in Redis with a TTL, and returns the token.
func (s *Service) createMergeToken(appID, userID, provider, providerUserID, email, name, firstName, lastName, picture, username, locale string, rawData json.RawMessage, accessToken string) (string, *errors.AppError) {
	payload := mergeTokenPayload{
		UserID:         userID,
		Provider:       provider,
		ProviderUserID: providerUserID,
		Email:          email,
		Name:           name,
		FirstName:      firstName,
		LastName:       lastName,
		ProfilePicture: picture,
		Username:       username,
		Locale:         locale,
		RawData:        rawData,
		AccessToken:    accessToken,
	}
	payloadJSON, err := json.Marshal(payload) // #nosec G117 -- access_token is a provider OAuth token stored transiently in Redis for the merge flow, not logged or exposed to clients
	if err != nil {
		return "", errors.NewAppError(errors.ErrInternal, "Failed to encode merge token payload")
	}
	token := uuid.New().String()
	if err := redis.SetMergeToken(appID, token, string(payloadJSON), MergeTokenTTL); err != nil {
		return "", errors.NewAppError(errors.ErrInternal, "Failed to store merge token")
	}
	return token, nil
}

// ConfirmMerge validates the merge token, verifies the user's existing password,
// links the social account, and returns a new session token pair.
func (s *Service) ConfirmMerge(appID uuid.UUID, mergeToken, password, ip, userAgent string) (accessToken, refreshToken string, appErr *errors.AppError) {
	// 1. Retrieve the pending merge payload from Redis
	payloadStr, err := redis.GetMergeToken(appID.String(), mergeToken)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired merge token")
	}

	var payload mergeTokenPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to decode merge token payload")
	}

	// 2. Load the existing user and verify the password
	existingUser, err := s.UserRepo.GetUserByID(payload.UserID)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrNotFound, "User not found")
	}
	if !existingUser.IsActive {
		return "", "", errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
	}
	if existingUser.PasswordHash == "" {
		return "", "", errors.NewAppError(errors.ErrBadRequest, "This account has no password set. Please use a social login provider.")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(existingUser.PasswordHash), []byte(password)); err != nil {
		return "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid password")
	}

	// 3. Create the social account link
	parsedAppID := appID
	parsedUserID := existingUser.ID
	newSocialAccount := &models.SocialAccount{
		AppID:          parsedAppID,
		UserID:         parsedUserID,
		Provider:       payload.Provider,
		ProviderUserID: payload.ProviderUserID,
		Email:          payload.Email,
		Name:           payload.Name,
		FirstName:      payload.FirstName,
		LastName:       payload.LastName,
		ProfilePicture: payload.ProfilePicture,
		Username:       payload.Username,
		Locale:         payload.Locale,
		RawData:        datatypes.JSON(payload.RawData),
		AccessToken:    payload.AccessToken,
	}
	if err := s.SocialRepo.CreateSocialAccount(newSocialAccount); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to link social account")
	}

	// 4. Consume the merge token (best-effort; failure is non-fatal)
	_ = redis.DeleteMergeToken(appID.String(), mergeToken)

	// 5. Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "social.linked", map[string]interface{}{
			"user_id":  payload.UserID,
			"provider": payload.Provider,
		})
	}

	// 6. Issue session tokens
	at, rt, sessionErr := s.CreateSessionOrTokens(appID.String(), payload.UserID, ip, userAgent)
	if sessionErr != nil {
		return "", "", sessionErr
	}
	return at, rt, nil
}

// IsAppTwoFAEnabled reports whether 2FA is enabled at the application level.
// Fail-open: if the DB query fails, returns true to preserve existing behaviour.
func (s *Service) IsAppTwoFAEnabled(appID uuid.UUID) bool {
	var app models.Application
	if err := s.SocialRepo.DB.Select("two_fa_enabled").First(&app, "id = ?", appID).Error; err != nil {
		return true // fail open
	}
	return app.TwoFAEnabled
}
