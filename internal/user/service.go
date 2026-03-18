package user

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	emailpkg "github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/session"
	"github.com/gjovanovicst/auth_api/internal/sms"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/gjovanovicst/auth_api/internal/webhook"
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

// RoleLookupFunc is a function that returns role names for a user in an app.
// Used to populate JWT claims with roles without importing the rbac package directly.
type RoleLookupFunc func(appID, userID string) ([]string, error)

// AssignDefaultRoleFunc is called after user registration to assign the default role.
type AssignDefaultRoleFunc func(appID, userID string) error

type Service struct {
	Repo              *Repository
	EmailService      *emailpkg.Service
	DB                *gorm.DB
	SessionService    *session.Service      // Session management for multi-device tracking
	LookupRoles       RoleLookupFunc        // Optional: if nil, tokens are generated without roles
	AssignDefaultRole AssignDefaultRoleFunc // Optional: if nil, no default role is assigned on registration
	WebhookService    *webhook.Service      // Optional: if nil, webhook dispatch is skipped
	SMSSender         sms.Sender            // Optional: if nil, SMS 2FA auto-send is skipped
}

func NewService(r *Repository, es *emailpkg.Service, db *gorm.DB) *Service {
	return &Service{Repo: r, EmailService: es, DB: db}
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

// LoginResult represents the result of a login attempt
type LoginResult struct {
	RequiresTwoFA      bool
	RequiresTwoFASetup bool
	PasswordExpired    bool // true when the password has exceeded its max age; no tokens are issued
	UserID             uuid.UUID
	AccessToken        string // #nosec G101,G117 -- This is a result field, not a hardcoded credential
	RefreshToken       string // #nosec G101,G117 -- This is a result field, not a hardcoded credential
	SessionID          string
	TwoFAResponse      *dto.TwoFARequiredResponse
	TwoFASetupResponse *dto.TwoFASetupRequiredResponse
}

func (s *Service) RegisterUser(appID uuid.UUID, email, password string) (uuid.UUID, *errors.AppError) {
	// Check if user already exists
	_, err := s.Repo.GetUserByEmail(appID.String(), email)
	if err == nil { // User found, meaning email is already registered
		return uuid.UUID{}, errors.NewAppError(errors.ErrConflict, "Email already registered")
	}

	// Load app for password policy
	var app models.Application
	if dbErr := s.DB.Select(
		"pw_min_length, pw_max_length, pw_require_upper, pw_require_lower, pw_require_digit, pw_require_symbol, pw_history_count",
	).First(&app, "id = ?", appID).Error; dbErr != nil {
		app = models.Application{} // no policy configured — use defaults
	}

	// Validate password against policy
	if pErr := ValidatePasswordPolicy(password, &app); pErr != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrBadRequest, pErr.Error())
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to hash password")
	}

	// Build initial password history (one entry: the new hash)
	newUser := &models.User{
		AppID:         appID,
		Email:         email,
		PasswordHash:  string(hashedPassword),
		EmailVerified: false,
	}
	now := time.Now()
	newUser.PasswordChangedAt = &now
	AppendPasswordHistory(newUser, string(hashedPassword), app.PwHistoryCount)

	if err := s.Repo.CreateUser(newUser); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to create user")
	}

	user := newUser

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "user.registered", map[string]interface{}{
			"user_id": user.ID.String(),
			"email":   user.Email,
		})
	}

	// Assign default 'member' role to the new user (non-fatal if it fails)
	if s.AssignDefaultRole != nil {
		if err := s.AssignDefaultRole(appID.String(), user.ID.String()); err != nil {
			log.Printf("Warning: failed to assign default role to user %s: %v", user.ID.String(), err)
		}
	}

	// Generate email verification token and send email
	verificationToken := uuid.New().String()

	if err := redis.SetEmailVerificationToken(appID.String(), user.ID.String(), verificationToken, 24*time.Hour); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to store verification token")
	}

	if err := s.EmailService.SendVerificationEmail(appID, user.Email, verificationToken, &user.ID); err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to send verification email")
	}

	return user.ID, nil
}

func (s *Service) LoginUser(appID uuid.UUID, email, password, ip, userAgent string) (*LoginResult, *errors.AppError) {
	user, err := s.Repo.GetUserByEmail(appID.String(), email)
	if err != nil { // User not found
		return nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid credentials")
	}

	// Check if account is locked (before password check to avoid timing attacks)
	if user.LockedAt != nil {
		// Check if lock has expired (auto-unlock)
		if user.LockExpiresAt != nil && time.Now().UTC().After(*user.LockExpiresAt) {
			// Auto-unlock: clear lockout fields
			if err := s.Repo.ClearLockout(user.ID.String()); err != nil {
				log.Printf("Warning: failed to clear lockout for user %s: %v", user.ID.String(), err)
			}
			user.LockedAt = nil
			user.LockReason = ""
			user.LockExpiresAt = nil
		} else {
			// Account is still locked
			return nil, errors.NewAppError(errors.ErrForbidden, "Account is temporarily locked due to too many failed login attempts")
		}
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

	// Load application flags once — used for 2FA gate, forced-setup check,
	// password expiry check, and TTL resolution.
	// Fail-open: if the query fails we treat all flags as safe defaults.
	var app models.Application
	appLoaded := s.DB.Select(
		"two_fa_enabled, two_fa_required, pw_max_age_days, access_token_ttl_minutes, refresh_token_ttl_hours",
	).First(&app, "id = ?", appID).Error == nil

	// Check if the user's password has expired (before issuing any session).
	if appLoaded && IsPasswordExpired(user, app.PwMaxAgeDays) {
		return &LoginResult{
			PasswordExpired: true,
			UserID:          user.ID,
		}, nil
	}

	// Check if 2FA is enabled for this user AND the app's master switch is ON.
	// If two_fa_enabled is false on the application, skip the 2FA challenge
	// entirely even when the individual user has 2FA configured — the admin has
	// explicitly disabled 2FA at the app level.
	if user.TwoFAEnabled && (!appLoaded || app.TwoFAEnabled) {
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
			if sendErr := s.EmailService.Send2FACodeEmail(appID, user.Email, code, &user.ID); sendErr != nil {
				log.Printf("Warning: Failed to send 2FA email code to %s: %v", user.Email, sendErr)
				return nil, errors.NewAppError(errors.ErrInternal, "Failed to send 2FA code email")
			}
		}

		// If the user uses SMS 2FA, generate and send the code now
		if twoFAMethod == emailpkg.TwoFAMethodSMS && s.SMSSender != nil {
			if user.PhoneVerified && user.PhoneNumber != "" {
				code := generateSecure6DigitCode()
				if storeErr := redis.Set2FASMSCode(appID.String(), user.ID.String(), code); storeErr != nil {
					return nil, errors.NewAppError(errors.ErrInternal, "Failed to prepare SMS 2FA verification")
				}
				body := fmt.Sprintf("Your verification code is: %s  (expires in 5 minutes)", code)
				if sendErr := s.SMSSender.Send(user.PhoneNumber, body); sendErr != nil {
					log.Printf("Warning: Failed to send SMS 2FA code to user %s: %v", user.ID, sendErr)
					return nil, errors.NewAppError(errors.ErrInternal, "Failed to send SMS 2FA code")
				}
			}
		}

		// If the user uses backup email 2FA, generate and send the code to the backup address now.
		// Always store the code in Redis so resend works even in dev mode (no email service).
		if twoFAMethod == emailpkg.TwoFAMethodBackupEmail {
			if user.BackupEmailVerified && user.BackupEmail != "" {
				code := generateSecure6DigitCode()
				if storeErr := redis.SetBackupEmail2FACode(appID.String(), user.ID.String(), code); storeErr != nil {
					return nil, errors.NewAppError(errors.ErrInternal, "Failed to prepare backup email 2FA verification")
				}
				if s.EmailService != nil {
					if sendErr := s.EmailService.Send2FACodeEmail(appID, user.BackupEmail, code, &user.ID); sendErr != nil {
						log.Printf("Warning: Failed to send backup email 2FA code to user %s: %v", user.ID, sendErr)
						return nil, errors.NewAppError(errors.ErrInternal, "Failed to send backup email 2FA code")
					}
				} else {
					log.Printf("[DEV MODE] Backup email 2FA code for user %s: %s", user.ID, code)
				}
			}
		}

		// For passkey 2FA, no server-side action needed here — the client
		// must call /2fa/passkey/begin + /2fa/passkey/finish using the temp token.

		return &LoginResult{
			RequiresTwoFA: true,
			UserID:        user.ID,
			TwoFAResponse: &dto.TwoFARequiredResponse{
				RequiresTwoFA: true,
				Message:       "2FA verification required",
				TempToken:     tempToken,
				Method:        twoFAMethod,
			},
		}, nil
	}

	// Check if this application requires 2FA setup for all users.
	// Reuse the already-loaded app record instead of issuing a second DB query.
	if appLoaded && app.TwoFARequired {
		// User doesn't have 2FA set up, but the app requires it.
		// Issue tokens via session so the user can authenticate to /2fa/generate, but flag the response.
		accessToken, refreshToken, sessionID, appErr := s.createSession(appID.String(), user.ID.String(), ip, userAgent, &app)
		if appErr != nil {
			return nil, appErr
		}

		return &LoginResult{
			RequiresTwoFASetup: true,
			UserID:             user.ID,
			AccessToken:        accessToken,
			RefreshToken:       refreshToken,
			SessionID:          sessionID,
			TwoFASetupResponse: &dto.TwoFASetupRequiredResponse{
				Message:      "2FA setup is required for this application",
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			},
		}, nil
	}

	// Standard login — create session
	var appPtr *models.Application
	if appLoaded {
		appPtr = &app
	}
	accessToken, refreshToken, sessionID, appErr := s.createSession(appID.String(), user.ID.String(), ip, userAgent, appPtr)
	if appErr != nil {
		return nil, appErr
	}

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "user.login", map[string]interface{}{
			"user_id": user.ID.String(),
			"email":   user.Email,
			"ip":      ip,
		})
	}

	return &LoginResult{
		RequiresTwoFA: false,
		UserID:        user.ID,
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		SessionID:     sessionID,
	}, nil
}

func (s *Service) RefreshUserToken(refreshToken string, accessTTL, refreshTTL time.Duration) (string, string, string, *errors.AppError) {
	// Delegate to session service if available (session-based refresh with token rotation)
	if s.SessionService != nil {
		return s.SessionService.RefreshSession(refreshToken, accessTTL, refreshTTL)
	}

	// Legacy fallback: refresh without session tracking
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

	// Generate new access and refresh tokens (re-fetch roles for freshness)
	roles := s.getUserRoles(claims.AppID, claims.UserID)
	newAccessToken, err := jwt.GenerateAccessToken(claims.AppID, claims.UserID, claims.SessionID, roles, accessTTL)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new access token")
	}
	newRefreshToken, err := jwt.GenerateRefreshToken(claims.AppID, claims.UserID, claims.SessionID, roles, refreshTTL)
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

// LogoutUser logs out a user by revoking their session and blacklisting their access token
func (s *Service) LogoutUser(appID, userID, sessionID, refreshToken, accessToken string) *errors.AppError {
	// Revoke the session if session service is available and sessionID is present
	if s.SessionService != nil && sessionID != "" {
		if appErr := s.SessionService.LogoutSession(appID, userID, sessionID, accessToken); appErr != nil {
			log.Printf("Warning: Failed to revoke session %s: %v\n", sessionID, appErr.Message)
		}
		return nil
	}

	// Legacy fallback: revoke refresh token directly
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

// createSession creates a new session via the session service, or falls back to legacy token storage.
func (s *Service) createSession(appID, userID, ip, userAgent string, app *models.Application) (accessToken, refreshToken, sessionID string, appErr *errors.AppError) {
	roles := s.getUserRoles(appID, userID)
	accessTTL, refreshTTL := ResolveTokenTTLs(app)

	if s.SessionService != nil {
		return s.SessionService.CreateSession(appID, userID, ip, userAgent, roles, accessTTL, refreshTTL)
	}

	// Legacy fallback: generate tokens without session tracking
	var err error
	accessToken, err = jwt.GenerateAccessToken(appID, userID, "", roles, accessTTL)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}
	refreshToken, err = jwt.GenerateRefreshToken(appID, userID, "", roles, refreshTTL)
	if err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}
	if err := redis.SetRefreshToken(appID, userID, refreshToken); err != nil {
		return "", "", "", errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}
	return accessToken, refreshToken, "", nil
}

// CreateSessionForUser creates a new authenticated session for a user by app+userID.
// Used by the trusted-device bypass in the Login handler to issue tokens when 2FA is skipped.
func (s *Service) CreateSessionForUser(appID, userID uuid.UUID, ip, userAgent string) (accessToken, refreshToken string, appErr *errors.AppError) {
	var app models.Application
	var appPtr *models.Application
	if s.DB.Select("access_token_ttl_minutes, refresh_token_ttl_hours").First(&app, "id = ?", appID).Error == nil {
		appPtr = &app
	}
	at, rt, _, err := s.createSession(appID.String(), userID.String(), ip, userAgent, appPtr)
	return at, rt, err
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

	// Resolve per-app frontend URL and reset-password path so the link points to the correct frontend.
	var app models.Application
	if dbErr := s.DB.Select("frontend_url, reset_password_path").First(&app, "id = ?", appID).Error; dbErr != nil {
		app.FrontendURL = ""
		app.ResetPasswordPath = ""
	}
	resetPath := util.ResolveLinkPath(app.ResetPasswordPath, util.DefaultResetPasswordPath)
	resetLink := fmt.Sprintf("%s%s?token=%s", util.ResolveFrontendURL(app.FrontendURL), resetPath, resetToken)
	if err := s.EmailService.SendPasswordResetEmail(appID, user.Email, resetLink, &user.ID); err != nil {
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

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "user.verified", map[string]interface{}{
			"user_id": userUUID.String(),
		})
	}

	return userUUID, nil
}

// ResendVerificationEmail resends the email verification link for a user.
// Returns nil even if the user is not found or already verified (to prevent email enumeration).
func (s *Service) ResendVerificationEmail(appID uuid.UUID, email string) *errors.AppError {
	user, err := s.Repo.GetUserByEmail(appID.String(), email)
	if err != nil {
		// User not found — return nil to prevent email enumeration
		return nil
	}

	if user.EmailVerified {
		// Already verified — return nil to prevent email enumeration
		return nil
	}

	userID := user.ID.String()

	// Invalidate any existing verification token for this user
	oldToken, err := redis.GetEmailVerificationTokenByUserID(appID.String(), userID)
	if err == nil && oldToken != "" {
		if delErr := redis.DeleteEmailVerificationToken(appID.String(), oldToken); delErr != nil {
			log.Printf("Warning: Failed to delete old email verification token from Redis: %v\n", delErr)
		}
	}

	// Generate and store new verification token
	verificationToken := uuid.New().String()
	if err := redis.SetEmailVerificationToken(appID.String(), userID, verificationToken, 24*time.Hour); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to store verification token")
	}

	if err := s.EmailService.SendVerificationEmail(appID, user.Email, verificationToken, &user.ID); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to send verification email")
	}

	return nil
}

func (s *Service) ConfirmPasswordReset(appID uuid.UUID, token, newPassword string) (uuid.UUID, *errors.AppError) {
	// Validate reset token from Redis
	userID, err := redis.GetPasswordResetToken(appID.String(), token)
	if err != nil || userID == "" {
		return uuid.UUID{}, errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired reset token")
	}

	// Load app for password policy
	var app models.Application
	if dbErr := s.DB.Select(
		"pw_min_length, pw_max_length, pw_require_upper, pw_require_lower, pw_require_digit, pw_require_symbol, pw_history_count",
	).First(&app, "id = ?", appID).Error; dbErr != nil {
		app = models.Application{}
	}

	// Validate new password against policy
	if pErr := ValidatePasswordPolicy(newPassword, &app); pErr != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrBadRequest, pErr.Error())
	}

	// Fetch user to check history
	resetUser, err := s.Repo.GetUserByID(userID)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrNotFound, "User not found")
	}
	if hErr := CheckPasswordHistory(newPassword, resetUser, app.PwHistoryCount); hErr != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrBadRequest, hErr.Error())
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return uuid.UUID{}, errors.NewAppError(errors.ErrInternal, "Failed to hash new password")
	}

	AppendPasswordHistory(resetUser, string(hashedPassword), app.PwHistoryCount)

	if err := s.Repo.UpdateUserPasswordWithHistory(userID, string(hashedPassword), resetUser.PasswordHistory); err != nil {
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

	if err := s.EmailService.SendVerificationEmail(appID, req.Email, verificationToken, &user.ID); err != nil {
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

	// Load app for password policy
	var app models.Application
	if dbErr := s.DB.Select(
		"pw_min_length, pw_max_length, pw_require_upper, pw_require_lower, pw_require_digit, pw_require_symbol, pw_history_count",
	).First(&app, "id = ?", appID).Error; dbErr != nil {
		app = models.Application{}
	}

	// Validate new password against policy
	if pErr := ValidatePasswordPolicy(req.NewPassword, &app); pErr != nil {
		return errors.NewAppError(errors.ErrBadRequest, pErr.Error())
	}

	// Check password history
	if hErr := CheckPasswordHistory(req.NewPassword, user, app.PwHistoryCount); hErr != nil {
		return errors.NewAppError(errors.ErrBadRequest, hErr.Error())
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcryptCost)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to hash new password")
	}

	AppendPasswordHistory(user, string(hashedPassword), app.PwHistoryCount)

	// Update password with history
	if err := s.Repo.UpdateUserPasswordWithHistory(userID, string(hashedPassword), user.PasswordHistory); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to update password")
	}

	// Revoke all existing tokens for security
	if err := s.RevokeAllUserTokens(appID.String(), userID); err != nil {
		log.Printf("Warning: Failed to revoke all user tokens after password change: %v\n", err.Message)
	}

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "user.password_changed", map[string]interface{}{
			"user_id": userID,
		})
	}

	return nil
}

// SetInitialPassword sets a password for a social-only user (no existing PasswordHash).
// Returns ErrConflict if the user already has a password — callers should use
// UpdateUserPassword instead in that case.
func (s *Service) SetInitialPassword(appID uuid.UUID, userID string, newPassword string) *errors.AppError {
	user, err := s.Repo.GetUserByID(userID)
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	if user.PasswordHash != "" {
		return errors.NewAppError(errors.ErrConflict, "Password already set. Use the change-password endpoint to update it.")
	}

	// Load app for password policy
	var app models.Application
	if dbErr := s.DB.Select(
		"pw_min_length, pw_max_length, pw_require_upper, pw_require_lower, pw_require_digit, pw_require_symbol, pw_history_count",
	).First(&app, "id = ?", appID).Error; dbErr != nil {
		app = models.Application{}
	}

	if pErr := ValidatePasswordPolicy(newPassword, &app); pErr != nil {
		return errors.NewAppError(errors.ErrBadRequest, pErr.Error())
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to hash password")
	}

	if err := s.Repo.UpdateUserPasswordWithHistory(userID, string(hashedPassword), user.PasswordHistory); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to set password")
	}

	// Dispatch webhook event (non-fatal)
	if s.WebhookService != nil {
		s.WebhookService.Dispatch(appID, "user.password_set", map[string]interface{}{
			"user_id": userID,
		})
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

// RequestMagicLink generates a magic link token and sends it via email.
// Returns nil even if the user is not found (to prevent email enumeration).
func (s *Service) RequestMagicLink(appID uuid.UUID, email string) *errors.AppError {
	// Check if magic link is enabled for this application
	var app models.Application
	if err := s.DB.Select("magic_link_enabled, frontend_url, magic_link_path").First(&app, "id = ?", appID).Error; err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to load application settings")
	}
	if !app.MagicLinkEnabled {
		return errors.NewAppError(errors.ErrBadRequest, "Magic link login is not enabled for this application")
	}

	user, err := s.Repo.GetUserByEmail(appID.String(), email)
	if err != nil {
		// User not found — return nil to prevent email enumeration
		return nil
	}

	// Check if account is active
	if !user.IsActive {
		// Return nil to prevent email enumeration
		return nil
	}

	// Check if email is verified
	if !user.EmailVerified {
		// Return nil to prevent email enumeration
		return nil
	}

	// Generate magic link token
	magicToken := uuid.New().String()

	// Store in Redis with 10-minute expiration (invalidates any previous token for this user)
	if err := redis.SetMagicLinkToken(appID.String(), user.ID.String(), magicToken, 10*time.Minute); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to generate magic link")
	}

	// Build the magic link URL — use per-app FrontendURL and MagicLinkPath, falling back to global defaults.
	magicPath := util.ResolveLinkPath(app.MagicLinkPath, util.DefaultMagicLinkPath)
	magicLink := fmt.Sprintf("%s%s?token=%s&app_id=%s", util.ResolveFrontendURL(app.FrontendURL), magicPath, magicToken, appID.String())

	if err := s.EmailService.SendMagicLinkEmail(appID, user.Email, magicLink, &user.ID); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to send magic link email")
	}

	return nil
}

// VerifyMagicLink verifies a magic link token and creates a session (passwordless login).
// 2FA is skipped since the magic link itself serves as email-based verification.
func (s *Service) VerifyMagicLink(appID uuid.UUID, token, ip, userAgent string) (*LoginResult, *errors.AppError) {
	// Check if magic link is enabled for this application
	var app models.Application
	if err := s.DB.Select("magic_link_enabled, access_token_ttl_minutes, refresh_token_ttl_hours").First(&app, "id = ?", appID).Error; err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to load application settings")
	}
	if !app.MagicLinkEnabled {
		return nil, errors.NewAppError(errors.ErrBadRequest, "Magic link login is not enabled for this application")
	}

	// Retrieve userID from Redis
	userID, err := redis.GetMagicLinkToken(appID.String(), token)
	if err != nil || userID == "" {
		return nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired magic link")
	}

	// Delete token immediately — magic links are single-use
	if delErr := redis.DeleteMagicLinkToken(appID.String(), token); delErr != nil {
		log.Printf("Warning: Failed to delete used magic link token from Redis: %v\n", delErr)
	}

	// Fetch user from DB
	userUUID, parseErr := uuid.Parse(userID)
	if parseErr != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Invalid user ID format")
	}

	user, err := s.Repo.GetUserByID(userUUID.String())
	if err != nil {
		return nil, errors.NewAppError(errors.ErrUnauthorized, "User not found")
	}

	// Verify account is still active
	if !user.IsActive {
		return nil, errors.NewAppError(errors.ErrForbidden, "Account is deactivated. Please contact your administrator.")
	}

	// Verify email is still verified
	if !user.EmailVerified {
		return nil, errors.NewAppError(errors.ErrForbidden, "Email not verified.")
	}

	// Create session (skip 2FA — magic link is itself an email-based verification factor)
	accessToken, refreshToken, sessionID, appErr := s.createSession(appID.String(), user.ID.String(), ip, userAgent, &app)
	if appErr != nil {
		return nil, appErr
	}

	return &LoginResult{
		RequiresTwoFA: false,
		UserID:        user.ID,
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		SessionID:     sessionID,
	}, nil
}
