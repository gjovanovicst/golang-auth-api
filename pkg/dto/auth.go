package dto

// RegisterRequest represents the request payload for user registration
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginRequest represents the request payload for user login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshTokenRequest represents the request payload for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest represents the request payload for user logout
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ForgotPasswordRequest represents the request payload for forgot password
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents the request payload for password reset
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// LoginResponse represents the response payload for successful login
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// TwoFARequiredResponse represents response when 2FA is required during login
type TwoFARequiredResponse struct {
	Message   string `json:"message"`
	TempToken string `json:"temp_token"`
}

// TwoFAVerifyRequest represents the request payload for TOTP verification
type TwoFAVerifyRequest struct {
	Code string `json:"code" validate:"required"`
}

// TwoFALoginRequest represents the request payload for 2FA login verification
type TwoFALoginRequest struct {
	TempToken    string `json:"temp_token" validate:"required"`
	Code         string `json:"code,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
}

// TwoFADisableRequest represents the request payload for disabling 2FA
type TwoFADisableRequest struct {
	Code string `json:"code" validate:"required"`
}

// TwoFAEnableResponse represents the response when 2FA is enabled
type TwoFAEnableResponse struct {
	Message       string   `json:"message"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// TwoFARecoveryCodesResponse represents the response for new recovery codes
type TwoFARecoveryCodesResponse struct {
	Message       string   `json:"message"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// UserResponse represents the user data in responses
type UserResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	TwoFAEnabled  bool   `json:"two_fa_enabled"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// MessageResponse represents a standard message response
type MessageResponse struct {
	Message string `json:"message"`
}
