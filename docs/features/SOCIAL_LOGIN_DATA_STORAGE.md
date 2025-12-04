# Social Login Data Storage

## Overview

This document describes how social login data is captured and stored in the authentication API. The system now stores comprehensive profile information from social providers (Google, Facebook, GitHub) to enhance user experience and provide complete user profiles.

## Architecture

### Data Storage Strategy

The system uses a dual-storage approach:

1. **User Model** (`users` table) - Stores normalized profile data across all sources
2. **Social Account Model** (`social_accounts` table) - Stores provider-specific data and raw responses

This design allows:
- Single source of truth for user profile data
- Provider-specific data retention for audit and debugging
- Flexibility to sync data from multiple social providers
- Future extensibility without schema changes (via `RawData` field)

## Data Captured by Provider

### Google OAuth2

**Endpoint:** `https://www.googleapis.com/oauth2/v2/userinfo`

**Fields Captured:**
```json
{
  "id": "string",
  "email": "string",
  "verified_email": "boolean",
  "name": "string",
  "given_name": "string",
  "family_name": "string",
  "picture": "string (URL)",
  "locale": "string (e.g., 'en')"
}
```

**Mapping:**
- `id` → `SocialAccount.ProviderUserID`
- `email` → `User.Email`, `SocialAccount.Email`
- `verified_email` → `User.EmailVerified`
- `name` → `User.Name`, `SocialAccount.Name`
- `given_name` → `User.FirstName`, `SocialAccount.FirstName`
- `family_name` → `User.LastName`, `SocialAccount.LastName`
- `picture` → `User.ProfilePicture`, `SocialAccount.ProfilePicture`
- `locale` → `User.Locale`, `SocialAccount.Locale`
- Complete response → `SocialAccount.RawData` (JSONB)

### Facebook Graph API

**Endpoint:** `https://graph.facebook.com/v18.0/me`

**Query Parameters:** `fields=id,name,email,first_name,last_name,picture.type(large),locale`

**Fields Captured:**
```json
{
  "id": "string",
  "email": "string",
  "name": "string",
  "first_name": "string",
  "last_name": "string",
  "picture": {
    "data": {
      "url": "string"
    }
  },
  "locale": "string"
}
```

**Mapping:**
- `id` → `SocialAccount.ProviderUserID`
- `email` → `User.Email`, `SocialAccount.Email`
- `name` → `User.Name`, `SocialAccount.Name`
- `first_name` → `User.FirstName`, `SocialAccount.FirstName`
- `last_name` → `User.LastName`, `SocialAccount.LastName`
- `picture.data.url` → `User.ProfilePicture`, `SocialAccount.ProfilePicture`
- `locale` → `User.Locale`, `SocialAccount.Locale`
- Email assumed verified → `User.EmailVerified = true`
- Complete response → `SocialAccount.RawData` (JSONB)

### GitHub API

**Primary Endpoint:** `https://api.github.com/user`

**Fallback Endpoint:** `https://api.github.com/user/emails` (if email is private)

**Fields Captured:**
```json
{
  "id": "integer",
  "login": "string",
  "email": "string",
  "name": "string",
  "avatar_url": "string",
  "bio": "string",
  "location": "string",
  "company": "string"
}
```

**Email Endpoint Response:**
```json
[
  {
    "email": "string",
    "primary": "boolean",
    "verified": "boolean"
  }
]
```

**Mapping:**
- `id` → `SocialAccount.ProviderUserID` (converted to string)
- `email` → `User.Email`, `SocialAccount.Email`
- `name` → `User.Name`, `SocialAccount.Name`
- `login` → `SocialAccount.Username`
- `avatar_url` → `User.ProfilePicture`, `SocialAccount.ProfilePicture`
- Primary verified email assumed verified → `User.EmailVerified = true`
- Complete response → `SocialAccount.RawData` (JSONB)

**Note:** GitHub's bio, location, and company fields are stored only in `RawData` for potential future use.

## Data Flow Scenarios

### Scenario 1: New User Registration via Social Login

**Flow:**
1. User authenticates with social provider
2. System fetches user data from provider API
3. Check if social account exists → **NOT FOUND**
4. Check if user with email exists → **NOT FOUND**
5. **Create new user** with all available profile data
6. **Create social account** with provider-specific data and link to user
7. Generate JWT tokens and return

**Result:** New user record with complete profile, linked social account.

### Scenario 2: Existing User Links Social Account

**Flow:**
1. User authenticates with social provider
2. System fetches user data from provider API
3. Check if social account exists → **NOT FOUND**
4. Check if user with email exists → **FOUND**
5. **Update user profile** (only empty fields) with social data
6. **Create social account** with provider-specific data and link to existing user
7. Generate JWT tokens and return

**Result:** Existing user enriched with social profile data, new social account linked.

**Smart Update Logic:**
```go
// Only update if field is currently empty
if user.Name == "" && socialData.Name != "" {
    user.Name = socialData.Name
}
```

### Scenario 3: Existing User Re-authenticates via Social Login

**Flow:**
1. User authenticates with social provider
2. System fetches user data from provider API
3. Check if social account exists → **FOUND**
4. **No updates** to user or social account data
5. Generate JWT tokens and return

**Result:** User authenticated, no data changes.

**Note:** Future enhancement could add profile sync option to refresh data.

## Database Schema

### Users Table

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR NOT NULL UNIQUE,
    password_hash VARCHAR,
    email_verified BOOLEAN DEFAULT false,
    name VARCHAR,                    -- NEW
    first_name VARCHAR,              -- NEW
    last_name VARCHAR,               -- NEW
    profile_picture VARCHAR,         -- NEW
    locale VARCHAR,                  -- NEW
    two_fa_enabled BOOLEAN DEFAULT false,
    two_fa_secret VARCHAR,
    two_fa_recovery_codes JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Social Accounts Table

```sql
CREATE TABLE social_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    provider VARCHAR NOT NULL,
    provider_user_id VARCHAR NOT NULL,
    email VARCHAR,                   -- NEW
    name VARCHAR,                    -- NEW
    first_name VARCHAR,              -- NEW
    last_name VARCHAR,               -- NEW
    profile_picture VARCHAR,         -- NEW
    username VARCHAR,                -- NEW
    locale VARCHAR,                  -- NEW
    raw_data JSONB,                  -- NEW
    access_token VARCHAR,
    refresh_token VARCHAR,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_user_id)
);

CREATE INDEX idx_social_accounts_user_id ON social_accounts(user_id);
CREATE INDEX idx_social_accounts_provider ON social_accounts(provider);
```

## API Response Examples

### GET /profile

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "email": "john.doe@gmail.com",
  "email_verified": true,
  "name": "John Doe",
  "first_name": "John",
  "last_name": "Doe",
  "profile_picture": "https://lh3.googleusercontent.com/a/...",
  "locale": "en",
  "two_fa_enabled": false,
  "created_at": "2025-11-01T10:00:00Z",
  "updated_at": "2025-11-08T14:30:00Z",
  "social_accounts": [
    {
      "id": "223e4567-e89b-12d3-a456-426614174111",
      "user_id": "123e4567-e89b-12d3-a456-426614174000",
      "provider": "google",
      "provider_user_id": "1234567890",
      "email": "john.doe@gmail.com",
      "name": "John Doe",
      "first_name": "John",
      "last_name": "Doe",
      "profile_picture": "https://lh3.googleusercontent.com/a/...",
      "username": "",
      "locale": "en",
      "raw_data": {
        "id": "1234567890",
        "email": "john.doe@gmail.com",
        "verified_email": true,
        "name": "John Doe",
        "given_name": "John",
        "family_name": "Doe",
        "picture": "https://lh3.googleusercontent.com/a/...",
        "locale": "en"
      },
      "expires_at": null,
      "created_at": "2025-11-01T10:00:00Z",
      "updated_at": "2025-11-01T10:00:00Z"
    }
  ]
}
```

### POST /auth/google/callback (Login Success)

**Response:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "123e4567-e89b-12d3-a456-426614174000"
}
```

## Security Considerations

### Data Privacy

1. **Sensitive Fields Hidden:**
   - `password_hash` - Never exposed via JSON
   - `two_fa_secret` - Never exposed via JSON
   - `two_fa_recovery_codes` - Never exposed via JSON
   - `access_token` (social) - Never exposed via JSON
   - `refresh_token` (social) - Never exposed via JSON

2. **Raw Data Exposure:**
   - `raw_data` field IS exposed in API responses
   - Contains complete provider response
   - **Consider:** Add `json:"-"` tag if sensitive data concerns arise

### Profile Picture URLs

- Stored as URLs, not downloaded or cached
- Provider URLs may expire based on provider policies
- Consider implementing CDN or local caching for production

### Email Privacy

- Social provider emails may differ from user's primary email
- GitHub users can hide emails; fallback logic implemented
- Respect user privacy settings from providers

## Best Practices

### For Developers

1. **Always check fields exist** before using from `RawData`:
   ```go
   if rawData["field"] != nil {
       value := rawData["field"].(string)
   }
   ```

2. **Validate provider-specific data** structure before unmarshaling

3. **Handle missing optional fields** gracefully

4. **Log provider API errors** for debugging

### For API Consumers

1. **Don't rely on optional fields** always being present
2. **Profile pictures may be null** for users without social login
3. **Raw data structure varies** by provider
4. **Check email_verified** before trusting email addresses

## Testing

### Test Cases

1. **New user registration:**
   - Via Google ✓
   - Via Facebook ✓
   - Via GitHub ✓
   - Verify all fields populated

2. **Existing user linking:**
   - Link Google to existing account ✓
   - Link Facebook to existing account ✓
   - Link GitHub to existing account ✓
   - Verify profile enrichment

3. **Multiple social accounts:**
   - Same user with Google and Facebook ✓
   - Profile data from first provider preserved ✓

4. **Edge cases:**
   - Missing optional fields (e.g., GitHub name)
   - Private GitHub email
   - Facebook user without profile picture
   - Invalid/expired provider tokens

### Manual Testing Commands

```bash
# Check user profile after social login
curl -H "Authorization: Bearer {token}" \
  http://localhost:8080/profile

# Verify database schema
psql -U user -d authdb -c "\d users"
psql -U user -d authdb -c "\d social_accounts"

# Query raw data
psql -U user -d authdb -c \
  "SELECT email, name, raw_data FROM social_accounts WHERE provider='google';"
```

## Future Enhancements

### Planned Features

1. **Profile Sync Endpoint**
   - `POST /profile/sync/:provider`
   - Refresh user data from social provider
   - Update stale profile information

2. **Profile Picture Management**
   - Download and cache profile pictures
   - Allow user-uploaded pictures
   - CDN integration

3. **Privacy Controls**
   - Let users choose which social data to display
   - Control profile picture visibility
   - Opt-out of data syncing

4. **Additional Providers**
   - Twitter/X OAuth
   - LinkedIn OAuth
   - Microsoft OAuth
   - Apple Sign In

5. **Data Retention Policies**
   - Automatic removal of old `raw_data`
   - Refresh stale profile data
   - Compliance with GDPR/privacy laws

### Technical Improvements

1. **Field Validation:**
   - URL validation for profile pictures
   - Locale format validation
   - Name length restrictions

2. **Performance:**
   - Cache profile pictures
   - Index frequently queried fields
   - Optimize JSONB queries on `raw_data`

3. **Monitoring:**
   - Track provider API response times
   - Alert on provider API failures
   - Log data quality issues

## Troubleshooting

### Common Issues

**Issue:** Profile picture not showing
- **Cause:** Provider URL expired
- **Solution:** Implement profile sync or download pictures

**Issue:** GitHub email is null
- **Cause:** User has private email settings
- **Solution:** Request `user:email` scope and fetch from `/user/emails`

**Issue:** Raw data too large
- **Cause:** Provider response includes extra data
- **Solution:** Increase JSONB column size or filter data before storage

**Issue:** Duplicate social accounts
- **Cause:** Unique constraint on (provider, provider_user_id)
- **Solution:** Check if account exists before creating

## Related Documentation

- [Migration Guide](migrations/MIGRATION_SOCIAL_LOGIN_DATA.md)
- [API Documentation](../README.md)
- [Security Patterns](../.cursor/rules/security-patterns.mdc)
- [Phase 3: Social Authentication](implementation_phases/Phase_3._Social_Authentication_Integration_Plan.md)

