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

The following event types are logged in the activity log system:

- `LOGIN` - User login
- `LOGOUT` - User logout
- `REGISTER` - User registration
- `PASSWORD_CHANGE` - Password changed via profile
- `PASSWORD_RESET` - Password reset via forgot password flow
- `EMAIL_VERIFY` - Email verification completed
- `EMAIL_CHANGE` - Email address changed
- `2FA_ENABLE` - Two-factor authentication enabled
- `2FA_DISABLE` - Two-factor authentication disabled
- `2FA_LOGIN` - Login with 2FA verification
- `TOKEN_REFRESH` - Access token refreshed
- `SOCIAL_LOGIN` - Social authentication login
- `PROFILE_ACCESS` - Profile accessed (GET /profile)
- `PROFILE_UPDATE` - Profile updated
- `ACCOUNT_DELETION` - Account deleted
- `RECOVERY_CODE_USED` - 2FA recovery code used
- `RECOVERY_CODE_GENERATE` - New recovery codes generated

---

For detailed API specifications including request/response schemas and authentication requirements, see the [Swagger documentation](http://localhost:8080/swagger/index.html) when the API is running.
