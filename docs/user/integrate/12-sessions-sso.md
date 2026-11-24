# Sessions and SSO

## Sessions

Every login creates a server-side session in Redis, keyed by application and session id.
A session stores the refresh token, the IP address, the user agent and timestamps, so a
user can see and revoke their own logins.

| Endpoint | Purpose |
|---|---|
| `GET /sessions` | List active sessions for the current user |
| `DELETE /sessions/:id` | Revoke one session |
| `DELETE /sessions` | Revoke every session except the current one |

```bash
curl -s http://localhost:8080/sessions \
  -H "X-App-ID: $APP" -H "Authorization: Bearer $ACCESS_TOKEN"
```

```json
[
  {
    "id": "6f1c…",
    "ip": "203.0.113.10",
    "user_agent": "Mozilla/5.0 …",
    "created_at": "2026-11-23T21:12:04Z",
    "last_active": "2026-11-23T22:41:19Z"
  }
]
```

Revoking a session deletes its refresh token, so it cannot be refreshed. The current
access token stays valid until it expires (15 minutes by default) unless the user was
blacklisted too — logout does both.

Sessions can also be revoked from the admin GUI (*Sessions* page) and from the user list
(*Users → Sessions → Revoke all*).

## Session groups (cross-app SSO)

A **session group** links several applications so one login is valid across them. It is
managed in the admin GUI: create a group, add applications, optionally enable
`global_logout`.

```
Session group "Acme suite"
 ├── Acme Web      (app 1)
 ├── Acme Docs     (app 2)
 └── Acme Billing  (app 3)
```

With `global_logout`, a logout (or an expired session) in any member application revokes
the user's sessions in every other member. Expiry detection runs on Redis keyspace
notifications, with a periodic scan as a fallback (`SESSION_GROUP_EXPIRY_SCAN_INTERVAL`).

### SSO token exchange

When a user is authenticated in app A and opens app B, the frontend can mint a short-lived
single-use token in A and redeem it in B:

```bash
# in app A (user is logged in there)
curl -sX POST http://localhost:8080/sso/token \
  -H "X-App-ID: $APP_A" -H "Authorization: Bearer $TOKEN_A"

# in app B
curl -sX POST http://localhost:8080/sso/exchange \
  -H "X-App-ID: $APP_B" -H "Content-Type: application/json" \
  -d '{"token":"'$SSO_TOKEN'"}'
```

`GET /sso/peers` returns the applications in the same group so the frontend knows where
the user can move without a password. The exchange token is valid for 60 seconds and can
be redeemed exactly once.

## OIDC provider

Any application can act as an OpenID Connect issuer. Enable it per application
(`oidc_enabled`) and register clients in the admin GUI (*OIDC Clients*).

| Endpoint | Purpose |
|---|---|
| `GET /oidc/:app_id/.well-known/openid-configuration` | Discovery document |
| `GET /oidc/:app_id/.well-known/jwks.json` | Public signing keys |
| `GET/POST /oidc/:app_id/authorize` | Login + consent |
| `POST /oidc/:app_id/token` | Authorization code or refresh grant |
| `GET/POST /oidc/:app_id/userinfo` | Claims for the access token |
| `POST /oidc/:app_id/introspect` | Token introspection |
| `POST /oidc/:app_id/revoke` | Token revocation |
| `GET/POST /oidc/:app_id/end_session` | RP-initiated logout |

A global redirect exists for convenience:
`GET /.well-known/openid-configuration` → the default application.

Clients support PKCE, confidential and public types, configurable login theme (inherit
from the application or force light/dark) and per-client branding.
