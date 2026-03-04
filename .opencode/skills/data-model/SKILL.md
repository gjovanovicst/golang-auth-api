---
name: data-model
description: All 17 GORM database models with fields, relationships, indexes, and the entity relationship diagram for the Auth API.
license: MIT
---

## Overview

All models are in `pkg/models/`. The project uses PostgreSQL with GORM. All models except SystemSetting and SchemaMigration use UUID primary keys (`gen_random_uuid()`). Sensitive fields use `json:"-"` to prevent API exposure.

## Entity Relationship Diagram

```
Tenant
  |-- has-many --> Application
                      |-- has-many --> OAuthProviderConfig
                      |-- has-one  --> EmailServerConfig (nullable AppID)
                      |-- has-many --> ApiKey (ON DELETE CASCADE)
                      |-- has-many --> Role (via AppID)
                      |-- has-many --> EmailTemplate (nullable AppID)
                      |-- has-many --> WebAuthnCredential (nullable AppID)

User (scoped to AppID)
  |-- has-many --> SocialAccount
  |-- has-many --> UserRole
  |-- has-many --> ActivityLog (implicit)
  |-- has-many --> WebAuthnCredential (nullable UserID)

Role (scoped to AppID)
  |-- many-to-many --> Permission (join table: role_permissions)

UserRole (join table)
  |-- belongs-to --> User
  |-- belongs-to --> Role

EmailTemplate
  |-- belongs-to --> EmailType
  |-- belongs-to --> EmailServerConfig (nullable)

AdminAccount (standalone, system-level)
  |-- has-many --> WebAuthnCredential (via AdminID)

SystemSetting (standalone key-value store)
SchemaMigration (standalone migration tracker)
```

## Models

### User (`pkg/models/user.go`)

Table: `users`

| Field | Type | Key Tags | Notes |
|-------|------|----------|-------|
| ID | uuid.UUID | `primaryKey` | |
| AppID | uuid.UUID | `uniqueIndex:idx_email_app_id` | Default: `00000000-...0001` |
| Email | string | `uniqueIndex:idx_email_app_id` | Composite unique with AppID |
| PasswordHash | string | | `json:"-"` |
| EmailVerified | bool | | Default: false |
| IsActive | bool | | Default: true |
| Name, FirstName, LastName | string | | |
| ProfilePicture | string | | URL from social login |
| Locale | string | | |
| TwoFAEnabled | bool | | |
| TwoFAMethod | string | | "totp" or "email" |
| TwoFASecret | string | | `json:"-"` encrypted |
| TwoFARecoveryCodes | datatypes.JSON | `type:jsonb` | `json:"-"` encrypted |
| SocialAccounts | []SocialAccount | `foreignKey:UserID` | Has-Many |

### Tenant (`pkg/models/tenant.go`)

Table: `tenants`

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| Name | string | |
| Apps | []Application | `foreignKey:TenantID` Has-Many |

### Application (`pkg/models/application.go`)

Table: `applications`

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| TenantID | uuid.UUID | FK to Tenant |
| Name | string | |
| Description | string | |
| TwoFAIssuerName | string | Custom name in authenticator apps |
| TwoFAEnabled | bool | Master switch for 2FA |
| TwoFARequired | bool | Force all users to set up 2FA |
| Email2FAEnabled | bool | Email-based 2FA toggle |
| Passkey2FAEnabled | bool | Passkey as 2FA method |
| PasskeyLoginEnabled | bool | Passwordless login via passkey |
| MagicLinkEnabled | bool | Passwordless login via magic link |
| TwoFAMethods | string | Comma-separated: "totp", "email", "passkey" |
| OAuthProviderConfigs | []OAuthProviderConfig | `foreignKey:AppID` Has-Many |
| EmailServerConfig | *EmailServerConfig | `foreignKey:AppID` Has-One |

### Role (`pkg/models/role.go`)

Table: `roles`

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| AppID | uuid.UUID | `uniqueIndex:idx_role_app_name` |
| Name | string | `uniqueIndex:idx_role_app_name` |
| Description | string | |
| IsSystem | bool | Cannot be deleted |
| Permissions | []Permission | `many2many:role_permissions` |

### Permission (`pkg/models/role.go`, same file as Role)

Table: `permissions`

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| Resource | string | `uniqueIndex:idx_permission_resource_action` |
| Action | string | `uniqueIndex:idx_permission_resource_action` |
| Description | string | |

### UserRole (`pkg/models/role.go`, same file)

Table: `user_roles`

| Field | Type | Notes |
|-------|------|-------|
| UserID | uuid.UUID | Composite PK with RoleID |
| RoleID | uuid.UUID | Composite PK with UserID |
| AppID | uuid.UUID | Denormalized for fast lookup |
| AssignedAt | time.Time | |
| AssignedBy | *uuid.UUID | Nullable |
| Role | Role | Belongs-To |
| User | User | Belongs-To |

### AdminAccount (`pkg/models/admin_account.go`)

Table: `admin_accounts` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| Username | string | `uniqueIndex` |
| Email | string | `uniqueIndex` |
| PasswordHash | string | `json:"-"` |
| LastLoginAt | *time.Time | |
| TwoFAEnabled, TwoFAMethod | | Same pattern as User |
| TwoFASecret, TwoFARecoveryCodes | | `json:"-"` |
| MagicLinkEnabled | bool | |

Standalone entity -- not scoped to any application.

### SocialAccount (`pkg/models/social_account.go`)

Table: `social_accounts`

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| AppID | uuid.UUID | `uniqueIndex:idx_provider_user_id_app_id` |
| UserID | uuid.UUID | FK to User |
| Provider | string | "google", "facebook", "github" |
| ProviderUserID | string | `uniqueIndex:idx_provider_user_id_app_id` |
| Email, Name, FirstName, LastName | string | From provider |
| Username | string | e.g., GitHub login |
| RawData | datatypes.JSON | Complete raw JSON from provider |
| AccessToken, RefreshToken | string | `json:"-"` |

Composite unique: `(AppID, Provider, ProviderUserID)`

### WebAuthnCredential (`pkg/models/webauthn_credential.go`)

Table: `web_authn_credentials`

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| UserID | *uuid.UUID | Set for regular users (nullable) |
| AppID | *uuid.UUID | Set for regular users (nullable) |
| AdminID | *uuid.UUID | Set for admin passkeys (nullable) |
| CredentialID | []byte | `uniqueIndex`, `json:"-"` |
| PublicKey | []byte | `json:"-"` |
| Name | string | User-friendly label |
| Transports | string | Comma-separated: "usb,ble,nfc,internal" |

Polymorphic ownership: regular users use `UserID+AppID`, admins use `AdminID`.

### ActivityLog (`pkg/models/activity_log.go`)

Table: `activity_logs` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| AppID | uuid.UUID | Default: sentinel UUID |
| UserID | uuid.UUID | `index:idx_user_timestamp`, `index:idx_cleanup` |
| EventType | string | Indexed |
| Timestamp | time.Time | |
| IPAddress, UserAgent | string | |
| Details | json.RawMessage | `type:jsonb` |
| Severity | string | CRITICAL, IMPORTANT, INFORMATIONAL |
| ExpiresAt | *time.Time | `index:idx_expires` |
| IsAnomaly | bool | |

### ApiKey (`pkg/models/api_key.go`)

Table: `api_keys` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| KeyType | string | "admin" or "app" |
| Name, Description | string | |
| KeyHash | string | SHA-256, `uniqueIndex`, `json:"-"` |
| KeyPrefix | string | First 8 chars for display |
| KeySuffix | string | Last 4 chars |
| AppID | *uuid.UUID | Required when `key_type = "app"` |
| ExpiresAt | *time.Time | Optional |
| IsRevoked | bool | |
| Application | *Application | `ON DELETE CASCADE` |

### EmailType (`pkg/models/email_type.go`)

Table: `email_types` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| Code | string | `uniqueIndex` (e.g., "email_verification") |
| Name | string | |
| DefaultSubject | string | |
| Variables | datatypes.JSON | Array of EmailTypeVariable structs |
| IsSystem | bool | System types cannot be deleted |
| IsActive | bool | |

### EmailTemplate (`pkg/models/email_template.go`)

Table: `email_templates` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| AppID | *uuid.UUID | NULL = global default |
| EmailTypeID | uuid.UUID | `uniqueIndex:idx_app_email_type` |
| Subject, BodyHTML, BodyText | string | |
| TemplateEngine | string | "go_template", "placeholder", "raw_html" |
| FromEmail, FromName | string | Optional sender override |
| ServerConfigID | *uuid.UUID | Optional FK to EmailServerConfig |
| EmailType | EmailType | Belongs-To |
| ServerConfig | *EmailServerConfig | Belongs-To |

Composite unique: `(AppID, EmailTypeID)` -- one template per type per app scope.

### EmailServerConfig (`pkg/models/email_server_config.go`)

Table: `email_server_configs` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| AppID | *uuid.UUID | NULL = global/system config |
| Name | string | Label (e.g., "Transactional") |
| SMTPHost, SMTPPort | string, int | |
| SMTPUsername, SMTPPassword | string | Password: `json:"-"` |
| FromAddress, FromName | string | |
| UseTLS | bool | Default: true |
| IsDefault | bool | One default per scope |
| IsActive | bool | |

### OAuthProviderConfig (`pkg/models/oauth_provider_config.go`)

Table: `oauth_provider_configs` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| ID | uuid.UUID | |
| AppID | uuid.UUID | `uniqueIndex:idx_app_provider` |
| Provider | string | "google", "facebook", "github" |
| ClientID | string | |
| ClientSecret | string | `json:"-"` |
| RedirectURL | string | |
| IsEnabled | bool | |

### SystemSetting (`pkg/models/system_setting.go`)

Table: `system_settings` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| Key | string | **String primary key** (matches env var name) |
| Value | string | |
| Category | string | Indexed |

Resolution: env var > DB value > hardcoded default.

### SchemaMigration (`pkg/models/schema_migration.go`)

Table: `schema_migrations` (explicit TableName())

| Field | Type | Notes |
|-------|------|-------|
| ID | uint | **Auto-increment PK** (only model not using UUID) |
| Version | string | `uniqueIndex`, YYYYMMDD_HHMMSS format |
| Name | string | |
| ExecutionTimeMs | int | |
| Success | bool | |
| Checksum | string | SHA256 of migration file |

## Cross-Cutting Patterns

1. **UUID PKs everywhere** except SystemSetting (string) and SchemaMigration (uint)
2. **Multi-tenancy via AppID** on User, SocialAccount, ActivityLog, Role, OAuthProviderConfig, EmailServerConfig, EmailTemplate, ApiKey, WebAuthnCredential
3. **Default sentinel AppID:** `00000000-0000-0000-0000-000000000001`
4. **Sensitive field hiding:** All secrets use `json:"-"`
5. **Composite unique indexes** enforce per-app uniqueness
6. **Nullable AppID** enables global-vs-app-specific resolution (EmailTemplate, EmailServerConfig)

## When To Use This Skill

Load this skill when working on database models, migrations, repository queries, or any feature that touches data persistence.
