package email

import (
	"fmt"
	"log"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// Service is the main email service that orchestrates template resolution,
// rendering, variable resolution, SMTP config resolution, and email sending.
type Service struct {
	repo     *Repository
	renderer *Renderer
	sender   *Sender
	resolver *VariableResolver
}

// NewService creates a new email Service with all its dependencies.
// The db parameter is used for variable resolution (user lookups, settings).
// If repo is nil, the service operates in legacy mode (no DB templates, global SMTP only).
func NewService(repo *Repository, db *gorm.DB) *Service {
	return &Service{
		repo:     repo,
		renderer: NewRenderer(),
		sender:   NewSender(),
		resolver: NewVariableResolver(db),
	}
}

// SendEmail is a backward-compatible wrapper around SendEmailWithContext.
// It sends an email without user context (no auto-populated user profile variables).
func (s *Service) SendEmail(appID uuid.UUID, emailTypeCode string, toEmail string, vars map[string]string) error {
	return s.SendEmailWithContext(appID, emailTypeCode, toEmail, nil, vars)
}

// SendEmailWithContext is the primary method for sending any email. It:
// 1. Resolves all template variables through the multi-source pipeline
// 2. Resolves the email template (app-specific -> global -> hardcoded default)
// 3. Renders the template with the resolved variables
// 4. Resolves the SMTP config (template-linked config -> per-app default -> global)
// 5. Sends the email
//
// Variable resolution priority (highest wins):
//   - Explicit vars passed by the caller
//   - User profile fields (when userID is provided)
//   - App/system settings (app_name, frontend_url, etc.)
//   - Static default values defined on the email type's variable declarations
func (s *Service) SendEmailWithContext(appID uuid.UUID, emailTypeCode string, toEmail string, userID *uuid.UUID, vars map[string]string) error {
	// Resolve all variables through the pipeline
	resolvedVars := s.resolver.ResolveVariables(appID, emailTypeCode, toEmail, userID, vars)

	// 1. Resolve template
	tmpl, err := s.resolveTemplate(appID, emailTypeCode)
	if err != nil {
		return fmt.Errorf("failed to resolve template for %s: %w", emailTypeCode, err)
	}
	if tmpl == nil {
		return fmt.Errorf("no template found for email type: %s", emailTypeCode)
	}

	// 2. Render template
	subject, htmlBody, textBody, err := s.renderer.RenderTemplate(tmpl, resolvedVars)
	if err != nil {
		return fmt.Errorf("failed to render template for %s: %w", emailTypeCode, err)
	}

	// 3. Resolve SMTP config (considers template's linked server config)
	smtpConfig := s.resolveSMTPConfigForTemplate(appID, tmpl)

	// 4. Send email
	return s.sender.Send(smtpConfig, toEmail, subject, htmlBody, textBody)
}

// SendVerificationEmail sends an email verification email.
// The userID parameter enables auto-population of user profile variables in the template.
func (s *Service) SendVerificationEmail(appID uuid.UUID, toEmail, token string, userID *uuid.UUID) error {
	frontendURL := viper.GetString("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:8080"
	}
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", frontendURL, token)

	return s.SendEmailWithContext(appID, TypeEmailVerification, toEmail, userID, map[string]string{
		VarVerificationLink:  verificationLink,
		VarVerificationToken: token,
	})
}

// SendPasswordResetEmail sends a password reset email.
// The userID parameter enables auto-population of user profile variables in the template.
func (s *Service) SendPasswordResetEmail(appID uuid.UUID, toEmail, resetLink string, userID *uuid.UUID) error {
	return s.SendEmailWithContext(appID, TypePasswordReset, toEmail, userID, map[string]string{
		VarResetLink:         resetLink,
		VarExpirationMinutes: "60",
	})
}

// Send2FACodeEmail sends a 2FA verification code via email.
// The userID parameter enables auto-population of user profile variables in the template.
func (s *Service) Send2FACodeEmail(appID uuid.UUID, toEmail, code string, userID *uuid.UUID) error {
	return s.SendEmailWithContext(appID, TypeTwoFACode, toEmail, userID, map[string]string{
		VarCode:              code,
		VarExpirationMinutes: "5",
	})
}

// SendWelcomeEmail sends a welcome email after successful email verification.
// The userID parameter enables auto-population of user profile variables in the template.
func (s *Service) SendWelcomeEmail(appID uuid.UUID, toEmail string, userID *uuid.UUID) error {
	return s.SendEmailWithContext(appID, TypeWelcome, toEmail, userID, map[string]string{})
}

// SendAccountDeactivatedEmail sends a notification when an account is deactivated.
// The userID parameter enables auto-population of user profile variables in the template.
func (s *Service) SendAccountDeactivatedEmail(appID uuid.UUID, toEmail string, userID *uuid.UUID) error {
	return s.SendEmailWithContext(appID, TypeAccountDeactivated, toEmail, userID, map[string]string{})
}

// SendPasswordChangedEmail sends a security notification when a password is changed.
// The userID parameter enables auto-population of user profile variables in the template.
func (s *Service) SendPasswordChangedEmail(appID uuid.UUID, toEmail, changeTime string, userID *uuid.UUID) error {
	return s.SendEmailWithContext(appID, TypePasswordChanged, toEmail, userID, map[string]string{
		VarChangeTime: changeTime,
	})
}

// SendAdmin2FACodeEmail sends a 2FA verification code to an admin's email address.
// This bypasses the app-scoped template/SMTP resolution and uses the global SMTP config
// with a simple hardcoded template, since admin accounts are not scoped to any application.
// Resolution chain: global DB config -> dev mode (log to stdout).
func (s *Service) SendAdmin2FACodeEmail(toEmail, code, adminUsername string) error {
	subject := "Auth API Admin - Your Login Verification Code"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f8f9fa; padding: 40px 20px;">
  <div style="max-width: 480px; margin: 0 auto; background: #fff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.08); padding: 32px;">
    <h2 style="margin: 0 0 8px; color: #212529;">Login Verification Code</h2>
    <p style="color: #6c757d; margin: 0 0 24px;">Hi <strong>%s</strong>, use the code below to complete your admin login.</p>
    <div style="background: #f0f4ff; border-radius: 6px; padding: 20px; text-align: center; margin-bottom: 24px;">
      <span style="font-size: 32px; font-weight: 700; letter-spacing: 6px; color: #0d6efd;">%s</span>
    </div>
    <p style="color: #6c757d; font-size: 14px; margin: 0;">This code expires in <strong>5 minutes</strong>. If you didn't request this, please ignore this email.</p>
  </div>
</body>
</html>`, adminUsername, code)

	textBody := fmt.Sprintf("Hi %s,\n\nYour login verification code is: %s\n\nThis code expires in 5 minutes.\n\nIf you didn't request this, please ignore this email.", adminUsername, code)

	// Use the global SMTP config (admin is not scoped to any app)
	var smtpConfig SMTPConfig
	if s.repo != nil {
		globalConfig, err := s.repo.GetGlobalServerConfig()
		if err != nil {
			log.Printf("Warning: failed to look up global SMTP config for admin 2FA email: %v", err)
		}
		if globalConfig != nil && globalConfig.IsActive {
			smtpConfig = SMTPConfig{
				Host:        globalConfig.SMTPHost,
				Port:        globalConfig.SMTPPort,
				Username:    globalConfig.SMTPUsername,
				Password:    globalConfig.SMTPPassword,
				FromAddress: globalConfig.FromAddress,
				FromName:    globalConfig.FromName,
				UseTLS:      globalConfig.UseTLS,
			}
		}
	}

	return s.sender.Send(smtpConfig, toEmail, subject, htmlBody, textBody)
}

// ============================================================================
// Resolution helpers
// ============================================================================

// resolveTemplate resolves the template to use for a given app and email type.
// Resolution order: DB app-specific -> DB global default -> hardcoded default.
func (s *Service) resolveTemplate(appID uuid.UUID, typeCode string) (*models.EmailTemplate, error) {
	// Try DB lookup first (app-specific -> global default)
	if s.repo != nil {
		tmpl, err := s.repo.GetTemplate(appID, typeCode)
		if err != nil {
			log.Printf("Warning: failed to look up email template from DB for %s: %v", typeCode, err)
			// Fall through to hardcoded default
		}
		if tmpl != nil {
			return tmpl, nil
		}
	}

	// Fall back to hardcoded default
	return GetDefaultTemplate(typeCode), nil
}

// resolveSMTPConfig resolves the SMTP configuration for an application.
// Resolution order: per-app DB config -> global DB config -> dev/fallback mode (logs to stdout).
func (s *Service) resolveSMTPConfig(appID uuid.UUID) SMTPConfig {
	// Try per-app config from DB
	if s.repo != nil {
		config, err := s.repo.GetServerConfig(appID)
		if err != nil {
			log.Printf("Warning: failed to look up SMTP config for app %s: %v", appID, err)
		}
		if config != nil && config.IsActive {
			return SMTPConfig{
				Host:        config.SMTPHost,
				Port:        config.SMTPPort,
				Username:    config.SMTPUsername,
				Password:    config.SMTPPassword,
				FromAddress: config.FromAddress,
				FromName:    config.FromName,
				UseTLS:      config.UseTLS,
			}
		}

		// Try global config from DB
		globalConfig, err := s.repo.GetGlobalServerConfig()
		if err != nil {
			log.Printf("Warning: failed to look up global SMTP config: %v", err)
		}
		if globalConfig != nil && globalConfig.IsActive {
			return SMTPConfig{
				Host:        globalConfig.SMTPHost,
				Port:        globalConfig.SMTPPort,
				Username:    globalConfig.SMTPUsername,
				Password:    globalConfig.SMTPPassword,
				FromAddress: globalConfig.FromAddress,
				FromName:    globalConfig.FromName,
				UseTLS:      globalConfig.UseTLS,
			}
		}
	}

	// No per-app or global config found; fall back to dev/fallback mode
	return SMTPConfig{}
}

// resolveSMTPConfigForTemplate resolves the SMTP config considering the template's
// optional linked server config and sender overrides.
// Resolution chain:
//  1. If template has a ServerConfigID, use that specific config
//  2. Otherwise fall back to resolveSMTPConfig (app default -> dev/fallback mode)
//  3. If template has FromEmail/FromName overrides, apply them on top
func (s *Service) resolveSMTPConfigForTemplate(appID uuid.UUID, tmpl *models.EmailTemplate) SMTPConfig {
	var smtpConfig SMTPConfig

	// Step 1: Try template-linked SMTP config
	if tmpl.ServerConfigID != nil && s.repo != nil {
		config, err := s.repo.GetServerConfigByID(*tmpl.ServerConfigID)
		if err != nil {
			log.Printf("Warning: failed to look up template-linked SMTP config %s: %v", tmpl.ServerConfigID, err)
		}
		if config != nil && config.IsActive {
			smtpConfig = SMTPConfig{
				Host:        config.SMTPHost,
				Port:        config.SMTPPort,
				Username:    config.SMTPUsername,
				Password:    config.SMTPPassword,
				FromAddress: config.FromAddress,
				FromName:    config.FromName,
				UseTLS:      config.UseTLS,
			}
		} else {
			// Linked config not found or inactive, fall back
			smtpConfig = s.resolveSMTPConfig(appID)
		}
	} else {
		// No template-linked config, use standard resolution
		smtpConfig = s.resolveSMTPConfig(appID)
	}

	// Step 2: Apply template-level sender overrides
	if tmpl.FromEmail != "" {
		smtpConfig.FromAddress = tmpl.FromEmail
	}
	if tmpl.FromName != "" {
		smtpConfig.FromName = tmpl.FromName
	}

	return smtpConfig
}

// resolveAppName determines the application name for use in email templates.
// Delegates to the resolver for consistency.
func (s *Service) resolveAppName(appID uuid.UUID) string {
	return s.resolver.resolveAppName(appID)
}

// GetWellKnownVariables returns the list of all variables the system can auto-resolve.
// This is useful for the admin GUI/API to show available variables when editing email types.
func (s *Service) GetWellKnownVariables() []models.EmailTypeVariable {
	return WellKnownVariables
}

// ============================================================================
// Admin/management methods (delegated to repository)
// ============================================================================

// GetServerConfig returns the active SMTP configuration for an application.
func (s *Service) GetServerConfig(appID uuid.UUID) (*models.EmailServerConfig, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetServerConfig(appID)
}

// GetServerConfigAny returns the SMTP configuration for an application regardless of active status.
// Used for admin listing and to check if a config already exists.
func (s *Service) GetServerConfigAny(appID uuid.UUID) (*models.EmailServerConfig, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetServerConfigAny(appID)
}

// SaveServerConfig creates or updates an SMTP configuration.
// For new configs (ID is zero), it creates a new record.
// For existing configs (ID is set), it updates the existing record.
// Handles is_default flag: if this config is set as default, clears the default flag on other configs for the same app.
func (s *Service) SaveServerConfig(config *models.EmailServerConfig) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}

	// Handle is_default: if setting this config as default, clear others first
	if config.IsDefault {
		if err := s.repo.ClearDefaultFlag(config.AppID); err != nil {
			return fmt.Errorf("failed to clear default flag: %w", err)
		}
	}

	if config.ID == uuid.Nil {
		// New config — create
		return s.repo.CreateServerConfig(config)
	}

	// Existing config — update
	return s.repo.UpdateServerConfig(config)
}

// GetServerConfigByID returns a specific SMTP configuration by its ID.
func (s *Service) GetServerConfigByID(id uuid.UUID) (*models.EmailServerConfig, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetServerConfigByID(id)
}

// GetServerConfigsByApp returns all SMTP configurations for a specific application.
func (s *Service) GetServerConfigsByApp(appID uuid.UUID) ([]models.EmailServerConfig, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetServerConfigsByApp(appID)
}

// GetAllServerConfigs returns all SMTP configurations across all applications.
func (s *Service) GetAllServerConfigs() ([]models.EmailServerConfig, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetAllServerConfigs()
}

// DeleteServerConfig removes the SMTP configuration for an application.
func (s *Service) DeleteServerConfig(appID uuid.UUID) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}
	return s.repo.DeleteServerConfig(appID)
}

// DeleteServerConfigByID removes a specific SMTP configuration by its ID.
func (s *Service) DeleteServerConfigByID(id uuid.UUID) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}
	return s.repo.DeleteServerConfigByID(id)
}

// GetAllEmailTypes returns all email types.
func (s *Service) GetAllEmailTypes() ([]models.EmailType, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetAllEmailTypes()
}

// GetEmailTypeByCode returns an email type by its code.
func (s *Service) GetEmailTypeByCode(code string) (*models.EmailType, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetEmailTypeByCode(code)
}

// CreateEmailType creates a new custom email type.
func (s *Service) CreateEmailType(emailType *models.EmailType) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}
	return s.repo.CreateEmailType(emailType)
}

// UpdateEmailType updates an existing email type.
func (s *Service) UpdateEmailType(emailType *models.EmailType) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}
	return s.repo.UpdateEmailType(emailType)
}

// GetEmailTypeByID returns an email type by its ID.
func (s *Service) GetEmailTypeByID(id uuid.UUID) (*models.EmailType, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetEmailTypeByID(id)
}

// DeleteEmailType deletes a custom email type (system types cannot be deleted).
func (s *Service) DeleteEmailType(id uuid.UUID) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}
	// Verify it exists and is not a system type
	emailType, err := s.repo.GetEmailTypeByID(id)
	if err != nil {
		return err
	}
	if emailType == nil {
		return fmt.Errorf("email type not found")
	}
	if emailType.IsSystem {
		return fmt.Errorf("system email types cannot be deleted")
	}
	return s.repo.DeleteEmailType(id)
}

// GetTemplatesByApp returns all templates for a specific application.
func (s *Service) GetTemplatesByApp(appID uuid.UUID) ([]models.EmailTemplate, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetTemplatesByApp(appID)
}

// GetGlobalDefaultTemplates returns all global default templates.
func (s *Service) GetGlobalDefaultTemplates() ([]models.EmailTemplate, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetGlobalDefaultTemplates()
}

// GetTemplateByID returns a specific template by ID.
func (s *Service) GetTemplateByID(id uuid.UUID) (*models.EmailTemplate, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("email repository not initialized")
	}
	return s.repo.GetTemplateByID(id)
}

// SaveAppTemplate creates or updates a template for a specific app and email type.
func (s *Service) SaveAppTemplate(appID uuid.UUID, emailTypeID uuid.UUID, template *models.EmailTemplate) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}
	return s.repo.UpsertAppTemplate(appID, emailTypeID, template)
}

// SaveGlobalTemplate creates or updates a global default template.
func (s *Service) SaveGlobalTemplate(emailTypeID uuid.UUID, template *models.EmailTemplate) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}
	return s.repo.UpsertGlobalTemplate(emailTypeID, template)
}

// DeleteTemplate removes a template by ID.
func (s *Service) DeleteTemplate(id uuid.UUID) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}
	return s.repo.DeleteTemplate(id)
}

// PreviewTemplate renders a template with sample data for preview purposes.
func (s *Service) PreviewTemplate(tmpl *models.EmailTemplate, vars map[string]string) (string, string, string, error) {
	return s.renderer.RenderTemplate(tmpl, vars)
}

// ResetTemplateToDefault overwrites a template's content with the hardcoded default.
// Only works for system email types that have a built-in default in defaults.go.
func (s *Service) ResetTemplateToDefault(id uuid.UUID) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}

	tmpl, err := s.repo.GetTemplateByID(id)
	if err != nil {
		return fmt.Errorf("template not found")
	}
	if tmpl == nil {
		return fmt.Errorf("template not found")
	}

	defaultTmpl := GetDefaultTemplate(tmpl.EmailType.Code)
	if defaultTmpl == nil {
		return fmt.Errorf("no built-in default available for email type '%s'", tmpl.EmailType.Code)
	}

	tmpl.Name = defaultTmpl.Name
	tmpl.Subject = defaultTmpl.Subject
	tmpl.BodyHTML = defaultTmpl.BodyHTML
	tmpl.BodyText = defaultTmpl.BodyText
	tmpl.TemplateEngine = defaultTmpl.TemplateEngine

	return s.repo.DB.Save(tmpl).Error
}

// SendTestEmail sends a test email using the specified app's default SMTP configuration.
func (s *Service) SendTestEmail(appID uuid.UUID, toEmail string) error {
	smtpConfig := s.resolveSMTPConfig(appID)
	appName := s.resolveAppName(appID)

	subject := fmt.Sprintf("[Test] Email from %s", appName)
	htmlBody := fmt.Sprintf(`<html><body>
<h2>Test Email</h2>
<p>This is a test email from <strong>%s</strong>.</p>
<p>If you received this email, your SMTP configuration is working correctly.</p>
</body></html>`, appName)
	textBody := fmt.Sprintf("Test Email\n\nThis is a test email from %s.\nIf you received this, your SMTP configuration is working correctly.", appName)

	return s.sender.SendTest(smtpConfig, toEmail, subject, htmlBody, textBody)
}

// SendTestEmailWithConfigID sends a test email using a specific SMTP config by ID.
func (s *Service) SendTestEmailWithConfigID(configID uuid.UUID, toEmail string) error {
	if s.repo == nil {
		return fmt.Errorf("email repository not initialized")
	}

	config, err := s.repo.GetServerConfigByID(configID)
	if err != nil {
		return fmt.Errorf("failed to look up SMTP config: %w", err)
	}
	if config == nil {
		return fmt.Errorf("SMTP configuration not found")
	}

	smtpConfig := SMTPConfig{
		Host:        config.SMTPHost,
		Port:        config.SMTPPort,
		Username:    config.SMTPUsername,
		Password:    config.SMTPPassword,
		FromAddress: config.FromAddress,
		FromName:    config.FromName,
		UseTLS:      config.UseTLS,
	}

	appName := "System"
	if config.AppID != nil {
		appName = s.resolveAppName(*config.AppID)
	}
	configName := config.Name
	if configName == "" {
		configName = "Default"
	}

	subject := fmt.Sprintf("[Test] Email from %s (%s)", appName, configName)
	htmlBody := fmt.Sprintf(`<html><body>
<h2>Test Email</h2>
<p>This is a test email from <strong>%s</strong> using SMTP config <strong>%s</strong>.</p>
<p>If you received this email, your SMTP configuration is working correctly.</p>
</body></html>`, appName, configName)
	textBody := fmt.Sprintf("Test Email\n\nThis is a test email from %s using SMTP config %s.\nIf you received this, your SMTP configuration is working correctly.", appName, configName)

	return s.sender.SendTest(smtpConfig, toEmail, subject, htmlBody, textBody)
}
