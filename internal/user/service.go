package user

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	emailpkg "github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// bcryptCost is the cost factor for password hashing. Set to 12 for stronger
// security (matches the admin account bcrypt cost in cmd/setup).
const bcryptCost = 12

type Service struct {
	Repo         *Repository
	EmailService *emailpkg.Service
	DB           *gorm.DB
}

func NewService(r *Repository, es *emailpkg.Service, db *gorm.DB) *Service {
	return &Service{Repo: r, EmailService: es, DB: db}
}

// LoginResult represents the result of a login attempt
type LoginResult struct {
	RequiresTwoFA      bool
	RequiresTwoFASetup bool
	UserID             uuid.UUID
	AccessToken        string // #nosec G101,G117 -- This is a result field, not a hardcoded credential
	RefreshToken       string // #nosec G101,G117 -- This is a result field, not a hardcoded credential
	TwoFAResponse      *dto.TwoFARequiredResponse
	TwoFASetupResponse *dto.TwoFASetupRequiredResponse
}

func (s *Service) RegisterUser(appID uuid.UUID, email, password string) (uuid.UUID, *errors.AppError) {
	// Check if user already exists
	_, err := s.Repo.GetUserByEmail(appID.String(), email)
	if err == nil { // User found, meaning email is already registered
		return uuid.UUID{}, errors.NewAppError(errors.ErrConflict, "Email already registered")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to hash password")
	}

	user := &models.User{
		AppID:         appID,
		Email:         email,
		PasswordHash:  string(hashedPassword),
		EmailVerified: false,
	}

	if err := s.Repo.CreateUser(user); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create user")
	}

	// Generate email verification token and send email
	verificationToken := uuid.New().String()

	if err := redis.SetEmailVerificationToken(appID.String(), user.ID.String(), verificationToken, 24*time.Hour); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store verification token")
	}

	if err := s.EmailService.SendVerificationEmail(appID, user.Email, verificationToken); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to send verification email")
	}

	return user.ID, nil
}

func (s *Service) LoginUser(appID uuid.UUID, email, password string) (*LoginResult, *errors.AppError) {
	user, err := s.Repo.GetUserByEmail(appID.String(), email)
	if err != nil { // User not found
		return nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid credentials")
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid credentials")
	}

	// Check if account is active
	if !user.IsActive {
		return nil, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
	}

	// Check if email is verified
	if !user.EmailVerified {
		return nil, errors.NewAppError(errors.ErrForbidden, "Email not verified. Please check your inbox.")
	}

	// Check if 2FA is enabled
	if user.TwoFAEnabled {
		// Generate temporary token for 2FA verification
		tempToken := uuid.New().String()
		if err := redis.SetTempUserSession(appID.String(), tempToken, user.ID.String(), 10*time.Minute); err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to create temporary session")
		}

		// Determine the user's 2FA method (default to TOTP for backward compatibility)
		twoFAMethod := user.TwoFAMethod
		if twoFAMethod == "" {
			twoFAMethod = emailpkg.TwoFAMethodTOTP
		}

		// If the user uses email 2FA, generate and send the code now
		if twoFAMethod == emailpkg.TwoFAMethodEmail && s.EmailService != nil {
			code := generateSecure6DigitCode()
			if storeErr := redis.Set2FAEmailCode(appID.String(), user.ID.String(), code); storeErr != nil {
				return nil, errors.NewAppError(errors.ErrInternal, "Failed to prepare 2FA verification")
			}
			if sendErr := s.EmailService.Send2FACodeEmail(appID, user.Email, code); sendErr != nil {
				log.Printf("Warning: Failed to send 2FA email code to %s: %v", user.Email, sendErr)
				return nil, errors.NewAppError(errors.ErrInternal, "Failed to send 2FA code email")
			}
		}

		return &LoginResult{
			RequiresTwoFA: true,
			UserID:        user.ID,
			TwoFAResponse: &dto.TwoFARequiredResponse{
				Message:   "2FA verification required",
				TempToken: tempToken,
				Method:    twoFAMethod,
			},
		}, nil
	}

	// Check if this application requires 2FA setup for all users
	var app models.Application
	if err := s.DB.Select("two_fa_required").First(&app, "id = ?", appID).Error; err == nil && app.TwoFARequired {
		// User doesn't have 2FA set up, but the app requires it.
		// Issue tokens so the user can authenticate to /2fa/generate, but flag the response.
		accessToken, err := jwt.GenerateAccessToken(appID.String(), user.ID.String())
		if err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
		}

		refreshToken, err := jwt.GenerateRefreshToken(appID.String(), user.ID.String())
		if err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
		}

		if err := redis.SetRefreshToken(appID.String(), user.ID.String(), refreshToken); err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
		}

		return &LoginResult{
			RequiresTwoFASetup: true,
			UserID:             user.ID,
			AccessToken:        accessToken,
			RefreshToken:       refreshToken,
			TwoFASetupResponse: &dto.TwoFASetupRequiredResponse{
				Message:      "2FA setup is required for this application",
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			},
		}, nil
	}

	// Generate JWTs for standard login
	accessToken, err := jwt.GenerateAccessToken(appID.String(), user.ID.String())
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}

	refreshToken, err := jwt.GenerateRefreshToken(appID.String(), user.ID.String())
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}

	// Store refresh token in Redis
	if err := redis.SetRefreshToken(appID.String(), user.ID.String(), refreshToken); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}

	return &LoginResult{
		RequiresTwoFA: false,
		UserID:        user.ID,
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
	}, nil
}

func (s *Service) RefreshUserToken(refreshToken string) (string, string, string, *errors.AppError) {
	claims, err := jwt.ParseToken(refreshToken)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid refresh token")
	}

	// Reject access tokens used as refresh tokens.
	// Empty TokenType is allowed for backward compatibility with pre-existing tokens.
	if claims.TokenType != "" && claims.TokenType != jwt.TokenTypeRefresh {
		return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid token type")
	}

	// Check if refresh token is blacklisted/revoked in Redis
	if revoked, err := redis.IsRefreshTokenRevoked(claims.AppID, claims.UserID, refreshToken); err != nil || revoked {
		return "", "", "", errors.NewAppError(errors.ErrUnauthorized, "Refresh token revoked or invalid")
	}

	// Generate new access and refresh tokens
	newAccessToken, err := jwt.GenerateAccessToken(claims.AppID, claims.UserID)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new access token")
	}
	newRefreshToken, err := jwt.GenerateRefreshToken(claims.AppID, claims.UserID)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new refresh token")
	}

	// Invalidate old refresh token and store new one in Redis
	if err := redis.RevokeRefreshToken(claims.AppID, claims.UserID, refreshToken); err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to revoke old refresh token")
	}
	if err := redis.SetRefreshToken(claims.AppID, claims.UserID, newRefreshToken); err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to store new refresh token")
	}

	return newAccessToken, newRefreshToken, claims.UserID, nil
}

// LogoutUser logs out a user by revoking their refresh token and blacklisting their access token
func (s *Service) LogoutUser(appID, userID, refreshToken, accessToken string) *errors.AppError {
	// Revoke the refresh token in Redis
	if err := redis.RevokeRefreshToken(appID, userID, refreshToken); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to revoke refresh token")
	}

	// Blacklist the access token to prevent further use
	// Parse the access token to get its expiration time
	claims, err := jwt.ParseToken(accessToken)
	if err == nil {
		// Calculate remaining TTL of access token for blacklist expiration
		remainingTime := time.Until(claims.ExpiresAt.Time)
		if remainingTime > 0 {
			// Only blacklist if token hasn't expired yet
			if err := redis.BlacklistAccessToken(appID, accessToken, userID, remainingTime); err != nil {
				// Log the error but don't fail logout completely
				log.Printf("Warning: Failed to blacklist access token: %v\n", err)
			}
		}
	} else {
		// Log the error but don't fail logout completely
		log.Printf("Warning: Failed to parse access token during logout: %v\n", err)
	}

	return nil
}

func (s *Service) RequestPasswordReset(appID uuid.UUID, email string) *errors.AppError {
	user, err := s.Repo.GetUserByEmail(appID.String(), email)
	if err != nil {
		// For security, always return a generic success message even if email not found
		return nil
	}

	resetToken := uuid.New().String()
	// Store token in Redis with expiration (e.g., 1 hour)
	if err := redis.SetPasswordResetToken(appID.String(), user.ID.String(), resetToken, time.Hour); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to generate reset token")
	}

	resetLink := fmt.Sprintf("http://your-frontend-app/reset-password?token=%s", resetToken)
	if err := s.EmailService.SendPasswordResetEmail(appID, user.Email, resetLink); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to send password reset email")
	}

	return nil
}

func (s *Service) VerifyEmail(appID uuid.UUID, token string) (uuid.UUID, *errors.AppError) {
	userID, err := redis.GetEmailVerificationToken(appID.String(), token)
	if err != nil || userID == "" {
		return uuid.UUID{}, errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired verification token")
	}

	// Update user's email_verified status in DB
	if err := s.Repo.UpdateUserEmailVerified(userID, true); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to verify email")
	}

	// Invalidate the token after use
	if err := redis.DeleteEmailVerificationToken(appID.String(), token); err != nil {
		log.Printf("Warning: Failed to delete used email verification token from Redis: %v\n", err)
	}

	// Parse UUID for return
	userUUID, parseErr := uuid.Parse(userID)
	if parseErr != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Invalid user ID format")
	}

	return userUUID, nil
}

func (s *Service) ConfirmPasswordReset(appID uuid.UUID, token, newPassword string) (uuid.UUID, *errors.AppError) {
	// Validate reset token from Redis
	userID, err := redis.GetPasswordResetToken(appID.String(), token)
	if err != nil || userID == "" {
		return uuid.UUID{}, errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired reset token")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to hash new password")
	}

	if err := s.Repo.UpdateUserPassword(userID, string(hashedPassword)); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to update password")
	}

	// Invalidate the token after use
	if err := redis.DeletePasswordResetToken(appID.String(), token); err != nil {
		log.Printf("Warning: Failed to delete used password reset token from Redis: %v\n", err)
	}

	// Security: Revoke all existing tokens for this user after password change
	if err := s.RevokeAllUserTokens(appID.String(), userID); err != nil {
		log.Printf("Warning: Failed to revoke all user tokens after password reset: %v\n", err.Message)
	}

	// Parse UUID for return
	userUUID, parseErr := uuid.Parse(userID)
	if parseErr != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Invalid user ID format")
	}

	return userUUID, nil
}

// RevokeAllUserTokens revokes all access and refresh tokens for a user
// This is used for security events like password changes, account compromise, etc.
func (s *Service) RevokeAllUserTokens(appID, userID string) *errors.AppError {
	// Revoke the current refresh token (if any)
	currentRefreshToken, err := redis.GetRefreshToken(appID, userID)
	if err == nil && currentRefreshToken != "" {
		if err := redis.RevokeRefreshToken(appID, userID, currentRefreshToken); err != nil {
			log.Printf("Warning: Failed to revoke refresh token for user %s: %v\n", userID, err)
		}
	}

	// Blacklist all tokens for this user for the maximum possible token lifetime
	// Use the longer of access token or refresh token expiration time
	maxTokenLifetime := time.Hour * time.Duration(24*30) // 30 days should cover most token lifetimes
	if err := redis.BlacklistAllUserTokens(appID, userID, maxTokenLifetime); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to blacklist user tokens")
	}

	return nil
}

// UpdateUserProfile updates the user profile information (name, first_name, last_name, profile_picture, locale)
func (s *Service) UpdateUserProfile(userID string, req dto.UpdateProfileRequest) *errors.AppError {
	// Build updates map with only provided fields
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.FirstName != "" {
		updates["first_name"] = req.FirstName
	}
	if req.LastName != "" {
		updates["last_name"] = req.LastName
	}
	if req.ProfilePicture != "" {
		updates["profile_picture"] = req.ProfilePicture
	}
	if req.Locale != "" {
		updates["locale"] = req.Locale
	}

	// If no fields to update, return early
	if len(updates) == 0 {
		return errors.NewAppError(errors.ErrBadRequest, "No fields provided for update")
	}

	// Update user profile in database
	if err := s.Repo.UpdateUserProfile(userID, updates); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to update profile")
	}

	return nil
}

// UpdateUserEmail updates the user's email address after verifying password
func (s *Service) UpdateUserEmail(appID uuid.UUID, userID string, req dto.UpdateEmailRequest) *errors.AppError {
	// Get current user to verify password
	user, err := s.Repo.GetUserByID(userID)
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid password")
	}

	// Check if new email is already in use
	existingUser, err := s.Repo.GetUserByEmail(appID.String(), req.Email)
	if err == nil && existingUser.ID != user.ID {
		return errors.NewAppError(errors.ErrConflict, "Email already in use")
	}

	// Update email and set email_verified to false
	if err := s.Repo.UpdateUserEmail(userID, req.Email); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to update email")
	}

	// Generate and send new email verification token
	verificationToken := uuid.New().String()
	if err := redis.SetEmailVerificationToken(appID.String(), userID, verificationToken, 24*time.Hour); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to generate verification token")
	}

	if err := s.EmailService.SendVerificationEmail(appID, req.Email, verificationToken); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to send verification email")
	}

	return nil
}

// UpdateUserPassword updates the user's password after verifying current password
func (s *Service) UpdateUserPassword(appID uuid.UUID, userID string, req dto.UpdatePasswordRequest) *errors.AppError {
	// Get current user to verify password
	user, err := s.Repo.GetUserByID(userID)
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid current password")
	}

	// Check new password is different from current
	if req.CurrentPassword == req.NewPassword {
		return errors.NewAppError(errors.ErrBadRequest, "New password must be different from current password")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcryptCost)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to hash new password")
	}

	// Update password
	if err := s.Repo.UpdateUserPassword(userID, string(hashedPassword)); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to update password")
	}

	// Revoke all existing tokens for security
	if err := s.RevokeAllUserTokens(appID.String(), userID); err != nil {
		log.Printf("Warning: Failed to revoke all user tokens after password change: %v\n", err.Message)
	}

	return nil
}

// DeleteUserAccount deletes the user account after verifying password
func (s *Service) DeleteUserAccount(appID uuid.UUID, userID string, req dto.DeleteAccountRequest) *errors.AppError {
	// Get current user to verify password
	user, err := s.Repo.GetUserByID(userID)
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	// Verify password (if user has password - social login users might not)
	if user.PasswordHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			return errors.NewAppError(errors.ErrUnauthorized, "Invalid password")
		}
	}

	// Verify confirmation flag
	if !req.ConfirmDeletion {
		return errors.NewAppError(errors.ErrBadRequest, "Account deletion must be confirmed")
	}

	// Revoke all tokens
	if err := s.RevokeAllUserTokens(appID.String(), userID); err != nil {
		log.Printf("Warning: Failed to revoke all user tokens before account deletion: %v\n", err.Message)
	}

	// Delete user from database (cascade will delete related records)
	if err := s.Repo.DeleteUser(userID); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to delete account")
	}

	return nil
}

// generateSecure6DigitCode generates a cryptographically secure 6-digit numeric code for email 2FA.
func generateSecure6DigitCode() string {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate secure random number for 2FA code: %v", err))
	}
	return fmt.Sprintf("%06d", n.Int64())
}
