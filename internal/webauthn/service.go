package webauthn

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	emailpkg "github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/user"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service provides WebAuthn/Passkey business logic.
type Service struct {
	Repo        *Repository
	UserRepo    *user.Repository
	DB          *gorm.DB
	AdminLookup func(string) (*models.AdminAccount, error) // Resolve admin account by ID (wired in main.go)
}

// NewService creates a new WebAuthn service.
func NewService(repo *Repository, userRepo *user.Repository, db *gorm.DB) *Service {
	return &Service{
		Repo:     repo,
		UserRepo: userRepo,
		DB:       db,
	}
}

// challengeTTL is the expiration time for WebAuthn challenges in Redis.
const challengeTTL = 5 * time.Minute

// ============================================================================
// Registration (Attestation)
// ============================================================================

// BeginRegistration starts the passkey registration ceremony.
// Returns the PublicKeyCredentialCreationOptions as JSON.
func (s *Service) BeginRegistration(appID, userID uuid.UUID) (json.RawMessage, *errors.AppError) {
	// Verify the app allows passkeys
	app, appErr := s.getAppWithPasskeyCheck(appID)
	if appErr != nil {
		return nil, appErr
	}

	// Fetch user
	usr, err := s.UserRepo.GetUserByID(userID.String())
	if err != nil {
		return nil, errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	// Fetch existing credentials to exclude from registration
	existingCreds, err := s.Repo.GetCredentialsByUserAndApp(userID, appID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to fetch existing credentials")
	}

	// Create WebAuthn user adapter
	webauthnUser := &WebAuthnUser{
		User:        usr,
		Credentials: existingCreds,
	}

	// Get WebAuthn instance for this app
	wan, err := GetWebAuthn(s.DB, app)
	if err != nil {
		log.Printf("Failed to initialize WebAuthn: %v", err)
		return nil, errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured")
	}

	// Begin registration ceremony
	options, session, err := wan.BeginRegistration(webauthnUser)
	if err != nil {
		log.Printf("Failed to begin WebAuthn registration: %v", err)
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to start passkey registration")
	}

	// Store session data in Redis
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to serialize session")
	}

	if err := redis.SetWebAuthnRegistrationChallenge(appID.String(), userID.String(), string(sessionJSON), challengeTTL); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to store registration challenge")
	}

	// Serialize options
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to serialize options")
	}

	return optionsJSON, nil
}

// FinishRegistration completes the passkey registration ceremony.
func (s *Service) FinishRegistration(appID, userID uuid.UUID, credentialName string, credentialJSON json.RawMessage) *errors.AppError {
	// Verify app
	app, appErr := s.getAppWithPasskeyCheck(appID)
	if appErr != nil {
		return appErr
	}

	// Fetch user
	usr, err := s.UserRepo.GetUserByID(userID.String())
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	// Fetch existing credentials
	existingCreds, err := s.Repo.GetCredentialsByUserAndApp(userID, appID)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to fetch existing credentials")
	}

	webauthnUser := &WebAuthnUser{
		User:        usr,
		Credentials: existingCreds,
	}

	// Retrieve session from Redis
	sessionJSON, err := redis.GetWebAuthnRegistrationChallenge(appID.String(), userID.String())
	if err != nil {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired registration session")
	}

	var session gowebauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to parse session data")
	}

	// Get WebAuthn instance
	wan, err := GetWebAuthn(s.DB, app)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured")
	}

	// Parse the credential response
	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(strings.NewReader(string(credentialJSON)))
	if err != nil {
		log.Printf("Failed to parse credential creation response: %v", err)
		return errors.NewAppError(errors.ErrBadRequest, "Invalid credential response")
	}

	// Finish registration
	credential, err := wan.CreateCredential(webauthnUser, session, parsedResponse)
	if err != nil {
		log.Printf("Failed to create WebAuthn credential: %v", err)
		return errors.NewAppError(errors.ErrBadRequest, "Failed to verify passkey registration")
	}

	// Default name if empty
	if credentialName == "" {
		credentialName = fmt.Sprintf("Passkey %d", len(existingCreds)+1)
	}

	// Store credential in database
	dbCred := &models.WebAuthnCredential{
		UserID:          &userID,
		AppID:           &appID,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          credential.Authenticator.AAGUID,
		SignCount:       credential.Authenticator.SignCount,
		Name:            credentialName,
		Transports:      serializeTransports(credential.Transport),
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
	}

	if err := s.Repo.CreateCredential(dbCred); err != nil {
		log.Printf("Failed to store WebAuthn credential: %v", err)
		return errors.NewAppError(errors.ErrInternal, "Failed to save passkey")
	}

	// Clear registration challenge from Redis
	if err := redis.DeleteWebAuthnRegistrationChallenge(appID.String(), userID.String()); err != nil {
		log.Printf("Warning: Failed to delete WebAuthn registration challenge: %v", err)
	}

	return nil
}

// ============================================================================
// 2FA Authentication (Assertion)
// ============================================================================

// BeginLogin starts the passkey assertion ceremony for 2FA verification.
// The user is identified by their temp session token from password login.
func (s *Service) BeginLogin(appID, userID uuid.UUID) (json.RawMessage, *errors.AppError) {
	app, appErr := s.getApp(appID)
	if appErr != nil {
		return nil, appErr
	}

	// Fetch user
	usr, err := s.UserRepo.GetUserByID(userID.String())
	if err != nil {
		return nil, errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	// Fetch credentials for this user+app
	creds, err := s.Repo.GetCredentialsByUserAndApp(userID, appID)
	if err != nil || len(creds) == 0 {
		return nil, errors.NewAppError(errors.ErrBadRequest, "No passkeys registered for this user")
	}

	webauthnUser := &WebAuthnUser{
		User:        usr,
		Credentials: creds,
	}

	wan, err := GetWebAuthn(s.DB, app)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured")
	}

	// Begin login ceremony (scoped to user's credentials)
	options, session, err := wan.BeginLogin(webauthnUser)
	if err != nil {
		log.Printf("Failed to begin WebAuthn login: %v", err)
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to start passkey verification")
	}

	// Store session in Redis
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to serialize session")
	}

	if err := redis.SetWebAuthnLoginChallenge(appID.String(), userID.String(), string(sessionJSON), challengeTTL); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to store login challenge")
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to serialize options")
	}

	return optionsJSON, nil
}

// FinishLogin completes the passkey assertion ceremony for 2FA verification.
func (s *Service) FinishLogin(appID, userID uuid.UUID, credentialJSON json.RawMessage) *errors.AppError {
	app, appErr := s.getApp(appID)
	if appErr != nil {
		return appErr
	}

	usr, err := s.UserRepo.GetUserByID(userID.String())
	if err != nil {
		return errors.NewAppError(errors.ErrNotFound, "User not found")
	}

	creds, err := s.Repo.GetCredentialsByUserAndApp(userID, appID)
	if err != nil || len(creds) == 0 {
		return errors.NewAppError(errors.ErrBadRequest, "No passkeys registered")
	}

	webauthnUser := &WebAuthnUser{
		User:        usr,
		Credentials: creds,
	}

	sessionJSON, err := redis.GetWebAuthnLoginChallenge(appID.String(), userID.String())
	if err != nil {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired login session")
	}

	var session gowebauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to parse session data")
	}

	wan, err := GetWebAuthn(s.DB, app)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured")
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(strings.NewReader(string(credentialJSON)))
	if err != nil {
		log.Printf("Failed to parse credential request response: %v", err)
		return errors.NewAppError(errors.ErrBadRequest, "Invalid credential response")
	}

	credential, err := wan.ValidateLogin(webauthnUser, session, parsedResponse)
	if err != nil {
		log.Printf("Failed to validate WebAuthn login: %v", err)
		return errors.NewAppError(errors.ErrUnauthorized, "Passkey verification failed")
	}

	// Update sign count in database
	s.updateCredentialAfterLogin(creds, credential)

	// Clear challenge from Redis
	if err := redis.DeleteWebAuthnLoginChallenge(appID.String(), userID.String()); err != nil {
		log.Printf("Warning: Failed to delete WebAuthn login challenge: %v", err)
	}

	return nil
}

// ============================================================================
// Passwordless Login (Discoverable Credentials)
// ============================================================================

// BeginPasswordlessLogin starts a passwordless login ceremony using discoverable credentials.
// Returns the assertion options and a session ID that the client must pass back.
func (s *Service) BeginPasswordlessLogin(appID uuid.UUID) (json.RawMessage, string, *errors.AppError) {
	app, appErr := s.getApp(appID)
	if appErr != nil {
		return nil, "", appErr
	}

	if !app.PasskeyLoginEnabled {
		return nil, "", errors.NewAppError(errors.ErrForbidden, "Passwordless login is not enabled for this application")
	}

	wan, err := GetWebAuthnForPasswordless(s.DB, app)
	if err != nil {
		return nil, "", errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured")
	}

	// Begin discoverable login — no allowCredentials list
	options, session, err := wan.BeginDiscoverableLogin()
	if err != nil {
		log.Printf("Failed to begin WebAuthn discoverable login: %v", err)
		return nil, "", errors.NewAppError(errors.ErrInternal, "Failed to start passwordless login")
	}

	// Generate a session ID for this passwordless flow
	sessionID := uuid.New().String()

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, "", errors.NewAppError(errors.ErrInternal, "Failed to serialize session")
	}

	if err := redis.SetWebAuthnLoginChallenge(appID.String(), sessionID, string(sessionJSON), challengeTTL); err != nil {
		return nil, "", errors.NewAppError(errors.ErrInternal, "Failed to store login challenge")
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, "", errors.NewAppError(errors.ErrInternal, "Failed to serialize options")
	}

	return optionsJSON, sessionID, nil
}

// FinishPasswordlessLogin completes the passwordless login ceremony.
// Returns the authenticated user ID.
func (s *Service) FinishPasswordlessLogin(appID uuid.UUID, sessionID string, credentialJSON json.RawMessage) (string, *errors.AppError) {
	app, appErr := s.getApp(appID)
	if appErr != nil {
		return "", appErr
	}

	if !app.PasskeyLoginEnabled {
		return "", errors.NewAppError(errors.ErrForbidden, "Passwordless login is not enabled for this application")
	}

	sessionJSON, err := redis.GetWebAuthnLoginChallenge(appID.String(), sessionID)
	if err != nil {
		return "", errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired login session")
	}

	var session gowebauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return "", errors.NewAppError(errors.ErrInternal, "Failed to parse session data")
	}

	wan, err := GetWebAuthnForPasswordless(s.DB, app)
	if err != nil {
		return "", errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured")
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(strings.NewReader(string(credentialJSON)))
	if err != nil {
		log.Printf("Failed to parse credential request response: %v", err)
		return "", errors.NewAppError(errors.ErrBadRequest, "Invalid credential response")
	}

	// Discoverable credential handler: resolve user from userHandle
	discoverHandler := func(rawID, userHandle []byte) (gowebauthn.User, error) {
		// userHandle contains the user ID bytes (set during registration via WebAuthnID())
		if len(userHandle) != 16 {
			return nil, fmt.Errorf("invalid user handle length")
		}

		uid, err := uuid.FromBytes(userHandle)
		if err != nil {
			return nil, fmt.Errorf("invalid user handle: %v", err)
		}

		usr, err := s.UserRepo.GetUserByID(uid.String())
		if err != nil {
			return nil, fmt.Errorf("user not found")
		}

		// Verify user belongs to this app
		if usr.AppID != appID {
			return nil, fmt.Errorf("user not found in this application")
		}

		// Verify user is active and email verified
		if !usr.IsActive {
			return nil, fmt.Errorf("account is deactivated")
		}
		if !usr.EmailVerified {
			return nil, fmt.Errorf("email not verified")
		}

		creds, err := s.Repo.GetCredentialsByUserAndApp(uid, appID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch credentials")
		}

		return &WebAuthnUser{
			User:        usr,
			Credentials: creds,
		}, nil
	}

	credential, err := wan.ValidateDiscoverableLogin(discoverHandler, session, parsedResponse)
	if err != nil {
		log.Printf("Failed to validate WebAuthn discoverable login: %v", err)
		return "", errors.NewAppError(errors.ErrUnauthorized, "Passkey verification failed")
	}

	// Find the credential in the DB to get the user ID and update sign count
	dbCred, err := s.Repo.GetCredentialByAppAndCredentialID(appID, credential.ID)
	if err != nil {
		return "", errors.NewAppError(errors.ErrInternal, "Failed to find credential")
	}

	// Update sign count
	if updateErr := s.Repo.UpdateCredentialSignCount(dbCred.ID, credential.Authenticator.SignCount); updateErr != nil {
		log.Printf("Warning: Failed to update sign count for credential %s: %v", dbCred.ID, updateErr)
	}

	// Verify user is active
	usr, err := s.UserRepo.GetUserByID(dbCred.UserID.String())
	if err != nil {
		return "", errors.NewAppError(errors.ErrNotFound, "User not found")
	}
	if !usr.IsActive {
		return "", errors.NewAppError(errors.ErrForbidden, "Account is deactivated")
	}
	if !usr.EmailVerified {
		return "", errors.NewAppError(errors.ErrForbidden, "Email not verified")
	}

	// Clear challenge from Redis
	if err := redis.DeleteWebAuthnLoginChallenge(appID.String(), sessionID); err != nil {
		log.Printf("Warning: Failed to delete WebAuthn login challenge: %v", err)
	}

	return dbCred.UserID.String(), nil
}

// ============================================================================
// Credential Management
// ============================================================================

// ListCredentials returns all passkeys for a user within an app.
func (s *Service) ListCredentials(userID, appID uuid.UUID) ([]models.WebAuthnCredential, *errors.AppError) {
	creds, err := s.Repo.GetCredentialsByUserAndApp(userID, appID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to fetch passkeys")
	}
	return creds, nil
}

// DeleteCredential removes a passkey for a user.
func (s *Service) DeleteCredential(userID, credentialUUID uuid.UUID) *errors.AppError {
	if err := s.Repo.DeleteCredential(credentialUUID, userID); err != nil {
		return errors.NewAppError(errors.ErrNotFound, "Passkey not found")
	}
	return nil
}

// RenameCredential updates the friendly name of a passkey.
func (s *Service) RenameCredential(userID, credentialUUID uuid.UUID, newName string) *errors.AppError {
	if err := s.Repo.RenameCredential(credentialUUID, userID, newName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.NewAppError(errors.ErrNotFound, "Passkey not found")
		}
		return errors.NewAppError(errors.ErrInternal, "Failed to rename passkey")
	}
	return nil
}

// HasPasskeys checks if a user has any passkeys registered for the given app.
func (s *Service) HasPasskeys(userID, appID uuid.UUID) (bool, *errors.AppError) {
	count, err := s.Repo.CountCredentialsByUserAndApp(userID, appID)
	if err != nil {
		return false, errors.NewAppError(errors.ErrInternal, "Failed to check passkeys")
	}
	return count > 0, nil
}

// IsPasskeyAllowed checks if the application allows passkey as a 2FA method.
func (s *Service) IsPasskeyAllowed(appID uuid.UUID) bool {
	var app models.Application
	if err := s.DB.Select("passkey2_fa_enabled, two_fa_methods").First(&app, "id = ?", appID).Error; err != nil {
		return false
	}
	if !app.Passkey2FAEnabled {
		return false
	}
	methods := strings.Split(app.TwoFAMethods, ",")
	for _, m := range methods {
		if strings.TrimSpace(m) == emailpkg.TwoFAMethodPasskey {
			return true
		}
	}
	return false
}

// ============================================================================
// Internal helpers
// ============================================================================

// getApp fetches the application by ID.
func (s *Service) getApp(appID uuid.UUID) (*models.Application, *errors.AppError) {
	var app models.Application
	if err := s.DB.First(&app, "id = ?", appID).Error; err != nil {
		return nil, errors.NewAppError(errors.ErrNotFound, "Application not found")
	}
	return &app, nil
}

// getAppWithPasskeyCheck fetches the app and verifies that passkey support is enabled.
func (s *Service) getAppWithPasskeyCheck(appID uuid.UUID) (*models.Application, *errors.AppError) {
	app, appErr := s.getApp(appID)
	if appErr != nil {
		return nil, appErr
	}

	// Passkey is allowed if either 2FA passkey or passwordless login is enabled
	if !app.Passkey2FAEnabled && !app.PasskeyLoginEnabled {
		return nil, errors.NewAppError(errors.ErrForbidden, "Passkey support is not enabled for this application")
	}

	return app, nil
}

// updateCredentialAfterLogin updates the sign count for the credential that was just used.
func (s *Service) updateCredentialAfterLogin(creds []models.WebAuthnCredential, credential *gowebauthn.Credential) {
	for _, c := range creds {
		if string(c.CredentialID) == string(credential.ID) {
			if updateErr := s.Repo.UpdateCredentialSignCount(c.ID, credential.Authenticator.SignCount); updateErr != nil {
				log.Printf("Warning: Failed to update sign count for credential %s: %v", c.ID, updateErr)
			}
			return
		}
	}
}

// ============================================================================
// Admin Account Passkey Operations
// ============================================================================

// BeginAdminRegistration starts a passkey registration ceremony for an admin account.
func (s *Service) BeginAdminRegistration(admin *models.AdminAccount) (json.RawMessage, *errors.AppError) {
	// Fetch existing admin credentials to exclude from registration
	existingCreds, err := s.Repo.GetCredentialsByAdminID(admin.ID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to fetch existing credentials")
	}

	adminUser := &AdminWebAuthnUser{
		Admin:       admin,
		Credentials: existingCreds,
	}

	wan, err := GetWebAuthnForAdmin()
	if err != nil {
		log.Printf("Failed to initialize admin WebAuthn: %v", err)
		return nil, errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured for admin")
	}

	options, session, err := wan.BeginRegistration(adminUser,
		gowebauthn.WithPublicKeyCredentialHints([]protocol.PublicKeyCredentialHints{
			protocol.PublicKeyCredentialHintClientDevice,
		}),
	)
	if err != nil {
		log.Printf("Failed to begin admin WebAuthn registration: %v", err)
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to start passkey registration")
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to serialize session")
	}

	// Use "admin" as the app scope in Redis key
	if err := redis.SetWebAuthnRegistrationChallenge("admin", admin.ID.String(), string(sessionJSON), challengeTTL); err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to store registration challenge")
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to serialize options")
	}

	return optionsJSON, nil
}

// FinishAdminRegistration completes the passkey registration ceremony for an admin account.
func (s *Service) FinishAdminRegistration(admin *models.AdminAccount, credentialName string, credentialJSON json.RawMessage) *errors.AppError {
	existingCreds, err := s.Repo.GetCredentialsByAdminID(admin.ID)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to fetch existing credentials")
	}

	adminUser := &AdminWebAuthnUser{
		Admin:       admin,
		Credentials: existingCreds,
	}

	sessionJSON, err := redis.GetWebAuthnRegistrationChallenge("admin", admin.ID.String())
	if err != nil {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired registration session")
	}

	var session gowebauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to parse session data")
	}

	wan, err := GetWebAuthnForAdmin()
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured for admin")
	}

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(strings.NewReader(string(credentialJSON)))
	if err != nil {
		log.Printf("Failed to parse admin credential creation response: %v", err)
		return errors.NewAppError(errors.ErrBadRequest, "Invalid credential response")
	}

	credential, err := wan.CreateCredential(adminUser, session, parsedResponse)
	if err != nil {
		log.Printf("Failed to create admin WebAuthn credential: %v", err)
		return errors.NewAppError(errors.ErrBadRequest, "Failed to verify passkey registration")
	}

	if credentialName == "" {
		credentialName = fmt.Sprintf("Admin Passkey %d", len(existingCreds)+1)
	}

	adminID := admin.ID
	dbCred := &models.WebAuthnCredential{
		AdminID:         &adminID,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          credential.Authenticator.AAGUID,
		SignCount:       credential.Authenticator.SignCount,
		Name:            credentialName,
		Transports:      serializeTransports(credential.Transport),
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
	}

	if err := s.Repo.CreateCredential(dbCred); err != nil {
		log.Printf("Failed to store admin WebAuthn credential: %v", err)
		return errors.NewAppError(errors.ErrInternal, "Failed to save passkey")
	}

	if err := redis.DeleteWebAuthnRegistrationChallenge("admin", admin.ID.String()); err != nil {
		log.Printf("Warning: Failed to delete admin WebAuthn registration challenge: %v", err)
	}

	return nil
}

// ListAdminCredentials returns all passkeys for an admin account.
func (s *Service) ListAdminCredentials(adminID uuid.UUID) ([]models.WebAuthnCredential, *errors.AppError) {
	creds, err := s.Repo.GetCredentialsByAdminID(adminID)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to fetch passkeys")
	}
	return creds, nil
}

// DeleteAdminCredential removes an admin passkey.
func (s *Service) DeleteAdminCredential(adminID, credentialUUID uuid.UUID) *errors.AppError {
	if err := s.Repo.DeleteAdminCredential(credentialUUID, adminID); err != nil {
		return errors.NewAppError(errors.ErrNotFound, "Passkey not found")
	}
	return nil
}

// RenameAdminCredential updates the friendly name of an admin passkey.
func (s *Service) RenameAdminCredential(adminID, credentialUUID uuid.UUID, newName string) *errors.AppError {
	if err := s.Repo.RenameAdminCredential(credentialUUID, adminID, newName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.NewAppError(errors.ErrNotFound, "Passkey not found")
		}
		return errors.NewAppError(errors.ErrInternal, "Failed to rename passkey")
	}
	return nil
}

// ============================================================================
// Admin Passkey Login (Discoverable Credentials)
// ============================================================================

// BeginAdminLogin starts a passwordless login ceremony for admin accounts
// using discoverable credentials. Returns the assertion options and a session ID.
func (s *Service) BeginAdminLogin() (json.RawMessage, string, *errors.AppError) {
	wan, err := GetWebAuthnForAdminLogin()
	if err != nil {
		log.Printf("Failed to initialize admin WebAuthn for login: %v", err)
		return nil, "", errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured for admin")
	}

	// Begin discoverable login — no allowCredentials list
	options, session, err := wan.BeginDiscoverableLogin(
		gowebauthn.WithAssertionPublicKeyCredentialHints([]protocol.PublicKeyCredentialHints{
			protocol.PublicKeyCredentialHintClientDevice,
		}),
	)
	if err != nil {
		log.Printf("Failed to begin admin WebAuthn discoverable login: %v", err)
		return nil, "", errors.NewAppError(errors.ErrInternal, "Failed to start passkey login")
	}

	// Generate a session ID for this login flow
	sessionID := uuid.New().String()

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, "", errors.NewAppError(errors.ErrInternal, "Failed to serialize session")
	}

	// Use "admin" as the app scope in Redis key
	if err := redis.SetWebAuthnLoginChallenge("admin", sessionID, string(sessionJSON), challengeTTL); err != nil {
		return nil, "", errors.NewAppError(errors.ErrInternal, "Failed to store login challenge")
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, "", errors.NewAppError(errors.ErrInternal, "Failed to serialize options")
	}

	return optionsJSON, sessionID, nil
}

// FinishAdminLogin completes the passwordless login ceremony for admin accounts.
// Returns the authenticated admin account ID.
func (s *Service) FinishAdminLogin(sessionID string, credentialJSON json.RawMessage) (uuid.UUID, *errors.AppError) {
	if s.AdminLookup == nil {
		return uuid.Nil, errors.NewAppError(errors.ErrInternal, "Admin lookup not configured")
	}

	sessionData, err := redis.GetWebAuthnLoginChallenge("admin", sessionID)
	if err != nil {
		return uuid.Nil, errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired login session")
	}

	var session gowebauthn.SessionData
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return uuid.Nil, errors.NewAppError(errors.ErrInternal, "Failed to parse session data")
	}

	wan, err := GetWebAuthnForAdminLogin()
	if err != nil {
		return uuid.Nil, errors.NewAppError(errors.ErrInternal, "WebAuthn is not configured for admin")
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(strings.NewReader(string(credentialJSON)))
	if err != nil {
		log.Printf("Failed to parse admin credential request response: %v", err)
		return uuid.Nil, errors.NewAppError(errors.ErrBadRequest, "Invalid credential response")
	}

	// Discoverable credential handler: resolve admin from userHandle
	discoverHandler := func(rawID, userHandle []byte) (gowebauthn.User, error) {
		// userHandle contains the admin's UUID bytes (set during registration via AdminWebAuthnUser.WebAuthnID())
		if len(userHandle) != 16 {
			return nil, fmt.Errorf("invalid user handle length")
		}

		adminUID, err := uuid.FromBytes(userHandle)
		if err != nil {
			return nil, fmt.Errorf("invalid user handle: %v", err)
		}

		admin, err := s.AdminLookup(adminUID.String())
		if err != nil {
			return nil, fmt.Errorf("admin account not found")
		}

		creds, err := s.Repo.GetCredentialsByAdminID(adminUID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch credentials")
		}

		return &AdminWebAuthnUser{
			Admin:       admin,
			Credentials: creds,
		}, nil
	}

	credential, err := wan.ValidateDiscoverableLogin(discoverHandler, session, parsedResponse)
	if err != nil {
		log.Printf("Failed to validate admin WebAuthn discoverable login: %v", err)
		return uuid.Nil, errors.NewAppError(errors.ErrUnauthorized, "Passkey verification failed")
	}

	// Find the credential in the DB to get the admin ID and update sign count
	// Use rawID from the validated credential to look up the DB record
	dbCred, err := s.Repo.GetCredentialByCredentialID(credential.ID)
	if err != nil {
		return uuid.Nil, errors.NewAppError(errors.ErrInternal, "Failed to find credential")
	}

	// Verify this credential belongs to an admin account
	if dbCred.AdminID == nil {
		return uuid.Nil, errors.NewAppError(errors.ErrUnauthorized, "Credential is not an admin passkey")
	}

	// Update sign count
	if updateErr := s.Repo.UpdateCredentialSignCount(dbCred.ID, credential.Authenticator.SignCount); updateErr != nil {
		log.Printf("Warning: Failed to update sign count for credential %s: %v", dbCred.ID, updateErr)
	}

	// Clear challenge from Redis
	if err := redis.DeleteWebAuthnLoginChallenge("admin", sessionID); err != nil {
		log.Printf("Warning: Failed to delete admin WebAuthn login challenge: %v", err)
	}

	return *dbCred.AdminID, nil
}
