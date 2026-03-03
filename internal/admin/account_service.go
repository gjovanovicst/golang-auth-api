package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"time"

	emailpkg "github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

// AccountService handles admin authentication and session management
type AccountService struct {
	Repo         *AccountRepository
	EmailService *emailpkg.Service
}

// NewAccountService creates a new AccountService
func NewAccountService(repo *AccountRepository, emailService *emailpkg.Service) *AccountService {
	return &AccountService{Repo: repo, EmailService: emailService}
}

// Authenticate validates admin credentials and updates the last login timestamp.
// Returns the admin account on success, or an error on failure.
func (s *AccountService) Authenticate(username, password string) (*models.AdminAccount, error) {
	account, err := s.Repo.GetByUsernameOrEmail(username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login timestamp (non-blocking, best effort)
	_ = s.Repo.UpdateLastLogin(account.ID.String())

	return account, nil
}

// CreateSession generates a secure session token and stores it in Redis.
// Returns the session ID that should be set as a cookie value.
func (s *AccountService) CreateSession(adminID string) (string, error) {
	sessionBytes := make([]byte, 32)
	if _, err := rand.Read(sessionBytes); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	sessionID := hex.EncodeToString(sessionBytes)

	expiration := s.sessionExpiration()
	if err := redis.SetAdminSession(sessionID, adminID, expiration); err != nil {
		return "", fmt.Errorf("failed to store session: %w", err)
	}

	return sessionID, nil
}

// Create2FATempSession stores a temporary session token for an admin who has
// passed password authentication but still needs to complete 2FA verification.
// Returns the temp token.
func (s *AccountService) Create2FATempSession(adminID string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate temp session token: %w", err)
	}
	tempToken := hex.EncodeToString(tokenBytes)

	if err := redis.SetAdmin2FATempSession(tempToken, adminID); err != nil {
		return "", fmt.Errorf("failed to store temp session: %w", err)
	}

	return tempToken, nil
}

// Validate2FATempSession validates a temporary 2FA session and returns the admin account.
func (s *AccountService) Validate2FATempSession(tempToken string) (*models.AdminAccount, error) {
	if tempToken == "" {
		return nil, fmt.Errorf("empty temp token")
	}

	adminID, err := redis.GetAdmin2FATempSession(tempToken)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired 2FA session")
	}

	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin account not found")
	}

	return account, nil
}

// Consume2FATempSession validates and then deletes a temporary 2FA session.
// Used after successful 2FA verification to prevent replay.
func (s *AccountService) Consume2FATempSession(tempToken string) (*models.AdminAccount, error) {
	account, err := s.Validate2FATempSession(tempToken)
	if err != nil {
		return nil, err
	}
	_ = redis.DeleteAdmin2FATempSession(tempToken)
	return account, nil
}

// ValidateSession checks if a session is valid and returns the associated admin account.
func (s *AccountService) ValidateSession(sessionID string) (*models.AdminAccount, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("empty session ID")
	}

	adminID, err := redis.GetAdminSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin account not found")
	}

	return account, nil
}

// Logout removes a session from Redis.
func (s *AccountService) Logout(sessionID string) error {
	return redis.DeleteAdminSession(sessionID)
}

// GenerateCSRFToken returns the existing CSRF token for the session if one
// exists, or creates and stores a new one. This ensures a stable token per
// session so that HTMX GET requests (which also pass through the middleware)
// don't invalidate the token held in the page's <meta> tag.
func (s *AccountService) GenerateCSRFToken(sessionID string) (string, error) {
	// Return existing token if still valid
	existing, err := redis.GetCSRFToken(sessionID)
	if err == nil && existing != "" {
		return existing, nil
	}

	// Generate a new token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiration := s.sessionExpiration()
	if err := redis.SetCSRFToken(sessionID, token, expiration); err != nil {
		return "", fmt.Errorf("failed to store CSRF token: %w", err)
	}

	return token, nil
}

// ValidateCSRFToken checks if the provided CSRF token matches the one stored for the session.
func (s *AccountService) ValidateCSRFToken(sessionID, token string) bool {
	if sessionID == "" || token == "" {
		return false
	}

	storedToken, err := redis.GetCSRFToken(sessionID)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(storedToken), []byte(token)) == 1
}

// ============================================================================
// Two-Factor Authentication Methods
// ============================================================================

// TwoFASetupResponse contains the TOTP secret and QR code for setup.
type TwoFASetupResponse struct {
	Secret     string `json:"secret"` // #nosec G101 -- TOTP secret for setup, not a hardcoded credential
	QRCodeData []byte `json:"qr_code_data"`
}

// GenerateTOTPSecret creates a new TOTP secret for an admin and stores it
// temporarily in Redis (10-minute TTL). Returns the secret and QR code data.
func (s *AccountService) GenerateTOTPSecret(adminID, username string) (*TwoFASetupResponse, error) {
	// Generate a random secret key
	secret := generateSecretKey()

	// Store temporary secret in Redis
	if err := redis.SetAdmin2FATempSecret(adminID, secret); err != nil {
		return nil, fmt.Errorf("failed to store temporary secret: %w", err)
	}

	// Generate provisioning URI for QR code
	issuer := "AuthAPI Admin"
	provisioningURI := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s",
		issuer, username, secret, issuer)

	// Generate QR code PNG
	qrCode, err := qrcode.Encode(provisioningURI, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	return &TwoFASetupResponse{
		Secret:     secret,
		QRCodeData: qrCode,
	}, nil
}

// VerifyTOTPSetup validates a TOTP code against the temporary secret stored in Redis.
// This is called during initial 2FA setup to confirm the user has correctly configured their app.
func (s *AccountService) VerifyTOTPSetup(adminID, code string) error {
	secret, err := redis.GetAdmin2FATempSecret(adminID)
	if err != nil {
		return fmt.Errorf("invalid or expired setup session — please generate a new QR code")
	}

	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid TOTP code")
	}

	return nil
}

// EnableTOTP finalizes TOTP-based 2FA for an admin. It persists the secret from
// Redis to the database, generates recovery codes, and cleans up the temp secret.
// Returns the recovery codes (shown once to the user).
func (s *AccountService) EnableTOTP(adminID string) ([]string, error) {
	secret, err := redis.GetAdmin2FATempSecret(adminID)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired setup session — please start again")
	}

	recoveryCodes := generateRecoveryCodes(8)
	recoveryCodesJSON, _ := json.Marshal(recoveryCodes)

	if err := s.Repo.Enable2FA(adminID, emailpkg.TwoFAMethodTOTP, secret, recoveryCodesJSON); err != nil {
		return nil, fmt.Errorf("failed to enable 2FA: %w", err)
	}

	_ = redis.DeleteAdmin2FATempSecret(adminID)

	return recoveryCodes, nil
}

// EnableEmail2FA enables email-based 2FA for an admin account.
// Requires that the admin has an email address set.
// Returns the recovery codes (shown once to the user).
func (s *AccountService) EnableEmail2FA(adminID string) ([]string, error) {
	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin account not found")
	}

	if account.Email == "" {
		return nil, fmt.Errorf("email address is required for email-based 2FA — please set your email first")
	}

	recoveryCodes := generateRecoveryCodes(8)
	recoveryCodesJSON, _ := json.Marshal(recoveryCodes)

	if err := s.Repo.Enable2FA(adminID, emailpkg.TwoFAMethodEmail, "", recoveryCodesJSON); err != nil {
		return nil, fmt.Errorf("failed to enable 2FA: %w", err)
	}

	return recoveryCodes, nil
}

// Disable2FA disables two-factor authentication for an admin after password verification.
func (s *AccountService) Disable2FA(adminID, password string) error {
	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return fmt.Errorf("admin account not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		return fmt.Errorf("invalid password")
	}

	return s.Repo.Disable2FA(adminID)
}

// VerifyTOTPCode validates a TOTP code for an admin with TOTP-based 2FA enabled.
func (s *AccountService) VerifyTOTPCode(adminID, code string) error {
	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return fmt.Errorf("admin account not found")
	}

	if !account.TwoFAEnabled || account.TwoFASecret == "" {
		return fmt.Errorf("TOTP 2FA is not enabled")
	}

	if !totp.Validate(code, account.TwoFASecret) {
		return fmt.Errorf("invalid TOTP code")
	}

	return nil
}

// GenerateAndSendEmail2FACode generates a 6-digit code, stores it in Redis,
// and sends it to the admin's email address. Used during login verification.
func (s *AccountService) GenerateAndSendEmail2FACode(adminID string) error {
	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return fmt.Errorf("admin account not found")
	}

	if account.Email == "" {
		return fmt.Errorf("no email address configured for this admin account")
	}

	code := generate6DigitCode()

	if err := redis.SetAdmin2FAEmailCode(adminID, code); err != nil {
		return fmt.Errorf("failed to store email code: %w", err)
	}

	if s.EmailService != nil {
		if err := s.EmailService.SendAdmin2FACodeEmail(account.Email, code, account.Username); err != nil {
			log.Printf("Warning: failed to send admin 2FA email to %s: %v", account.Email, err)
			// Don't fail the operation — the code is stored in Redis and logged in dev mode
		}
	} else {
		log.Printf("Admin 2FA code for %s: %s (email service not available)", account.Username, code)
	}

	return nil
}

// VerifyEmail2FACode validates a 6-digit email code for an admin.
func (s *AccountService) VerifyEmail2FACode(adminID, code string) error {
	storedCode, err := redis.GetAdmin2FAEmailCode(adminID)
	if err != nil {
		return fmt.Errorf("invalid or expired verification code")
	}

	if subtle.ConstantTimeCompare([]byte(storedCode), []byte(code)) != 1 {
		return fmt.Errorf("invalid verification code")
	}

	_ = redis.DeleteAdmin2FAEmailCode(adminID)
	return nil
}

// VerifyRecoveryCode validates and consumes a single-use recovery code.
func (s *AccountService) VerifyRecoveryCode(adminID, recoveryCode string) error {
	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return fmt.Errorf("admin account not found")
	}

	if !account.TwoFAEnabled {
		return fmt.Errorf("2FA is not enabled")
	}

	var codes []string
	if err := json.Unmarshal(account.TwoFARecoveryCodes, &codes); err != nil {
		return fmt.Errorf("failed to parse recovery codes")
	}

	found := false
	for i, code := range codes {
		if subtle.ConstantTimeCompare([]byte(code), []byte(recoveryCode)) == 1 {
			// Remove the used code
			codes = append(codes[:i], codes[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("invalid recovery code")
	}

	updatedCodes, _ := json.Marshal(codes)
	if err := s.Repo.UpdateRecoveryCodes(adminID, updatedCodes); err != nil {
		return fmt.Errorf("failed to update recovery codes")
	}

	return nil
}

// RegenerateRecoveryCodes generates a new set of 8 recovery codes after password verification.
// Returns the new codes (shown once to the user).
func (s *AccountService) RegenerateRecoveryCodes(adminID, password string) ([]string, error) {
	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return nil, fmt.Errorf("admin account not found")
	}

	if !account.TwoFAEnabled {
		return nil, fmt.Errorf("2FA is not enabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	codes := generateRecoveryCodes(8)
	codesJSON, _ := json.Marshal(codes)

	if err := s.Repo.UpdateRecoveryCodes(adminID, codesJSON); err != nil {
		return nil, fmt.Errorf("failed to update recovery codes")
	}

	return codes, nil
}

// UpdateEmail updates the email address for an admin account.
func (s *AccountService) UpdateEmail(adminID, email string) error {
	return s.Repo.UpdateEmail(adminID, email)
}

// ChangePassword updates the admin's password after verifying the current one.
func (s *AccountService) ChangePassword(adminID, currentPassword, newPassword string) error {
	account, err := s.Repo.GetByID(adminID)
	if err != nil {
		return fmt.Errorf("admin account not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	return s.Repo.UpdatePassword(adminID, string(hashedPassword))
}

// ============================================================================
// Helper functions
// ============================================================================

// sessionExpiration returns the configured session TTL
func (s *AccountService) sessionExpiration() time.Duration {
	hours := viper.GetInt("ADMIN_SESSION_EXPIRATION_HOURS")
	if hours <= 0 {
		hours = 8 // default: 8 hours
	}
	return time.Duration(hours) * time.Hour
}

// generateSecretKey generates a random base32-encoded secret key for TOTP.
func generateSecretKey() string {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic(fmt.Sprintf("failed to generate secure random bytes: %v", err))
	}
	return base32.StdEncoding.EncodeToString(secret)
}

// generate6DigitCode generates a cryptographically secure 6-digit numeric code.
func generate6DigitCode() string {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(fmt.Sprintf("failed to generate secure random number: %v", err))
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// generateRecoveryCodes generates a set of cryptographically secure hex recovery codes.
func generateRecoveryCodes(count int) []string {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code := make([]byte, 8)
		if _, err := rand.Read(code); err != nil {
			panic(fmt.Sprintf("failed to generate secure random bytes: %v", err))
		}
		codes[i] = fmt.Sprintf("%x", code)
	}
	return codes
}
