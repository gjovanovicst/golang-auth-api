## Phase 5: Email Verification and Redis Integration Plan

This phase focuses on implementing email verification for new user registrations and integrating Redis for efficient token storage and management. Email verification is crucial for confirming user identity and preventing abuse, while Redis provides a high-performance, in-memory data store ideal for temporary data like JWT refresh tokens and email verification codes.

### 5.1 Email Verification

Email verification ensures that the registered email address belongs to the user. This is typically done by sending a unique, time-limited token to the user's email, which they must click to activate their account.

**Process Flow (Sending Verification Email):**
1.  **Generate Verification Token:** After a user registers, generate a unique, cryptographically secure token (e.g., a UUID or a random string).
2.  **Store Token in Redis:** Store this token in Redis, associated with the user's ID, and set an expiration time (e.g., 24 hours). This token should be single-use.
3.  **Construct Verification Link:** Create a verification URL that includes the token (e.g., `http://your-api-domain/verify-email?token=YOUR_TOKEN`).
4.  **Send Email:** Use an email sending library (e.g., `gopkg.in/mail.v2`) to send an email to the user's registered address. The email should contain the verification link and clear instructions.

**Process Flow (Verifying Email):**
1.  **Receive Verification Request:** The API endpoint (`GET /verify-email`) receives the verification token from the user's click.
2.  **Validate Token:** Retrieve the user ID associated with the token from Redis. Check if the token exists and is not expired or already used.
3.  **Update User Status:** If the token is valid, update the `EmailVerified` field for the corresponding user in the PostgreSQL database to `true`.
4.  **Invalidate Token:** Delete the verification token from Redis to ensure it cannot be reused.
5.  **Respond:** Redirect the user to a success page or return a success message.

**Example Code Snippets (Conceptual):**

**`internal/email/service.go` (Email Sending Service):**

```go
package email

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
	"gopkg.in/mail.v2"
)

type Service struct {
	// Configuration for SMTP server
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) SendVerificationEmail(toEmail, token string) error {
	from := viper.GetString("EMAIL_FROM")
	subject := "Verify Your Email Address"
	body := fmt.Sprintf("Please verify your email address by clicking on the link: http://localhost:8080/verify-email?token=%s", token)

	m := mail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	d := mail.NewDialer(
		viper.GetString("EMAIL_HOST"),
		viper.GetInt("EMAIL_PORT"),
		viper.GetString("EMAIL_USERNAME"),
		viper.GetString("EMAIL_PASSWORD"),
	)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send verification email to %s: %v", toEmail, err)
		return err
	}
	log.Printf("Verification email sent to %s", toEmail)
	return nil
}

func (s *Service) SendPasswordResetEmail(toEmail, resetLink string) error {
	from := viper.GetString("EMAIL_FROM")
	subject := "Password Reset Request"
	body := fmt.Sprintf("You requested a password reset. Please click on the link to reset your password: %s", resetLink)

	m := mail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	d := mail.NewDialer(
		viper.GetString("EMAIL_HOST"),
		viper.GetInt("EMAIL_PORT"),
		viper.GetString("EMAIL_USERNAME"),
		viper.GetString("EMAIL_PASSWORD"),
	)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send password reset email to %s: %v", toEmail, err)
		return err
	}
	log.Printf("Password reset email sent to %s", toEmail)
	return nil
}
```

**`internal/user/service.go` (Update RegisterUser and add VerifyEmail):**

```go
package user

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/your_username/your_project/internal/email"
	"github.com/your_username/your_project/internal/redis"
	"github.com/your_username/your_project/pkg/errors"
	"github.com/your_username/your_project/pkg/models"
)

// ... (existing Service struct and NewService function)

func (s *Service) RegisterUser(email, password string) *errors.AppError {
	// ... (existing code)

	user := &models.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		EmailVerified: false,
	}

	if err := s.Repo.CreateUser(user); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to create user")
	}

	// Generate email verification token and send email
	verificationToken := uuid.New().String()
	if err := redis.SetEmailVerificationToken(user.ID.String(), verificationToken, 24*time.Hour); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to store verification token")
	}

	if err := s.EmailService.SendVerificationEmail(user.Email, verificationToken); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to send verification email")
	}

	return nil
}

func (s *Service) VerifyEmail(token string) *errors.AppError {
	userID, err := redis.GetEmailVerificationToken(token)
	if err != nil || userID == "" {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired verification token")
	}

	// Update user's email_verified status in DB
	if err := s.Repo.UpdateUserEmailVerified(userID, true); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to verify email")
	}

	// Invalidate the token after use
	if err := redis.DeleteEmailVerificationToken(token); err != nil {
		fmt.Printf("Warning: Failed to delete used email verification token from Redis: %v\n", err)
	}

	return nil
}
```

**`internal/user/handler.go` (Add VerifyEmail Handler):**

```go
package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/your_username/your_project/pkg/errors"
)

// ... (existing Handler struct and NewHandler function)

func (h *Handler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token is missing"})
		return
	}

	if err := h.Service.VerifyEmail(token); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Code, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully!"})
}
```

### 5.2 Redis Integration for Token Storage

Redis will be used as an in-memory data store for temporary and frequently accessed data, specifically for JWT refresh tokens, email verification tokens, and password reset tokens. Its speed and support for time-to-live (TTL) make it ideal for these use cases.

**Configuration:**
-   `REDIS_ADDR` (e.g., `localhost:6379`)
-   `REDIS_PASSWORD` (if any)
-   `REDIS_DB` (database number)

**Key Operations:**
-   **Store Token with TTL:** Store a token (e.g., refresh token, verification token) as a key-value pair with an expiration time.
-   **Retrieve Token:** Retrieve a token by its key.
-   **Delete Token:** Remove a token from Redis (e.g., after it's used or revoked).
-   **Check Existence:** Check if a token exists.

**`internal/redis/redis.go` (Redis Client Setup and Functions):**

```go
package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

var Rdb *redis.Client
var ctx = context.Background()

func ConnectRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     viper.GetString("REDIS_ADDR"),
		Password: viper.GetString("REDIS_PASSWORD"),
		DB:       viper.GetInt("REDIS_DB"),
	})

	_, err := Rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	log.Println("Connected to Redis!")
}

// SetRefreshToken stores a refresh token with its expiration
func SetRefreshToken(userID, token string) error {
	key := fmt.Sprintf("refresh_token:%s", userID)
	expiration := time.Hour * time.Duration(viper.GetInt("REFRESH_TOKEN_EXPIRATION_HOURS"))
	return Rdb.Set(ctx, key, token, expiration).Err()
}

// GetRefreshToken retrieves a refresh token
func GetRefreshToken(userID string) (string, error) {
	key := fmt.Sprintf("refresh_token:%s", userID)
	return Rdb.Get(ctx, key).Result()
}

// RevokeRefreshToken deletes a refresh token (effectively blacklisting it)
func RevokeRefreshToken(userID, token string) error {
	// For simplicity, we'll just delete the token associated with the user ID.
	// A more robust solution might involve a blacklist set for specific tokens.
	key := fmt.Sprintf("refresh_token:%s", userID)
	val, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil // Token already gone or never existed
	} else if err != nil {
		return err
	}

	if val == token {
		return Rdb.Del(ctx, key).Err()
	}
	return nil // Token found but doesn't match, might be an older token
}

// IsRefreshTokenRevoked checks if a refresh token is revoked (by checking if it exists)
func IsRefreshTokenRevoked(userID, token string) (bool, error) {
	key := fmt.Sprintf("refresh_token:%s", userID)
	val, err := Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return true, nil // Token not found, so it's considered revoked or expired
	} else if err != nil {
		return false, err
	}
	return val != token, nil // If value doesn't match, it means a new token was issued, old one is implicitly revoked
}

// SetEmailVerificationToken stores an email verification token
func SetEmailVerificationToken(userID, token string, expiration time.Duration) error {
	key := fmt.Sprintf("email_verify:%s", token)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// GetEmailVerificationToken retrieves an email verification token
func GetEmailVerificationToken(token string) (string, error) {
	key := fmt.Sprintf("email_verify:%s", token)
	return Rdb.Get(ctx, key).Result()
}

// DeleteEmailVerificationToken deletes an email verification token
func DeleteEmailVerificationToken(token string) error {
	key := fmt.Sprintf("email_verify:%s", token)
	return Rdb.Del(ctx, key).Err()
}

// SetPasswordResetToken stores a password reset token
func SetPasswordResetToken(userID, token string, expiration time.Duration) error {
	key := fmt.Sprintf("password_reset:%s", token)
	return Rdb.Set(ctx, key, userID, expiration).Err()
}

// GetPasswordResetToken retrieves a password reset token
func GetPasswordResetToken(token string) (string, error) {
	key := fmt.Sprintf("password_reset:%s", token)
	return Rdb.Get(ctx, key).Result()
}

// DeletePasswordResetToken deletes a password reset token
func DeletePasswordResetToken(token string) error {
	key := fmt.Sprintf("password_reset:%s", token)
	return Rdb.Del(ctx, key).Err()
}
```

This phase completes the implementation of email verification and integrates Redis for efficient token management, laying the groundwork for secure and scalable authentication.

