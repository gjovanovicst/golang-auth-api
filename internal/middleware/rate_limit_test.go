package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/web"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestRouter creates a Gin engine with the given middleware and a 200 OK handler.
func newTestRouter(mw gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.POST("/test", mw, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

// doRequest sends a POST /test and returns the recorder.
func doRequest(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	r.ServeHTTP(w, req)
	return w
}

// clearRateLimitState removes all entries from the in-memory fallback store
// AND flushes any matching rate-limit keys from Redis, so that tests don't
// interfere with each other across runs or across tests within the same run.
func clearRateLimitState(keyPrefixes ...string) {
	// 1. Clear in-memory fallback store.
	fallback.entries.Range(func(key, _ any) bool {
		fallback.entries.Delete(key)
		return true
	})

	// 2. Clear Redis rate-limit keys (if Redis is available).
	if redis.Rdb == nil {
		return
	}
	ctx := redis.Rdb.Context()
	if _, err := redis.Rdb.Ping(ctx).Result(); err != nil {
		return // Redis not reachable — nothing to clean.
	}
	for _, prefix := range keyPrefixes {
		// Delete keys for all identifiers using a SCAN + pattern match.
		pattern := fmt.Sprintf("rl:%s:*", prefix)
		var cursor uint64
		for {
			keys, next, err := redis.Rdb.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				redis.Rdb.Del(ctx, keys...)
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
}

// clearFallback is a convenience alias that clears the in-memory store and
// all known test key prefixes from Redis.
func clearFallback() {
	clearRateLimitState(
		"test:under", "test:over", "test:lockout",
		"test:ctxkey", "test:customkey", "test:window",
		"gui:login",
	)
}

// ---------------------------------------------------------------------------
// memStore unit tests
// ---------------------------------------------------------------------------

func TestMemStoreGetOrCreate(t *testing.T) {
	clearFallback()

	e1 := fallback.getOrCreate("test:key1")
	if e1 == nil {
		t.Fatal("expected non-nil entry")
	}

	// Second call should return the same entry.
	e2 := fallback.getOrCreate("test:key1")
	if e1 != e2 {
		t.Error("expected same entry for same key")
	}

	// Different key should produce a different entry.
	e3 := fallback.getOrCreate("test:key2")
	if e1 == e3 {
		t.Error("expected different entry for different key")
	}
}

func TestMemIncr(t *testing.T) {
	clearFallback()

	e := fallback.getOrCreate("incr:test")
	window := 60 * time.Second

	// Increment three times.
	n1 := memIncr(e, window)
	n2 := memIncr(e, window)
	n3 := memIncr(e, window)

	if n1 != 1 || n2 != 2 || n3 != 3 {
		t.Errorf("expected increments 1,2,3 — got %d,%d,%d", n1, n2, n3)
	}
}

func TestMemIncrWindowReset(t *testing.T) {
	clearFallback()

	e := fallback.getOrCreate("incr:window")
	// Use a very short window so it expires quickly.
	window := 1 * time.Millisecond

	memIncr(e, window)
	memIncr(e, window)
	// Wait for the window to expire.
	time.Sleep(5 * time.Millisecond)

	// After expiry the count should reset.
	n := memIncr(e, window)
	if n != 1 {
		t.Errorf("expected count to reset to 1 after window expiry, got %d", n)
	}
}

func TestMemGetAttempts(t *testing.T) {
	clearFallback()

	e := fallback.getOrCreate("getattempts:test")
	window := 60 * time.Second

	// No attempts yet.
	if c := memGetAttempts(e, window); c != 0 {
		t.Errorf("expected 0 attempts, got %d", c)
	}

	memIncr(e, window)
	memIncr(e, window)

	if c := memGetAttempts(e, window); c != 2 {
		t.Errorf("expected 2 attempts, got %d", c)
	}
}

func TestMemGetAttemptsWindowExpiry(t *testing.T) {
	clearFallback()

	e := fallback.getOrCreate("getattempts:expiry")
	window := 1 * time.Millisecond

	memIncr(e, window)
	memIncr(e, window)
	time.Sleep(5 * time.Millisecond)

	// After window expires, getAttempts should return 0 and reset.
	if c := memGetAttempts(e, window); c != 0 {
		t.Errorf("expected 0 attempts after expiry, got %d", c)
	}
}

func TestMemIsLocked(t *testing.T) {
	clearFallback()

	e := fallback.getOrCreate("lock:test")

	// Not locked initially.
	if memIsLocked(e) {
		t.Error("expected not locked initially")
	}

	// Lock it.
	memSetLockout(e, 5*time.Second)
	if !memIsLocked(e) {
		t.Error("expected locked after memSetLockout")
	}
}

func TestMemIsLockedExpiry(t *testing.T) {
	clearFallback()

	e := fallback.getOrCreate("lock:expiry")

	// Lock with very short duration.
	memSetLockout(e, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	// Should be unlocked after expiry.
	if memIsLocked(e) {
		t.Error("expected lockout to expire")
	}
}

func TestMemClearAttempts(t *testing.T) {
	clearFallback()

	key := "clear:test"
	e := fallback.getOrCreate(key)
	memIncr(e, 60*time.Second)
	memIncr(e, 60*time.Second)

	// Clear using the exported function.
	MemClearAttempts("clear", "test")

	// After clear, a new getOrCreate should give a fresh entry.
	e2 := fallback.getOrCreate(key)
	if c := memGetAttempts(e2, 60*time.Second); c != 0 {
		t.Errorf("expected 0 after clear, got %d", c)
	}
}

// ---------------------------------------------------------------------------
// Middleware integration tests (in-memory path only — no Redis)
// ---------------------------------------------------------------------------

func TestRateLimitAllowsUnderLimit(t *testing.T) {
	clearFallback()

	cfg := RateLimitConfig{
		KeyPrefix:   "test:under",
		MaxAttempts: 3,
		Window:      60 * time.Second,
	}
	r := newTestRouter(RateLimitMiddleware(cfg))

	// Three requests should all succeed.
	for i := 0; i < 3; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitBlocksOverLimit(t *testing.T) {
	clearFallback()

	cfg := RateLimitConfig{
		KeyPrefix:   "test:over",
		MaxAttempts: 2,
		Window:      60 * time.Second,
	}
	r := newTestRouter(RateLimitMiddleware(cfg))

	// First 2 requests pass.
	for i := 0; i < 2; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Third request should be rate-limited (429).
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}

	// Verify JSON error body.
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected non-empty error message in 429 response")
	}
}

func TestRateLimitLockoutThreshold(t *testing.T) {
	clearFallback()

	cfg := RateLimitConfig{
		KeyPrefix:        "test:lockout",
		MaxAttempts:      3,
		Window:           60 * time.Second,
		LockoutThreshold: 5,
		LockoutDuration:  5 * time.Second,
	}
	r := newTestRouter(RateLimitMiddleware(cfg))

	// First 3 requests pass (under MaxAttempts).
	for i := 0; i < 3; i++ {
		w := doRequest(r)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Requests 4 and 5 are over soft limit (429, "too many requests").
	// Request 5 also triggers the lockout threshold.
	for i := 4; i <= 5; i++ {
		w := doRequest(r)
		if w.Code != http.StatusTooManyRequests {
			t.Errorf("request %d: expected 429, got %d", i, w.Code)
		}
	}

	// Request 6 should hit the hard lockout check.
	w := doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("request 6: expected 429 (locked), got %d", w.Code)
	}
}

func TestRateLimitLockoutExpiry(t *testing.T) {
	// Test the in-memory lockout expiry directly, bypassing Redis.
	clearFallback()

	e := fallback.getOrCreate("lockexp:direct")
	window := 60 * time.Second

	// Increment to threshold.
	memIncr(e, window)
	memIncr(e, window)

	// Lock with very short duration.
	memSetLockout(e, 5*time.Millisecond)
	if !memIsLocked(e) {
		t.Fatal("expected locked immediately after memSetLockout")
	}

	// Wait for lockout to expire.
	time.Sleep(10 * time.Millisecond)

	if memIsLocked(e) {
		t.Error("expected lockout to have expired")
	}

	// After clearing the entry, a new window should start fresh.
	fallback.entries.Delete("lockexp:direct")
	e2 := fallback.getOrCreate("lockexp:direct")
	if c := memGetAttempts(e2, window); c != 0 {
		t.Errorf("expected 0 attempts after clear, got %d", c)
	}
}

func TestRateLimitUseContextKey(t *testing.T) {
	clearFallback()

	cfg := RateLimitConfig{
		KeyPrefix:     "test:ctxkey",
		MaxAttempts:   1,
		Window:        60 * time.Second,
		UseContextKey: true,
	}

	var capturedError string
	r := gin.New()
	r.POST("/test", RateLimitMiddleware(cfg), func(c *gin.Context) {
		if errMsg, exists := c.Get(web.RateLimitErrorKey); exists {
			capturedError = errMsg.(string)
			c.JSON(http.StatusTooManyRequests, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First request passes.
	w := doRequest(r)
	if w.Code != http.StatusOK {
		t.Fatalf("request 1: expected 200, got %d", w.Code)
	}

	// Second request: middleware should NOT abort, but set context key.
	w = doRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("request 2: expected 429 from handler, got %d", w.Code)
	}
	if capturedError == "" {
		t.Error("expected rate limit error in context, but none found")
	}
}

func TestRateLimitCustomKeyFunc(t *testing.T) {
	clearFallback()

	cfg := RateLimitConfig{
		KeyPrefix:   "test:customkey",
		MaxAttempts: 1,
		Window:      60 * time.Second,
		KeyFunc: func(c *gin.Context) string {
			return c.GetHeader("X-Custom-Key")
		},
	}
	r := newTestRouter(RateLimitMiddleware(cfg))

	// Request with key "alice" — should succeed.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Custom-Key", "alice")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("alice request 1: expected 200, got %d", w.Code)
	}

	// Second request with key "alice" — should be blocked.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Custom-Key", "alice")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("alice request 2: expected 429, got %d", w.Code)
	}

	// Request with key "bob" — different key, should succeed.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Custom-Key", "bob")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("bob request 1: expected 200, got %d", w.Code)
	}
}

func TestRateLimitWindowExpiry(t *testing.T) {
	// Test the in-memory window expiry directly, bypassing Redis
	// (Redis truncates sub-second TTLs to 1s, making sub-second windows unreliable).
	clearFallback()

	e := fallback.getOrCreate("winexp:direct")
	window := 5 * time.Millisecond

	// Increment once — within limit.
	n := memIncr(e, window)
	if n != 1 {
		t.Fatalf("expected count 1, got %d", n)
	}

	// Increment again — over the 1-attempt limit.
	n = memIncr(e, window)
	if n != 2 {
		t.Fatalf("expected count 2, got %d", n)
	}

	// Wait for window to expire.
	time.Sleep(10 * time.Millisecond)

	// After expiry, getAttempts should return 0.
	c := memGetAttempts(e, window)
	if c != 0 {
		t.Errorf("expected 0 after window expiry, got %d", c)
	}

	// Increment should restart from 1.
	n = memIncr(e, window)
	if n != 1 {
		t.Errorf("expected count 1 after window reset, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// Pre-built config factory tests
// ---------------------------------------------------------------------------

func TestPreBuiltConfigFactories(t *testing.T) {
	// Verify that each factory returns a non-nil handler.
	factories := map[string]func() gin.HandlerFunc{
		"LoginRateLimitMiddleware": LoginRateLimitMiddleware,
		"APILoginRateLimit":        APILoginRateLimit,
		"APIRegisterRateLimit":     APIRegisterRateLimit,
		"APIForgotPasswordLimit":   APIForgotPasswordRateLimit,
		"APIRefreshTokenLimit":     APIRefreshTokenRateLimit,
		"APIResetPasswordLimit":    APIResetPasswordRateLimit,
		"API2FAVerifyLimit":        API2FAVerifyRateLimit,
	}

	for name, factory := range factories {
		h := factory()
		if h == nil {
			t.Errorf("%s returned nil handler", name)
		}
	}
}

func TestLoginRateLimitUsesContextKey(t *testing.T) {
	clearFallback()

	// LoginRateLimitMiddleware uses UseContextKey=true.
	mw := LoginRateLimitMiddleware()

	var contextKeySet bool
	r := gin.New()
	r.POST("/test", mw, func(c *gin.Context) {
		if _, exists := c.Get(web.RateLimitErrorKey); exists {
			contextKeySet = true
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Exhaust the 5-request limit.
	for i := 0; i < 5; i++ {
		doRequest(r)
	}

	// 6th request should set context key instead of aborting.
	doRequest(r)
	if !contextKeySet {
		t.Error("LoginRateLimitMiddleware should use context key mode, but rate limit error key was not set")
	}
}
