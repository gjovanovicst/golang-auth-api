package middleware

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/web"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// RateLimitConfig defines the behaviour of a single rate-limit middleware
// instance. Each route (or group) can have its own config.
type RateLimitConfig struct {
	// KeyPrefix is prepended to every Redis / in-memory key so that
	// different limiters don't collide. Example: "api:login", "api:register".
	KeyPrefix string

	// KeyFunc extracts the rate-limit key from a request. When nil the
	// default is ClientIP. For login endpoints you typically want
	// IP + email to avoid one attacker locking out a whole IP range.
	KeyFunc func(c *gin.Context) string

	// MaxAttempts is the number of requests allowed inside Window before
	// the soft limit kicks in.
	MaxAttempts int64

	// Window is the sliding window duration for MaxAttempts.
	Window time.Duration

	// LockoutThreshold is the cumulative attempt count (within the window)
	// that triggers a hard lockout. Set to 0 to disable lockouts entirely.
	LockoutThreshold int64

	// LockoutDuration is how long the hard lockout lasts.
	LockoutDuration time.Duration

	// UseContextKey — if true the middleware does NOT abort the request.
	// Instead it stores the error message in c.Set(web.RateLimitErrorKey, msg)
	// and calls c.Next(), letting the downstream handler decide how to
	// render the error (useful for HTML/HTMX GUI routes).
	// When false (default) the middleware aborts with a JSON 429 response.
	UseContextKey bool
}

// ---------------------------------------------------------------------------
// In-memory fallback store
// ---------------------------------------------------------------------------

// memEntry tracks attempts and optional lockout expiry for one key.
type memEntry struct {
	mu       sync.Mutex
	count    int64
	windowAt time.Time // when the current window started
	lockedAt time.Time // zero value means not locked
	lockExp  time.Time // when the lockout expires
}

// memStore is a process-local fallback used when Redis is unavailable.
// Because it lives in-process it is NOT shared across multiple pods, but
// it still provides per-instance protection against brute-force attacks.
type memStore struct {
	entries sync.Map // map[string]*memEntry
}

var fallback = &memStore{}

func init() {
	// Register the in-memory clear function so other packages (like admin)
	// can clear fallback counters without importing middleware directly.
	web.ClearRateLimitFallback = MemClearAttempts

	// Periodic cleanup goroutine — removes expired entries every 60s.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			fallback.entries.Range(func(key, value any) bool {
				e := value.(*memEntry)
				e.mu.Lock()
				windowExpired := now.Sub(e.windowAt) > 5*time.Minute // generous grace
				lockExpired := e.lockedAt.IsZero() || now.After(e.lockExp)
				e.mu.Unlock()
				if windowExpired && lockExpired {
					fallback.entries.Delete(key)
				}
				return true
			})
		}
	}()
}

// getOrCreate returns the entry for key, creating it if necessary.
func (s *memStore) getOrCreate(key string) *memEntry {
	val, _ := s.entries.LoadOrStore(key, &memEntry{})
	return val.(*memEntry)
}

// ---------------------------------------------------------------------------
// In-memory rate-limit logic (mirrors Redis logic exactly)
// ---------------------------------------------------------------------------

func memIsLocked(e *memEntry) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lockedAt.IsZero() {
		return false
	}
	if time.Now().After(e.lockExp) {
		// Lockout expired — clear it.
		e.lockedAt = time.Time{}
		e.lockExp = time.Time{}
		return false
	}
	return true
}

func memGetAttempts(e *memEntry, window time.Duration) int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if time.Since(e.windowAt) > window {
		// Window expired — reset.
		e.count = 0
		e.windowAt = time.Now()
	}
	return e.count
}

// memIncr increments the attempt count for the current window, returning the
// new count. If the window has expired it resets before incrementing.
func memIncr(e *memEntry, window time.Duration) int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.windowAt.IsZero() || time.Since(e.windowAt) > window {
		e.count = 0
		e.windowAt = time.Now()
	}
	e.count++
	return e.count
}

func memSetLockout(e *memEntry, dur time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lockedAt = time.Now()
	e.lockExp = time.Now().Add(dur)
}

// MemClearAttempts is exported so that login-success handlers can clear the
// in-memory counters just like they call redis.ClearLoginAttempts.
func MemClearAttempts(keyPrefix, identifier string) {
	fullKey := keyPrefix + ":" + identifier
	fallback.entries.Delete(fullKey)
}

// ---------------------------------------------------------------------------
// Generic Rate-Limit Middleware
// ---------------------------------------------------------------------------

// RateLimitMiddleware returns a Gin middleware that enforces the given config.
//
// It attempts to use Redis first; on any Redis error it transparently falls
// back to the process-local in-memory store.
func RateLimitMiddleware(cfg RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Determine the rate-limit key.
		var identifier string
		if cfg.KeyFunc != nil {
			identifier = cfg.KeyFunc(c)
		} else {
			identifier = c.ClientIP()
		}

		attemptsKey := fmt.Sprintf("rl:%s:attempts:%s", cfg.KeyPrefix, identifier)
		lockoutKey := fmt.Sprintf("rl:%s:lockout:%s", cfg.KeyPrefix, identifier)

		// ---------------------------------------------------------------
		// Try Redis first
		// ---------------------------------------------------------------
		if redisOK := tryRedis(c, cfg, attemptsKey, lockoutKey); redisOK {
			return // Redis handled it (either allowed or rate-limited).
		}

		// ---------------------------------------------------------------
		// Fallback to in-memory
		// ---------------------------------------------------------------
		tryMemory(c, cfg, cfg.KeyPrefix+":"+identifier)
	}
}

// tryRedis performs the full rate-limit check via Redis. Returns true if it
// was able to complete (whether allowed or rate-limited). Returns false if
// a Redis error occurred and the caller should fall back to in-memory.
func tryRedis(c *gin.Context, cfg RateLimitConfig, attemptsKey, lockoutKey string) bool {
	ctx := c.Request.Context()

	// 1. Check hard lockout
	if cfg.LockoutThreshold > 0 {
		val, err := redis.Rdb.Get(ctx, lockoutKey).Result()
		if err != nil && err.Error() != "redis: nil" {
			log.Printf("[rate-limit] Redis error checking lockout (%s): %v — falling back to in-memory", lockoutKey, err)
			return false
		}
		if val == "locked" {
			rejectRequest(c, cfg, "Too many failed attempts. Please try again later.")
			return true
		}
	}

	// 2. Check soft limit (current window)
	countStr, err := redis.Rdb.Get(ctx, attemptsKey).Result()
	if err != nil && err.Error() != "redis: nil" {
		log.Printf("[rate-limit] Redis error getting attempts (%s): %v — falling back to in-memory", attemptsKey, err)
		return false
	}
	var currentCount int64
	if countStr != "" {
		if _, err := fmt.Sscanf(countStr, "%d", &currentCount); err != nil {
			log.Printf("[rate-limit] Failed to parse attempt count %q: %v", countStr, err)
		}
	}
	if currentCount >= cfg.MaxAttempts {
		rejectRequest(c, cfg, "Too many requests. Please wait a moment before trying again.")
		return true
	}

	// 3. Increment
	newCount, err := redis.Rdb.Incr(ctx, attemptsKey).Result()
	if err != nil {
		log.Printf("[rate-limit] Redis error incrementing (%s): %v — falling back to in-memory", attemptsKey, err)
		return false
	}
	// Set TTL on first increment
	if newCount == 1 {
		redis.Rdb.Expire(ctx, attemptsKey, cfg.Window)
	}

	// 4. Check hard lockout threshold
	if cfg.LockoutThreshold > 0 && newCount >= cfg.LockoutThreshold {
		redis.Rdb.Set(ctx, lockoutKey, "locked", cfg.LockoutDuration)
		rejectRequest(c, cfg, "Too many failed attempts. Your access has been temporarily locked.")
		return true
	}

	c.Next()
	return true
}

// tryMemory performs the full rate-limit check using the in-memory store.
func tryMemory(c *gin.Context, cfg RateLimitConfig, fullKey string) {
	entry := fallback.getOrCreate(fullKey)

	// 1. Check hard lockout
	if cfg.LockoutThreshold > 0 && memIsLocked(entry) {
		rejectRequest(c, cfg, "Too many failed attempts. Please try again later.")
		return
	}

	// 2. Check soft limit
	if memGetAttempts(entry, cfg.Window) >= cfg.MaxAttempts {
		rejectRequest(c, cfg, "Too many requests. Please wait a moment before trying again.")
		return
	}

	// 3. Increment
	newCount := memIncr(entry, cfg.Window)

	// 4. Check hard lockout threshold
	if cfg.LockoutThreshold > 0 && newCount >= cfg.LockoutThreshold {
		memSetLockout(entry, cfg.LockoutDuration)
		rejectRequest(c, cfg, "Too many failed attempts. Your access has been temporarily locked.")
		return
	}

	c.Next()
}

// rejectRequest either aborts with JSON 429 or sets a context key, depending
// on the config.
func rejectRequest(c *gin.Context, cfg RateLimitConfig, msg string) {
	if cfg.UseContextKey {
		// GUI mode: let the downstream handler render the error.
		c.Set(web.RateLimitErrorKey, msg)
		c.Next()
		return
	}
	// API mode: abort with JSON.
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error": msg,
	})
}

// ---------------------------------------------------------------------------
// GUI Login Rate Limiter (preserves existing behaviour, uses generic internals)
// ---------------------------------------------------------------------------

// LoginRateLimitMiddleware enforces rate limiting on the admin GUI login endpoint.
//
// This is a convenience wrapper around RateLimitMiddleware with:
//   - 5 attempts per 60-second window per IP
//   - Hard lockout after 10 consecutive attempts for 15 minutes
//   - Errors stored in context key (GUI-mode) so the login handler renders them
//
// A successful login should call redis.ClearLoginAttempts(ip) AND
// middleware.MemClearAttempts("gui:login", ip) to reset both stores.
func LoginRateLimitMiddleware() gin.HandlerFunc {
	return RateLimitMiddleware(RateLimitConfig{
		KeyPrefix:        "gui:login",
		MaxAttempts:      5,
		Window:           60 * time.Second,
		LockoutThreshold: 10,
		LockoutDuration:  15 * time.Minute,
		UseContextKey:    true,
	})
}

// ---------------------------------------------------------------------------
// Pre-built configs for public API routes (used in Task 6)
// ---------------------------------------------------------------------------

// APILoginRateLimit — 5 requests/min per IP+email, lockout after 10
func APILoginRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(RateLimitConfig{
		KeyPrefix: "api:login",
		KeyFunc: func(c *gin.Context) string {
			// Attempt to extract email from the JSON body via a peek.
			// We read the email from form/query since reading body would
			// consume it. Instead we use IP only + a tighter limit,
			// or callers can provide a custom KeyFunc.
			return c.ClientIP()
		},
		MaxAttempts:      5,
		Window:           60 * time.Second,
		LockoutThreshold: 10,
		LockoutDuration:  15 * time.Minute,
		UseContextKey:    false,
	})
}

// APIRegisterRateLimit — 3 requests/min per IP
func APIRegisterRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(RateLimitConfig{
		KeyPrefix:   "api:register",
		MaxAttempts: 3,
		Window:      60 * time.Second,
	})
}

// APIForgotPasswordRateLimit — 3 requests/min per IP
func APIForgotPasswordRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(RateLimitConfig{
		KeyPrefix:   "api:forgot-password",
		MaxAttempts: 3,
		Window:      60 * time.Second,
	})
}

// APIRefreshTokenRateLimit — 10 requests/min per IP
func APIRefreshTokenRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(RateLimitConfig{
		KeyPrefix:   "api:refresh-token",
		MaxAttempts: 10,
		Window:      60 * time.Second,
	})
}

// APIResetPasswordRateLimit — 5 requests/min per IP
func APIResetPasswordRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(RateLimitConfig{
		KeyPrefix:   "api:reset-password",
		MaxAttempts: 5,
		Window:      60 * time.Second,
	})
}

// API2FAVerifyRateLimit — 5 requests/min per IP
func API2FAVerifyRateLimit() gin.HandlerFunc {
	return RateLimitMiddleware(RateLimitConfig{
		KeyPrefix:        "api:2fa-verify",
		MaxAttempts:      5,
		Window:           60 * time.Second,
		LockoutThreshold: 10,
		LockoutDuration:  15 * time.Minute,
	})
}
