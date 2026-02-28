# Admin GUI

The Admin GUI is a built-in web panel for managing the Authentication API. It runs from the same binary at `/gui/*` and requires no separate frontend deployment.

---

## Initial Setup

Create the admin account using the interactive CLI wizard:

```bash
go run cmd/setup/main.go
```

You will be prompted for a username and password (masked input). The account is stored with a bcrypt-hashed password in the database.

---

## Accessing the GUI

Once the server is running, navigate to:

```
http://localhost:8080/gui/login
```

Log in with the credentials created during setup.

---

## Pages

| Page | Description |
|------|-------------|
| **Dashboard** | Overview of tenants, apps, users, and recent activity |
| **Tenants** | Create, edit, delete tenant organizations |
| **Applications** | Manage apps per tenant with flat list and tenant filter |
| **OAuth Configs** | Configure OAuth providers per-app with inline toggle |
| **Users** | Search users, view details, toggle active/inactive |
| **Activity Logs** | View and filter activity logs with inline detail |
| **API Keys** | Manage admin and per-app API keys |
| **Settings** | View and override system settings |

---

## Technology Stack

- **Go Templates** with layout/partial composition
- **HTMX** for single-page interactions without full page reloads
- **Bootstrap 5** for responsive UI
- **Bootstrap Icons** for iconography
- All assets embedded via `go:embed` -- no external CDN dependencies
