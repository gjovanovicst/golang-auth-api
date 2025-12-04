# Fix: Profile Endpoint Missing Fields

## Problem

Profile endpoint only returns:
```json
{
  "id": "...",
  "email": "gjovanovic.st@gmail.com",
  "email_verified": true,
  "two_fa_enabled": true,
  "created_at": "...",
  "updated_at": "..."
}
```

**Missing:** `name`, `first_name`, `last_name`, `profile_picture`, `locale`, `social_accounts`

## Root Cause

The new database columns haven't been created yet because:
- Code has been updated ✅
- Database migration **has NOT run** ❌

## Solution

### Step 1: Stop Current Application

```bash
# Press Ctrl+C in the terminal running the app
# OR find and kill the process
```

### Step 2: Rebuild Application

```bash
cd /c/work/AI/Cursor/auth_api/v1.0.0

# Rebuild with new code
go build -o auth_api.exe cmd/api/main.go
```

### Step 3: Start Application (Migration Runs Automatically)

```bash
./auth_api.exe
```

**Watch for these log messages:**
```
Database connected successfully!
Database migration completed!  ← This adds the new columns
Server starting on port 8080
```

### Step 4: Verify Database Schema

Connect to your PostgreSQL database and run:

```sql
-- Check if new columns exist
SELECT column_name, data_type 
FROM information_schema.columns
WHERE table_name = 'users'
  AND column_name IN ('name', 'first_name', 'last_name', 'profile_picture', 'locale')
ORDER BY column_name;

-- Should return 5 rows like this:
-- first_name     | character varying
-- last_name      | character varying
-- locale         | character varying
-- name           | character varying
-- profile_picture| character varying
```

### Step 5: Re-login via Social Provider

Now that columns exist, login again to populate them:

```
# Visit in browser
http://localhost:8080/auth/google/login

# Complete OAuth flow
# This will now populate the new fields
```

### Step 6: Check Profile Again

```bash
curl -H "Authorization: Bearer YOUR_NEW_TOKEN" \
     http://localhost:8080/profile
```

**Expected result:**
```json
{
  "id": "a65aec73-3c91-450c-b51f-a49391d6c3ba",
  "email": "gjovanovic.st@gmail.com",
  "email_verified": true,
  "name": "Your Name",                    ← NEW
  "first_name": "First",                  ← NEW
  "last_name": "Last",                    ← NEW
  "profile_picture": "https://...",       ← NEW
  "locale": "en",                         ← NEW
  "two_fa_enabled": true,
  "created_at": "2025-07-31T22:34:10Z",
  "updated_at": "2025-11-08T17:35:55Z",
  "social_accounts": [                    ← NEW
    {
      "id": "...",
      "provider": "google",
      "email": "gjovanovic.st@gmail.com",
      "name": "Your Name",
      "profile_picture": "https://...",
      ...
    }
  ]
}
```

## Why This Happens

### GORM AutoMigrate Process

```
Application Starts
    ↓
database.ConnectDatabase() - Connects to DB
    ↓
database.MigrateDatabase() - Reads Go models
    ↓
GORM AutoMigrate checks:
  - Do users table columns match User struct?
  - NO → ALTER TABLE users ADD COLUMN name VARCHAR
  - NO → ALTER TABLE users ADD COLUMN first_name VARCHAR
  - ... (adds all missing columns)
    ↓
"Database migration completed!" logged
    ↓
Server starts - New columns now available
```

**If you don't restart:** Old binary runs with old code, no migration happens.

## Verification Steps

### 1. Check Application is Using New Binary

```bash
# Check binary modification time
ls -lh auth_api.exe

# Should show recent timestamp (today)
```

### 2. Check Migration Ran

Look in application logs for:
```
2025/11/08 18:00:00 Database migration completed!
```

If you see this, migration ran successfully.

### 3. Check Database Directly

```sql
-- Users table should have new columns
\d users

-- Should show:
-- ...
-- name                | character varying           |
-- first_name          | character varying           |
-- last_name           | character varying           |
-- profile_picture     | character varying           |
-- locale              | character varying           |
-- ...
```

### 4. Check User Record

```sql
SELECT 
    email,
    name,
    first_name,
    last_name,
    profile_picture,
    locale
FROM users
WHERE email = 'gjovanovic.st@gmail.com';
```

**If columns don't exist:** You'll get error:
```
ERROR: column "name" does not exist
```
→ Migration hasn't run, restart application

**If columns exist but data is NULL:**
```
email                   | name | first_name | last_name | profile_picture | locale
gjovanovic.st@gmail.com | NULL | NULL       | NULL      | NULL            | NULL
```
→ Columns exist, but you need to login again to populate them

**If data exists:**
```
email                   | name          | first_name | last_name  | profile_picture      | locale
gjovanovic.st@gmail.com | Goran Jovanovic | Goran     | Jovanovic  | https://lh3.google... | en
```
→ Everything working! ✅

## Quick Fix Script

Run all these commands in sequence:

```bash
# 1. Navigate to project
cd /c/work/AI/Cursor/auth_api/v1.0.0

# 2. Stop old application (Ctrl+C if running)

# 3. Rebuild
go build -o auth_api.exe cmd/api/main.go

# 4. Start and watch logs
./auth_api.exe

# Wait for "Database migration completed!"

# 5. In new terminal, check database
# (Use your actual database credentials)
psql -U postgres -d auth_api_db -c "\d users"

# 6. Login via browser to populate data
# Visit: http://localhost:8080/auth/google/login

# 7. Get new token and check profile
# curl -H "Authorization: Bearer NEW_TOKEN" http://localhost:8080/profile
```

## Alternative: Manual Migration

If AutoMigrate isn't working, manually add columns:

```sql
-- Connect to your database
-- Run these commands:

ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR;
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name VARCHAR;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name VARCHAR;
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture VARCHAR;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locale VARCHAR;

ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS email VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS name VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS first_name VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS last_name VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS profile_picture VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS username VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS locale VARCHAR;
ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS raw_data JSONB;

-- Verify
\d users
\d social_accounts
```

Then restart the application and login again.

## Summary

**Why you're seeing limited data:**
- New columns don't exist in database yet
- JSON serialization only outputs existing fields
- Migration hasn't run

**Fix:**
1. ✅ Stop application
2. ✅ Rebuild: `go build -o auth_api.exe cmd/api/main.go`
3. ✅ Start: `./auth_api.exe`
4. ✅ Wait for "Database migration completed!"
5. ✅ Login via social provider again
6. ✅ Check profile - should have all fields now

**After fix, your profile will look like:**
```json
{
  "id": "a65aec73-3c91-450c-b51f-a49391d6c3ba",
  "email": "gjovanovic.st@gmail.com",
  "email_verified": true,
  "name": "Goran Jovanovic",           ← ✨
  "first_name": "Goran",               ← ✨
  "last_name": "Jovanovic",            ← ✨
  "profile_picture": "https://lh3...", ← ✨
  "locale": "en",                      ← ✨
  "two_fa_enabled": true,
  "created_at": "2025-07-31T22:34:10Z",
  "updated_at": "2025-11-08T17:35:55Z",
  "social_accounts": [...]             ← ✨
}
```

