# Quick Reference: Social Login Data Storage

## Summary

Enhanced user and social account models to store complete profile data from social login providers.

## What Changed?

### User Model - New Fields
```go
Name           string  // Full name
FirstName      string  // First name
LastName       string  // Last name
ProfilePicture string  // Picture URL
Locale         string  // Language/locale
```

### Social Account Model - New Fields
```go
Email          string          // Email from provider
Name           string          // Name from provider
FirstName      string          // First name
LastName       string          // Last name
ProfilePicture string          // Picture URL
Username       string          // GitHub login, etc.
Locale         string          // Locale
RawData        datatypes.JSON  // Complete provider response (JSONB)
```

### New Repository Method
```go
func (r *Repository) UpdateUser(user *models.User) error
```

## Data Captured by Provider

| Provider | Key Fields Captured |
|----------|-------------------|
| **Google** | id, email, verified_email, name, given_name, family_name, picture, locale |
| **Facebook** | id, email, name, first_name, last_name, picture.data.url, locale |
| **GitHub** | id, login, email, name, avatar_url, bio, location, company |

## Migration

- **Type:** GORM AutoMigrate (automatic)
- **Execution:** Runs on application startup
- **Impact:** Adds 5 columns to `users`, 8 columns to `social_accounts`
- **Breaking:** No - all fields nullable and backward compatible

## Files Modified

1. `pkg/models/user.go` - User model
2. `pkg/models/social_account.go` - Social account model
3. `internal/social/service.go` - Provider handlers
4. `internal/user/repository.go` - UpdateUser method

## Testing Quick Commands

```bash
# Build application
go build -o auth_api cmd/api/main.go

# Run application (migration runs automatically)
./auth_api

# Test social login
# 1. Visit: http://localhost:8080/auth/google/login
# 2. Complete OAuth flow
# 3. Get profile: curl -H "Authorization: Bearer {token}" http://localhost:8080/profile

# Check database
psql -U user -d authdb -c "SELECT name, first_name, profile_picture FROM users WHERE email='test@gmail.com';"
```

## API Response Example

```json
{
  "id": "...",
  "email": "john@gmail.com",
  "name": "John Doe",
  "first_name": "John",
  "last_name": "Doe",
  "profile_picture": "https://...",
  "locale": "en",
  "social_accounts": [
    {
      "provider": "google",
      "email": "john@gmail.com",
      "name": "John Doe",
      "raw_data": { /* complete Google response */ }
    }
  ]
}
```

## Key Behaviors

1. **New User:** All fields populated from social provider
2. **Linking Social Account:** Only updates empty user fields
3. **Existing Login:** No data changes on re-authentication

## Security Notes

- Profile pictures: URLs only (not downloaded)
- Raw data: Exposed in API (consider hiding if needed)
- Tokens: Still hidden from JSON responses
- All new fields: Nullable and optional

## Documentation

- Full Details: [docs/SOCIAL_LOGIN_DATA_STORAGE.md](SOCIAL_LOGIN_DATA_STORAGE.md)
- Migration: [docs/migrations/MIGRATION_SOCIAL_LOGIN_DATA.md](migrations/MIGRATION_SOCIAL_LOGIN_DATA.md)
- Changelog: [CHANGELOG.md](../CHANGELOG.md)

