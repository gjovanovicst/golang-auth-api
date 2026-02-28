# Multi-Tenancy

The API supports multi-tenancy, allowing you to serve multiple organizations (tenants) and applications from a single deployment. Each application has isolated users, OAuth configurations, and activity logs.

---

## Hierarchy

```
Tenant (Organization)
 └── Application (Mobile App, Web App, etc.)
      ├── Users (isolated per app)
      ├── OAuth Providers (per-app credentials)
      └── Activity Logs (per-app audit trail)
```

---

## Default Setup

On first installation, a default tenant and application are created automatically:

- **Default Tenant ID:** `00000000-0000-0000-0000-000000000001`
- **Default Application ID:** `00000000-0000-0000-0000-000000000001`

---

## Required Header

All API requests must include the `X-App-ID` header:

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123!@#"
  }'
```

**Exceptions (no header required):**
- `/swagger/*` - Swagger documentation
- `/admin/*` - Admin API endpoints
- OAuth callbacks (app_id passed in state parameter)

---

## Creating Tenants and Applications

### 1. Create a Tenant

```bash
curl -X POST http://localhost:8080/admin/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"name": "Acme Corporation"}'
```

Response:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Acme Corporation",
  "created_at": "2026-01-19T12:00:00Z",
  "updated_at": "2026-01-19T12:00:00Z"
}
```

### 2. Create an Application

```bash
curl -X POST http://localhost:8080/admin/apps \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Mobile App",
    "description": "iOS and Android application"
  }'
```

### 3. Configure OAuth for an Application

```bash
curl -X POST http://localhost:8080/admin/oauth-providers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "app_id": "660e8400-e29b-41d4-a716-446655440000",
    "provider": "google",
    "client_id": "your-google-client-id.apps.googleusercontent.com",
    "client_secret": "your-google-client-secret",
    "redirect_url": "https://mobile-app.example.com/auth/google/callback",
    "is_enabled": true
  }'
```

---

## OAuth Configuration

OAuth credentials are stored in the database per-application:

- Different OAuth credentials per application
- Runtime configuration changes (no restart needed)
- Centralized management via Admin API
- Fallback to environment variables for the default app

To migrate existing credentials from `.env` to the database:

```bash
go run cmd/migrate_oauth/main.go
```

---

## Data Isolation

Complete isolation between applications:

- Users are scoped to `app_id` (same email can exist in different apps)
- Social accounts linked per application
- Activity logs segmented by application
- JWT tokens include `app_id` claim (prevents cross-app token reuse)
- 2FA secrets and recovery codes isolated per app

Database-level enforcement:

```sql
-- Email uniqueness is per-application (not global)
CREATE UNIQUE INDEX idx_email_app_id ON users(email, app_id);

-- All user data has foreign key to applications
ALTER TABLE users ADD CONSTRAINT fk_users_app
  FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;
```

---

## Use Cases

**SaaS Providers** - Serve multiple clients from a single deployment with isolated data and per-client OAuth branding.

**Multiple Applications** - Same company running different apps (mobile, web, desktop) with separate user bases and analytics.

**White-Label Solutions** - Deploy once, serve many brands with customized OAuth and complete data separation.
