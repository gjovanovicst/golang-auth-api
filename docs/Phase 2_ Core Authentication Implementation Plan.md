## Phase 3: Core Authentication Implementation Plan

This phase details the implementation of the core authentication functionalities, including user registration, secure password handling, user login, JSON Web Token (JWT) generation and validation, and mechanisms for token refreshing and password resets. These components form the backbone of a secure and functional authentication system.

### 3.1 User Registration

User registration involves collecting user credentials (email and password) and securely storing them in the database. A critical step is to hash the user\'s password before storage to prevent plaintext exposure in case of a data breach. After successful registration, a verification email will be sent to the user (detailed in Phase 5).

**Process Flow:**
1.  **Receive Registration Request:** The API endpoint (`POST /register`) receives a JSON payload containing the user\'s email and password.
2.  **Input Validation:** Validate the email format and password strength (e.g., minimum length, complexity requirements). Use `github.com/go-playground/validator/v10` for this.
3.  **Check for Existing User:** Query the database to ensure no user with the provided email already exists.
4.  **Password Hashing:** Hash the provided password using a strong, one-way hashing algorithm like bcrypt. `golang.org/x/crypto/bcrypt` is the recommended package.
5.  **Create User Record:** Create a new `User` record in the database with the hashed password and `EmailVerified` set to `false`.
6.  **Generate Verification Token:** Generate a unique email verification token (details in Phase 5).
7.  **Send Verification Email:** Send an email to the registered user containing a link with the verification token (details in Phase 5).
8.  **Respond:** Return a success response, indicating that the user has been registered and needs to verify their email.

**Example Code Snippets (Conceptual):**

**`internal/user/handler.go` (Registration Handler):**

```go
package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/your_username/your_project/pkg/dto"
	"github.com/your_username/your_project/pkg/errors"
)

type Handler struct {
	Service *Service
}

func NewHandler(s *Service) *Handler {
	return &Handler{Service: s}
}

func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Service.RegisterUser(req.Email, req.Password); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Code, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully. Please check your email for verification."})
}
```

**`internal/user/service.go` (Registration Logic):**

```go
package user

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/your_username/your_project/internal/email"
	"github.com/your_username/your_project/pkg/errors"
	"github.com/your_username/your_project/pkg/models"
)

type Service struct {
	Repo        *Repository
	EmailService *email.Service
}

func NewService(r *Repository, es *email.Service) *Service {
	return &Service{Repo: r, EmailService: es}
}

func (s *Service) RegisterUser(email, password string) *errors.AppError {
	// Check if user already exists
	_, err := s.Repo.GetUserByEmail(email)
	if err == nil { // User found, meaning email is already registered
		return errors.NewAppError(errors.ErrConflict, "Email already registered")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to hash password")
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		EmailVerified: false,
	}

	if err := s.Repo.CreateUser(user); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to create user")
	}

	// TODO: Generate email verification token and send email (Phase 5)
	// verificationToken := util.GenerateRandomString(32)
	// s.EmailService.SendVerificationEmail(user.Email, verificationToken)

	return nil
}
```

### 3.2 Password Hashing

Bcrypt is chosen for password hashing due to its adaptive nature, which makes it resistant to brute-force attacks even with increasing computational power. The `bcrypt.DefaultCost` provides a good balance between security and performance. When a user registers or changes their password, the plaintext password will be hashed and stored. During login, the provided password will be hashed and compared against the stored hash.

**Key functions:**
-   `bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)`: To hash a password.
-   `bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))`: To compare a plaintext password with a hashed one.

### 3.3 User Login

User login involves authenticating the user based on their provided email and password. Upon successful authentication, a JSON Web Token (JWT) will be issued to the user for subsequent authenticated requests.

**Process Flow:**
1.  **Receive Login Request:** The API endpoint (`POST /login`) receives a JSON payload with the user\'s email and password.
2.  **Input Validation:** Validate the email and password fields.
3.  **Retrieve User:** Fetch the user record from the database using the provided email.
4.  **Compare Passwords:** Compare the provided password with the stored hashed password using `bcrypt.CompareHashAndPassword`.
5.  **Check Email Verification:** If email verification is enabled, ensure the user\'s email is verified before allowing login.
6.  **Generate JWTs:** If authentication is successful, generate an Access Token and a Refresh Token. The Access Token will be short-lived, and the Refresh Token will be long-lived.
7.  **Store Refresh Token:** Store the Refresh Token (or its hash) in Redis for revocation purposes (details in Phase 5).
8.  **Respond:** Return the Access Token and Refresh Token to the client.

**Example Code Snippets (Conceptual):**

**`internal/user/handler.go` (Login Handler):**

```go
package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/your_username/your_project/pkg/dto"
	"github.com/your_username/your_project/pkg/errors"
)

// ... (existing Handler struct and NewHandler function)

func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accessToken, refreshToken, err := h.Service.LoginUser(req.Email, req.Password)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Code, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}
```

**`internal/user/service.go` (Login Logic):**

```go
package user

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/your_username/your_project/pkg/errors"
	"github.com/your_username/your_project/pkg/jwt"
	"github.com/your_username/your_project/internal/redis"
)

// ... (existing Service struct and NewService function)

func (s *Service) LoginUser(email, password string) (string, string, *errors.AppError) {
	user, err := s.Repo.GetUserByEmail(email)
	if err != nil { // User not found
		return "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid credentials")
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid credentials")
	}

	// Check if email is verified
	if !user.EmailVerified {
		return "", "", errors.NewAppError(errors.ErrForbidden, "Email not verified. Please check your inbox.")
	}

	// Generate JWTs
	accessToken, err := jwt.GenerateAccessToken(user.ID.String())
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate access token")
	}

	refreshToken, err := jwt.GenerateRefreshToken(user.ID.String())
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate refresh token")
	}

	// Store refresh token in Redis (Phase 5)
	if err := redis.SetRefreshToken(user.ID.String(), refreshToken); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to store refresh token")
	}

	return accessToken, refreshToken, nil
}
```

### 3.4 JWT Generation and Validation

JSON Web Tokens (JWTs) will be used for stateless authentication. An Access Token will be issued upon successful login, allowing the client to access protected resources. A Refresh Token will be used to obtain new Access Tokens without requiring the user to re-authenticate with their credentials.

**Access Token:**
-   **Payload:** Contains claims such as `user_id`, `exp` (expiration time), `iat` (issued at time).
-   **Expiration:** Short-lived (e.g., 15 minutes) to minimize the window of opportunity for token compromise.
-   **Signing:** Signed with a secret key using an algorithm like HS256.

**Refresh Token:**
-   **Payload:** Contains claims such as `user_id`, `exp`.
-   **Expiration:** Long-lived (e.g., 30 days) to reduce the frequency of re-login.
-   **Storage:** Stored securely on the client side (e.g., HTTP-only cookie) and in Redis on the server side for revocation.

**Key functions (`pkg/jwt/jwt.go`):**

```go
package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

var jwtSecret = []byte(viper.GetString("JWT_SECRET"))

// Claims struct that will be embedded in JWT
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateAccessToken generates a new access token
func GenerateAccessToken(userID string) (string, error) {
	expirationTime := time.Now().Add(time.Minute * time.Duration(viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES")))
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// GenerateRefreshToken generates a new refresh token
func GenerateRefreshToken(userID string) (string, error) {
	expirationTime := time.Now().Add(time.Hour * time.Duration(viper.GetInt("REFRESH_TOKEN_EXPIRATION_HOURS")))
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken parses and validates a JWT token
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}
```

### 3.5 Token Refresh Mechanism

The token refresh mechanism allows clients to obtain a new Access Token using a valid Refresh Token, without needing to re-enter their credentials. This enhances user experience and security by keeping Access Tokens short-lived.

**Process Flow:**
1.  **Receive Refresh Request:** The API endpoint (`POST /refresh-token`) receives a JSON payload containing the Refresh Token.
2.  **Validate Refresh Token:** Parse and validate the Refresh Token using the JWT library. Check its expiration and signature.
3.  **Check Redis for Revocation:** Verify that the Refresh Token is still valid and has not been revoked or blacklisted in Redis (details in Phase 5).
4.  **Retrieve User ID:** Extract the `user_id` from the Refresh Token claims.
5.  **Generate New JWTs:** Generate a new Access Token and a new Refresh Token for the user.
6.  **Update Redis:** Invalidate the old Refresh Token in Redis and store the new one.
7.  **Respond:** Return the new Access Token and Refresh Token to the client.

**Example Code Snippets (Conceptual):**

**`internal/user/handler.go` (Refresh Token Handler):**

```go
package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/your_username/your_project/pkg/dto"
	"github.com/your_username/your_project/pkg/errors"
)

// ... (existing Handler struct and NewHandler function)

func (h *Handler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newAccessToken, newRefreshToken, err := h.Service.RefreshUserToken(req.RefreshToken)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Code, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
	})
}
```

**`internal/user/service.go` (Refresh Token Logic):**

```go
package user

import (
	"github.com/your_username/your_project/pkg/errors"
	"github.com/your_username/your_project/pkg/jwt"
	"github.com/your_username/your_project/internal/redis"
)

// ... (existing Service struct and NewService function)

func (s *Service) RefreshUserToken(refreshToken string) (string, string, *errors.AppError) {
	claims, err := jwt.ParseToken(refreshToken)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrUnauthorized, "Invalid refresh token")
	}

	// Check if refresh token is blacklisted/revoked in Redis
	if revoked, err := redis.IsRefreshTokenRevoked(claims.UserID, refreshToken); err != nil || revoked {
		return "", "", errors.NewAppError(errors.ErrUnauthorized, "Refresh token revoked or invalid")
	}

	// Generate new access and refresh tokens
	newAccessToken, err := jwt.GenerateAccessToken(claims.UserID)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new access token")
	}
	newRefreshToken, err := jwt.GenerateRefreshToken(claims.UserID)
	if err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to generate new refresh token")
	}

	// Invalidate old refresh token and store new one in Redis
	if err := redis.RevokeRefreshToken(claims.UserID, refreshToken); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to revoke old refresh token")
	}
	if err := redis.SetRefreshToken(claims.UserID, newRefreshToken); err != nil {
		return "", "", errors.NewAppError(errors.ErrInternal, "Failed to store new refresh token")
	}

	return newAccessToken, newRefreshToken, nil
}
```

### 3.6 Password Reset Functionality

Password reset allows users to regain access to their accounts if they forget their password. This process typically involves sending a time-limited token to the user\'s registered email address.

**Process Flow (Request Password Reset):**
1.  **Receive Request:** The API endpoint (`POST /forgot-password`) receives the user\'s email address.
2.  **Input Validation:** Validate the email format.
3.  **Retrieve User:** Fetch the user record by email.
4.  **Generate Reset Token:** Generate a unique, cryptographically secure, and time-limited password reset token (e.g., UUID or a random string).
5.  **Store Reset Token:** Store the token in Redis, associated with the user ID and an expiration time (e.g., 1 hour). This token should be single-use.
6.  **Send Reset Email:** Send an email to the user containing a link with the reset token. The link will direct the user to a frontend page where they can enter a new password.
7.  **Respond:** Return a success response, indicating that a password reset email has been sent.

**Process Flow (Confirm Password Reset):**
1.  **Receive Request:** The API endpoint (`POST /reset-password`) receives the reset token and the new password.
2.  **Input Validation:** Validate the new password strength.
3.  **Validate Reset Token:** Retrieve the user ID associated with the token from Redis. Check if the token exists and is not expired or already used.
4.  **Hash New Password:** Hash the new password using bcrypt.
5.  **Update User Password:** Update the user\'s password in the database with the new hashed password.
6.  **Invalidate Token:** Delete the reset token from Redis to ensure it cannot be reused.
7.  **Respond:** Return a success response.

**Example Code Snippets (Conceptual):**

**`internal/user/handler.go` (Forgot Password Handler):**

```go
package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/your_username/your_project/pkg/dto"
	"github.com/your_username/your_project/pkg/errors"
)

// ... (existing Handler struct and NewHandler function)

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Service.RequestPasswordReset(req.Email); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Code, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password reset request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists, a password reset link has been sent."})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Service.ConfirmPasswordReset(req.Token, req.NewPassword); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Code, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully."})
}
```

**`internal/user/service.go` (Password Reset Logic):**

```go
package user

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/your_username/your_project/internal/email"
	"github.com/your_username/your_project/internal/redis"
	"github.com/your_username/your_project/pkg/errors"
)

// ... (existing Service struct and NewService function)

func (s *Service) RequestPasswordReset(email string) *errors.AppError {
	user, err := s.Repo.GetUserByEmail(email)
	if err != nil {
		// For security, always return a generic success message even if email not found
		return nil
	}

	resetToken := uuid.New().String()
	// Store token in Redis with expiration (e.g., 1 hour)
	if err := redis.SetPasswordResetToken(user.ID.String(), resetToken, time.Hour); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to generate reset token")
	}

	resetLink := fmt.Sprintf("http://your-frontend-app/reset-password?token=%s", resetToken)
	s.EmailService.SendPasswordResetEmail(user.Email, resetLink)

	return nil
}

func (s *Service) ConfirmPasswordReset(token, newPassword string) *errors.AppError {
	userID, err := redis.GetPasswordResetToken(token)
	if err != nil || userID == "" {
		return errors.NewAppError(errors.ErrUnauthorized, "Invalid or expired reset token")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to hash new password")
	}

	if err := s.Repo.UpdateUserPassword(userID, string(hashedPassword)); err != nil {
		return errors.NewAppError(errors.ErrInternal, "Failed to update password")
	}

	// Invalidate the token after use
	if err := redis.DeletePasswordResetToken(token); err != nil {
		// Log this error, but don\'t block the user from resetting password
		fmt.Printf("Warning: Failed to delete used password reset token from Redis: %v\n", err)
	}

	return nil
}
```

This concludes the core authentication implementation plan.

