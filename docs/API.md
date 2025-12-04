# API Documentation

## Authentication Endpoints

### Register
- `POST /register`
- Request: `{ "email": "user@example.com", "password": "..." }`
- Response: `{ "message": "User registered successfully. Please check your email for verification." }`

### Login
- `POST /login`
- Request: `{ "email": "user@example.com", "password": "..." }`
- Response: `{ "access_token": "...", "refresh_token": "..." }`
- If 2FA enabled: `{ "message": "2FA verification required", "temp_token": "..." }`

### Logout
- `POST /logout`
- Header: `Authorization: Bearer <access_token>`
- Request: `{ "refresh_token": "...", "access_token": "..." }`
- Response: `{ "message": "Successfully logged out" }`

### Refresh Token
- `POST /refresh-token`
- Request: `{ "refresh_token": "..." }`
- Response: `{ "access_token": "...", "refresh_token": "..." }`

### Forgot Password
- `POST /forgot-password`
- Request: `{ "email": "user@example.com" }`
- Response: `{ "message": "If an account with that email exists, a password reset link has been sent." }`

### Reset Password
- `POST /reset-password`
- Request: `{ "token": "...", "new_password": "..." }`
- Response: `{ "message": "Password has been reset successfully." }`

### Email Verification
- `GET /verify-email?token=...`
- Response: `{ "message": "Email verified successfully!" }`

### Token Validation (for external services)
- `GET /auth/validate`
- Header: `Authorization: Bearer <token>`
- Response: `{ "valid": true, "userID": "uuid", "email": "user@example.com" }`

## Social Authentication Endpoints

### Google OAuth2
- `GET /auth/google/login` - Initiate Google login
- `GET /auth/google/callback` - Google callback handler

### Facebook OAuth2
- `GET /auth/facebook/login` - Initiate Facebook login
- `GET /auth/facebook/callback` - Facebook callback handler

### GitHub OAuth2
- `GET /auth/github/login` - Initiate GitHub login
- `GET /auth/github/callback` - GitHub callback handler

## User Profile Endpoints (Protected)

All profile endpoints require JWT authentication via `Authorization: Bearer <token>` header.

### Get Profile
- `GET /profile`
- Response: User profile with social accounts
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "email_verified": true,
  "name": "John Doe",
  "first_name": "John",
  "last_name": "Doe",
  "profile_picture": "https://...",
  "locale": "en-US",
  "two_fa_enabled": false,
  "created_at": "2023-01-01T00:00:00Z",
  "updated_at": "2023-01-01T00:00:00Z",
  "social_accounts": [...]
}
```

### Update Profile
- `PUT /profile`
- Request (all fields optional):
```json
{
  "name": "John Doe",
  "first_name": "John",
  "last_name": "Doe",
  "profile_picture": "https://example.com/avatar.jpg",
  "locale": "en-US"
}
```
- Response: Updated user profile (same format as GET /profile)

### Update Email
- `PUT /profile/email`
- Request:
```json
{
  "email": "newemail@example.com",
  "password": "currentpassword"
}
```
- Response: `{ "message": "Email updated successfully. Please check your new email for verification." }`
- Note: Email verification required for new email address

### Update Password
- `PUT /profile/password`
- Request:
```json
{
  "current_password": "oldpassword123",
  "new_password": "newpassword123"
}
```
- Response: `{ "message": "Password updated successfully. All sessions have been logged out for security." }`
- Note: All existing tokens will be revoked for security

### Delete Account
- `DELETE /profile`
- Request:
```json
{
  "password": "currentpassword",
  "confirm_deletion": true
}
```
- Response: `{ "message": "Account deleted successfully. We're sorry to see you go." }`
- Note: This action is permanent and cannot be undone

## Two-Factor Authentication Endpoints (Protected)

### Generate 2FA Setup
- `POST /2fa/generate`
- Response: QR code and secret for TOTP setup

### Verify 2FA Setup
- `POST /2fa/verify-setup`
- Request: `{ "code": "123456" }`
- Response: Verification status

### Enable 2FA
- `POST /2fa/enable`
- Request: `{ "code": "123456" }`
- Response: Recovery codes

### Disable 2FA
- `POST /2fa/disable`
- Request: `{ "code": "123456" }`
- Response: `{ "message": "2FA disabled successfully" }`

### Generate New Recovery Codes
- `POST /2fa/recovery-codes`
- Request: `{ "code": "123456" }`
- Response: New recovery codes

## Activity Log Endpoints (Protected)

### Get User Activity Logs
- `GET /activity-logs`
- Query parameters: `page`, `limit`, `event_type`, `start_date`, `end_date`
- Response: Paginated list of user's activity logs

### Get Activity Log by ID
- `GET /activity-logs/:id`
- Response: Single activity log entry

### Get Available Event Types
- `GET /activity-logs/event-types`
- Response: List of available event types for filtering

## Admin Endpoints (Protected)

### Get All Activity Logs (Admin only)
- `GET /admin/activity-logs`
- Query parameters: `page`, `limit`, `user_id`, `event_type`, `start_date`, `end_date`
- Response: Paginated list of all users' activity logs

---

## Event Types

The activity log system uses a tiered approach with event categorization by severity and configurable logging.

### Event Severity Levels

#### Critical Events (365-day retention, always logged)
- `LOGIN` - User login
- `LOGOUT` - User logout
- `REGISTER` - User registration
- `PASSWORD_CHANGE` - Password changed via profile
- `PASSWORD_RESET` - Password reset via forgot password flow
- `EMAIL_CHANGE` - Email address changed
- `2FA_ENABLE` - Two-factor authentication enabled
- `2FA_DISABLE` - Two-factor authentication disabled
- `ACCOUNT_DELETION` - Account deleted
- `RECOVERY_CODE_USED` - 2FA recovery code used

#### Important Events (180-day retention, always logged)
- `EMAIL_VERIFY` - Email verification completed
- `2FA_LOGIN` - Login with 2FA verification
- `SOCIAL_LOGIN` - Social authentication login
- `PROFILE_UPDATE` - Profile updated
- `RECOVERY_CODE_GENERATE` - New recovery codes generated

#### Informational Events (90-day retention, conditional logging)
- `TOKEN_REFRESH` - Access token refreshed (disabled by default, only logs anomalies)
- `PROFILE_ACCESS` - Profile accessed (disabled by default, only logs anomalies)

### Smart Logging Features

**Anomaly Detection**: Informational events are only logged when unusual patterns are detected:
- New IP address
- New device/browser (user agent)
- Unusual access times

**Automatic Cleanup**: Logs are automatically deleted after their retention period expires.

**Configuration**: Event logging behavior can be customized via environment variables (see Configuration section).

---

## Activity Log Configuration

Control activity logging behavior with these environment variables:

```bash
# Cleanup Service
LOG_CLEANUP_ENABLED=true              # Enable automatic log cleanup (default: true)
LOG_CLEANUP_INTERVAL=24h              # Cleanup frequency (default: 24h)
LOG_CLEANUP_BATCH_SIZE=1000           # Cleanup batch size (default: 1000)

# Event Control
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS  # Comma-separated list
LOG_TOKEN_REFRESH=false               # Enable TOKEN_REFRESH logging (default: false)
LOG_PROFILE_ACCESS=false              # Enable PROFILE_ACCESS logging (default: false)

# Sampling (when enabled)
LOG_SAMPLE_TOKEN_REFRESH=0.01         # Log 1% of token refreshes (default: 0.01)
LOG_SAMPLE_PROFILE_ACCESS=0.01        # Log 1% of profile accesses (default: 0.01)

# Anomaly Detection
LOG_ANOMALY_DETECTION_ENABLED=true    # Enable anomaly detection (default: true)
LOG_ANOMALY_NEW_IP=true               # Log on new IP address (default: true)
LOG_ANOMALY_NEW_USER_AGENT=true       # Log on new user agent (default: true)
LOG_ANOMALY_SESSION_WINDOW=720h       # Pattern analysis window - 30 days (default: 720h)

# Retention Policies (in days)
LOG_RETENTION_CRITICAL=365            # Critical events retention (default: 365)
LOG_RETENTION_IMPORTANT=180           # Important events retention (default: 180)
LOG_RETENTION_INFORMATIONAL=90        # Informational events retention (default: 90)
```

---

For detailed API specifications including request/response schemas and authentication requirements, see the [Swagger documentation](http://localhost:8080/swagger/index.html) when the API is running.
