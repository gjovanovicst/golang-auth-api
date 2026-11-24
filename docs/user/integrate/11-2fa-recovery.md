# Two-factor and recovery

A user can protect their account with TOTP, email codes, SMS, a backup email or a
passkey. Whatever the primary factor is, there must be a way back in when the user
loses it — this page covers both halves.

## Which methods are available

```bash
curl -s http://localhost:8080/2fa/methods -H "X-App-ID: $APP"
```

Returns the methods enabled for the application (`available_methods`) and convenience
booleans (`totp_enabled`, `email_2fa_enabled`, `passkey_enabled`, `sms_enabled`).
The application's master switches live in the admin GUI under *Applications → 2FA*.

## TOTP (authenticator app)

```bash
# 1. generate a secret (JWT required)
curl -sX POST http://localhost:8080/2fa/generate -H "X-App-ID: $APP" -H "Authorization: Bearer $TOKEN"

# 2. prove the device works
curl -sX POST http://localhost:8080/2fa/verify-setup -H "X-App-ID: $APP" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"code":"123456"}'

# 3. turn it on; the response contains recovery codes
curl -sX POST http://localhost:8080/2fa/enable -H "X-App-ID: $APP" -H "Authorization: Bearer $TOKEN"
```

The enrollment response contains the shared secret and an `otpauth://` URI you render as
a QR code. Show the recovery codes exactly once.

## Email, SMS and backup email

| Method | Enable | Notes |
|---|---|---|
| Email | `POST /2fa/email/enable` | Code sent at login to the account email |
| SMS | `POST /phone` → `POST /phone/verify` → `POST /2fa/sms/enable` | Requires an SMS provider and `sms_2fa_enabled` on the app |
| Backup email | `POST /2fa/backup-email` → verify link → `POST /2fa/backup-email/enable` | Second address used only for codes |

## Passkey 2FA

`POST /2fa/passkey/begin` returns a WebAuthn challenge, `POST /2fa/passkey/finish`
verifies the assertion. The primary factor stays whatever the user had; the passkey
replaces the code entry.

## Verifying at login

`POST /login` returns `requires_2fa` plus a `temp_token` (10 minutes). Submit the factor:

```bash
# a code for the user's primary method (TOTP/email/SMS/backup email)
curl -sX POST http://localhost:8080/2fa/login-verify \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"temp_token":"'$TEMP'","code":"123456"}'

# or a recovery code
curl -sX POST http://localhost:8080/2fa/login-verify \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"temp_token":"'$TEMP'","recovery_code":"0a1b2c3d4e5f6071"}'
```

On success the response is the normal token pair. `remember_device: true` with a
`device_name` stores a trusted device (if the application enables it), so the next login
from that browser skips 2FA for `trusted_device_max_days`.

Resend endpoints: `/2fa/email/resend`, `/2fa/sms/resend`, `/2fa/backup-email/resend`
(all take `{"temp_token":"…"}`).

## Recovery codes

`POST /2fa/enable` returns eight single-use codes. Each is 16 hex characters and is
removed from the database when used. Regenerate with:

```bash
curl -sX POST http://localhost:8080/2fa/recovery-codes \
  -H "X-App-ID: $APP" -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" -d '{"code":"123456"}'
```

## SMS recovery fallback

When the user still knows their password but lost the authenticator, SMS can act as a
recovery channel **even if SMS is not their primary factor**.

```bash
# 1. during the 2FA challenge, ask for a code by SMS
curl -sX POST http://localhost:8080/2fa/sms/recovery/send \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"temp_token":"'$TEMP'"}'

# 2. submit it with an explicit method
curl -sX POST http://localhost:8080/2fa/login-verify \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"temp_token":"'$TEMP'","method":"sms_recovery","code":"654321"}'
```

Requirements: `sms_2fa_enabled` on the application, a configured SMS provider, 2FA
active on the user and a verified phone number. The `/login` response sets
`sms_recovery_available: true` when all of those hold, so the UI only offers the fallback
when it can work. The code lives five minutes, is single-use and is logged as
`RECOVERY_CODE_USED` with `details.method = "sms_recovery"`.

## Trusted devices

| Endpoint | Purpose |
|---|---|
| `GET /2fa/trusted-devices` | List devices (`settings:read`) |
| `DELETE /2fa/trusted-devices/:id` | Revoke one |
| `DELETE /2fa/trusted-devices` | Revoke all |

A trusted device is a hashed token in an `HttpOnly` cookie. It bypasses 2FA for the
configured number of days, and the list shows the user agent and IP that created it.

## Disabling 2FA

`POST /2fa/disable` with the current code. The application can require 2FA for every
user (`two_fa_required`), in which case the account is asked to enrol at the next login
and the GUI shows a setup-required response instead of tokens.
