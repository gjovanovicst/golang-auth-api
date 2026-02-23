package dto

// ============================================================================
// Email Server Configuration DTOs
// ============================================================================

// EmailServerConfigRequest represents the request payload for creating/updating SMTP config
type EmailServerConfigRequest struct {
	Name         string `json:"name,omitempty"`
	SMTPHost     string `json:"smtp_host" validate:"required"`
	SMTPPort     int    `json:"smtp_port" validate:"required,min=1,max=65535"`
	SMTPUsername string `json:"smtp_username,omitempty"`
	SMTPPassword string `json:"smtp_password,omitempty"` // #nosec G101 -- This is a DTO field
	FromAddress  string `json:"from_address" validate:"required,email"`
	FromName     string `json:"from_name,omitempty"`
	UseTLS       bool   `json:"use_tls"`
	IsDefault    bool   `json:"is_default"`
	IsActive     bool   `json:"is_active"`
}

// EmailServerConfigResponse represents the SMTP config in API responses
type EmailServerConfigResponse struct {
	ID           string `json:"id"`
	AppID        string `json:"app_id"`
	Name         string `json:"name"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	FromAddress  string `json:"from_address"`
	FromName     string `json:"from_name"`
	UseTLS       bool   `json:"use_tls"`
	IsDefault    bool   `json:"is_default"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ============================================================================
// Email Type DTOs
// ============================================================================

// EmailTypeResponse represents an email type in API responses
type EmailTypeResponse struct {
	ID             string                      `json:"id"`
	Code           string                      `json:"code"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description"`
	DefaultSubject string                      `json:"default_subject"`
	Variables      []EmailTypeVariableResponse `json:"variables"`
	IsSystem       bool                        `json:"is_system"`
	IsActive       bool                        `json:"is_active"`
	CreatedAt      string                      `json:"created_at"`
	UpdatedAt      string                      `json:"updated_at"`
}

// EmailTypeVariableResponse represents a template variable in API responses
type EmailTypeVariableResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// CreateEmailTypeRequest represents the request payload for creating a custom email type
type CreateEmailTypeRequest struct {
	Code           string                      `json:"code" validate:"required,min=2,max=50"`
	Name           string                      `json:"name" validate:"required,min=2,max=100"`
	Description    string                      `json:"description,omitempty"`
	DefaultSubject string                      `json:"default_subject,omitempty"`
	Variables      []EmailTypeVariableResponse `json:"variables,omitempty"`
}

// UpdateEmailTypeRequest represents the request payload for updating an email type
type UpdateEmailTypeRequest struct {
	Name           string                      `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Description    string                      `json:"description,omitempty"`
	DefaultSubject string                      `json:"default_subject,omitempty"`
	Variables      []EmailTypeVariableResponse `json:"variables,omitempty"`
	IsActive       *bool                       `json:"is_active,omitempty"`
}

// ============================================================================
// Email Template DTOs
// ============================================================================

// EmailTemplateRequest represents the request payload for creating/updating a template
type EmailTemplateRequest struct {
	Name           string  `json:"name" validate:"required,min=2,max=100"`
	Subject        string  `json:"subject" validate:"required,min=2,max=255"`
	BodyHTML       string  `json:"body_html,omitempty"`
	BodyText       string  `json:"body_text,omitempty"`
	TemplateEngine string  `json:"template_engine" validate:"required,oneof=go_template placeholder raw_html"`
	FromEmail      string  `json:"from_email,omitempty"`
	FromName       string  `json:"from_name,omitempty"`
	ServerConfigID *string `json:"server_config_id,omitempty"`
	IsActive       bool    `json:"is_active"`
}

// EmailTemplateResponse represents an email template in API responses
type EmailTemplateResponse struct {
	ID             string  `json:"id"`
	AppID          *string `json:"app_id"`
	EmailTypeID    string  `json:"email_type_id"`
	EmailTypeCode  string  `json:"email_type_code,omitempty"`
	EmailTypeName  string  `json:"email_type_name,omitempty"`
	Name           string  `json:"name"`
	Subject        string  `json:"subject"`
	BodyHTML       string  `json:"body_html"`
	BodyText       string  `json:"body_text"`
	TemplateEngine string  `json:"template_engine"`
	FromEmail      string  `json:"from_email,omitempty"`
	FromName       string  `json:"from_name,omitempty"`
	ServerConfigID *string `json:"server_config_id,omitempty"`
	IsActive       bool    `json:"is_active"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// ============================================================================
// Email Preview & Test DTOs
// ============================================================================

// EmailPreviewRequest represents a request to preview a rendered template
type EmailPreviewRequest struct {
	Subject        string            `json:"subject" validate:"required"`
	BodyHTML       string            `json:"body_html,omitempty"`
	BodyText       string            `json:"body_text,omitempty"`
	TemplateEngine string            `json:"template_engine" validate:"required,oneof=go_template placeholder raw_html"`
	Variables      map[string]string `json:"variables"`
}

// EmailPreviewResponse represents the rendered preview result
type EmailPreviewResponse struct {
	Subject  string `json:"subject"`
	BodyHTML string `json:"body_html"`
	BodyText string `json:"body_text"`
}

// EmailTestRequest represents a request to send a test email
type EmailTestRequest struct {
	ToEmail string `json:"to_email" validate:"required,email"`
}

// ============================================================================
// Send Email API DTOs
// ============================================================================

// SendEmailRequest represents a request to send an email of a specific type
type SendEmailRequest struct {
	TypeCode  string            `json:"type_code" validate:"required"`
	ToEmail   string            `json:"to_email" validate:"required,email"`
	Variables map[string]string `json:"variables,omitempty"`
}

// SendEmailResponse represents the response after sending an email
type SendEmailResponse struct {
	Message  string `json:"message"`
	TypeCode string `json:"type_code"`
	ToEmail  string `json:"to_email"`
}

// ============================================================================
// 2FA Method DTOs
// ============================================================================

// TwoFAMethodsResponse represents the available 2FA methods for an application
type TwoFAMethodsResponse struct {
	AvailableMethods []string `json:"available_methods"`
	Email2FAEnabled  bool     `json:"email_2fa_enabled"`
	TOTPEnabled      bool     `json:"totp_enabled"`
}

// TwoFASetMethodRequest represents a request to set the user's preferred 2FA method
type TwoFASetMethodRequest struct {
	Method string `json:"method" validate:"required,oneof=totp email"`
}

// TwoFAEmail2FASetupRequest represents a request to set up email-based 2FA
type TwoFAEmail2FASetupRequest struct {
	// No body needed - uses the user's registered email
}

// TwoFAEmailCodeRequest represents a request to verify an email 2FA code
type TwoFAEmailCodeRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}
