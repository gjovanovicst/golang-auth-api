# QUICK FIX - Profile Missing Fields

## Problem
Profile only shows: `id`, `email`, `email_verified`, `two_fa_enabled`, `created_at`, `updated_at`

Missing: `name`, `first_name`, `last_name`, `profile_picture`, `locale`, `social_accounts`

## Why
**Database columns don't exist yet.** Migration needs to run.

## Fix (3 Steps)

### 1. Stop & Rebuild
```bash
# Stop current app (Ctrl+C)
cd /c/work/AI/Cursor/auth_api/v1.0.0
go build -o auth_api.exe cmd/api/main.go
```

### 2. Start & Watch
```bash
./auth_api.exe

# WAIT FOR THIS MESSAGE:
# "Database migration completed!"
```

### 3. Re-login
```
Visit: http://localhost:8080/auth/google/login
Complete OAuth flow
Get new token
```

### 4. Check Profile
```bash
curl -H "Authorization: Bearer YOUR_NEW_TOKEN" \
     http://localhost:8080/profile
```

**Should now show all fields!** ✅

---

## Verify Migration Ran

Check database has new columns:
```sql
\d users

-- Look for these columns:
-- name
-- first_name  
-- last_name
-- profile_picture
-- locale
```

If columns missing, see [FIX_MISSING_FIELDS.md](FIX_MISSING_FIELDS.md)

---

## One-Liner Diagnosis

```bash
# Check if columns exist
psql -U <user> -d <database> -c "SELECT column_name FROM information_schema.columns WHERE table_name='users' AND column_name IN ('name','first_name','last_name','profile_picture','locale');"

# Should return 5 rows
# If returns 0 rows → restart application to run migration
```

---

**TL;DR:** Restart app → Migration adds columns → Login again → All fields appear ✅

