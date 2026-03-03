# Plan: Admin GUI Session Management Dashboard

## Overview

Implement a full session monitoring dashboard in the admin GUI, including:
- A top-level "Sessions" page showing all active sessions across all users/apps
- Session stats on the main dashboard
- Per-user session section in the user detail view
- Revoke individual sessions, all sessions for a user, or all sessions for an app

## Architecture Decision: Per-App Redis SET Index

Add `app:{appID}:all_sessions` Redis SETs to enable efficient enumeration of all sessions. This is consistent with the existing `app:{appID}:user_sessions:{userID}` per-user index pattern.

---

## Phase 1: Redis — Add per-app session index

**File:** `internal/redis/redis.go`

### 1a. Modify `CreateSession()` (line 225-232)

After the user session index `SADD`, add:

```go
// Add to app-level session index (for admin dashboard enumeration)
appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
Rdb.SAdd(ctx, appIndexKey, sessionID)
Rdb.Expire(ctx, appIndexKey, ttl+24*time.Hour)
```

### 1b. Modify `DeleteSession()` (line 272-274)

After the user session index `SREM`, add:

```go
// Remove from app-level session index
appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
Rdb.SRem(ctx, appIndexKey, sessionID)
```

### 1c. Modify `DeleteAllUserSessions()` (line 314-320)

Inside the loop where sessions are deleted, also remove from app index:

```go
for _, sid := range sessionIDs {
    if sid == exceptSessionID {
        continue
    }
    sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sid)
    Rdb.Del(ctx, sessionKey)
    // Also remove from app-level session index
    appIndexKey := fmt.Sprintf("app:%s:all_sessions", appID)
    Rdb.SRem(ctx, appIndexKey, sid)
}
```

### 1d. Add new functions (after `SessionExists`, ~line 343)

```go
// GetAppSessionIDs returns all session IDs for an app from the app-level session index.
// Performs lazy cleanup: removes IDs whose session hash has expired.
func GetAppSessionIDs(appID string) ([]string, error) {
    indexKey := fmt.Sprintf("app:%s:all_sessions", appID)
    sessionIDs, err := Rdb.SMembers(ctx, indexKey).Result()
    if err != nil {
        return nil, err
    }

    var validIDs []string
    for _, sid := range sessionIDs {
        sessionKey := fmt.Sprintf("app:%s:session:%s", appID, sid)
        exists, err := Rdb.Exists(ctx, sessionKey).Result()
        if err != nil {
            continue
        }
        if exists == 0 {
            Rdb.SRem(ctx, indexKey, sid)
            continue
        }
        validIDs = append(validIDs, sid)
    }
    return validIDs, nil
}

// CountAppSessions returns the count of entries in the app-level session index.
// Note: may include stale entries until lazy cleanup runs via GetAppSessionIDs.
func CountAppSessions(appID string) (int64, error) {
    indexKey := fmt.Sprintf("app:%s:all_sessions", appID)
    return Rdb.SCard(ctx, indexKey).Result()
}

// GetAllSessionsForApp returns full session metadata for all active sessions in an app.
// Each returned map contains: session_id, user_id, refresh_token, ip, user_agent, created_at, last_active.
func GetAllSessionsForApp(appID string) ([]map[string]string, error) {
    sessionIDs, err := GetAppSessionIDs(appID)
    if err != nil {
        return nil, err
    }

    var sessions []map[string]string
    for _, sid := range sessionIDs {
        data, err := GetSession(appID, sid)
        if err != nil {
            continue
        }
        data["session_id"] = sid
        sessions = append(sessions, data)
    }
    return sessions, nil
}
```

---

## Phase 2: Dashboard stats — Add active session count

### 2a. Modify `internal/admin/dashboard_service.go`

Add `ActiveSessions int64` field to `DashboardStats` struct.

In `GetStats()`, query all app IDs from DB, then sum `redis.CountAppSessions(appID)` for each. The DashboardService needs no new dependency — it can call the Redis package functions directly (they are package-level functions).

### 2b. Modify `web/templates/partials/dashboard_stats.tmpl`

Add a 5th stat card:
```html
<div class="col-md-3">
    <div class="card border-0 shadow-sm h-100">
        <div class="card-body">
            <div class="d-flex align-items-center">
                <div class="flex-shrink-0">
                    <div class="bg-info bg-opacity-10 p-3 rounded">
                        <i class="bi bi-broadcast text-info fs-4"></i>
                    </div>
                </div>
                <div class="flex-grow-1 ms-3">
                    <h6 class="text-muted mb-1">Active Sessions</h6>
                    <h3 class="mb-0">{{.ActiveSessions}}</h3>
                </div>
            </div>
        </div>
    </div>
</div>
```

---

## Phase 3: Sessions page + templates

### 3a. `web/templates/pages/sessions.tmpl` (NEW)

Full page template following the `activity_logs.tmpl` pattern:
- App filter dropdown (populated from DB)
- User email search input (with 300ms debounce)
- IP search input
- `#session-list-container` div with HTMX lazy load

### 3b. `web/templates/partials/session_list.tmpl` (NEW)

Table columns: User (email), Application, IP Address, User Agent, Created, Last Active, Actions
- Revoke button per row (htmx DELETE)
- "Revoke All for User" button per user group
- Pagination footer

### 3c. `web/templates/partials/session_detail.tmpl` (NEW)

Detail card with full session metadata, following the `activity_log_detail.tmpl` pattern.

### 3d. `web/templates/partials/session_revoke_confirm.tmpl` (NEW)

Confirmation dialog for single session revocation, following `api_key_revoke_confirm.tmpl` pattern.

### 3e. `web/templates/partials/session_revoke_all_confirm.tmpl` (NEW)

Confirmation dialog for bulk revocation (all sessions for a user or app).

---

## Phase 4: Sidebar navigation

**File:** `web/templates/layouts/base.tmpl`

Add a "Sessions" nav item after Activity Logs (or after Users). Icon: `bi-broadcast`.

```html
<li class="nav-item">
    <a class="nav-link sidebar-link{{if eq .ActivePage "sessions"}} active{{end}}" 
       href="/gui/sessions"
       hx-get="/gui/sessions" hx-target="#page-content" hx-select="#page-content"
       hx-swap="outerHTML show:no-scroll" hx-push-url="true"
       data-page="sessions">
        <i class="bi bi-broadcast me-2"></i>Sessions
    </a>
</li>
```

Also update the `titleMap` JS object to include `"sessions": "Sessions"`.

---

## Phase 5: GUI handler methods

**File:** `internal/admin/gui_handler.go`

Add these handler methods:

### `SessionsPage(c *gin.Context)`
- Load app list from repo for filter dropdown
- Render `sessions` page template

### `SessionList(c *gin.Context)`
- Parse query: `page`, `app_id`, `user_search`, `ip_search`
- If `app_id` specified: `redis.GetAllSessionsForApp(appID)`
- If no app_id: iterate all apps, collect all sessions
- Apply in-memory filters (user email substring, IP substring)
- For user email display: batch `SELECT id, email FROM users WHERE id IN (...)`
- Sort by `last_active` desc
- Paginate (20 per page)
- Render `session_list` partial

### `SessionDetail(c *gin.Context)`
- Parse `app_id` and `session_id` from params
- `redis.GetSession(appID, sessionID)`
- Look up user email from DB
- Render `session_detail` partial

### `SessionRevokeConfirm(c *gin.Context)`
- Render `session_revoke_confirm` partial with session info

### `SessionRevoke(c *gin.Context)`
- Parse `app_id`, `session_id`, `user_id`
- Call `redis.DeleteSession(appID, sessionID, userID)`
- Set `HX-Trigger: sessionListRefresh`
- Render updated session list

### `SessionRevokeAllForUser(c *gin.Context)`
- Parse `app_id`, `user_id`
- Call `redis.DeleteAllUserSessions(appID, userID, "")`
- Set `HX-Trigger: sessionListRefresh`

---

## Phase 6: Register routes

**File:** `cmd/api/main.go`

Add under `guiAuth` group:

```go
// Session management
guiAuth.GET("/sessions", guiHandler.SessionsPage)
guiAuth.GET("/sessions/list", guiHandler.SessionList)
guiAuth.GET("/sessions/:id/detail", guiHandler.SessionDetail)
guiAuth.GET("/sessions/:id/revoke", guiHandler.SessionRevokeConfirm)
guiAuth.DELETE("/sessions/:id", guiHandler.SessionRevoke)
guiAuth.GET("/sessions/revoke-all-user", guiHandler.SessionRevokeAllForUserConfirm)
guiAuth.DELETE("/sessions/revoke-all-user", guiHandler.SessionRevokeAllForUser)
```

---

## Phase 7: User detail integration

**Files:** `web/templates/partials/user_detail.tmpl`, `internal/admin/gui_handler.go`

Add a "Sessions" section to the user detail view:
- Show active session count and list for that user
- HTMX-loaded partial showing the user's sessions
- Revoke buttons per session and "Revoke All" button

---

## Phase 8: Template registration + build

**File:** `web/renderer.go`

Register all new templates:
- Page: `sessions.tmpl`
- Partials: `session_list.tmpl`, `session_detail.tmpl`, `session_revoke_confirm.tmpl`, `session_revoke_all_confirm.tmpl`

Run `make build` to verify compilation.

---

## Files Summary

| File | Action |
|------|--------|
| `internal/redis/redis.go` | Modify (add app-level index) |
| `internal/admin/dashboard_service.go` | Modify (add session count) |
| `internal/admin/gui_handler.go` | Modify (add ~6-7 handler methods) |
| `cmd/api/main.go` | Modify (add routes) |
| `web/templates/layouts/base.tmpl` | Modify (sidebar item) |
| `web/templates/partials/dashboard_stats.tmpl` | Modify (add stat card) |
| `web/templates/partials/user_detail.tmpl` | Modify (add sessions section) |
| `web/renderer.go` | Modify (register templates) |
| `web/templates/pages/sessions.tmpl` | **New** |
| `web/templates/partials/session_list.tmpl` | **New** |
| `web/templates/partials/session_detail.tmpl` | **New** |
| `web/templates/partials/session_revoke_confirm.tmpl` | **New** |
| `web/templates/partials/session_revoke_all_confirm.tmpl` | **New** |
