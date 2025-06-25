package user

import (
	"fmt"
	"time"

	"github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	Repo         *Repository
	EmailService *email.Service
}

func NewService(r *Repository, es *email.Service) *Service {
	return &Service{Repo: r, EmailService: es}
}

// LoginResult represents the result of a login attempt
type LoginResult struct {
	RequiresTwoFA bool
	AccessToken   string
	RefreshToken  string
	TwoFAResponse *dto.TwoFARequiredResponse
}

func (s *Service) RegisterUser(email, password string) *errors.AppError {
	// Check if user already exists
	_, err := s.Repo.GetUserByEmail(email)
	if err == nil { // User found, meaning email is already registered
		return errors.NewAppError(errors.ErrConflict, "Email already registered")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to hash password")
	}

	user := &models.User{
		Email:         email,
		PasswordHash:  string(hashedPassword),
		EmailVerified: false,
	}

	if err := s.Repo.CreateUser(user); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to create user")
	}

	// Generate email verification token and send email
	verificationToken := uuid.New().String()
	fmt.Printf("DEBUG: Generated verification token: %s for user: %s\n", verificationToken, user.Email)

	if err := redis.SetEmailVerificationToken(user.ID.String(), verificationToken, 24*time.Hour); err != nil {
		fmt.Printf("DEBUG: Failed to store token in Redis: %v\n", err)
		return errors.NewAppError(errors.ErrInternal, "Failed to store verification token")
	}
	fmt.Printf("DEBUG: Token stored in Redis successfully\n")

	fmt.Printf("DEBUG: About to send verification email to: %s\n", user.Email)
	if err := s.EmailService.SendVerificationEmail(user.Email, verificationToken); err != nil {
		fmt.Printf("DEBUG: Email service returned error: %v\n", err)
		return errors.NewAppError(errors.ErrInternal, "Failed to send verification email")
	}
	fmt.Printf("DEBUG: Email service call completed successfully\n")

	return nil
}

func (s *Service) LoginUser(email, password string) (*LoginResult, *errors.AppError) {
	user, err := s.Repo.GetUserByEmail(email)
	if err != nil { // User not found
		return nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid credentials")
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid credentials")
	}

	// Check if email is verified
	if !user.EmailVerified {
		return nil, errors.NewAppError(errors.ErrForbidden, "Email not verified. Please check your inbox.")
	}

	// Check if 2FA is enabled
	if user.TwoFAEnabled {
		// Generate temporary token for 2FA verification
		tempToken := uuid.New().String()
		if err := redis.SetTempUserSession(tempToken, user.ID.String(), 10*time.Minute); err != nil {
			return nil, errors.NewAppError(errors.ErrInternal, "Failed to create temporary session")
		}

		return &LoginResult{
			RequiresTwoFA: true,
			TwoFAResponse: &dto.TwoFARequiredResponse{
				Message:   "2FA verification required",
				TempToken: tempToken,
			},
		}, nil
	}

	// Generate JWTs for standard login
	accessToken, err := jwt.GenerateAccessToken(user.ID.String())
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}

	refreshToken, err := jwt.GenerateRefreshToken(user.ID.String())
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}

	// Store refresh token in Redis
	if err := redis.SetRefreshToken(user.ID.String(), refreshToken); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}

	return &LoginResult{
		RequiresTwoFA: false,
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
	}, nil
}

func (s *Service) RefreshUserToken(refreshToken string) (string, string, *errors.AppError) {
	claims, err := jwt.ParseToken(refreshToken)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid refresh token")
	}

	// Check if refresh token is blacklisted/revoked in Redis
	if revoked, err := redis.IsRefreshTokenRevoked(claims.UserID, refreshToken); err != nil || revoked {
		return "", "", errors.NewAppError(errors.ErrUnauthorized, "Refresh token revoked or invalid")
	}

	// Generate new access and refresh tokens
	newAccessToken, err := jwt.GenerateAccessToken(claims.UserID)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new access token")
	}
	newRefreshToken, err := jwt.GenerateRefreshToken(claims.UserID)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new refresh token")
	}

	// Invalidate old refresh token and store new one in Redis
	if err := redis.RevokeRefreshToken(claims.UserID, refreshToken); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to revoke old refresh token")
	}
	if err := redis.SetRefreshToken(claims.UserID, newRefreshToken); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to store new refresh token")
	}

	return newAccessToken, newRefreshToken, nil
}

// LogoutUser logs out a user by revoking their refresh token
func (s *Service) LogoutUser(userID, refreshToken string) *errors.AppError {
	// Revoke the refresh token in Redis
	if err := redis.RevokeRefreshToken(userID, refreshToken); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to revoke refresh token")
	}
	return nil
}

func (s *Service) RequestPasswordReset(email string) *errors.AppError {
	user, err := s.Repo.GetUserByEmail(email)
	if err != nil {
		// For security, always return a generic success message even if email not found
		return nil
	}

	resetToken := uuid.New().String()
	// Store token in Redis with expiration (e.g., 1 hour)
	if err := redis.SetPasswordResetToken(user.ID.String(), resetToken, time.Hour); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to generate reset token")
	}

	resetLink := fmt.Sprintf("http://your-frontend-app/reset-password?token=%s", resetToken)
	if err := s.EmailService.SendPasswordResetEmail(user.Email, resetLink); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to send password reset email")
	}

	return nil
}

func (s *Service) VerifyEmail(token string) *errors.AppError {
	userID, err := redis.GetEmailVerificationToken(token)
	if err != nil || userID == "" {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired verification token")
	}

	// Update user's email_verified status in DB
	if err := s.Repo.UpdateUserEmailVerified(userID, true); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to verify email")
	}

	// Invalidate the token after use
	if err := redis.DeleteEmailVerificationToken(token); err != nil {
		fmt.Printf("Warning: Failed to delete used email verification token from Redis: %v\n", err)
	}

	return nil
}

func (s *Service) ConfirmPasswordReset(token, newPassword string) *errors.AppError {
	// Validate reset token from Redis
	userID, err := redis.GetPasswordResetToken(token)
	if err != nil || userID == "" {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired reset token")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to hash new password")
	}

	if err := s.Repo.UpdateUserPassword(userID, string(hashedPassword)); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to update password")
	}

	// Invalidate the token after use
	if err := redis.DeletePasswordResetToken(token); err != nil {
		fmt.Printf("Warning: Failed to delete used password reset token from Redis: %v\n", err)
	}

	return nil
}
