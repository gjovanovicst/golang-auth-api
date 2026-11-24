# Errors, limits and retries

## Error shape

Every error is JSON with a single `error` field:

```json
{ "error": "Invalid email or password" }
```

Validation errors return the validator's message, so surface it next to the field rather
than as a toast.

## Status codes

| Status | Meaning | What the client should do |
|---|---|---|
| `400` | Malformed body, missing `X-App-ID`, invalid value | Fix the request; do not retry |
| `401` | Missing/expired token, wrong credentials, invalid 2FA code | Re-authenticate or refresh the token |
| `403` | Email not verified, 2FA/feature disabled for the app, permission denied | Explain the state to the user |
| `404` | Unknown resource or route | Check the id |
| `409` | Conflict (duplicate email in the application, duplicate social link) | Treat as "already exists" |
| `423` | Account locked by brute-force protection | Show the unlock time from the body |
| `429` | Rate limited | Back off, then retry |
| `500` | Server error | Retry with backoff; report with logs |

## Rate limits

Public endpoints are rate limited per IP and per application: registration, login,
refresh, password reset, 2FA verification, magic links, passkeys, OIDC authorize/token
and SSO. Exceeding a limit returns `429`:

```json
{ "error": "Too many requests. Please wait a moment before trying again." }
```

The limits are deliberately strict on authentication endpoints. A frontend should:

1. Disable the submit button while a request is in flight.
2. On `429`, show a countdown instead of retrying immediately.
3. Never retry a failed login automatically — it counts against the same window.

## Brute-force protection

Beyond rate limiting, the application can lock an account after repeated failures,
introduce progressive delays and require a CAPTCHA (per-application overrides in the GUI,
defaults from the environment). A locked account returns `423`; the admin GUI can unlock
it, and `POST /login` accepts `captcha_token` when CAPTCHA is required.

Failures are counted per application and identifier (email or IP), so retrying from a
different network does not reset the email counter.

## Token errors

| Situation | Result |
|---|---|
| Access token expired | `401` — call `POST /refresh-token` |
| Access token blacklisted (logout, password change) | `401` — force a new login |
| Refresh token already rotated | `401` — treat as compromised, log the user out |
| Temporary 2FA token expired (10 min) | `401` — restart the login |
| Session revoked | Refresh fails; the access token expires within its TTL |

## Idempotency and retries

The API does not implement idempotency keys. Safe-to-retry calls are the read endpoints
and `POST /refresh-token` (with the current token). Do **not** blindly retry:

- `POST /register` — a retry may hit the duplicate-email conflict.
- `POST /2fa/login-verify` — codes are single-use.
- `POST /magic-link/verify` and `POST /sso/exchange` — tokens are single-use.
- `POST /reset-password` — the reset token is consumed on success.

## Correlation

Every response carries the standard server headers; the API logs the request line, status,
latency, IP and user agent. When reporting a bug, include the timestamp (with timezone),
the endpoint, the `X-App-ID` and the response body — never include tokens.
