package user

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// ValidatePasswordPolicy checks a plaintext password against the application's
// configured password policy. Returns a descriptive error if the password
// does not meet requirements, or nil if it is acceptable.
func ValidatePasswordPolicy(password string, app *models.Application) error {
	minLen := app.PwMinLength
	if minLen <= 0 {
		minLen = 8 // hard minimum
	}
	maxLen := app.PwMaxLength
	if maxLen <= 0 {
		maxLen = 128
	}

	if len(password) < minLen {
		return fmt.Errorf("password must be at least %d characters long", minLen)
	}
	if len(password) > maxLen {
		return fmt.Errorf("password must not exceed %d characters", maxLen)
	}

	if app.PwRequireUpper || app.PwRequireLower || app.PwRequireDigit || app.PwRequireSymbol {
		var hasUpper, hasLower, hasDigit, hasSymbol bool
		for _, r := range password {
			switch {
			case unicode.IsUpper(r):
				hasUpper = true
			case unicode.IsLower(r):
				hasLower = true
			case unicode.IsDigit(r):
				hasDigit = true
			case unicode.IsPunct(r) || unicode.IsSymbol(r):
				hasSymbol = true
			}
		}

		var missing []string
		if app.PwRequireUpper && !hasUpper {
			missing = append(missing, "one uppercase letter")
		}
		if app.PwRequireLower && !hasLower {
			missing = append(missing, "one lowercase letter")
		}
		if app.PwRequireDigit && !hasDigit {
			missing = append(missing, "one digit")
		}
		if app.PwRequireSymbol && !hasSymbol {
			missing = append(missing, "one special character")
		}
		if len(missing) > 0 {
			return fmt.Errorf("password must contain at least %s", strings.Join(missing, ", "))
		}
	}

	return nil
}

// CheckPasswordHistory verifies that the new plaintext password has not been
// used recently. It compares against the last `historyCount` hashes stored on
// the user record. Returns an error if the password was recently used.
// Returns nil when historyCount is 0 (feature disabled).
func CheckPasswordHistory(newPassword string, user *models.User, historyCount int) error {
	if historyCount <= 0 || len(user.PasswordHistory) == 0 {
		return nil
	}

	var hashes []string
	if err := json.Unmarshal(user.PasswordHistory, &hashes); err != nil {
		// Corrupted history — fail open (don't block the user)
		return nil
	}

	limit := historyCount
	if limit > len(hashes) {
		limit = len(hashes)
	}

	for _, h := range hashes[:limit] {
		if err := bcrypt.CompareHashAndPassword([]byte(h), []byte(newPassword)); err == nil {
			return fmt.Errorf("password has been used recently; please choose a different password")
		}
	}

	return nil
}

// AppendPasswordHistory prepends the new bcrypt hash to the user's password
// history JSONB array and trims the array to at most keepCount entries.
// If keepCount is 0 the function is a no-op.
func AppendPasswordHistory(user *models.User, newHash string, keepCount int) {
	if keepCount <= 0 {
		return
	}

	var hashes []string
	if len(user.PasswordHistory) > 0 {
		// Ignore unmarshal errors — start fresh if corrupted
		_ = json.Unmarshal(user.PasswordHistory, &hashes)
	}

	// Prepend new hash so index 0 is always the most recent
	hashes = append([]string{newHash}, hashes...)

	// Trim to keepCount
	if len(hashes) > keepCount {
		hashes = hashes[:keepCount]
	}

	encoded, _ := json.Marshal(hashes)
	user.PasswordHistory = encoded
}

// IsPasswordExpired reports whether the user's password has exceeded the
// application's maximum age. Returns false when maxAgeDays is 0 (disabled)
// or when PasswordChangedAt is nil (password was never explicitly changed —
// treat as never expired to avoid locking out legacy accounts on first deploy).
func IsPasswordExpired(user *models.User, maxAgeDays int) bool {
	if maxAgeDays <= 0 || user.PasswordChangedAt == nil {
		return false
	}
	expiry := user.PasswordChangedAt.Add(time.Duration(maxAgeDays) * 24 * time.Hour)
	return time.Now().After(expiry)
}

// ResolveTokenTTLs returns the effective access and refresh token TTLs for an
// application. When the app has non-zero per-app overrides those are used;
// otherwise the function falls back to the global jwt defaults.
func ResolveTokenTTLs(app *models.Application) (accessTTL, refreshTTL time.Duration) {
	if app != nil && app.AccessTokenTTLMinutes > 0 {
		accessTTL = time.Minute * time.Duration(app.AccessTokenTTLMinutes)
	} else {
		accessTTL = 0 // jwt.GenerateAccessToken will use global default when 0
	}

	if app != nil && app.RefreshTokenTTLHours > 0 {
		refreshTTL = time.Hour * time.Duration(app.RefreshTokenTTLHours)
	} else {
		refreshTTL = 0 // jwt.GenerateRefreshToken will use global default when 0
	}

	return accessTTL, refreshTTL
}
