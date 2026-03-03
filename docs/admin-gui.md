# Admin GUI

The Admin GUI is a built-in web panel for managing the Authentication API. It runs from the same binary at `/gui/*` and requires no separate frontend deployment.

---

## Initial Setup

Create the admin account using the interactive CLI wizard:

```bash
go run cmd/setup/main.go
```

You will be prompted for a username, email, and password (masked input). The account is stored with a bcrypt-hashed password in the database.

---

## Accessing the GUI

Once the server is running, navigate to:

```
http://localhost:8080/gui/login
```

The login page supports three authentication methods:

- **Username/Password** -- Standard credential-based login
- **Passkey** -- Passwordless login using a registered FIDO2 passkey
- **Magic Link** -- Passwordless login via email (requires magic link to be enabled on the admin account)

If the admin account has two-factor authentication enabled, a 2FA verification step is required after the initial login.

---

## Pages

| Page | Description |
|------|-------------|
| **Dashboard** | Overview of tenants, apps, users, and recent activity |
| **Tenants** | Create, edit, delete tenant organizations |
| **Applications** | Manage apps per tenant with flat list and tenant filter |
| **OAuth Configs** | Configure OAuth providers per-app with inline toggle |
| **Users** | Search users, view details, toggle active/inactive, view sessions, manage social accounts |
| **Roles** | Create, edit, delete roles per application with permission assignment |
| **Permissions** | Create and manage granular permissions (resource:action format) |
| **User Roles** | Assign and revoke roles for users across applications |
| **Sessions** | View all active sessions across users, revoke individual or bulk sessions |
| **Activity Logs** | View and filter activity logs with inline detail |
| **API Keys** | Manage admin and per-app API keys |
| **Email Servers** | Configure SMTP email servers per application |
| **Email Templates** | Manage email templates with preview and reset to default |
| **Email Types** | Configure email type settings |
| **Settings** | View and override system settings |
| **My Account** | Admin profile, 2FA setup, passkey management, magic link toggle |

---

## My Account

The My Account page allows admin users to manage their own account:

- **Update Email** -- Change the admin account email address
- **Change Password** -- Update the admin account password
- **Two-Factor Authentication** -- Enable/disable TOTP or email-based 2FA, regenerate recovery codes
- **Passkey Management** -- Register, rename, and delete FIDO2 passkeys for passwordless login
- **Magic Link** -- Enable/disable magic link authentication for the admin account
- **Social Accounts** -- View and unlink social accounts (when applicable)

---

## Session Management

The Sessions page provides administrative oversight of all active user sessions:

- **Session List** -- View all active sessions with user, IP address, user agent, and timestamps
- **Session Detail** -- Inspect individual session metadata
- **Revoke Session** -- Terminate a specific user session
- **Revoke All Sessions** -- Terminate all sessions for a specific user
- **User Sessions** -- View sessions for a specific user from the user detail panel

---

## RBAC Management

The Roles, Permissions, and User Roles pages provide full RBAC administration:

- **Roles** are scoped per-application. System roles (admin, member) cannot be deleted.
- **Permissions** use a `resource:action` format (e.g., `users:read`, `articles:publish`).
- **User Roles** can be assigned and revoked with application and user search/filtering.
- Role permissions can be managed inline with checkbox-based assignment.

Default roles seeded on first run:
- **admin** -- Full administrative access (system role)
- **member** -- Standard user access (system role, auto-assigned to new users)

---

## Technology Stack

- **Go Templates** with layout/partial composition
- **HTMX** for single-page interactions without full page reloads
- **Bootstrap 5** for responsive UI
- **Bootstrap Icons** for iconography
- All assets embedded via `go:embed` -- no external CDN dependencies
