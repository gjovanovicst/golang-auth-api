# Authentication

The core flow is the same for every client: register, verify the email, log in, keep
the token pair fresh, log out.

## Register

```bash
curl -sX POST http://localhost:8080/register \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Str0ng!Passw0rd"}'
```

- `201` — `{"message":"User registered successfully. Please check your email for verification."}`
- The password is validated against the application's policy (`pw_min_length`,
  `pw_require_*`, `pw_history_count`).
- A verification email is sent. Without SMTP the API logs the message instead
  (development fallback) and the account stays unverified.

## Verify the email

The verification link contains a token; your frontend forwards it to:

```bash
curl -s "http://localhost:8080/verify-email?token=$TOKEN" -H "X-App-ID: $APP"
```

Resend with `POST /resend-verification` and `{"email":"user@example.com"}`.

## Log in

```bash
curl -sX POST http://localhost:8080/login \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Str0ng!Passw0rd"}'
```

Successful response:

```json
{
  "access_token": "eyJhbGciOi…",
  "refresh_token": "eyJhbGciOi…"
}
```

2FA challenge response:

```json
{
  "requires_2fa": true,
  "message": "2FA verification required",
  "temp_token": "5daf6d7f-…",
  "method": "totp",
  "sms_recovery_available": true
}
```

Submit the second factor to [`POST /2fa/login-verify`](11-2fa-recovery.md).

| Status | Meaning |
|---|---|
| `400` | Invalid body, or CAPTCHA required |
| `401` | Wrong email or password |
| `403` | Email not verified |
| `423` | Account locked by brute-force protection |
| `429` | Rate limited |

If the application enforces password rotation, a login with an expired password
returns `password_expired: true` and **no tokens**; send the user through the reset flow.

## Refresh and rotate

```bash
curl -sX POST http://localhost:8080/refresh-token \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"refresh_token":"'$REFRESH_TOKEN'"}'
```

Each call returns a new pair and invalidates the old refresh token. Reusing a rotated
token is rejected — treat that as a signal to log the user out.

## Log out

```bash
curl -sX POST http://localhost:8080/logout \
  -H "X-App-ID: $APP" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"access_token":"'$ACCESS_TOKEN'","refresh_token":"'$REFRESH_TOKEN'"}'
```

The access token is blacklisted in Redis for its remaining lifetime and the session is
removed. With session groups enabled the user may also be logged out of peer apps.

## Profile and password

| Endpoint | Permission | Purpose |
|---|---|---|
| `GET /profile` | `user:read` | Current user |
| `PUT /profile` | `user:write` | Name, locale, picture |
| `PUT /profile/email` | `user:write` | Change email (re-verification required) |
| `PUT /profile/password` | `user:write` | Change password (current password required) |
| `POST /profile/set-password` | `user:write` | Set a password on a social-only account |
| `DELETE /profile` | `user:delete` | Delete the account |
| `GET /auth/validate` | — | Validate the current access token |

Password changes invalidate other sessions and blacklist outstanding access tokens.

## Forgot and reset

```bash
curl -sX POST http://localhost:8080/forgot-password \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'

curl -sX POST http://localhost:8080/reset-password \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"token":"'$RESET_TOKEN'","new_password":"N3w!Passw0rd"}'
```

`forgot-password` always returns the same response, whether or not the email exists.

## Magic link (passwordless)

```bash
curl -sX POST http://localhost:8080/magic-link/request \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'

curl -sX POST http://localhost:8080/magic-link/verify \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"token":"'$TOKEN'"}'
```

The link is single-use and expires. `verify` accepts an optional `app_id` in the body for
multi-application frontends.

## Social login

| Step | Endpoint |
|---|---|
| Start | `GET /auth/{google\|facebook\|github}/login?redirect_uri=…` |
| Callback | provider → `GET /auth/{provider}/callback` (handled server-side, then redirected) |
| Link to an existing account | `GET /auth/{provider}/link` (JWT) → callback |
| Confirm an account merge | `POST /auth/merge/confirm` with the `merge_token` and the existing password |

OAuth `state` is HMAC-signed, single-use and bound to the initiating browser with an
`HttpOnly` cookie. Cross-site frontends must start the flow with credentials included:

```js
window.location = `${API}/auth/google/login?redirect_uri=${encodeURIComponent(callback)}`;
```

The API sets the binding cookie and redirects to Google.

## Passkeys (passwordless)

| Step | Endpoint |
|---|---|
| Begin | `POST /passkey/login/begin` |
| Finish | `POST /passkey/login/finish` |
| Register a passkey | `POST /passkey/register/begin` → `POST /passkey/register/finish` (JWT) |
| Manage | `GET /passkeys`, `PUT /passkeys/:id`, `DELETE /passkeys/:id` |

The begin/finish dance speaks WebAuthn JSON; the browser's
`navigator.credentials.create()` / `.get()` payloads are passed through as-is.
