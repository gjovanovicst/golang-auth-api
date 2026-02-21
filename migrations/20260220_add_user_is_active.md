# Add is_active Column to Users Table

## Changes
- Add `is_active` BOOLEAN column to `users` table (default: TRUE)
- All existing users remain active after migration
- Used by Admin GUI to deactivate/reactivate user accounts
- Login and social login flows check this field before authenticating

## Rollback
- Drop `is_active` column from `users` table
