package email

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
	VarVerificationLink  = "verification_link"
	VarVerificationToken = "verification_token"
	VarResetLink         = "reset_link"
	VarCode              = "code"
	VarExpirationMinutes = "expiration_minutes"
	VarChangeTime        = "change_time"
)

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
