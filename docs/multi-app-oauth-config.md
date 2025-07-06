# Multi-Application OAuth Configuration

## Overview
The authentication API now supports multiple applications using the same OAuth endpoints with different redirect URIs. This allows you to have multiple frontends (e.g., admin panel, user portal, mobile app) that all use the same authentication backend.

## Configuration

### Environment Variables

Add these variables to your `.env` file:

```env
# Multi-Application Support
# Comma-separated list of allowed redirect domains for OAuth callbacks
ALLOWED_REDIRECT_DOMAINS=localhost:3000,localhost:5173,localhost:8080,example.com,.example.com

# Default redirect URI when none is specified
DEFAULT_REDIRECT_URI=http://localhost:5173/auth/callback

# Production frontend URL for CORS
FRONTEND_URL=https://your-production-domain.com
```

### Domain Configuration

#### Development
For local development, include all your local ports:
```env
ALLOWED_REDIRECT_DOMAINS=localhost:3000,localhost:5173,localhost:8080,127.0.0.1:3000,127.0.0.1:5173
```

#### Production
For production, specify your allowed domains:
```env
# Allow specific domains
ALLOWED_REDIRECT_DOMAINS=myapp.com,admin.myapp.com

# Allow all subdomains with dot notation
ALLOWED_REDIRECT_DOMAINS=.myapp.com,anotherapp.com
```

## Usage

### Frontend Integration

#### 1. Basic Usage (Default Redirect)
```javascript
// Will redirect to DEFAULT_REDIRECT_URI after OAuth
window.location.href = '/auth/google/login';
```

#### 2. Custom Redirect URI
```javascript
// Will redirect to specified URI after OAuth
const redirectUri = 'https://admin.myapp.com/auth/callback';
window.location.href = `/auth/google/login?redirect_uri=${encodeURIComponent(redirectUri)}`;
```

#### 3. Handling Callbacks
Your frontend should handle the callback URL parameters:

```javascript
// In your /auth/callback route
const urlParams = new URLSearchParams(window.location.search);

if (urlParams.get('error')) {
  // Handle error
  console.error('OAuth error:', urlParams.get('error'));
} else if (urlParams.get('access_token')) {
  // Handle success
  const accessToken = urlParams.get('access_token');
  const refreshToken = urlParams.get('refresh_token');
  const provider = urlParams.get('provider'); // 'google', 'facebook', or 'github'
  
  // Store tokens and redirect to app
  localStorage.setItem('access_token', accessToken);
  localStorage.setItem('refresh_token', refreshToken);
  window.location.href = '/dashboard';
}
```

### API Endpoints

All OAuth providers support the same pattern:

- **Google**: 
  - Login: `GET /auth/google/login?redirect_uri=...`
  - Callback: `GET /auth/google/callback`

- **Facebook**: 
  - Login: `GET /auth/facebook/login?redirect_uri=...`
  - Callback: `GET /auth/facebook/callback`

- **GitHub**: 
  - Login: `GET /auth/github/login?redirect_uri=...`
  - Callback: `GET /auth/github/callback`

## Security Features

### 1. Domain Whitelist
- Only domains in `ALLOWED_REDIRECT_DOMAINS` are accepted
- Prevents open redirect vulnerabilities
- Supports subdomain wildcards with dot notation

### 2. Secure State Management
- OAuth state parameter contains encrypted redirect URI
- Includes timestamp to prevent replay attacks
- Cryptographically secure random nonce generation

### 3. CORS Configuration
- Automatic CORS headers for allowed domains
- Supports credentials for secure cookie handling
- Production-ready with configurable origins

## Examples

### Multi-Domain Setup
```env
# Support multiple applications
ALLOWED_REDIRECT_DOMAINS=app.mycompany.com,admin.mycompany.com,mobile.mycompany.com
DEFAULT_REDIRECT_URI=https://app.mycompany.com/auth/callback
```

### Development Setup
```env
# Support local development
ALLOWED_REDIRECT_DOMAINS=localhost:3000,localhost:5173,localhost:8080
DEFAULT_REDIRECT_URI=http://localhost:5173/auth/callback
```

### Production with Subdomains
```env
# Allow all subdomains of mycompany.com
ALLOWED_REDIRECT_DOMAINS=.mycompany.com,mycompany.com
DEFAULT_REDIRECT_URI=https://mycompany.com/auth/callback
```

## Error Handling

The API will redirect to the frontend with error parameters:

- `?error=invalid_state` - Invalid or expired OAuth state
- `?error=authorization_code_missing` - OAuth provider didn't return code
- `?error=missing_state` - OAuth state parameter missing
- `?error=Invalid%20redirect%20URI` - Redirect URI not in whitelist

## Migration Guide

### From Single Application
If you were using the hardcoded redirect URI:

1. Add the environment variables to your `.env` file
2. Your existing setup will continue to work with default values
3. Optionally, start using the `redirect_uri` parameter for flexibility

### For New Applications
1. Set up the environment variables
2. Add your domain to `ALLOWED_REDIRECT_DOMAINS`
3. Use the `redirect_uri` parameter in your OAuth login URLs 