package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gjovanovicst/auth_api/internal/redis"
	pkgjwt "github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// RoleLookupFunc is a callback to fetch a user's role names for a given app.
// Injected from main.go to avoid an import cycle with the rbac package.
type RoleLookupFunc func(appID, userID string) ([]string, error)

// Service orchestrates all OIDC provider logic.
type Service struct {
	repo       *Repository
	roleLookup RoleLookupFunc
}

// NewService constructs the OIDC Service.
func NewService(repo *Repository, roleLookup RoleLookupFunc) *Service {
	return &Service{repo: repo, roleLookup: roleLookup}
}

// ─── Key management ────────────────────────────────────────────────────────────

// GetOrCreateRSAKey returns the RSA private key for the given application.
// If the application does not yet have a key, one is generated and persisted.
func (s *Service) GetOrCreateRSAKey(appID uuid.UUID) (*rsa.PrivateKey, error) {
	app, err := s.repo.GetApplication(appID)
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}

	if app.OIDCRSAPrivateKey != "" {
		key, err := PEMToPrivateKey(app.OIDCRSAPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("parse stored rsa key: %w", err)
		}
		return key, nil
	}

	// Generate and persist a new key
	key, err := GenerateRSAKey()
	if err != nil {
		return nil, fmt.Errorf("generate rsa key: %w", err)
	}
	pem, err := PrivateKeyToPEM(key)
	if err != nil {
		return nil, fmt.Errorf("encode rsa key: %w", err)
	}
	if err := s.repo.SaveRSAPrivateKey(appID, pem); err != nil {
		return nil, fmt.Errorf("save rsa key: %w", err)
	}
	log.Printf("[OIDC] Generated RSA key for application %s", appID)
	return key, nil
}

// IssuerURL returns the canonical OIDC issuer URL for the given application.
// If the application has a custom issuer URL configured, that is returned.
// Otherwise the URL is constructed from PUBLIC_URL + app ID.
func (s *Service) IssuerURL(app *models.Application) string {
	if app.OIDCIssuerURL != "" {
		return app.OIDCIssuerURL
	}
	base := strings.TrimRight(viper.GetString("PUBLIC_URL"), "/")
	return fmt.Sprintf("%s/oidc/%s", base, app.ID.String())
}

// ─── OIDC Client management ────────────────────────────────────────────────────

// CreateClient registers a new OIDC client for the given application.
// Returns the client model *and* the plain-text secret (shown only once).
func (s *Service) CreateClient(appID uuid.UUID, name, description, redirectURIs, grantTypes, scopes string, requireConsent, isConfidential, pkceRequired bool, logoURL string) (*models.OIDCClient, string, error) {
	clientID := generateClientID()
	plainSecret, hash, err := generateClientSecret()
	if err != nil {
		return nil, "", fmt.Errorf("generate client secret: %w", err)
	}

	client := &models.OIDCClient{
		AppID:             appID,
		Name:              name,
		Description:       description,
		ClientID:          clientID,
		ClientSecretHash:  hash,
		RedirectURIs:      redirectURIs,
		AllowedGrantTypes: grantTypes,
		AllowedScopes:     scopes,
		RequireConsent:    requireConsent,
		IsConfidential:    isConfidential,
		PKCERequired:      pkceRequired,
		LogoURL:           logoURL,
		IsActive:          true,
	}
	if err := s.repo.CreateClient(client); err != nil {
		return nil, "", fmt.Errorf("create oidc client: %w", err)
	}
	return client, plainSecret, nil
}

// GetClient fetches a single OIDC client by UUID.
func (s *Service) GetClient(id uuid.UUID) (*models.OIDCClient, error) {
	return s.repo.GetClientByID(id)
}

// ListClients returns all OIDC clients for a given application.
func (s *Service) ListClients(appID uuid.UUID) ([]models.OIDCClient, error) {
	return s.repo.ListClientsByApp(appID)
}

// UpdateClient applies partial updates to an OIDC client.
func (s *Service) UpdateClient(id uuid.UUID, name, description, redirectURIs, grantTypes, scopes, logoURL string, requireConsent, isConfidential, pkceRequired, isActive *bool) (*models.OIDCClient, error) {
	client, err := s.repo.GetClientByID(id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		client.Name = name
	}
	if description != "" {
		client.Description = description
	}
	if redirectURIs != "" {
		client.RedirectURIs = redirectURIs
	}
	if grantTypes != "" {
		client.AllowedGrantTypes = grantTypes
	}
	if scopes != "" {
		client.AllowedScopes = scopes
	}
	if logoURL != "" {
		client.LogoURL = logoURL
	}
	if requireConsent != nil {
		client.RequireConsent = *requireConsent
	}
	if isConfidential != nil {
		client.IsConfidential = *isConfidential
	}
	if pkceRequired != nil {
		client.PKCERequired = *pkceRequired
	}
	if isActive != nil {
		client.IsActive = *isActive
	}
	if err := s.repo.UpdateClient(client); err != nil {
		return nil, err
	}
	return client, nil
}

// DeleteClient removes an OIDC client.
func (s *Service) DeleteClient(id uuid.UUID) error {
	return s.repo.DeleteClient(id)
}

// RotateClientSecret generates a new secret for the client and returns the plain-text value.
func (s *Service) RotateClientSecret(id uuid.UUID) (*models.OIDCClient, string, error) {
	client, err := s.repo.GetClientByID(id)
	if err != nil {
		return nil, "", err
	}
	plain, hash, err := generateClientSecret()
	if err != nil {
		return nil, "", fmt.Errorf("generate secret: %w", err)
	}
	client.ClientSecretHash = hash
	if err := s.repo.UpdateClient(client); err != nil {
		return nil, "", err
	}
	return client, plain, nil
}

// ─── Authorization Code flow ───────────────────────────────────────────────────

// ValidateAuthRequest validates the authorization request parameters and returns
// the client record. It does NOT yet issue a code or redirect.
func (s *Service) ValidateAuthRequest(clientID, redirectURI, responseType string, scopes []string) (*models.OIDCClient, error) {
	client, err := s.repo.GetClientByClientID(clientID)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("unknown client_id")
		}
		return nil, err
	}
	if !client.IsActive {
		return nil, fmt.Errorf("client is disabled")
	}
	if responseType != "code" {
		return nil, fmt.Errorf("unsupported response_type: only 'code' is supported")
	}
	if !isRedirectURIAllowed(client.RedirectURIs, redirectURI) {
		return nil, fmt.Errorf("redirect_uri not allowed")
	}
	if !containsScope(client.AllowedScopes, "openid") || !sliceContains(scopes, "openid") {
		return nil, fmt.Errorf("scope must include 'openid'")
	}
	return client, nil
}

// IssueAuthCode creates and persists a new authorization code after user consent.
func (s *Service) IssueAuthCode(appID uuid.UUID, clientID string, userID uuid.UUID, redirectURI, scopeStr, nonce, codeChallenge, codeChallengeMethod string) (*models.OIDCAuthCode, error) {
	codeBytes := make([]byte, 32)
	if _, err := rand.Read(codeBytes); err != nil {
		return nil, fmt.Errorf("generate auth code: %w", err)
	}
	code := hex.EncodeToString(codeBytes)

	ttl := time.Minute * time.Duration(viper.GetInt("OIDC_AUTH_CODE_EXPIRATION_MINUTES"))
	if ttl == 0 {
		ttl = 10 * time.Minute
	}

	ac := &models.OIDCAuthCode{
		AppID:               appID,
		ClientID:            clientID,
		UserID:              userID,
		Code:                code,
		RedirectURI:         redirectURI,
		Scopes:              scopeStr,
		Nonce:               nonce,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		ExpiresAt:           time.Now().Add(ttl),
	}
	if err := s.repo.CreateAuthCode(ac); err != nil {
		return nil, err
	}
	return ac, nil
}

// ExchangeCode validates an authorization code and returns the user + auth code record.
// The caller is responsible for minting tokens after this returns successfully.
func (s *Service) ExchangeCode(code, clientID, redirectURI, codeVerifier string) (*models.OIDCAuthCode, *models.User, error) {
	ac, err := s.repo.GetAuthCode(code)
	if err != nil {
		if isNotFound(err) {
			return nil, nil, fmt.Errorf("invalid_grant: code not found")
		}
		return nil, nil, err
	}
	if ac.Used {
		return nil, nil, fmt.Errorf("invalid_grant: code already used")
	}
	if time.Now().After(ac.ExpiresAt) {
		return nil, nil, fmt.Errorf("invalid_grant: code expired")
	}
	if ac.ClientID != clientID {
		return nil, nil, fmt.Errorf("invalid_grant: client_id mismatch")
	}
	if ac.RedirectURI != redirectURI {
		return nil, nil, fmt.Errorf("invalid_grant: redirect_uri mismatch")
	}

	// PKCE verification
	if ac.CodeChallenge != "" {
		if codeVerifier == "" {
			return nil, nil, fmt.Errorf("invalid_grant: code_verifier required")
		}
		if !verifyPKCE(ac.CodeChallenge, ac.CodeChallengeMethod, codeVerifier) {
			return nil, nil, fmt.Errorf("invalid_grant: code_verifier mismatch")
		}
	}

	// Mark code as used
	if err := s.repo.MarkAuthCodeUsed(ac.ID); err != nil {
		return nil, nil, fmt.Errorf("mark code used: %w", err)
	}

	user, err := s.repo.GetUserByID(ac.UserID.String())
	if err != nil {
		return nil, nil, fmt.Errorf("load user: %w", err)
	}
	return ac, user, nil
}

// MintTokensForUser generates HS256 access + refresh tokens (reusing existing jwt pkg)
// plus an RS256 ID token. Returns access, refresh, id tokens and expiry in seconds.
func (s *Service) MintTokensForUser(app *models.Application, client *models.OIDCClient, user *models.User, scopes []string, nonce string) (accessToken, refreshToken, idToken string, expiresIn int, err error) {
	roles, _ := s.roleLookup(app.ID.String(), user.ID.String())

	// Reuse existing session token infrastructure (HS256)
	sessionID := uuid.New().String()
	accessToken, err = pkgjwt.GenerateAccessToken(app.ID.String(), user.ID.String(), sessionID, roles)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, err = pkgjwt.GenerateRefreshToken(app.ID.String(), user.ID.String(), sessionID, roles)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("generate refresh token: %w", err)
	}

	// Persist session in Redis so AuthMiddleware's SessionExists check passes.
	sessionTTL := time.Hour * time.Duration(viper.GetInt("REFRESH_TOKEN_EXPIRATION_HOURS"))
	if sessionTTL <= 0 {
		sessionTTL = 720 * time.Hour
	}
	if err := redis.CreateSession(app.ID.String(), sessionID, user.ID.String(), refreshToken, "", "", sessionTTL); err != nil {
		return "", "", "", 0, fmt.Errorf("create session: %w", err)
	}

	// RS256 ID token
	rsaKey, err := s.GetOrCreateRSAKey(app.ID)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("get rsa key: %w", err)
	}
	ttlSec := app.OIDCIDTokenTTL
	if ttlSec <= 0 {
		ttlSec = viper.GetInt("OIDC_ID_TOKEN_EXPIRATION_MINUTES") * 60
	}
	if ttlSec <= 0 {
		ttlSec = 3600
	}
	idToken, err = MintIDToken(MintIDTokenParams{
		Issuer:   s.IssuerURL(app),
		Audience: client.ClientID,
		User:     user,
		Roles:    roles,
		Scopes:   scopes,
		Nonce:    nonce,
		TTL:      time.Duration(ttlSec) * time.Second,
		Kid:      app.ID.String(),
		Key:      rsaKey,
	})
	if err != nil {
		return "", "", "", 0, fmt.Errorf("mint id token: %w", err)
	}

	accessTTL := viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES") * 60
	if accessTTL <= 0 {
		accessTTL = 900
	}
	return accessToken, refreshToken, idToken, accessTTL, nil
}

// ─── Client credentials grant ─────────────────────────────────────────────────

// ClientCredentialsGrant validates client credentials and returns an access token.
// No user context — used for machine-to-machine auth.
func (s *Service) ClientCredentialsGrant(app *models.Application, clientID, clientSecret, scopeStr string) (accessToken string, expiresIn int, err error) {
	client, err := s.repo.GetClientByClientID(clientID)
	if err != nil {
		if isNotFound(err) {
			return "", 0, fmt.Errorf("invalid_client: unknown client")
		}
		return "", 0, err
	}
	if !client.IsActive {
		return "", 0, fmt.Errorf("invalid_client: client disabled")
	}
	if !containsGrantType(client.AllowedGrantTypes, "client_credentials") {
		return "", 0, fmt.Errorf("unauthorized_client: grant type not allowed")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(client.ClientSecretHash), []byte(clientSecret)); err != nil {
		return "", 0, fmt.Errorf("invalid_client: bad credentials")
	}

	sessionID := uuid.New().String()
	accessToken, err = pkgjwt.GenerateAccessToken(app.ID.String(), client.ID.String(), sessionID, nil)
	if err != nil {
		return "", 0, fmt.Errorf("generate access token: %w", err)
	}
	accessTTL := viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES") * 60
	if accessTTL <= 0 {
		accessTTL = 900
	}
	return accessToken, accessTTL, nil
}

// ─── UserInfo ─────────────────────────────────────────────────────────────────

// GetUserInfo returns user claims for a given user + scopes.
func (s *Service) GetUserInfo(app *models.Application, accessToken string, scopes []string) (*models.User, error) {
	claims, err := pkgjwt.ParseToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid_token")
	}
	user, err := s.repo.GetUserByID(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	if user.AppID != app.ID {
		return nil, fmt.Errorf("invalid_token")
	}
	return user, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func generateClientID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateClientSecret() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(b)
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}
	return plain, string(hashed), nil
}

func isNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func isRedirectURIAllowed(allowedJSON, redirectURI string) bool {
	// Simple contains check — the stored value is a JSON array string
	return strings.Contains(allowedJSON, redirectURI)
}

func containsScope(allowedCSV, scope string) bool {
	for _, s := range strings.Split(allowedCSV, ",") {
		if strings.TrimSpace(s) == scope {
			return true
		}
	}
	return false
}

func containsGrantType(allowedCSV, grantType string) bool {
	for _, g := range strings.Split(allowedCSV, ",") {
		if strings.TrimSpace(g) == grantType {
			return true
		}
	}
	return false
}

func sliceContains(slice []string, val string) bool {
	for _, s := range slice {
		if strings.TrimSpace(s) == val {
			return true
		}
	}
	return false
}

// verifyPKCE checks a PKCE code_verifier against a stored code_challenge (RFC 7636).
func verifyPKCE(challenge, method, verifier string) bool {
	switch strings.ToUpper(method) {
	case "S256":
		h := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(h[:])
		return computed == challenge
	default:
		// plain (discouraged but spec-compliant)
		return verifier == challenge
	}
}
