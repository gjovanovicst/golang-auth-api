# Session Group Expiry Detection

## Overview

This feature extends the existing session group functionality to automatically revoke a user's sessions in all applications within the same session group when a session expires (due to refresh token TTL). Previously, only explicit logout triggered group-wide revocation.

## Architecture

### Key Components

1. **Session Metadata Keys** (`session_meta:{appID}:{userID}:{sessionID}`)
   - Created alongside each session in Redis with the same TTL
   - Used to detect which app/user/session expired
   - Deleted when session is manually revoked

2. **Session Group Revoker** (`internal/sessiongroup/revoke.go`)
   - Shared utility for group-wide session revocation
   - Used by both logout flow and expiry detection
   - Implements `ExpiryHandlerInterface`

3. **Expiry Detection Service** (`internal/sessiongroup/expiry.go`)
   - Real-time detection via Redis keyspace notifications
   - Fallback periodic scanning (5-minute interval)
   - Configurable via environment variables

## Configuration

### Environment Variables

```bash
# Enable Redis keyspace notifications for real-time session expiry detection
# Set to "Ex" for expired key events (recommended)
REDIS_NOTIFY_KEYSPACE_EVENTS=Ex

# Enable session group expiry-triggered revocation (default: true)
SESSION_GROUP_EXPIRY_REVOCATION_ENABLED=true

# Fallback scan interval for expired sessions when keyspace notifications are disabled
# Format: 5m, 10m, 1h, etc.
SESSION_GROUP_EXPIRY_SCAN_INTERVAL=5m

# Enable keyspace notification listener (default: true if REDIS_NOTIFY_KEYSPACE_EVENTS is set)
SESSION_GROUP_KEYSYSPACE_NOTIF_ENABLED=true
```

### Docker Compose Configuration

The Redis service in `docker-compose.yml` is configured with:
```yaml
command: redis-server --notify-keyspace-events Ex
```

## How It Works

### Session Creation Flow
1. User logs into App A (part of Session Group X with `GlobalLogout=true`)
2. `redis.CreateSession()` stores:
   - Session hash: `app:{appA}:session:{sessionID}`
   - Session metadata: `session_meta:{appA}:{userID}:{sessionID}` (same TTL)

### Session Expiration Flow
1. Redis session TTL expires (refresh token lifetime)
2. Redis emits keyspace notification: `__keyevent@0__:expired` → `session_meta:{appA}:{userID}:{sessionID}`
3. `ExpiryService.listenForKeyExpirations()` receives notification
4. Service extracts `appA`, `userID` from key
5. Checks if App A belongs to a session group with `GlobalLogout=true`
6. If yes, revokes user's sessions in all other apps in the same group

### Fallback Scanning
If keyspace notifications are disabled:
1. `ExpiryService.periodicScanner()` runs every 5 minutes (configurable)
2. Scans for `session_meta:*` keys with TTL ≤ 0
3. Processes each expired key same as real-time flow

## Integration Points

### Main Application Startup (`cmd/api/main.go`)
```go
// Create session group revoker for shared logout/expiry logic
sessionGroupRevoker := sessiongroup.NewRevoker(adminRepo, userRepo, sessionService)

// Wire into user service logout
userService.GroupLogoutFunc = func(appID, userEmail string) {
    sessionGroupRevoker.RevokeAllUserSessionsInGroup(appID, userEmail)
}

// Wire into OIDC handler logout  
oidcHandler.GroupLogoutFunc = func(appID, userEmail string) {
    sessionGroupRevoker.RevokeAllUserSessionsInGroup(appID, userEmail)
}

// Start expiry detection service
expiryService := sessiongroup.NewExpiryService(sessionGroupRevoker)
expiryService.Start()
defer expiryService.Stop()
```

### Redis Session Management (`internal/redis/redis.go`)
```go
// CreateSession stores session metadata alongside session hash
metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sessionID)
if err := Rdb.Set(ctx, metaKey, "1", ttl).Err(); err != nil {
    log.Printf("Warning: Failed to create session metadata key: %v", err)
}

// DeleteSession removes metadata key when session is manually revoked
metaKey := fmt.Sprintf("session_meta:%s:%s:%s", appID, userID, sessionID)
Rdb.Del(ctx, metaKey)
```

## Testing Scenarios

### Scenario 1: Real-time Expiry Detection
1. User has active sessions in App A, App B, App C (same session group)
2. App A session expires (Redis TTL reaches 0)
3. Within milliseconds: User sessions in App B and App C are revoked
4. User must re-authenticate in all apps

### Scenario 2: Manual Logout
1. User logs out of App A
2. Existing `GroupLogoutFunc` triggers group-wide revocation
3. Sessions in App B and App C are revoked
4. Same behavior as before (backward compatible)

### Scenario 3: No Session Group
1. App is not in any session group
2. Session expiry only affects that app
3. Other app sessions remain active

### Scenario 4: GlobalLogout Disabled
1. Session group exists but `GlobalLogout=false`
2. Session expiry only affects the expiring app
3. Other app sessions in group remain active

## Monitoring and Logging

The service logs key events:
```
[SessionGroup] Started keyspace notification listener for session expiry
[SessionGroup] Expiry detection service started (scan interval: 5m0s)
[SessionGroup] Session expired: app={appID}, user={userID}, session={sessionID}
[SessionGroup] Revoked sessions for user {email} in app {otherAppID} (session group: {groupName})
```

## Performance Considerations

1. **Database Queries**: Each expiry triggers DB queries to:
   - Get session group for app
   - Get user by ID (for email)
   - Get user by email in other apps
   
   Consider caching group-to-apps mapping if performance becomes an issue.

2. **Redis Subscription**: One persistent connection for keyspace notifications.

3. **Periodic Scanning**: SCAN operations every 5 minutes (configurable).

4. **Concurrent Processing**: Expiry events are processed sequentially to avoid race conditions.

## Security Implications

1. **Immediate Revocation**: Sessions are revoked immediately upon expiry, enhancing security.

2. **No Grace Period**: If a refresh happens before expiry, TTL is reset. No false positives.

3. **Admin Override**: Admin revocation remains unconditional (ignores `GlobalLogout` flag).

4. **Defense in Depth**: Works alongside existing token blacklisting and session validation.

## Migration Notes

1. **Backward Compatible**: Existing logout behavior unchanged.
2. **Opt-in via Configuration**: Disable with `SESSION_GROUP_EXPIRY_REVOCATION_ENABLED=false`.
3. **Redis Configuration Required**: For real-time detection, Redis must be configured with `notify-keyspace-events Ex`.
4. **No Data Migration Required**: Works with existing session groups and `GlobalLogout` flag.