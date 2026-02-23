package dto

// RegisterRequest represents the request payload for user registration
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=128"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
}

// LoginRequest represents the request payload for user login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,max=128"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
}

// RefreshTokenRequest represents the request payload for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
}

// LogoutRequest represents the request payload for user logout
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
	AccessToken  string `json:"access_token" validate:"required"`  // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
}

// ForgotPasswordRequest represents the request payload for forgot password
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents the request payload for password reset
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"` // #nosec G101 -- This is a DTO field, not a hardcoded credential
	NewPassword string `json:"new_password" validate:"required,min=8,max=128"`
}

// LoginResponse represents the response payload for successful login
type LoginResponse struct {
	AccessToken  string `json:"access_token"`  // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
	RefreshToken string `json:"refresh_token"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
}

// TwoFARequiredResponse represents response when 2FA is required during login
type TwoFARequiredResponse struct {
	Message   string `json:"message"`
	TempToken string `json:"temp_token"`
	Method    string `json:"method"` // "totp" or "email" - indicates which 2FA method the user has configured
}

// TwoFASetupRequiredResponse represents response when 2FA setup is mandatory for the application
// The user receives tokens so they can authenticate to the /2fa/generate endpoint
type TwoFASetupRequiredResponse struct {
	Message      string `json:"message"`
	AccessToken  string `json:"access_token"`  // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
	RefreshToken string `json:"refresh_token"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
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

// SocialAccountResponse represents social account data in user profile
type SocialAccountResponse struct {
	ID             string `json:"id"`
	Provider       string `json:"provider"`
	ProviderUserID string `json:"provider_user_id"`
	Email          string `json:"email,omitempty"`
	Name           string `json:"name,omitempty"`
	FirstName      string `json:"first_name,omitempty"`
	LastName       string `json:"last_name,omitempty"`
	ProfilePicture string `json:"profile_picture,omitempty"`
	Username       string `json:"username,omitempty"`
	Locale         string `json:"locale,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// UserResponse represents the user data in responses
type UserResponse struct {
	ID             string                  `json:"id"`
	Email          string                  `json:"email"`
	EmailVerified  bool                    `json:"email_verified"`
	Name           string                  `json:"name,omitempty"`
	FirstName      string                  `json:"first_name,omitempty"`
	LastName       string                  `json:"last_name,omitempty"`
	ProfilePicture string                  `json:"profile_picture,omitempty"`
	Locale         string                  `json:"locale,omitempty"`
	TwoFAEnabled   bool                    `json:"two_fa_enabled"`
	TwoFAMethod    string                  `json:"two_fa_method,omitempty"` // "totp" or "email"
	CreatedAt      string                  `json:"created_at"`
	UpdatedAt      string                  `json:"updated_at"`
	SocialAccounts []SocialAccountResponse `json:"social_accounts,omitempty"`
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// MessageResponse represents a standard message response
type MessageResponse struct {
	Message string `json:"message"`
}

// UpdateProfileRequest represents the request payload for profile update
type UpdateProfileRequest struct {
	Name           string `json:"name,omitempty" validate:"omitempty,min=1,max=100" example:"John Doe"`
	FirstName      string `json:"first_name,omitempty" validate:"omitempty,min=1,max=50" example:"John"`
	LastName       string `json:"last_name,omitempty" validate:"omitempty,min=1,max=50" example:"Doe"`
	ProfilePicture string `json:"profile_picture,omitempty" validate:"omitempty,url" example:"https://example.com/avatar.jpg"`
	Locale         string `json:"locale,omitempty" validate:"omitempty,min=2,max=10" example:"en-US"`
}

// UpdateEmailRequest represents the request payload for email update
type UpdateEmailRequest struct {
	Email    string `json:"email" validate:"required,email" example:"newemail@example.com"`
	Password string `json:"password" validate:"required,max=128" example:"currentpassword123"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
}

// UpdatePasswordRequest represents the request payload for password update
type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,max=128" example:"oldpassword123"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=128" example:"newpassword123"`
}

// DeleteAccountRequest represents the request payload for account deletion
type DeleteAccountRequest struct {
	Password        string `json:"password" validate:"required,max=128" example:"password123"` // #nosec G101,G117 -- This is a DTO field, not a hardcoded credential
	ConfirmDeletion bool   `json:"confirm_deletion" validate:"required,eq=true" example:"true"`
}
