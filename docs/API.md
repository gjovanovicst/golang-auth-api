# API Documentation

## Authentication Endpoints

### Register
- `POST /register`
- Request: `{ "email": "user@example.com", "password": "..." }`
- Response: `{ "success": true, "data": { "token": "..." } }`

### Login
- `POST /login`
- Request: `{ "email": "user@example.com", "password": "..." }`
- Response: `{ "success": true, "data": { "token": "..." } }`

### Logout
- `POST /logout`
- Header: `Authorization: Bearer <access_token>`
- Request: `{ "refresh_token": "..." }`
- Response: `{ "message": "Successfully logged out" }`

### Refresh Token
- `POST /refresh-token`
- Request: `{ "refresh_token": "..." }`
- Response: `{ "success": true, "data": { "token": "..." } }`

### Forgot Password
- `POST /forgot-password`
- Request: `{ "email": "user@example.com" }`
- Response: `{ "success": true }`

### Reset Password
- `POST /reset-password`
- Request: `{ "token": "...", "new_password": "..." }`
- Response: `{ "success": true }`

### Email Verification
- `GET /verify-email?token=...`
- Response: `{ "success": true }`

### Social Login
- `GET /auth/{provider}/login`
- `GET /auth/{provider}/callback`

### Protected Profile
- `GET /profile`
- Header: `Authorization: Bearer <token>`
- Response: `{ "success": true, "data": { ...user... } }`

---
For more details, see the OpenAPI spec (if available) or code comments.
