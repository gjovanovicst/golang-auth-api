package twofa

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/user"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/viper"
)

type Service struct {
	UserRepo *user.Repository
}

func NewService(userRepo *user.Repository) *Service {
	return &Service{
		UserRepo: userRepo,
	}
}

type TwoFASetupResponse struct {
	Secret     string `json:"secret"`
	QRCodeURL  string `json:"qr_code_url"`
	QRCodeData []byte `json:"qr_code_data,omitempty"`
}

// Generate2FASecret generates a new TOTP secret for a user
func (s *Service) Generate2FASecret(appID uuid.UUID, userID string) (*TwoFASetupResponse, *errors.AppError) {
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

	// Create provisioning URI
	issuer := viper.GetString("APP_NAME")
	if issuer == "" {
		issuer = "Auth API"
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

// VerifySetup verifies the initial 2FA setup with a TOTP code
func (s *Service) VerifySetup(appID uuid.UUID, userID, totpCode string) *errors.AppError {
	// Get the temporary secret from Redis
	secret, err := redis.GetTempTwoFASecret(appID.String(), userID)
	if err != nil {
		fmt.Printf("Failed to get temp 2FA secret for user %s: %v\n", userID, err)
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired setup session")
	}

	fmt.Printf("DEBUG: Validating TOTP code %s for user %s with secret length %d\n", totpCode, userID, len(secret))

	// Verify the TOTP code
	if !totp.Validate(totpCode, secret) {
		fmt.Printf("TOTP validation failed for user %s with code %s and secret %s\n", userID, totpCode, secret)
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid TOTP code")
	}

	fmt.Printf("TOTP validation successful for user %s\n", userID)
	return nil
}

// Enable2FA enables 2FA for a user after successful verification
func (s *Service) Enable2FA(appID uuid.UUID, userID string) ([]string, *errors.AppError) {
	// Get the temporary secret from Redis
	secret, err := redis.GetTempTwoFASecret(appID.String(), userID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired setup session")
	}

	// Generate recovery codes
	recoveryCodes := generateRecoveryCodes(8)
	recoveryCodesJSON, _ := json.Marshal(recoveryCodes)

	// Update user in database
	if err := s.UserRepo.Enable2FA(userID, secret, string(recoveryCodesJSON)); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to enable 2FA")
	}

	// Remove temporary secret from Redis
	if err := redis.DeleteTempTwoFASecret(appID.String(), userID); err != nil {
		// Log the error but don't fail the entire operation since 2FA was already enabled
		fmt.Printf("Warning: Failed to delete temporary 2FA secret for user %s: %v\n", userID, err)
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
