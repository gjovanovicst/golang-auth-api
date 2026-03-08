package oidc

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/golang-jwt/jwt/v5"
)

// IDTokenClaims are the JWT claims for an OIDC ID token (RS256).
// All standard OIDC claims are included; optional claims are omitted when empty.
type IDTokenClaims struct {
	// Standard OIDC claims
	Nonce  string `json:"nonce,omitempty"`
	AtHash string `json:"at_hash,omitempty"` // Access token hash (OIDC Core §3.3.2.11)

	// profile scope claims
	Name       string `json:"name,omitempty"`
	GivenName  string `json:"given_name,omitempty"`
	FamilyName string `json:"family_name,omitempty"`
	Picture    string `json:"picture,omitempty"`
	Locale     string `json:"locale,omitempty"`

	// email scope claims
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`

	// roles scope claims (custom extension)
	Roles []string `json:"roles,omitempty"`

	jwt.RegisteredClaims
}

// MintIDTokenParams carries everything needed to create an ID token.
type MintIDTokenParams struct {
	Issuer      string          // e.g. "https://auth.example.com/oidc/<app_id>"
	Audience    string          // client_id of the relying party
	User        *models.User    // end user
	Roles       []string        // user's role names in this app (may be nil)
	Scopes      []string        // granted scopes (controls which claims to include)
	Nonce       string          // from the original authorization request
	AccessToken string          // HS256 access token — used to compute at_hash (may be empty)
	TTL         time.Duration   // ID token lifetime
	Kid         string          // RSA key identifier (app UUID)
	Key         *rsa.PrivateKey // RSA signing key
}

// computeAtHash computes the at_hash claim value for a given access token.
// Per OIDC Core §3.3.2.11: SHA-256 the ASCII representation, take the left half,
// then base64url-encode without padding.
func computeAtHash(accessToken string) string {
	h := sha256.Sum256([]byte(accessToken))
	half := h[:len(h)/2]
	return base64.RawURLEncoding.EncodeToString(half)
}

// MintIDToken signs and returns a new RS256 ID token.
func MintIDToken(p MintIDTokenParams) (string, error) {
	now := time.Now().UTC()

	claims := IDTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    p.Issuer,
			Subject:   p.User.ID.String(),
			Audience:  jwt.ClaimStrings{p.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(p.TTL)),
		},
		Nonce: p.Nonce,
	}

	// Compute at_hash when an access token is provided (OIDC Core §3.3.2.11).
	if p.AccessToken != "" {
		claims.AtHash = computeAtHash(p.AccessToken)
	}

	scopeSet := make(map[string]bool, len(p.Scopes))
	for _, s := range p.Scopes {
		scopeSet[strings.TrimSpace(s)] = true
	}

	if scopeSet["profile"] {
		claims.Name = p.User.Name
		claims.GivenName = p.User.FirstName
		claims.FamilyName = p.User.LastName
		claims.Picture = p.User.ProfilePicture
		claims.Locale = p.User.Locale
	}

	if scopeSet["email"] {
		claims.Email = p.User.Email
		claims.EmailVerified = p.User.EmailVerified
	}

	if scopeSet["roles"] && len(p.Roles) > 0 {
		claims.Roles = p.Roles
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = p.Kid

	signed, err := token.SignedString(p.Key)
	if err != nil {
		return "", fmt.Errorf("sign id token: %w", err)
	}
	return signed, nil
}

// ParseIDToken verifies an RS256 ID token and returns its claims.
// Use this in the /userinfo and /introspect handlers.
func ParseIDToken(tokenStr string, key *rsa.PrivateKey) (*IDTokenClaims, error) {
	pub := &key.PublicKey
	t, err := jwt.ParseWithClaims(tokenStr, &IDTokenClaims{}, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
		}
		return pub, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := t.Claims.(*IDTokenClaims); ok && t.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid id token")
}
