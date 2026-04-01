package oidc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/health"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	pkgjwt "github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

// Handler holds HTTP handlers for all OIDC endpoints.
type Handler struct {
	Service *Service
	Repo    *Repository
	// GroupLogoutFunc, if set, is called on RP-initiated logout to revoke the
	// user's sessions in all peer apps that share the same session group.
	// Signature mirrors userService.GroupLogoutFunc: (appID, userEmail string).
	GroupLogoutFunc func(appID, userEmail string)
}

// NewHandler constructs the OIDC Handler.
func NewHandler(svc *Service, repo *Repository) *Handler {
	return &Handler{Service: svc, Repo: repo}
}

// ─── Discovery ─────────────────────────────────────────────────────────────────

// WellKnownConfiguration handles GET /oidc/:app_id/.well-known/openid-configuration
// @Summary OIDC discovery document
// @Tags OIDC
// @Produce json
// @Param app_id path string true "Application UUID"
// @Success 200 {object} dto.OIDCDiscoveryDocument
// @Failure 404 {object} dto.ErrorResponse
// @Router /oidc/{app_id}/.well-known/openid-configuration [get]
func (h *Handler) WellKnownConfiguration(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}

	issuer := h.Service.IssuerURL(app)
	base := strings.TrimRight(viper.GetString("PUBLIC_URL"), "/")
	prefix := fmt.Sprintf("%s/oidc/%s", base, app.ID.String())

	doc := dto.OIDCDiscoveryDocument{
		Issuer:                            issuer,
		AuthorizationEndpoint:             prefix + "/authorize",
		TokenEndpoint:                     prefix + "/token",
		UserinfoEndpoint:                  prefix + "/userinfo",
		JwksURI:                           prefix + "/.well-known/jwks.json",
		IntrospectionEndpoint:             prefix + "/introspect",
		RevocationEndpoint:                prefix + "/revoke",
		EndSessionEndpoint:                prefix + "/end_session",
		ResponseTypesSupported:            []string{"code"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		ScopesSupported:                   []string{"openid", "profile", "email", "roles", "offline_access"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post", "none"},
		GrantTypesSupported:               []string{"authorization_code", "client_credentials", "refresh_token"},
		ClaimsSupported:                   []string{"sub", "iss", "aud", "exp", "iat", "nonce", "name", "given_name", "family_name", "email", "email_verified", "picture", "locale", "roles"},
		CodeChallengeMethodsSupported:     []string{"S256"},
	}
	c.JSON(http.StatusOK, doc)
}

// JWKS handles GET /oidc/:app_id/.well-known/jwks.json
// @Summary JWKS endpoint
// @Tags OIDC
// @Produce json
// @Param app_id path string true "Application UUID"
// @Success 200 {object} JWKS
// @Router /oidc/{app_id}/.well-known/jwks.json [get]
func (h *Handler) JWKS(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}
	key, err := h.Service.GetOrCreateRSAKey(app.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "key unavailable"})
		return
	}
	jwk := PublicKeyToJWK(&key.PublicKey, app.ID.String())
	c.JSON(http.StatusOK, JWKS{Keys: []JWK{jwk}})
}

// ─── Authorize endpoint ────────────────────────────────────────────────────────

// Authorize handles GET /oidc/:app_id/authorize
// Validates the authorization request and either renders the login page (if not
// authenticated) or the consent page.
// @Summary OIDC authorization endpoint
// @Tags OIDC
// @Param app_id path string true "Application UUID"
// @Param response_type query string true "Must be 'code'"
// @Param client_id query string true "OIDC client ID"
// @Param redirect_uri query string true "Registered redirect URI"
// @Param scope query string true "Requested scopes (must include 'openid')"
// @Param state query string false "Opaque state value"
// @Param nonce query string false "Nonce"
// @Param code_challenge query string false "PKCE code challenge"
// @Param code_challenge_method query string false "PKCE method (S256)"
// @Router /oidc/{app_id}/authorize [get]
func (h *Handler) Authorize(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}
	if !app.OIDCEnabled {
		h.renderError(c, app, "OIDC is not enabled for this application")
		return
	}

	var req dto.OIDCAuthorizeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.renderError(c, app, "invalid request parameters")
		return
	}

	scopes := strings.Fields(req.Scope)
	client, err := h.Service.ValidateAuthRequest(req.ClientID, req.RedirectURI, req.ResponseType, scopes, req.CodeChallenge)
	if err != nil {
		h.renderError(c, app, err.Error())
		return
	}

	// Build a short-lived consent token that encodes the full auth request so
	// the POST handler can reconstruct it without storing state server-side.
	consentToken := h.buildConsentToken(app.ID, req)

	// Check if the session cookie has an authenticated user
	userID := h.sessionUserID(c, app.ID.String())
	uiThemeOverride := c.Query("ui_theme")
	theme, primaryColor := clientThemeWithOverride(client, app, uiThemeOverride)
	if userID == "" {
		// Not logged in — render the OIDC login page
		c.HTML(http.StatusOK, "oidc_login", gin.H{
			"AppID":        app.ID.String(),
			"AppName":      app.Name,
			"ClientName":   client.Name,
			"ClientLogo":   client.LogoURL,
			"ConsentToken": consentToken,
			"Scopes":       scopes,
			"Error":        "",
			"Theme":        theme,
			"PrimaryColor": primaryColor,
		})
		return
	}

	// Already authenticated — skip to consent (or auto-approve)
	if !client.RequireConsent {
		// Still enforce 2FA even for already-authenticated sessions
		user, err := h.Repo.GetUserByID(userID)
		if err == nil && user.TwoFAEnabled {
			tempToken := uuid.New().String()
			if err := redis.SetTempUserSession(app.ID.String(), tempToken, userID, 10*time.Minute); err != nil {
				h.renderError(c, app, "Failed to create 2FA session")
				return
			}
			redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true",
				req.RedirectURI, url.QueryEscape(tempToken))
			c.Redirect(http.StatusFound, redirectURL)
			return
		}
		h.issueCodeAndRedirect(c, app, client, userID, req, consentToken)
		return
	}

	c.HTML(http.StatusOK, "oidc_consent", gin.H{
		"AppID":        app.ID.String(),
		"AppName":      app.Name,
		"ClientName":   client.Name,
		"ClientLogo":   client.LogoURL,
		"ConsentToken": consentToken,
		"Scopes":       scopes,
		"UserID":       userID,
		"Theme":        theme,
		"PrimaryColor": primaryColor,
	})
}

// AuthorizeSubmit handles POST /oidc/:app_id/authorize
// Processes login or consent form submission.
// @Summary OIDC authorize form submit
// @Tags OIDC
// @Router /oidc/{app_id}/authorize [post]
func (h *Handler) AuthorizeSubmit(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}
	if !app.OIDCEnabled {
		h.renderError(c, app, "OIDC is not enabled for this application")
		return
	}

	// The form always carries a consent_token (encodes original auth params)
	var req dto.OIDCConsentSubmitRequest
	if err := c.ShouldBind(&req); err != nil {
		h.renderError(c, app, "missing form data")
		return
	}

	origReq, err := h.parseConsentToken(req.ConsentToken)
	if err != nil {
		h.renderError(c, app, "invalid or expired session token")
		return
	}

	client, err := h.Repo.GetClientByClientID(origReq.ClientID)
	if err != nil {
		h.renderError(c, app, "unknown client")
		return
	}

	// ── Login form submission ──────────────────────────────────────────────
	email := c.PostForm("email")
	password := c.PostForm("password") // #nosec G101 -- form field, not a credential constant
	// ui_theme may be forwarded as a hidden form field from the login page
	postUITheme := c.PostForm("ui_theme")
	theme, primaryColor := clientThemeWithOverride(client, app, postUITheme)

	if email != "" && password != "" {
		// Validate credentials
		user, authErr := h.authenticateUser(app, email, password)
		if authErr != nil {
			scopes := strings.Fields(origReq.Scope)
			c.HTML(http.StatusOK, "oidc_login", gin.H{
				"AppID":        app.ID.String(),
				"AppName":      app.Name,
				"ClientName":   client.Name,
				"ClientLogo":   client.LogoURL,
				"ConsentToken": req.ConsentToken,
				"Scopes":       scopes,
				"Error":        "Invalid email or password",
				"Theme":        theme,
				"PrimaryColor": primaryColor,
				"UITheme":      postUITheme,
			})
			return
		}

		// Authenticated — check 2FA before proceeding
		if user.TwoFAEnabled {
			tempToken := uuid.New().String()
			if err := redis.SetTempUserSession(app.ID.String(), tempToken, user.ID.String(), 10*time.Minute); err != nil {
				h.renderError(c, app, "Failed to create 2FA session")
				return
			}
			// Redirect to the OIDC redirect URI with 2FA params.
			// SocialLoginCallback.tsx already handles ?temp_token=...&requires_2fa=true
			// by storing the temp token and navigating to /login?social_2fa=true.
			redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true", origReq.RedirectURI, url.QueryEscape(tempToken))
			c.Redirect(http.StatusFound, redirectURL)
			return
		}

		// Proceed to consent or auto-approve
		if !client.RequireConsent {
			h.issueCodeAndRedirectForUser(c, app, client, user.ID.String(), origReq)
			return
		}
		scopes := strings.Fields(origReq.Scope)
		c.HTML(http.StatusOK, "oidc_consent", gin.H{
			"AppID":        app.ID.String(),
			"AppName":      app.Name,
			"ClientName":   client.Name,
			"ClientLogo":   client.LogoURL,
			"ConsentToken": req.ConsentToken,
			"Scopes":       scopes,
			"UserID":       user.ID.String(),
			"Theme":        theme,
			"PrimaryColor": primaryColor,
			"UITheme":      postUITheme,
		})
		return
	}

	// ── Consent form submission ────────────────────────────────────────────
	userID := c.PostForm("user_id")
	action := c.PostForm("action")

	if action == "deny" {
		redirectWithError(c, origReq.RedirectURI, origReq.State, "access_denied", "User denied access")
		return
	}

	// Enforce 2FA on consent approval too
	consentUser, err := h.Repo.GetUserByID(userID)
	if err == nil && consentUser.TwoFAEnabled {
		tempToken := uuid.New().String()
		if err := redis.SetTempUserSession(app.ID.String(), tempToken, userID, 10*time.Minute); err != nil {
			h.renderError(c, app, "Failed to create 2FA session")
			return
		}
		redirectURL := fmt.Sprintf("%s?temp_token=%s&requires_2fa=true",
			origReq.RedirectURI, url.QueryEscape(tempToken))
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	h.issueCodeAndRedirectForUser(c, app, client, userID, origReq)
}

// ─── Token endpoint ────────────────────────────────────────────────────────────

// Token handles POST /oidc/:app_id/token
// @Summary OIDC token endpoint
// @Tags OIDC
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param app_id path string true "Application UUID"
// @Success 200 {object} dto.OIDCTokenResponse
// @Failure 400 {object} dto.OIDCTokenErrorResponse
// @Router /oidc/{app_id}/token [post]
func (h *Handler) Token(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}
	if !app.OIDCEnabled {
		c.JSON(http.StatusBadRequest, dto.OIDCTokenErrorResponse{Error: "invalid_request", ErrorDescription: "OIDC is not enabled"})
		return
	}

	var req dto.OIDCTokenRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.OIDCTokenErrorResponse{Error: "invalid_request", ErrorDescription: err.Error()})
		return
	}

	// Support Basic Auth for confidential clients
	if clientID, clientSecret, ok := c.Request.BasicAuth(); ok {
		req.ClientID = clientID
		req.ClientSecret = clientSecret
	}

	switch req.GrantType {
	case "authorization_code":
		h.handleAuthCodeGrant(c, app, req)
	case "client_credentials":
		h.handleClientCredentialsGrant(c, app, req)
	case "refresh_token":
		h.handleRefreshTokenGrant(c, app, req)
	default:
		c.JSON(http.StatusBadRequest, dto.OIDCTokenErrorResponse{
			Error:            "unsupported_grant_type",
			ErrorDescription: fmt.Sprintf("grant type %q is not supported", req.GrantType),
		})
	}
}

func (h *Handler) handleAuthCodeGrant(c *gin.Context, app *models.Application, req dto.OIDCTokenRequest) {
	// Validate client
	client, err := h.validateClientCredentials(req.ClientID, req.ClientSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.OIDCTokenErrorResponse{Error: "invalid_client", ErrorDescription: err.Error()})
		return
	}

	ac, user, err := h.Service.ExchangeCode(req.Code, req.ClientID, req.RedirectURI, req.CodeVerifier)
	if err != nil {
		code := "invalid_grant"
		if strings.HasPrefix(err.Error(), "invalid_grant:") {
			code = "invalid_grant"
		}
		c.JSON(http.StatusBadRequest, dto.OIDCTokenErrorResponse{Error: code, ErrorDescription: err.Error()})
		return
	}

	scopes := strings.Fields(ac.Scopes)
	accessToken, refreshToken, idToken, expiresIn, err := h.Service.MintTokensForUser(app, client, user, scopes, ac.Nonce)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.OIDCTokenErrorResponse{Error: "server_error", ErrorDescription: "failed to mint tokens"})
		return
	}

	resp := dto.OIDCTokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		IDToken:     idToken,
		Scope:       ac.Scopes,
	}
	if containsScope(ac.Scopes, "offline_access") {
		resp.RefreshToken = refreshToken
	}

	// Log successful OIDC login and increment the Authentication Metrics counter.
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogOIDCLogin(app.ID, user.ID, ipAddress, userAgent, req.ClientID)
	health.IncLoginSuccess(app.ID.String())

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) handleClientCredentialsGrant(c *gin.Context, app *models.Application, req dto.OIDCTokenRequest) {
	accessToken, expiresIn, err := h.Service.ClientCredentialsGrant(app, req.ClientID, req.ClientSecret, req.Scope)
	if err != nil {
		errCode := "invalid_client"
		if strings.HasPrefix(err.Error(), "unauthorized_client") {
			errCode = "unauthorized_client"
		}
		c.JSON(http.StatusUnauthorized, dto.OIDCTokenErrorResponse{Error: errCode, ErrorDescription: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.OIDCTokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		Scope:       req.Scope,
	})
}

func (h *Handler) handleRefreshTokenGrant(c *gin.Context, app *models.Application, req dto.OIDCTokenRequest) {
	client, err := h.validateClientCredentials(req.ClientID, req.ClientSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.OIDCTokenErrorResponse{Error: "invalid_client", ErrorDescription: err.Error()})
		return
	}

	claims, err := pkgjwt.ParseToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.OIDCTokenErrorResponse{Error: "invalid_grant", ErrorDescription: "invalid refresh_token"})
		return
	}

	// Fix #12: Enforce that this token is actually a refresh token
	if claims.TokenType != pkgjwt.TokenTypeRefresh {
		c.JSON(http.StatusBadRequest, dto.OIDCTokenErrorResponse{Error: "invalid_grant", ErrorDescription: "token is not a refresh token"})
		return
	}

	// Fix #7: Blacklist the old refresh token so it cannot be reused
	if claims.ExpiresAt != nil {
		if ttl := time.Until(claims.ExpiresAt.Time); ttl > 0 {
			_ = redis.BlacklistAccessToken(app.ID.String(), req.RefreshToken, claims.UserID, ttl)
		}
	}

	user, err := h.Repo.GetUserByID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.OIDCTokenErrorResponse{Error: "invalid_grant", ErrorDescription: "user not found"})
		return
	}

	// Fix #4: Validate that the requested scopes are a subset of the originally
	// granted scopes. Look up the granted scopes from Redis using the session ID
	// embedded in the refresh token claims.
	originalScopeStr, _ := redis.GetOIDCGrantedScopes(app.ID.String(), claims.SessionID)
	originalScopes := strings.Fields(originalScopeStr)

	requestedScopes := strings.Fields(req.Scope)
	if len(requestedScopes) == 0 {
		// No scope requested — default to what was originally granted.
		requestedScopes = originalScopes
		if len(requestedScopes) == 0 {
			requestedScopes = []string{"openid", "profile", "email"}
		}
	} else if len(originalScopes) > 0 {
		// Ensure every requested scope was in the original grant.
		for _, s := range requestedScopes {
			if !sliceContains(originalScopes, s) {
				c.JSON(http.StatusBadRequest, dto.OIDCTokenErrorResponse{
					Error:            "invalid_scope",
					ErrorDescription: fmt.Sprintf("scope %q was not granted in the original authorization", s),
				})
				return
			}
		}
	}

	accessToken, refreshToken, idToken, expiresIn, err := h.Service.MintTokensForUser(app, client, user, requestedScopes, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.OIDCTokenErrorResponse{Error: "server_error", ErrorDescription: "failed to mint tokens"})
		return
	}
	c.JSON(http.StatusOK, dto.OIDCTokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		IDToken:      idToken,
		RefreshToken: refreshToken,
		Scope:        strings.Join(requestedScopes, " "),
	})
}

// ─── UserInfo endpoint ─────────────────────────────────────────────────────────

// UserInfo handles GET /oidc/:app_id/userinfo
// @Summary OIDC userinfo endpoint
// @Tags OIDC
// @Security BearerAuth
// @Param app_id path string true "Application UUID"
// @Success 200 {object} dto.OIDCUserInfoResponse
// @Failure 401 {object} dto.ErrorResponse
// @Router /oidc/{app_id}/userinfo [get]
func (h *Handler) UserInfo(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}
	if !app.OIDCEnabled {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "OIDC not enabled"})
		return
	}

	bearer := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if bearer == "" {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "missing bearer token"})
		return
	}

	scopes, err := h.Service.GetGrantedScopes(bearer, []string{"openid", "profile", "email", "roles"})
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: err.Error()})
		return
	}
	user, err := h.Service.GetUserInfo(app, bearer, scopes)
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: err.Error()})
		return
	}

	// Fix #11: Only include roles in the UserInfo response when the 'roles'
	// scope was part of the original grant. This prevents leaking role
	// information to clients that never requested it.
	var roleNames []string
	if sliceContains(scopes, "roles") {
		roleNames, _ = h.Service.roleLookup(app.ID.String(), user.ID.String())
	}
	resp := dto.OIDCUserInfoResponse{
		Sub:           user.ID.String(),
		Name:          user.Name,
		GivenName:     user.FirstName,
		FamilyName:    user.LastName,
		Picture:       user.ProfilePicture,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Locale:        user.Locale,
		Roles:         roleNames,
	}
	c.JSON(http.StatusOK, resp)
}

// ─── Introspection endpoint ────────────────────────────────────────────────────

// Introspect handles POST /oidc/:app_id/introspect
// @Summary OIDC token introspection (RFC 7662)
// @Tags OIDC
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param app_id path string true "Application UUID"
// @Success 200 {object} dto.OIDCIntrospectResponse
// @Router /oidc/{app_id}/introspect [post]
func (h *Handler) Introspect(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}

	var req dto.OIDCIntrospectRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	// Authenticate the introspecting client (Basic Auth or form)
	clientID, clientSecret, _ := c.Request.BasicAuth()
	if clientID == "" {
		clientID = c.PostForm("client_id")
		clientSecret = c.PostForm("client_secret")
	}
	if _, err := h.validateClientCredentials(clientID, clientSecret); err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid_client"})
		return
	}

	pkgJWT := pkgjwt.ParseToken
	claims, err := pkgJWT(req.Token)
	if err != nil || claims == nil {
		c.JSON(http.StatusOK, dto.OIDCIntrospectResponse{Active: false})
		return
	}
	if claims.AppID != app.ID.String() {
		c.JSON(http.StatusOK, dto.OIDCIntrospectResponse{Active: false})
		return
	}

	// Fix #6: Reflect the actual token type from the JWT claims rather than
	// hardcoding "Bearer" — a refresh token and an access token are distinct types.
	tokenType := claims.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	resp := dto.OIDCIntrospectResponse{
		Active:    true,
		Sub:       claims.UserID,
		Iss:       h.Service.IssuerURL(app),
		TokenType: tokenType,
	}
	if claims.ExpiresAt != nil {
		resp.Exp = claims.ExpiresAt.Unix()
	}
	if claims.IssuedAt != nil {
		resp.Iat = claims.IssuedAt.Unix()
	}
	c.JSON(http.StatusOK, resp)
}

// ─── Revocation endpoint (RFC 7009) ───────────────────────────────────────────

// Revoke handles POST /oidc/:app_id/revoke
// Per RFC 7009 §2.2 the server MUST respond 200 OK for any valid revocation
// request, including tokens that are already invalid or unknown. Only
// confidential-client authentication failures return an error.
// @Summary OIDC token revocation (RFC 7009)
// @Tags OIDC
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param app_id path string true "Application UUID"
// @Param token formData string true "Token to revoke"
// @Param token_type_hint formData string false "access_token or refresh_token"
// @Param client_id formData string false "Client ID (confidential clients)"
// @Param client_secret formData string false "Client secret (confidential clients)"
// @Success 200
// @Failure 401 {object} dto.OIDCTokenErrorResponse
// @Router /oidc/{app_id}/revoke [post]
func (h *Handler) Revoke(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}

	var req dto.OIDCRevokeRequest
	if err := c.ShouldBind(&req); err != nil {
		// Per RFC 7009 §2.2 — still return 200 for malformed requests unless
		// we can determine it is an authentication failure.
		c.Status(http.StatusOK)
		return
	}

	// Support Basic Auth for confidential clients.
	if clientID, clientSecret, ok := c.Request.BasicAuth(); ok {
		req.ClientID = clientID
		req.ClientSecret = clientSecret
	}

	// If a client_id is provided, authenticate it. Confidential clients that
	// fail authentication get 401 (RFC 7009 §2.1).
	if req.ClientID != "" {
		if _, err := h.validateClientCredentials(req.ClientID, req.ClientSecret); err != nil {
			c.JSON(http.StatusUnauthorized, dto.OIDCTokenErrorResponse{
				Error:            "invalid_client",
				ErrorDescription: err.Error(),
			})
			return
		}
	}

	// Attempt to parse and blacklist the token. Ignore parse errors — unknown
	// or expired tokens are silently accepted per the spec.
	claims, err := pkgjwt.ParseToken(req.Token)
	if err == nil && claims != nil && claims.AppID == app.ID.String() {
		ttl := time.Duration(0)
		if claims.ExpiresAt != nil {
			remaining := time.Until(claims.ExpiresAt.Time)
			if remaining > 0 {
				ttl = remaining
			}
		}
		// Best-effort: ignore blacklist errors.
		_ = redis.BlacklistAccessToken(app.ID.String(), req.Token, claims.UserID, ttl)
	}

	c.Status(http.StatusOK)
}

// ─── End Session endpoint (RP-Initiated Logout) ────────────────────────────────

// EndSession handles GET+POST /oidc/:app_id/end_session
// Implements OpenID Connect RP-Initiated Logout 1.0.
// Clears the OIDC session cookie and redirects to post_logout_redirect_uri if
// provided, otherwise renders a "signed out" confirmation page.
// @Summary OIDC RP-Initiated Logout
// @Tags OIDC
// @Param app_id path string true "Application UUID"
// @Param id_token_hint query string false "Previously-issued ID token"
// @Param post_logout_redirect_uri query string false "URI to redirect to after logout"
// @Param state query string false "Opaque value returned with redirect"
// @Success 302
// @Success 200
// @Router /oidc/{app_id}/end_session [get]
func (h *Handler) EndSession(c *gin.Context) {
	app, ok := h.loadApp(c)
	if !ok {
		return
	}

	postLogoutRedirectURI := c.Query("post_logout_redirect_uri")
	if postLogoutRedirectURI == "" {
		postLogoutRedirectURI = c.PostForm("post_logout_redirect_uri")
	}
	state := c.Query("state")
	if state == "" {
		state = c.PostForm("state")
	}

	// Fix #8: Validate id_token_hint — parse the ID token (RS256) and verify it
	// belongs to a real user in this application before honouring the logout.
	idTokenHint := c.Query("id_token_hint")
	if idTokenHint == "" {
		idTokenHint = c.PostForm("id_token_hint")
	}
	// logoutUserID / logoutUserEmail are populated below from the hint or the
	// OIDC browser session cookie so we can revoke the user's JWT sessions.
	var logoutUserID, logoutUserEmail string
	if idTokenHint != "" {
		rsaKey, err := h.Service.GetOrCreateRSAKey(app.ID)
		if err == nil {
			idClaims, err := ParseIDToken(idTokenHint, rsaKey)
			if err != nil || idClaims == nil {
				// Hint present but invalid — reject rather than silently ignore.
				h.renderError(c, app, "invalid id_token_hint")
				return
			}
			// Confirm the subject belongs to this application.
			user, err := h.Repo.GetUserByID(idClaims.Subject)
			if err != nil || user == nil {
				h.renderError(c, app, "id_token_hint refers to an unknown user")
				return
			}
			logoutUserID = idClaims.Subject
			logoutUserEmail = user.Email
		}
	}

	// Clear the OIDC session cookie and remove the Redis mapping.
	cookieName := "oidc_session_" + app.ID.String()
	sessionToken, cookieErr := c.Cookie(cookieName)
	c.SetCookie(cookieName, "", -1, "/", "", false, true)
	if cookieErr == nil && sessionToken != "" {
		// If we didn't get a userID from id_token_hint, resolve it from the
		// browser session mapping before we delete it.
		if logoutUserID == "" {
			if uid, err := redis.GetOIDCBrowserSession(app.ID.String(), sessionToken); err == nil && uid != "" {
				logoutUserID = uid
				if user, err := h.Repo.GetUserByID(uid); err == nil && user != nil {
					logoutUserEmail = user.Email
				}
			}
		}
		_ = redis.DeleteOIDCBrowserSession(app.ID.String(), sessionToken)
	}

	// Revoke JWT sessions so they disappear from the admin panel and
	// AuthMiddleware rejects any further requests with those tokens.
	if logoutUserID != "" {
		appIDStr := app.ID.String()
		accessTokenTTL := time.Duration(viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES")) * time.Minute
		_ = redis.DeleteAllUserSessions(appIDStr, logoutUserID, "")
		_ = redis.BlacklistAllUserTokens(appIDStr, logoutUserID, accessTokenTTL)
		// Cross-app SSO logout: revoke sessions in all peer apps that share the
		// same session group (mirrors userService.GroupLogoutFunc behaviour).
		if h.GroupLogoutFunc != nil && logoutUserEmail != "" {
			h.GroupLogoutFunc(appIDStr, logoutUserEmail)
		}
	}

	if postLogoutRedirectURI != "" {
		// Fix #14: Validate post_logout_redirect_uri against registered client redirect URIs
		allowed := false
		if clients, err := h.Repo.ListClientsByApp(app.ID); err == nil {
			for i := range clients {
				if h.Service.IsRedirectURIAllowed(clients[i].RedirectURIs, postLogoutRedirectURI) {
					allowed = true
					break
				}
			}
		}
		if allowed {
			u, err := url.Parse(postLogoutRedirectURI)
			if err == nil {
				if state != "" {
					q := u.Query()
					q.Set("state", state)
					u.RawQuery = q.Encode()
				}
				c.Redirect(http.StatusFound, u.String())
				return
			}
		}
	}

	// No redirect URI — render signed-out confirmation page.
	theme, primaryColor := appTheme(app)
	c.HTML(http.StatusOK, "oidc_logout", gin.H{
		"AppID":        app.ID.String(),
		"AppName":      app.Name,
		"Theme":        theme,
		"PrimaryColor": primaryColor,
	})
}

// ─── Admin API — OIDC Client CRUD ─────────────────────────────────────────────

// AdminCreateClient handles POST /admin/oidc/apps/:id/clients
// @Summary Create an OIDC client
// @Tags Admin OIDC
// @Accept json
// @Produce json
// @Param id path string true "Application UUID"
// @Param request body dto.CreateOIDCClientRequest true "Client data"
// @Success 201 {object} dto.OIDCClientResponse
// @Security AdminApiKey
// @Router /admin/oidc/apps/{id}/clients [post]
func (h *Handler) AdminCreateClient(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid app id"})
		return
	}

	var req dto.CreateOIDCClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	client, plainSecret, err := h.Service.CreateClient(
		appID, req.Name, req.Description, req.RedirectURIs,
		req.AllowedGrantTypes, req.AllowedScopes,
		req.RequireConsent, req.IsConfidential, req.PKCERequired, req.LogoURL,
		req.LoginTheme, req.LoginPrimaryColor,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create client"})
		return
	}

	resp := clientToResponse(client)
	resp.ClientSecret = plainSecret
	c.JSON(http.StatusCreated, resp)
}

// AdminListClients handles GET /admin/oidc/apps/:id/clients
// @Summary List OIDC clients for an app
// @Tags Admin OIDC
// @Param id path string true "Application UUID"
// @Success 200 {array} dto.OIDCClientResponse
// @Security AdminApiKey
// @Router /admin/oidc/apps/{id}/clients [get]
func (h *Handler) AdminListClients(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid app id"})
		return
	}
	clients, err := h.Service.ListClients(appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list clients"})
		return
	}
	resp := make([]dto.OIDCClientResponse, len(clients))
	for i, cl := range clients {
		resp[i] = clientToResponse(&cl)
	}
	c.JSON(http.StatusOK, resp)
}

// AdminGetClient handles GET /admin/oidc/apps/:id/clients/:cid
// @Summary Get a single OIDC client
// @Tags Admin OIDC
// @Param id path string true "Application UUID"
// @Param cid path string true "Client UUID"
// @Success 200 {object} dto.OIDCClientResponse
// @Security AdminApiKey
// @Router /admin/oidc/apps/{id}/clients/{cid} [get]
func (h *Handler) AdminGetClient(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid client id"})
		return
	}
	client, err := h.Service.GetClient(cid)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "client not found"})
		return
	}
	c.JSON(http.StatusOK, clientToResponse(client))
}

// AdminUpdateClient handles PUT /admin/oidc/apps/:id/clients/:cid
// @Summary Update an OIDC client
// @Tags Admin OIDC
// @Accept json
// @Produce json
// @Param id path string true "Application UUID"
// @Param cid path string true "Client UUID"
// @Param request body dto.UpdateOIDCClientRequest true "Update data"
// @Success 200 {object} dto.OIDCClientResponse
// @Security AdminApiKey
// @Router /admin/oidc/apps/{id}/clients/{cid} [put]
func (h *Handler) AdminUpdateClient(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid client id"})
		return
	}
	var req dto.UpdateOIDCClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	client, err := h.Service.UpdateClient(cid, req.Name, req.Description, req.RedirectURIs, req.AllowedGrantTypes, req.AllowedScopes, req.LogoURL, req.LoginTheme, req.LoginPrimaryColor, req.RequireConsent, req.IsConfidential, req.PKCERequired, req.IsActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to update client"})
		return
	}
	c.JSON(http.StatusOK, clientToResponse(client))
}

// AdminDeleteClient handles DELETE /admin/oidc/apps/:id/clients/:cid
// @Summary Delete an OIDC client
// @Tags Admin OIDC
// @Param id path string true "Application UUID"
// @Param cid path string true "Client UUID"
// @Success 204
// @Security AdminApiKey
// @Router /admin/oidc/apps/{id}/clients/{cid} [delete]
func (h *Handler) AdminDeleteClient(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid client id"})
		return
	}
	if err := h.Service.DeleteClient(cid); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete client"})
		return
	}
	c.Status(http.StatusNoContent)
}

// AdminRotateClientSecret handles POST /admin/oidc/apps/:id/clients/:cid/rotate-secret
// @Summary Rotate client secret
// @Tags Admin OIDC
// @Param id path string true "Application UUID"
// @Param cid path string true "Client UUID"
// @Success 200 {object} dto.OIDCClientResponse
// @Security AdminApiKey
// @Router /admin/oidc/apps/{id}/clients/{cid}/rotate-secret [post]
func (h *Handler) AdminRotateClientSecret(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid client id"})
		return
	}
	client, plain, err := h.Service.RotateClientSecret(cid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to rotate secret"})
		return
	}
	resp := clientToResponse(client)
	resp.ClientSecret = plain
	c.JSON(http.StatusOK, resp)
}

// ─── Internal helpers ──────────────────────────────────────────────────────────

// loadApp fetches the Application for the :app_id path parameter.
func (h *Handler) loadApp(c *gin.Context) (*models.Application, bool) {
	appIDStr := c.Param("app_id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid app_id"})
		return nil, false
	}
	app, err := h.Repo.GetApplication(appID)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "application not found"})
		return nil, false
	}
	return app, true
}

// renderError renders the OIDC error page.
func (h *Handler) renderError(c *gin.Context, app *models.Application, msg string) {
	theme, primaryColor := appTheme(app)
	c.HTML(http.StatusBadRequest, "oidc_error", gin.H{
		"AppID":        app.ID.String(),
		"AppName":      app.Name,
		"Error":        msg,
		"Theme":        theme,
		"PrimaryColor": primaryColor,
	})
}

// appTheme returns the login theme and primary color for an Application,
// used as the fallback when no OIDC client is available (logout, error pages).
func appTheme(app *models.Application) (string, string) {
	theme := app.LoginTheme
	if theme == "" {
		theme = "auto"
	}
	return theme, app.LoginPrimaryColor
}

// clientThemeWithOverride returns the login theme and primary color for an OIDC client,
// with an optional ui_theme query parameter override.
//
// Theme resolution order (highest priority first):
//  1. uiThemeOverride ("light", "dark", "auto") — set via ?ui_theme= query param or hidden form field
//  2. client.LoginTheme == "app" → delegate to app-level theme/color (app.LoginTheme / app.LoginPrimaryColor)
//  3. client.LoginTheme ("auto", "light", "dark") + client.LoginPrimaryColor
func clientThemeWithOverride(client *models.OIDCClient, app *models.Application, uiThemeOverride string) (string, string) {
	theme := client.LoginTheme
	primaryColor := client.LoginPrimaryColor

	// "app" sentinel: inherit theme and primary color from the owning Application.
	if theme == "app" {
		theme, primaryColor = appTheme(app)
	}
	if theme == "" {
		theme = "auto"
	}

	// ?ui_theme= / hidden form field always wins over everything.
	switch uiThemeOverride {
	case "light", "dark", "auto":
		theme = uiThemeOverride
	}
	return theme, primaryColor
}

// buildConsentToken encodes the original authorization request into a signed token
// stored in a hidden form field. This avoids storing state server-side.
// Simple implementation: hex-encoded random bytes + base64 of the request params.
func (h *Handler) buildConsentToken(appID uuid.UUID, req dto.OIDCAuthorizeRequest) string {
	v := url.Values{}
	v.Set("app_id", appID.String())
	v.Set("client_id", req.ClientID)
	v.Set("redirect_uri", req.RedirectURI)
	v.Set("scope", req.Scope)
	v.Set("state", req.State)
	v.Set("nonce", req.Nonce)
	v.Set("code_challenge", req.CodeChallenge)
	v.Set("code_challenge_method", req.CodeChallengeMethod)
	// Add a random nonce to prevent replay
	rnd := make([]byte, 8)
	_, _ = rand.Read(rnd)
	v.Set("_r", hex.EncodeToString(rnd))
	// Embed expiry
	v.Set("_exp", strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10))
	payload := v.Encode()
	mac := hmac.New(sha256.New, []byte(viper.GetString("JWT_SECRET")))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// parseConsentToken decodes a consent token back to an OIDCAuthorizeRequest.
func (h *Handler) parseConsentToken(token string) (*dto.OIDCAuthorizeRequest, error) {
	// Split on the last dot: payload.signature
	dotIdx := strings.LastIndex(token, ".")
	if dotIdx < 0 {
		return nil, fmt.Errorf("invalid token format")
	}
	payload := token[:dotIdx]
	gotSig := token[dotIdx+1:]

	// Verify HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(viper.GetString("JWT_SECRET")))
	mac.Write([]byte(payload))
	wantSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(gotSig), []byte(wantSig)) {
		return nil, fmt.Errorf("invalid token signature")
	}

	v, err := url.ParseQuery(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid token params")
	}

	// Validate embedded expiry
	if expStr := v.Get("_exp"); expStr != "" {
		exp, err := strconv.ParseInt(expStr, 10, 64)
		if err != nil || time.Now().Unix() > exp {
			return nil, fmt.Errorf("consent token expired")
		}
	}

	return &dto.OIDCAuthorizeRequest{
		ClientID:            v.Get("client_id"),
		RedirectURI:         v.Get("redirect_uri"),
		Scope:               v.Get("scope"),
		State:               v.Get("state"),
		Nonce:               v.Get("nonce"),
		CodeChallenge:       v.Get("code_challenge"),
		CodeChallengeMethod: v.Get("code_challenge_method"),
	}, nil
}

// sessionUserID checks the OIDC session cookie for an authenticated user.
// The cookie holds an opaque random token (not the user UUID) that is resolved
// via Redis to defend against session fixation / UUID-guessing attacks.
// Returns empty string if not authenticated.
func (h *Handler) sessionUserID(c *gin.Context, appID string) string {
	cookieName := "oidc_session_" + appID
	val, err := c.Cookie(cookieName)
	if err != nil || val == "" {
		return ""
	}
	// Resolve the opaque token to a userID in Redis (Fix #2).
	userID, err := redis.GetOIDCBrowserSession(appID, val)
	if err != nil || userID == "" {
		return ""
	}
	return userID
}

// issueCodeAndRedirect creates an auth code for a pre-authenticated user.
func (h *Handler) issueCodeAndRedirect(c *gin.Context, app *models.Application, client *models.OIDCClient, userID string, req dto.OIDCAuthorizeRequest, consentToken string) {
	h.issueCodeAndRedirectForUser(c, app, client, userID, &req)
}

func (h *Handler) issueCodeAndRedirectForUser(c *gin.Context, app *models.Application, client *models.OIDCClient, userID string, req *dto.OIDCAuthorizeRequest) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		h.renderError(c, app, "invalid user session")
		return
	}
	ac, err := h.Service.IssueAuthCode(app.ID, client.ClientID, uid, req.RedirectURI, req.Scope, req.Nonce, req.CodeChallenge, req.CodeChallengeMethod)
	if err != nil {
		h.renderError(c, app, "failed to issue authorization code")
		return
	}

	// Fix #2: Set an opaque random session token as the cookie value (not the
	// user UUID) and store the userID mapping in Redis to prevent session
	// fixation / UUID-guessing attacks.
	const browserSessionTTL = 3600 * time.Second
	sessionToken := uuid.New().String() // random, unguessable
	if err := redis.SetOIDCBrowserSession(app.ID.String(), sessionToken, userID, browserSessionTTL); err != nil {
		// Non-fatal — the code is already issued; the user just won't be
		// pre-authenticated for the next visit in this browser.
		sessionToken = ""
	}

	cookieName := "oidc_session_" + app.ID.String()
	cookieDomain := ""
	if rawPublicURL := viper.GetString("PUBLIC_URL"); rawPublicURL != "" {
		if parsed, err := url.Parse(rawPublicURL); err == nil {
			cookieDomain = parsed.Hostname()
		}
	}
	if sessionToken != "" {
		c.SetCookie(cookieName, sessionToken, int(browserSessionTTL/time.Second), "/", cookieDomain, strings.HasPrefix(viper.GetString("PUBLIC_URL"), "https://"), true)
	}

	redirectURL, _ := url.Parse(req.RedirectURI)
	q := redirectURL.Query()
	q.Set("code", ac.Code)
	if req.State != "" {
		q.Set("state", req.State)
	}
	redirectURL.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, redirectURL.String())
}

// authenticateUser validates email + password for the OIDC login form.
func (h *Handler) authenticateUser(app *models.Application, email, password string) (*models.User, error) {
	user, err := h.Repo.GetUserByEmail(app.ID.String(), email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !user.IsActive {
		return nil, fmt.Errorf("account is inactive")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return user, nil
}

// validateClientCredentials checks client_id + secret from the token endpoint.
func (h *Handler) validateClientCredentials(clientID, clientSecret string) (*models.OIDCClient, error) {
	if clientID == "" {
		return nil, fmt.Errorf("client_id required")
	}
	client, err := h.Repo.GetClientByClientID(clientID)
	if err != nil {
		return nil, fmt.Errorf("unknown client")
	}
	if !client.IsActive {
		return nil, fmt.Errorf("client is disabled")
	}
	if client.IsConfidential && clientSecret == "" {
		return nil, fmt.Errorf("client_secret required")
	}
	if client.IsConfidential {
		if err := bcrypt.CompareHashAndPassword([]byte(client.ClientSecretHash), []byte(clientSecret)); err != nil {
			return nil, fmt.Errorf("invalid client_secret")
		}
	}
	return client, nil
}

// redirectWithError appends an OAuth2 error to the redirect URI and redirects.
func redirectWithError(c *gin.Context, redirectURI, state, errCode, errDesc string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: errDesc})
		return
	}
	q := u.Query()
	q.Set("error", errCode)
	q.Set("error_description", errDesc)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

// clientToResponse maps an OIDCClient model to the admin API response DTO.
func clientToResponse(c *models.OIDCClient) dto.OIDCClientResponse {
	return dto.OIDCClientResponse{
		ID:                c.ID.String(),
		AppID:             c.AppID.String(),
		Name:              c.Name,
		Description:       c.Description,
		ClientID:          c.ClientID,
		RedirectURIs:      c.RedirectURIs,
		AllowedGrantTypes: c.AllowedGrantTypes,
		AllowedScopes:     c.AllowedScopes,
		RequireConsent:    c.RequireConsent,
		IsConfidential:    c.IsConfidential,
		PKCERequired:      c.PKCERequired,
		LogoURL:           c.LogoURL,
		LoginTheme:        c.LoginTheme,
		LoginPrimaryColor: c.LoginPrimaryColor,
		IsActive:          c.IsActive,
		CreatedAt:         c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         c.UpdatedAt.Format(time.RFC3339),
	}
}
