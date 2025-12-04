# Profile Sync on Social Login

## Overview

The authentication API now automatically synchronizes user profile data from social providers (Google, Facebook, GitHub) **every time** a user logs in. This ensures that profile information stays up-to-date with changes made on social platforms.

## Feature Description

### What Gets Updated

When a user logs in via social provider, the system updates:

#### Both Social Account and User Profile:
- **Profile Picture** - Always updated to latest from provider
- **Name** - Full name from social profile
- **First Name** - User's first name
- **Last Name** - User's last name
- **Locale** - Language preference

#### Social Account Only:
- **Email** - Provider email (may differ from primary email)
- **Username** - GitHub login, etc.
- **Raw Data** - Complete provider response (JSONB)
- **Access Token** - Latest OAuth token

### Update Strategy

The system uses a **smart update strategy**:

1. **Social Account Data**: Always updated with latest from provider
2. **User Profile Data**: Only updated if the value has changed
3. **Non-Breaking**: Failed updates log errors but don't prevent authentication
4. **Transactional**: Updates happen before token generation

### Example Use Cases

#### Use Case 1: User Changes Profile Picture on Google
```
1. User changes profile picture on Google account
2. User logs into your app via Google
3. System fetches new profile picture URL
4. Updates social_accounts.profile_picture
5. Updates users.profile_picture
6. User sees new picture in app immediately
```

#### Use Case 2: User Changes Name on Facebook
```
1. User changes name on Facebook (e.g., after marriage)
2. User logs into your app via Facebook
3. System fetches new name data
4. Updates social_accounts: name, first_name, last_name
5. Updates users: name, first_name, last_name
6. App displays updated name everywhere
```

#### Use Case 3: GitHub User Updates Avatar
```
1. User updates GitHub avatar
2. User logs into your app via GitHub
3. System fetches new avatar_url
4. Updates both social_accounts and users tables
5. Profile picture reflects GitHub change
```

## Technical Implementation

### Code Flow

```
User clicks "Login with Google"
    ↓
OAuth callback received
    ↓
Fetch user data from Google API
    ↓
Check if social account exists
    ↓
YES → Update Flow
    ├─ Update social account with ALL new data
    ├─ Fetch user record
    ├─ Compare each field
    ├─ Update only changed fields in user profile
    ├─ Log any errors (but continue)
    └─ Generate tokens and authenticate
    
NO → New User/Link Flow
    └─ Create new records with data
```

### Google Profile Sync

**Updated Fields:**
```go
socialAccount.Email = googleUser.Email
socialAccount.Name = googleUser.Name
socialAccount.FirstName = googleUser.GivenName
socialAccount.LastName = googleUser.FamilyName
socialAccount.ProfilePicture = googleUser.Picture
socialAccount.Locale = googleUser.Locale
socialAccount.RawData = complete_google_response
socialAccount.AccessToken = latest_token
```

**User Profile Updates:**
```go
// Only updates if value changed and not empty
if user.Name != googleUser.Name && googleUser.Name != "" {
    user.Name = googleUser.Name
}
// Same for: FirstName, LastName, ProfilePicture, Locale
```

### Facebook Profile Sync

**Additional Facebook-Specific:**
- Fetches large profile picture: `picture.type(large)`
- Updates picture from nested structure: `facebookUser.Picture.Data.URL`

### GitHub Profile Sync

**GitHub-Specific:**
- Updates `Username` field with GitHub login
- Handles private email scenario (fetches from `/user/emails`)
- Stores avatar URL

## Database Impact

### Updated Tables

**social_accounts table:**
- All profile fields updated on each login
- `updated_at` timestamp reflects last login
- `raw_data` contains latest provider response

**users table:**
- Profile fields updated only if changed
- `updated_at` timestamp reflects last profile change
- Preserves user data if provider data is empty

### SQL Generated (Example)

```sql
-- Social account update
UPDATE social_accounts 
SET email = 'user@gmail.com',
    name = 'John Doe',
    first_name = 'John',
    last_name = 'Doe',
    profile_picture = 'https://new-picture-url',
    locale = 'en',
    raw_data = '{"id":"123","email":"user@gmail.com",...}',
    access_token = 'new_token',
    updated_at = CURRENT_TIMESTAMP
WHERE id = 'social-account-uuid';

-- User profile update (only if changed)
UPDATE users 
SET profile_picture = 'https://new-picture-url',
    updated_at = CURRENT_TIMESTAMP
WHERE id = 'user-uuid';
```

## API Behavior

### Login Response

**No changes to API response structure:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "123e4567-e89b-12d3-a456-426614174000"
}
```

### Profile Endpoint

**GET /profile** now returns updated data:
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "email": "user@gmail.com",
  "name": "John Doe",
  "first_name": "John",
  "last_name": "Doe",
  "profile_picture": "https://latest-picture-url",  ← Updated on login
  "locale": "en",
  "email_verified": true,
  "social_accounts": [
    {
      "provider": "google",
      "name": "John Doe",  ← Updated on login
      "profile_picture": "https://latest-picture-url",  ← Updated on login
      "updated_at": "2025-11-08T15:30:00Z"  ← Reflects last login
    }
  ]
}
```

## Performance Considerations

### Database Operations

**Per Social Login (existing user):**
- 1 SELECT (check social account exists)
- 1 UPDATE (social account)
- 1 SELECT (get user profile)
- 0-1 UPDATE (user profile, only if changed)
- Total: 2-3 queries

**Performance Impact:**
- Negligible for typical use cases
- Uses efficient indexed queries
- Updates only when needed

### Caching

The system does NOT cache profile data because:
- Users expect fresh data after social platform changes
- Login frequency is relatively low
- Database queries are fast (indexed)

## Error Handling

### Non-Blocking Updates

Profile updates **never fail authentication**:

```go
if err := s.UserRepo.UpdateUser(user); err != nil {
    // Log error but don't fail authentication
    log.Printf("Failed to update user profile: %v", err)
}
// Authentication continues even if update fails
```

### Logged Errors

Errors are logged for monitoring:
- Database connection issues
- Update failures
- Data validation problems

**Users can still log in** even if sync fails.

## Security Considerations

### Data Validation

- No client-provided data - all from trusted providers
- Provider responses validated before storage
- Empty values don't overwrite existing data
- Malformed data logged but doesn't break flow

### Token Security

- New access token stored in social account
- Old tokens not retained
- Tokens never exposed via API responses

### Privacy

- Users cannot opt-out of sync (by design)
- All provider data stored (including raw response)
- Consider adding user preferences in future

## Configuration Options

### Current Behavior (No Configuration)

Profile sync is **always enabled** and cannot be disabled. This is intentional to ensure data consistency.

### Future Enhancement Ideas

1. **User Preferences:**
   ```json
   {
     "sync_profile_on_login": true,
     "sync_profile_picture": true,
     "sync_name": false  // User wants to keep custom name
   }
   ```

2. **Admin Settings:**
   ```env
   ENABLE_PROFILE_SYNC=true
   SYNC_INTERVAL_HOURS=24  # Only sync if last update > 24h ago
   ```

3. **Selective Sync:**
   - Sync only specific fields
   - Preserve user overrides
   - Merge strategy options

## Testing

### Manual Testing

1. **Change profile picture on Google:**
   ```bash
   # Go to Google account settings
   # Change profile picture
   # Login to app: http://localhost:8080/auth/google/login
   # Check profile: curl -H "Authorization: Bearer {token}" http://localhost:8080/profile
   # Verify picture URL updated
   ```

2. **Change name on Facebook:**
   ```bash
   # Update name on Facebook
   # Login to app: http://localhost:8080/auth/facebook/login
   # Query database:
   SELECT name, first_name, last_name, updated_at 
   FROM users WHERE email = 'your@email.com';
   # Verify name fields updated
   ```

3. **Change GitHub avatar:**
   ```bash
   # Update GitHub avatar
   # Login to app: http://localhost:8080/auth/github/login
   # Check social account:
   SELECT profile_picture, updated_at 
   FROM social_accounts 
   WHERE provider = 'github' AND user_id = 'user-uuid';
   ```

### Automated Testing

**Test scenarios to implement:**

```go
func TestGoogleProfileSync(t *testing.T) {
    // Create user with old profile data
    // Mock Google API with new profile data
    // Call HandleGoogleCallback
    // Assert social account updated
    // Assert user profile updated
}

func TestProfileSyncOnlyUpdatesChangedFields(t *testing.T) {
    // User has custom name
    // Google returns same name
    // Call HandleGoogleCallback
    // Assert user.updated_at NOT changed
}

func TestProfileSyncDoesNotFailAuth(t *testing.T) {
    // Mock database update failure
    // Call HandleGoogleCallback
    // Assert authentication successful
    // Assert tokens generated
}
```

### Database Verification Queries

```sql
-- Check last sync time
SELECT 
    u.email,
    u.name,
    u.profile_picture,
    u.updated_at as user_updated,
    sa.provider,
    sa.name as social_name,
    sa.profile_picture as social_picture,
    sa.updated_at as social_updated
FROM users u
JOIN social_accounts sa ON u.id = sa.user_id
WHERE u.email = 'test@gmail.com';

-- Verify sync happened
SELECT 
    provider,
    COUNT(*) as login_count,
    MAX(updated_at) as last_sync
FROM social_accounts
GROUP BY provider;
```

## Troubleshooting

### Issue: Profile Not Updating

**Symptoms:** User changes picture on Google, but it doesn't update in app

**Possible Causes:**
1. Application not restarted after code changes
2. Database update failing (check logs)
3. Provider API returning old data
4. Caching somewhere (browser, CDN)

**Debug Steps:**
```bash
# 1. Check application logs for errors
grep "Failed to update" application.log

# 2. Verify API is returning new data
curl "https://www.googleapis.com/oauth2/v2/userinfo?access_token=TOKEN"

# 3. Check database directly
psql -c "SELECT profile_picture, updated_at FROM users WHERE email='user@gmail.com';"

# 4. Force update manually
UPDATE users SET profile_picture = 'new-url' WHERE email = 'user@gmail.com';
```

### Issue: Some Fields Update, Others Don't

**Cause:** Smart update logic only updates changed non-empty fields

**Example:**
```go
// If provider returns empty name, user's name is preserved
if user.Name != googleUser.Name && googleUser.Name != "" {
    user.Name = googleUser.Name  // Only if new name not empty
}
```

**Solution:** This is by design. Empty provider values don't overwrite existing data.

### Issue: Performance Degradation

**Symptoms:** Social login becomes slow

**Debug:**
```sql
-- Check slow queries
SELECT query, mean_exec_time 
FROM pg_stat_statements 
WHERE query LIKE '%social_accounts%' 
ORDER BY mean_exec_time DESC;

-- Verify indexes exist
\d social_accounts
-- Should have indexes on: provider, provider_user_id, user_id
```

## Migration Notes

### Existing Users

For users who registered **before** profile sync was implemented:

1. **First login after update:** Full profile sync happens
2. **Data backfill:** All fields populated from provider
3. **No data loss:** Existing data preserved if provider returns empty

### Rollback

If you need to disable profile sync:

```go
// In internal/social/service.go, comment out update logic:
if err == nil { // Social account found, user exists
    // TODO: PROFILE SYNC DISABLED FOR ROLLBACK
    // // Update social account...
    // if err := s.SocialRepo.UpdateSocialAccount(socialAccount); err != nil {
    //     return "", "", uuid.UUID{}, errors.NewAppError(...)
    // }
    
    // Just authenticate without updates
    accessToken, err := jwt.GenerateAccessToken(socialAccount.UserID.String())
    // ... rest of authentication
}
```

## Related Documentation

- [Social Login Data Storage](SOCIAL_LOGIN_DATA_STORAGE.md)
- [Migration Guide](migrations/MIGRATION_SOCIAL_LOGIN_DATA.md)
- [Troubleshooting](TROUBLESHOOTING_SOCIAL_LOGIN.md)

## Changelog

**Version 1.1.0 - 2025-11-08**
- Added automatic profile sync on social login
- Updates both social account and user profile data
- Smart update strategy (only changed fields)
- Non-blocking error handling
- Support for Google, Facebook, and GitHub

---

## Summary

✅ **Profile data automatically syncs** on every social login  
✅ **Changes on social platforms** reflected immediately in app  
✅ **Smart updates** - only changed fields updated  
✅ **Non-breaking** - authentication succeeds even if update fails  
✅ **All providers** - Google, Facebook, GitHub supported  

