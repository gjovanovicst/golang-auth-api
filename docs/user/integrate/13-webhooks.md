# Webhooks

Webhooks push authentication events to your systems: a Slack alert on a new social link, a
CRM sync on registration, an audit sink for password changes.

## Events

| Event | Fires when |
|---|---|
| `user.registered` | A user registers (before or after email verification, depending on the app) |
| `user.verified` | The email address is verified |
| `user.login` | A successful login (including after 2FA) |
| `user.password_changed` | The password is changed by the user |
| `user.password_set` | A password is set on a social-only account |
| `2fa.enabled` / `2fa.disabled` | A second factor is enabled or disabled |
| `2fa.sms_recovery.sent` | An SMS recovery code was requested during login |
| `social.linked` / `social.unlinked` | A social account is attached or removed |

## Managing endpoints

Via the admin API (global, `X-Admin-API-Key`) or the app API (scoped, `X-App-ID` +
`X-App-API-Key`):

| Action | Admin | App-scoped |
|---|---|---|
| List | `GET /admin/webhooks` | `GET /app/:id/webhooks` |
| Create | `POST /admin/webhooks/apps/:app_id` | `POST /app/:id/webhooks` |
| Toggle | `PUT /admin/webhooks/:id/toggle` | `PUT /app/:id/webhooks/:webhook_id/toggle` |
| Delete | `DELETE /admin/webhooks/:id` | `DELETE /app/:id/webhooks/:webhook_id` |
| Deliveries | `GET /admin/webhooks/:id/deliveries` | `GET /app/:id/webhooks/deliveries` |

The same endpoints exist in the admin GUI under *Webhooks*.

## Delivery format

Each delivery is a `POST` with a JSON body and three headers:

```
Content-Type: application/json
X-Webhook-Event: user.registered
X-Webhook-App-ID: 00000000-0000-0000-0000-000000000001
X-Webhook-Signature: sha256=<hex>
```

The signature is an HMAC-SHA256 of the raw request body using the endpoint's secret:

```js
const expected = "sha256=" + crypto
  .createHmac("sha256", endpointSecret)
  .update(rawBody)
  .digest("hex");

if (!crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(header))) {
  return res.status(401).end();
}
```

Always compare in constant time and compute the HMAC over the **raw** body, before JSON
parsing.

## Retries

A delivery that returns a non-2xx status is retried with backoff; every attempt is stored
with its status code, response body, latency and the next retry time. Inspect them in the
GUI (*Webhooks → Deliveries*) or through the deliveries endpoint above. Endpoints can be
disabled without deleting their history.

## Choosing between webhooks and polling

- **Webhooks** — react to an event in near real time. Treat them as at-least-once: make
  the handler idempotent using the event type plus a stable identifier from the payload.
- **Polling** — `GET /admin/activity-logs` (or the per-user logs endpoint) is the durable
  record; use it for reconciliation when a webhook was missed.
