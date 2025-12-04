# Troubleshooting: Social Login Data Not Inserted

## Issue
User `gjovanovic.st@gmail.com` logged in via social login but data was not inserted into the database.

## Possible Causes & Solutions

### 1. Application Not Restarted After Code Changes ⚠️

**Problem:** The new columns don't exist in the database yet because GORM AutoMigrate hasn't run.

**Solution:**
```bash
# Stop the running application (Ctrl+C or kill process)

# Rebuild the application
go build -o auth_api.exe cmd/api/main.go

# Start the application
./auth_api.exe
```

**What to look for in logs:**
```
Database connected successfully!
Database migration completed!
```

### 2. Migration Failed Silently

**Check Migration Status:**

Run this SQL query in your database to check if new columns exist:

```sql
-- Check users table structure
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'users'
ORDER BY ordinal_position;

-- You should see: name, first_name, last_name, profile_picture, locale
```

```sql
-- Check social_accounts table structure
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'social_accounts'
ORDER BY ordinal_position;

-- You should see: email, name, first_name, last_name, profile_picture, username, locale, raw_data
```

### 3. User Already Exists (Re-login Scenario)

**Problem:** If the user already logged in before the code changes, the system doesn't update existing records on re-authentication.

**Check if user exists:**
```sql
SELECT id, email, name, first_name, last_name, profile_picture, locale, email_verified, created_at
FROM users
WHERE email = 'gjovanovic.st@gmail.com';
```

**Check social accounts:**
```sql
SELECT sa.id, sa.provider, sa.email, sa.name, sa.first_name, sa.last_name, 
       sa.profile_picture, sa.username, sa.locale, sa.created_at, sa.updated_at
FROM social_accounts sa
JOIN users u ON sa.user_id = u.id
WHERE u.email = 'gjovanovic.st@gmail.com';
```

**If user exists but data is NULL:**
The social login flow for existing users doesn't update data on re-authentication. You need to either:

**Option A: Delete and re-register** (for testing)
```sql
-- Delete social accounts first (foreign key)
DELETE FROM social_accounts WHERE user_id IN (
    SELECT id FROM users WHERE email = 'gjovanovic.st@gmail.com'
);

-- Delete user
DELETE FROM users WHERE email = 'gjovanovic.st@gmail.com';

-- Now try logging in again via social provider
```

**Option B: Manually update the record** (to test the flow worked)
```sql
-- Update user with sample data
UPDATE users 
SET name = 'Goran Jovanovic',
    first_name = 'Goran',
    last_name = 'Jovanovic',
    profile_picture = 'https://example.com/pic.jpg',
    locale = 'en'
WHERE email = 'gjovanovic.st@gmail.com';
```

### 4. Code Compilation Issue

**Verify code is up-to-date:**
```bash
# Check if build was successful
go build -o auth_api.exe cmd/api/main.go
echo $?  # Should output: 0

# Check if the new binary exists and is recent
ls -lh auth_api.exe

# Verify the binary has today's date
```

### 5. Database Connection Issue

**Check logs for errors:**
Look for any errors in the application output when the social login happens.

**Enable verbose logging:**
The database logger is already set to `logger.Info` level, so you should see SQL queries in the logs.

**What to look for:**
```
INSERT INTO "users" (..., "name", "first_name", "last_name", "profile_picture", "locale", ...)
```

### 6. Social Provider API Issue

**Check if data is being fetched from provider:**

Add temporary logging to verify data is coming from Google:

You can check the application logs for output like:
```
[GIN] 2025/11/08 - 15:30:00 | 200 | GET /auth/google/callback
```

## Quick Diagnostic Checklist

- [ ] **Application restarted** after code changes
- [ ] **Migration ran successfully** (check logs for "Database migration completed!")
- [ ] **New columns exist** in database (run SQL queries above)
- [ ] **User record exists** (query users table)
- [ ] **Social account record exists** (query social_accounts table)
- [ ] **Application binary is recent** (check file date)
- [ ] **No errors in application logs**

## Step-by-Step Verification

### Step 1: Restart Application
```bash
# Stop current instance
# Ctrl+C or kill the process

# Rebuild
go build -o auth_api.exe cmd/api/main.go

# Run
./auth_api.exe

# Watch for:
# "Database connected successfully!"
# "Database migration completed!"
# "Server starting on port 8080"
```

### Step 2: Verify Database Schema

Connect to your database and run:
```sql
-- This should show the new columns
\d users

-- Or use this query
SELECT column_name 
FROM information_schema.columns 
WHERE table_name = 'users' 
  AND column_name IN ('name', 'first_name', 'last_name', 'profile_picture', 'locale');

-- Should return 5 rows if migration succeeded
```

### Step 3: Clean Test (Recommended)

For a clean test, delete the test user and try again:

```sql
-- Backup first (optional)
CREATE TABLE users_backup AS SELECT * FROM users WHERE email = 'gjovanovic.st@gmail.com';
CREATE TABLE social_accounts_backup AS 
    SELECT sa.* FROM social_accounts sa
    JOIN users u ON sa.user_id = u.id
    WHERE u.email = 'gjovanovic.st@gmail.com';

-- Delete for clean test
DELETE FROM social_accounts WHERE user_id IN (
    SELECT id FROM users WHERE email = 'gjovanovic.st@gmail.com'
);
DELETE FROM users WHERE email = 'gjovanovic.st@gmail.com';
```

### Step 4: Test Social Login Again

1. Open browser: `http://localhost:8080/auth/google/login`
2. Complete OAuth flow
3. You should get redirected with access token
4. Check database immediately:

```sql
SELECT id, email, name, first_name, last_name, profile_picture, locale, created_at
FROM users
WHERE email = 'gjovanovic.st@gmail.com'
ORDER BY created_at DESC
LIMIT 1;

SELECT provider, email, name, first_name, last_name, profile_picture, username, locale
FROM social_accounts
WHERE user_id = (SELECT id FROM users WHERE email = 'gjovanovic.st@gmail.com');
```

### Step 5: Check Application Logs

Look for SQL INSERT statements in the logs:
```sql
INSERT INTO "users" 
("id", "email", "password_hash", "email_verified", 
 "name", "first_name", "last_name", "profile_picture", "locale",  -- NEW FIELDS
 "two_fa_enabled", "two_fa_secret", "two_fa_recovery_codes", 
 "created_at", "updated_at") 
VALUES (...)
```

## Common Issues

### Issue: "Column does not exist"

**Error in logs:**
```
ERROR: column "name" of relation "users" does not exist
```

**Cause:** Migration didn't run or failed.

**Solution:**
1. Check if AutoMigrate is being called in `main.go`
2. Restart the application to trigger migration
3. Check database logs for permission issues

### Issue: Data is NULL in database

**Symptoms:** User and social account created, but new fields are NULL.

**Cause:** Application is running old code (before changes).

**Solution:**
1. Rebuild: `go build -o auth_api.exe cmd/api/main.go`
2. Stop old instance
3. Start new instance
4. Delete test user and try again

### Issue: User already exists, data not updated

**Cause:** The social login logic for **existing users who re-authenticate** doesn't update data (by design).

**Where it happens:**
```go
// In HandleGoogleCallback
socialAccount, err := s.SocialRepo.GetSocialAccountByProviderAndUserID("google", googleUser.ID)
if err == nil { // Social account found, user exists
    // Authenticate existing user - NO DATA UPDATE
    return accessToken, refreshToken, socialAccount.UserID, nil
}
```

**Solution:** This is intentional. For testing with an existing user:
1. Delete the user from database
2. Log in again fresh

## Manual Migration (If AutoMigrate Fails)

If GORM AutoMigrate is not working, you can manually add the columns:

```sql
-- Add columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR;
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name VARCHAR;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name VARCHAR;
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture VARCHAR;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locale VARCHAR;

-- Add columns to social_accounts table
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS email VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS name VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS first_name VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS last_name VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS profile_picture VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS username VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS locale VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS raw_data JSONB;

-- Verify columns were added
\d users
\d social_accounts
```

## Testing Commands

```bash
# 1. Check if application is running
curl http://localhost:8080/swagger/index.html

# 2. Initiate Google login (in browser)
# Visit: http://localhost:8080/auth/google/login

# 3. After successful login, get profile with the token
curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
     http://localhost:8080/profile

# 4. Expected response should include:
{
  "email": "gjovanovic.st@gmail.com",
  "name": "Your Name",
  "first_name": "First",
  "last_name": "Last",
  "profile_picture": "https://...",
  "locale": "en",
  ...
}
```

## Need More Help?

If the issue persists, gather this information:

1. **Application logs** (from startup to social login)
2. **Database schema** (output of `\d users` and `\d social_accounts`)
3. **User record** (SELECT * FROM users WHERE email = 'gjovanovic.st@gmail.com')
4. **Social account record** (SELECT * FROM social_accounts WHERE user_id = ...)
5. **Application version** (check binary date: `ls -lh auth_api.exe`)

## Most Likely Solution

**90% of the time, this is the issue:**

1. Code was updated ✓
2. Application was **NOT restarted** ✗
3. Old code is still running
4. New columns don't exist in database yet

**Quick Fix:**
```bash
# Stop application (Ctrl+C)
go build -o auth_api.exe cmd/api/main.go
./auth_api.exe
# Wait for "Database migration completed!"
# Delete test user from database
# Try social login again
```

