package twofa

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	emailpkg "github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/user"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type Service struct {
	UserRepo     *user.Repository
	DB           *gorm.DB
	EmailService *emailpkg.Service
}

func NewService(userRepo *user.Repository, db *gorm.DB, emailService *emailpkg.Service) *Service {
	return &Service{
		UserRepo:     userRepo,
		DB:           db,
		EmailService: emailService,
	}
}

type TwoFASetupResponse struct {
	Secret     string `json:"secret"` // #nosec G101,G117 -- This is a response field for TOTP secret, not a hardcoded credential
	QRCodeURL  string `json:"qr_code_url"`
	QRCodeData []byte `json:"qr_code_data,omitempty"`
}

// Generate2FASecret generates a new TOTP secret for a user
func (s *Service) Generate2FASecret(appID uuid.UUID, userID string) (*TwoFASetupResponse, *errors.AppError) {
	// Fetch app settings for 2FA gate check and issuer resolution
	var app models.Application
	appFound := false
	if err := s.DB.Select("name, two_fa_issuer_name, two_fa_enabled").First(&app, "id = ?", appID).Error; err == nil {
		appFound = true
		if !app.TwoFAEnabled {
			return nil, errors.NewAppError(errors.ErrForbidden, "2FA is not available for this application")
		}
	}

	user, err := s.UserRepo.GetUserByID(userID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	// Generate a new secret
	secret := generateSecretKey()

	// Store the secret temporarily in Redis (expires in 10 minutes)
	if err := redis.SetTempTwoFASecret(appID.String(), userID, secret, 10*time.Minute); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to store temporary secret")
	}

	// Resolve issuer name: TwoFAIssuerName > app name > global APP_NAME > default
	var issuer string
	if appFound {
		if app.TwoFAIssuerName != "" {
			issuer = app.TwoFAIssuerName
		} else if app.Name != "" {
			issuer = app.Name
		}
	}
	if issuer == "" {
		issuer = viper.GetString("APP_NAME")
		if issuer == "" {
			issuer = "Auth API"
		}
	}

	provisioningURI := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s",
		issuer, user.Email, secret, issuer)

	// Generate QR code
	qrCodeData, err := qrcode.Encode(provisioningURI, qrcode.Medium, 256)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to generate QR code")
	}

	return &TwoFASetupResponse{
		Secret:     secret,
		QRCodeURL:  provisioningURI,
		QRCodeData: qrCodeData,
	}, nil
}

// checkTwoFAEnabled verifies that 2FA is enabled for the given application.
// Returns nil if 2FA is allowed, or an error if the feature is disabled.
func (s *Service) checkTwoFAEnabled(appID uuid.UUID) *errors.AppError {
	var app models.Application
	if err := s.DB.Select("two_fa_enabled").First(&app, "id = ?", appID).Error; err == nil && !app.TwoFAEnabled {
		return errors.NewAppError(errors.ErrForbidden, "2FA is not available for this application")
	}
	return nil
}

// VerifySetup verifies the initial 2FA setup with a TOTP code
func (s *Service) VerifySetup(appID uuid.UUID, userID, totpCode string) *errors.AppError {
	if appErr := s.checkTwoFAEnabled(appID); appErr != nil {
		return appErr
	}

	// Get the temporary secret from Redis
	secret, err := redis.GetTempTwoFASecret(appID.String(), userID)
	if err != nil {
		log.Printf("Failed to get temp 2FA secret for user %s: %v", userID, err)
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired setup session")
	}

	// Verify the TOTP code
	if !totp.Validate(totpCode, secret) {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid TOTP code")
	}

	return nil
}

// Enable2FA enables 2FA for a user after successful verification
func (s *Service) Enable2FA(appID uuid.UUID, userID string) ([]string, *errors.AppError) {
	if appErr := s.checkTwoFAEnabled(appID); appErr != nil {
		return nil, appErr
	}

	// Get the temporary secret from Redis
	secret, err := redis.GetTempTwoFASecret(appID.String(), userID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired setup session")
	}

	// Generate recovery codes
	recoveryCodes := generateRecoveryCodes(8)
	recoveryCodesJSON, _ := json.Marshal(recoveryCodes)

	// Update user in database
	if err := s.UserRepo.Enable2FAWithMethod(userID, secret, string(recoveryCodesJSON), emailpkg.TwoFAMethodTOTP); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to enable 2FA")
	}

	// Remove temporary secret from Redis
	if err := redis.DeleteTempTwoFASecret(appID.String(), userID); err != nil {
		// Log the error but don't fail the entire operation since 2FA was already enabled
		log.Printf("Warning: Failed to delete temporary 2FA secret for user %s: %v", userID, err)
	}

	return recoveryCodes, nil
}

// Disable2FA disables 2FA for a user
func (s *Service) Disable2FA(userID string) *errors.AppError {
	if err := s.UserRepo.Disable2FA(userID); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to disable 2FA")
	}
	return nil
}

// VerifyTOTP verifies a TOTP code for an already enabled 2FA user
func (s *Service) VerifyTOTP(userID, totpCode string) *errors.AppError {
	user, err := s.UserRepo.GetUserByID(userID)
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	if !user.TwoFAEnabled || user.TwoFASecret == "" {
		return errors.NewAppError(errors.ErrBadRequest, "2FA is not enabled for this user")
	}

	// Verify the TOTP code
	if !totp.Validate(totpCode, user.TwoFASecret) {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid TOTP code")
	}

	return nil
}

// VerifyRecoveryCode verifies a recovery code
func (s *Service) VerifyRecoveryCode(userID, recoveryCode string) *errors.AppError {
	user, err := s.UserRepo.GetUserByID(userID)
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	if !user.TwoFAEnabled {
		return errors.NewAppError(errors.ErrBadRequest, "2FA is not enabled for this user")
	}

	// Parse recovery codes
	var recoveryCodes []string
	if err := json.Unmarshal(user.TwoFARecoveryCodes, &recoveryCodes); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to parse recovery codes")
	}

	// Check if the recovery code exists
	for i, code := range recoveryCodes {
		if code == recoveryCode {
			// Remove the used recovery code
			recoveryCodes = append(recoveryCodes[:i], recoveryCodes[i+1:]...)
			updatedCodes, _ := json.Marshal(recoveryCodes)

			// Update the database
			if err := s.UserRepo.UpdateRecoveryCodes(userID, string(updatedCodes)); err != nil {
				return errors.NewAppError(errors.ErrInternal, "Failed to update recovery codes")
			}

			return nil
		}
	}

	return errors.NewAppError(errors.ErrUnauthorized, "Invalid recovery code")
}

// GenerateNewRecoveryCodes generates new recovery codes for a user
func (s *Service) GenerateNewRecoveryCodes(userID string) ([]string, *errors.AppError) {
	user, err := s.UserRepo.GetUserByID(userID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	if !user.TwoFAEnabled {
		return nil, errors.NewAppError(errors.ErrBadRequest, "2FA is not enabled for this user")
	}

	// Generate new recovery codes
	recoveryCodes := generateRecoveryCodes(8)
	recoveryCodesJSON, _ := json.Marshal(recoveryCodes)

	// Update the database
	if err := s.UserRepo.UpdateRecoveryCodes(userID, string(recoveryCodesJSON)); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to update recovery codes")
	}

	return recoveryCodes, nil
}

// ============================================================================
// Email 2FA methods
// ============================================================================

// EnableEmail2FA enables email-based 2FA for a user (no TOTP secret needed).
// Recovery codes are still generated for account recovery.
func (s *Service) EnableEmail2FA(appID uuid.UUID, userID string) ([]string, *errors.AppError) {
	if appErr := s.checkTwoFAEnabled(appID); appErr != nil {
		return nil, appErr
	}

	// Verify that this application allows email 2FA
	if !s.isEmail2FAAllowed(appID) {
		return nil, errors.NewAppError(errors.ErrForbidden, "Email 2FA is not available for this application")
	}

	// Generate recovery codes
	recoveryCodes := generateRecoveryCodes(8)
	recoveryCodesJSON, _ := json.Marshal(recoveryCodes)

	// Enable 2FA with email method — no TOTP secret needed
	if err := s.UserRepo.Enable2FAWithMethod(userID, "", string(recoveryCodesJSON), emailpkg.TwoFAMethodEmail); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to enable email 2FA")
	}

	return recoveryCodes, nil
}

// GenerateEmail2FACode generates a 6-digit code, stores it in Redis, and sends it via email.
// This is called during login when the user has email 2FA enabled.
func (s *Service) GenerateEmail2FACode(appID uuid.UUID, userID string) *errors.AppError {
	usr, err := s.UserRepo.GetUserByID(userID)
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	if !usr.TwoFAEnabled || usr.TwoFAMethod != emailpkg.TwoFAMethodEmail {
		return errors.NewAppError(errors.ErrBadRequest, "Email 2FA is not enabled for this user")
	}

	// Generate a 6-digit code
	code := generate6DigitCode()

	// Store code in Redis with 5-minute expiration
	if err := redis.Set2FAEmailCode(appID.String(), userID, code); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to store 2FA code")
	}

	// Send the code via email
	if s.EmailService != nil {
		if err := s.EmailService.Send2FACodeEmail(appID, usr.Email, code); err != nil {
			log.Printf("Error sending 2FA email to %s: %v", usr.Email, err)
			return errors.NewAppError(errors.ErrInternal, "Failed to send 2FA code email")
		}
	} else {
		log.Printf("[DEV MODE] 2FA email code for user %s: %s", userID, code)
	}

	return nil
}

// VerifyEmail2FACode verifies a 6-digit email 2FA code from Redis.
func (s *Service) VerifyEmail2FACode(appID uuid.UUID, userID, code string) *errors.AppError {
	storedCode, err := redis.Get2FAEmailCode(appID.String(), userID)
	if err != nil {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired 2FA code")
	}

	if storedCode != code {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid 2FA code")
	}

	// Delete the code after successful verification (one-time use)
	if err := redis.Delete2FAEmailCode(appID.String(), userID); err != nil {
		log.Printf("Warning: Failed to delete 2FA email code for user %s: %v", userID, err)
	}

	return nil
}

// ResendEmail2FACode resends a new 2FA code (generates a fresh one).
func (s *Service) ResendEmail2FACode(appID uuid.UUID, userID string) *errors.AppError {
	return s.GenerateEmail2FACode(appID, userID)
}

// GetAvailableMethods returns the 2FA methods available for an application.
func (s *Service) GetAvailableMethods(appID uuid.UUID) []string {
	var app models.Application
	if err := s.DB.Select("two_fa_methods, two_fa_enabled, email_2fa_enabled").First(&app, "id = ?", appID).Error; err != nil {
		return []string{emailpkg.TwoFAMethodTOTP} // default
	}

	if !app.TwoFAEnabled {
		return nil
	}

	methods := strings.Split(app.TwoFAMethods, ",")
	var result []string
	for _, m := range methods {
		m = strings.TrimSpace(m)
		if m == emailpkg.TwoFAMethodEmail && !app.Email2FAEnabled {
			continue
		}
		if m != "" {
			result = append(result, m)
		}
	}
	if len(result) == 0 {
		return []string{emailpkg.TwoFAMethodTOTP}
	}
	return result
}

// isEmail2FAAllowed checks if the application allows email-based 2FA.
func (s *Service) isEmail2FAAllowed(appID uuid.UUID) bool {
	var app models.Application
	if err := s.DB.Select("email_2fa_enabled, two_fa_methods").First(&app, "id = ?", appID).Error; err != nil {
		return false
	}
	if !app.Email2FAEnabled {
		return false
	}
	methods := strings.Split(app.TwoFAMethods, ",")
	for _, m := range methods {
		if strings.TrimSpace(m) == emailpkg.TwoFAMethodEmail {
			return true
		}
	}
	return false
}

// GetUserTwoFAMethod returns the 2FA method for a user.
func (s *Service) GetUserTwoFAMethod(userID string) (string, *errors.AppError) {
	usr, err := s.UserRepo.GetUserByID(userID)
	if err != nil {
		return "", errors.NewAppError(errors.ErrNotFound, "User not found")
	}
	if !usr.TwoFAEnabled {
		return "", errors.NewAppError(errors.ErrBadRequest, "2FA is not enabled for this user")
	}
	method := usr.TwoFAMethod
	if method == "" {
		method = emailpkg.TwoFAMethodTOTP // backward compat: existing users default to TOTP
	}
	return method, nil
}

// generateSecretKey generates a random base32 encoded secret key
func generateSecretKey() string {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// In case of random number generation failure, use a timestamp-based fallback
		// This should never happen in practice, but we handle the error for security scanning
		panic(fmt.Sprintf("Failed to generate secure random bytes: %v", err))
	}
	return base32.StdEncoding.EncodeToString(secret)
}

// generate6DigitCode generates a cryptographically secure 6-digit numeric code.
func generate6DigitCode() string {
	max := big.NewInt(1000000) // 0-999999
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate secure random number for 2FA code: %v", err))
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// generateRecoveryCodes generates recovery codes
func generateRecoveryCodes(count int) []string {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code := make([]byte, 8)
		if _, err := rand.Read(code); err != nil {
			// In case of random number generation failure, panic as this is critical for security
			panic(fmt.Sprintf("Failed to generate secure random bytes for recovery code: %v", err))
		}
		codes[i] = fmt.Sprintf("%x", code)
	}
	return codes
}
