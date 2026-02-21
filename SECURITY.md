# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 3.x     | Yes       |
| 2.x     | Security fixes only |
| 1.x     | No        |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** disclose the vulnerability publicly until it has been addressed.
2. Open a private security advisory or contact the maintainer directly.
3. Provide as much detail as possible: reproduction steps, affected versions, and potential impact.

We aim to acknowledge reports within 48 hours and provide a fix or mitigation plan within 7 days.

## Security Architecture

### Authentication

- **JWT Tokens** — Short-lived access tokens (15 min default) and long-lived refresh tokens (720 hours default)
- **Token Type Enforcement** — JWT claims include a `type` field (`"access"` or `"refresh"`); middleware rejects refresh tokens used as access tokens
- **Token Blacklisting** — Redis-backed token blacklisting for immediate logout and user deactivation
- **Password Hashing** — bcrypt with cost factor 12 (above OWASP minimum recommendation of 10)
- **Two-Factor Authentication** — TOTP-based 2FA with recovery codes

### Admin GUI Security

- **Session Management** — Redis-backed sessions with cryptographically random 256-bit session IDs
- **Cookie Security** — HttpOnly, Secure (in production), SameSite=Strict
- **CSRF Protection** — Token-based CSRF protection on all mutation endpoints; tokens validated using `crypto/subtle.ConstantTimeCompare` to prevent timing attacks
- **Rate Limiting** — GUI login rate-limited to 5 attempts per 60 seconds per IP, with hard lockout after 10 attempts for 15 minutes
- **Admin Setup** — Initial admin account created via CLI wizard with masked password input

### API Security

- **Rate Limiting** — Applied to all public authentication endpoints:
  - `/register` — 3 requests/minute per IP
  - `/login` — 5 requests/minute per IP, lockout after 10 attempts for 15 minutes
  - `/refresh-token` — 10 requests/minute per IP
  - `/forgot-password` — 3 requests/minute per IP
  - `/reset-password` — 5 requests/minute per IP
  - `/2fa/login-verify` — 5 requests/minute per IP, lockout after 10 attempts for 15 minutes
- **In-Memory Fallback** — Rate limiting continues to function (per-instance) when Redis is unavailable
- **API Key Authentication** — SHA-256 hashed API keys for admin and per-application access; raw keys shown once at creation
- **Input Validation** — All request DTOs validated with struct tags; password fields capped at 128 characters to prevent bcrypt DoS
- **Error Sanitization** — Internal error details never exposed to API clients

### HTTP Security Headers

All responses include the following security headers:

| Header | Value | Purpose |
|--------|-------|---------|
| X-Frame-Options | DENY | Prevents clickjacking |
| X-Content-Type-Options | nosniff | Prevents MIME-type sniffing |
| Referrer-Policy | strict-origin-when-cross-origin | Limits referrer leakage |
| X-XSS-Protection | 0 | Disables legacy XSS filter (CSP is used instead) |
| Permissions-Policy | camera=(), microphone=(), geolocation=(), payment=() | Restricts browser features |
| Content-Security-Policy | Route-aware (strict for API, relaxed for GUI) | Controls resource loading |
| Strict-Transport-Security | max-age=31536000; includeSubDomains | Enforces HTTPS (when TLS detected) |

### Database Security

- **Parameterized Queries** — All database queries use GORM's parameterized query builder (no raw SQL concatenation)
- **Multi-Tenant Isolation** — All user data scoped by `app_id` at the database level
- **Encrypted OAuth Secrets** — OAuth client secrets stored with `json:"-"` tag, never exposed in API responses

### CORS

- **Production Mode** — Localhost origins are excluded from the CORS allowlist in release mode
- **Frontend URL Required** — Warning logged if `FRONTEND_URL` is not configured

## Security Scanning

Run the following before each release:

```bash
make security          # gosec static analysis + nancy dependency audit
make lint              # golangci-lint
go test -race ./...    # Race condition detection
```

## JWT Secret Requirements

- Minimum 32 bytes (256 bits)
- The application will refuse to start if the secret is empty or too short
- Use a cryptographically random value in production

## Dependencies

Security-relevant Go dependencies:

- `golang.org/x/crypto/bcrypt` — Password hashing
- `github.com/golang-jwt/jwt/v5` — JWT token generation and validation
- `github.com/go-redis/redis/v8` — Redis session and rate limit storage
- `crypto/subtle` — Constant-time comparisons for CSRF tokens
- `crypto/sha256` — API key hashing
