package dto

// ─── Admin API DTOs ────────────────────────────────────────────────────────────

// CreateOIDCClientRequest is the payload for POST /admin/oidc/apps/:id/clients
type CreateOIDCClientRequest struct {
	Name              string `json:"name" validate:"required,max=100"`
	Description       string `json:"description,omitempty"`
	RedirectURIs      string `json:"redirect_uris" validate:"required"`       // JSON array string, e.g. '["https://app.example.com/cb"]'
	AllowedGrantTypes string `json:"allowed_grant_types" validate:"required"` // comma-separated
	AllowedScopes     string `json:"allowed_scopes" validate:"required"`      // comma-separated
	RequireConsent    bool   `json:"require_consent"`
	IsConfidential    bool   `json:"is_confidential"`
	PKCERequired      bool   `json:"pkce_required"`
	LogoURL           string `json:"logo_url,omitempty"`
	// LoginTheme controls the color scheme of OIDC pages: "app" (inherit from Application),
	// "auto" (default, follow OS preference), "light", or "dark".
	LoginTheme string `json:"login_theme,omitempty"`
	// LoginPrimaryColor overrides Bootstrap's default primary color (e.g. "#4f46e5"). Empty = Bootstrap default.
	LoginPrimaryColor string `json:"login_primary_color,omitempty"`
}

// UpdateOIDCClientRequest is the payload for PUT /admin/oidc/apps/:id/clients/:cid
type UpdateOIDCClientRequest struct {
	Name              string `json:"name" validate:"omitempty,max=100"`
	Description       string `json:"description,omitempty"`
	RedirectURIs      string `json:"redirect_uris,omitempty"`
	AllowedGrantTypes string `json:"allowed_grant_types,omitempty"`
	AllowedScopes     string `json:"allowed_scopes,omitempty"`
	RequireConsent    *bool  `json:"require_consent,omitempty"`
	IsConfidential    *bool  `json:"is_confidential,omitempty"`
	PKCERequired      *bool  `json:"pkce_required,omitempty"`
	LogoURL           string `json:"logo_url,omitempty"`
	IsActive          *bool  `json:"is_active,omitempty"`
	// LoginTheme controls the color scheme of OIDC pages: "app" (inherit from Application),
	// "auto", "light", or "dark".
	LoginTheme string `json:"login_theme,omitempty"`
	// LoginPrimaryColor overrides Bootstrap's default primary color. Empty = Bootstrap default.
	LoginPrimaryColor string `json:"login_primary_color,omitempty"`
}

// OIDCClientResponse is the read-only view returned by the admin API.
// The plain-text secret is only included once, on creation or rotation.
type OIDCClientResponse struct {
	ID                string `json:"id"`
	AppID             string `json:"app_id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	ClientID          string `json:"client_id"`
	ClientSecret      string `json:"client_secret,omitempty"` // #nosec G101 -- only present on create/rotate
	RedirectURIs      string `json:"redirect_uris"`
	AllowedGrantTypes string `json:"allowed_grant_types"`
	AllowedScopes     string `json:"allowed_scopes"`
	RequireConsent    bool   `json:"require_consent"`
	IsConfidential    bool   `json:"is_confidential"`
	PKCERequired      bool   `json:"pkce_required"`
	LogoURL           string `json:"logo_url"`
	// LoginTheme: "app" (inherit from Application), "auto", "light", or "dark".
	LoginTheme        string `json:"login_theme"`
	LoginPrimaryColor string `json:"login_primary_color"`
	IsActive          bool   `json:"is_active"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// ─── OIDC Discovery ────────────────────────────────────────────────────────────

// OIDCDiscoveryDocument is the /.well-known/openid-configuration response.
type OIDCDiscoveryDocument struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	EndSessionEndpoint                string   `json:"end_session_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
}

// OIDCRevokeRequest is the form-encoded body for POST /oidc/:app_id/revoke (RFC 7009)
type OIDCRevokeRequest struct {
	Token         string `form:"token" validate:"required"`
	TokenTypeHint string `form:"token_type_hint"` // "access_token" or "refresh_token"
	ClientID      string `form:"client_id"`
	ClientSecret  string `form:"client_secret"` // #nosec G101 -- DTO field
}

// ─── Token endpoint ────────────────────────────────────────────────────────────

// OIDCTokenRequest is the form-encoded body sent to POST /oidc/:app_id/token
type OIDCTokenRequest struct {
	GrantType    string `form:"grant_type"`
	Code         string `form:"code"`
	RedirectURI  string `form:"redirect_uri"`
	ClientID     string `form:"client_id"`
	ClientSecret string `form:"client_secret"` // #nosec G101 -- DTO field
	// PKCE
	CodeVerifier string `form:"code_verifier"`
	// Refresh token grant
	RefreshToken string `form:"refresh_token"` // #nosec G101 -- DTO field
	// Client credentials grant
	Scope string `form:"scope"`
}

// OIDCTokenResponse is returned by the token endpoint.
type OIDCTokenResponse struct {
	AccessToken  string `json:"access_token"`            // #nosec G101 -- DTO field
	TokenType    string `json:"token_type"`              // always "Bearer"
	ExpiresIn    int    `json:"expires_in"`              // seconds
	IDToken      string `json:"id_token,omitempty"`      // #nosec G101 -- DTO field
	RefreshToken string `json:"refresh_token,omitempty"` // #nosec G101 -- DTO field
	Scope        string `json:"scope,omitempty"`
}

// OIDCTokenErrorResponse is the RFC 6749 error object for the token endpoint.
type OIDCTokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// ─── Introspection endpoint (RFC 7662) ────────────────────────────────────────

// OIDCIntrospectRequest is the form-encoded body for POST /oidc/:app_id/introspect
type OIDCIntrospectRequest struct {
	Token         string `form:"token" validate:"required"`
	TokenTypeHint string `form:"token_type_hint"`
}

// OIDCIntrospectResponse is the response from the introspection endpoint.
type OIDCIntrospectResponse struct {
	Active    bool     `json:"active"`
	Sub       string   `json:"sub,omitempty"`
	ClientID  string   `json:"client_id,omitempty"`
	Scope     string   `json:"scope,omitempty"`
	Exp       int64    `json:"exp,omitempty"`
	Iat       int64    `json:"iat,omitempty"`
	Iss       string   `json:"iss,omitempty"`
	Aud       []string `json:"aud,omitempty"`
	TokenType string   `json:"token_type,omitempty"`
}

// ─── UserInfo endpoint ─────────────────────────────────────────────────────────

// OIDCUserInfoResponse is returned by GET/POST /oidc/:app_id/userinfo
type OIDCUserInfoResponse struct {
	Sub           string   `json:"sub"`
	Name          string   `json:"name,omitempty"`
	GivenName     string   `json:"given_name,omitempty"`
	FamilyName    string   `json:"family_name,omitempty"`
	Picture       string   `json:"picture,omitempty"`
	Email         string   `json:"email,omitempty"`
	EmailVerified bool     `json:"email_verified,omitempty"`
	Locale        string   `json:"locale,omitempty"`
	Roles         []string `json:"roles,omitempty"`
}

// ─── Authorize endpoint ────────────────────────────────────────────────────────

// OIDCAuthorizeRequest contains the query parameters for GET /oidc/:app_id/authorize
type OIDCAuthorizeRequest struct {
	ResponseType        string `form:"response_type"`
	ClientID            string `form:"client_id"`
	RedirectURI         string `form:"redirect_uri"`
	Scope               string `form:"scope"`
	State               string `form:"state"`
	Nonce               string `form:"nonce"`
	CodeChallenge       string `form:"code_challenge"`
	CodeChallengeMethod string `form:"code_challenge_method"`
}

// OIDCConsentSubmitRequest is the form body for POST /oidc/:app_id/authorize (consent grant)
type OIDCConsentSubmitRequest struct {
	ConsentToken string `form:"consent_token" validate:"required"` // #nosec G101 -- CSRF-like token
	Action       string `form:"action"`                            // "approve" or "deny"
}
