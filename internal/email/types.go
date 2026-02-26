package email

import "github.com/gjovanovicst/auth_api/pkg/models"

// Email type code constants
const (
	TypeEmailVerification  = "email_verification"
	TypePasswordReset      = "password_reset"
	TypeTwoFACode          = "two_fa_code"
	TypeWelcome            = "welcome"
	TypeAccountDeactivated = "account_deactivated"
	TypePasswordChanged    = "password_changed"
)

// Template variable names used across email types
const (
	VarAppName           = "app_name"
	VarUserEmail         = "user_email"
	VarUserName          = "user_name"
	VarFirstName         = "first_name"
	VarLastName          = "last_name"
	VarLocale            = "locale"
	VarProfilePicture    = "profile_picture"
	VarFrontendURL       = "frontend_url"
	VarVerificationLink  = "verification_link"
	VarVerificationToken = "verification_token"
	VarResetLink         = "reset_link"
	VarCode              = "code"
	VarExpirationMinutes = "expiration_minutes"
	VarChangeTime        = "change_time"
)

// WellKnownVariables is the registry of all variables the system can auto-resolve.
// Admins can reference this list when adding variables to email types.
// Variables with Source="user" are auto-populated from the user profile when a userID is provided.
// Variables with Source="setting" are auto-populated from app/system settings.
// Variables with Source="explicit" must be passed by the caller at send time.
var WellKnownVariables = []models.EmailTypeVariable{
	// User profile variables (auto-resolved when userID is provided)
	{Name: VarUserEmail, Description: "User's email address", Source: models.VarSourceUser},
	{Name: VarUserName, Description: "User's display name", Source: models.VarSourceUser},
	{Name: VarFirstName, Description: "User's first name", Source: models.VarSourceUser},
	{Name: VarLastName, Description: "User's last name", Source: models.VarSourceUser},
	{Name: VarLocale, Description: "User's locale/language preference", Source: models.VarSourceUser},
	{Name: VarProfilePicture, Description: "User's profile picture URL", Source: models.VarSourceUser},

	// App/system settings variables (auto-resolved from config)
	{Name: VarAppName, Description: "Application name", Source: models.VarSourceSetting},
	{Name: VarFrontendURL, Description: "Frontend base URL", Source: models.VarSourceSetting},

	// Explicit variables (must be passed by the caller)
	{Name: VarVerificationLink, Description: "Email verification URL (built from token + frontend URL)", Source: models.VarSourceExplicit},
	{Name: VarVerificationToken, Description: "Raw email verification token", Source: models.VarSourceExplicit},
	{Name: VarResetLink, Description: "Password reset URL", Source: models.VarSourceExplicit},
	{Name: VarCode, Description: "2FA verification code", Source: models.VarSourceExplicit},
	{Name: VarExpirationMinutes, Description: "Expiration time in minutes", Source: models.VarSourceExplicit},
	{Name: VarChangeTime, Description: "Timestamp when the change occurred", Source: models.VarSourceExplicit},
}

// SMTPConfig holds the resolved SMTP configuration for sending emails.
// This can come from per-app settings, global system settings, or .env defaults.
type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
	UseTLS      bool
}

// EmailData holds all the data needed to render and send an email.
type EmailData struct {
	To           string
	Subject      string
	HTMLBody     string
	TextBody     string
	TemplateVars map[string]string
}

// TwoFAMethod constants
const (
	TwoFAMethodTOTP  = "totp"
	TwoFAMethodEmail = "email"
)
