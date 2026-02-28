package email

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/gjovanovicst/auth_api/pkg/models"
)

// Renderer handles template rendering for all supported engines.
type Renderer struct{}

// NewRenderer creates a new Renderer.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// RenderTemplate renders an email template with the given variables.
// Returns (renderedSubject, renderedHTML, renderedText, error).
func (r *Renderer) RenderTemplate(tmpl *models.EmailTemplate, vars map[string]string) (string, string, string, error) {
	if tmpl == nil {
		return "", "", "", fmt.Errorf("template is nil")
	}

	switch tmpl.TemplateEngine {
	case models.TemplateEngineGoTemplate:
		return r.renderGoTemplate(tmpl, vars)
	case models.TemplateEnginePlaceholder:
		return r.renderPlaceholder(tmpl, vars)
	case models.TemplateEngineRawHTML:
		return r.renderRawHTML(tmpl, vars)
	default:
		// Default to go_template if engine is not recognized
		return r.renderGoTemplate(tmpl, vars)
	}
}

// RenderSubject renders just the subject line using Go template syntax.
func (r *Renderer) RenderSubject(subject string, vars map[string]string) (string, error) {
	data := r.buildTemplateData(vars)
	tmpl, err := template.New("subject").Parse(subject)
	if err != nil {
		// If parsing fails, try placeholder-style rendering
		return r.replacePlaceholders(subject, vars), nil
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return r.replacePlaceholders(subject, vars), nil
	}
	return buf.String(), nil
}

// renderGoTemplate uses Go's html/template engine.
// Template syntax: {{.AppName}}, {{.UserEmail}}, etc.
func (r *Renderer) renderGoTemplate(tmpl *models.EmailTemplate, vars map[string]string) (string, string, string, error) {
	data := r.buildTemplateData(vars)

	// Render subject
	subject, err := r.RenderSubject(tmpl.Subject, vars)
	if err != nil {
		subject = tmpl.Subject
	}

	// Render HTML body
	var htmlBody string
	if tmpl.BodyHTML != "" {
		htmlTmpl, err := template.New("html").Parse(tmpl.BodyHTML)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to parse HTML template: %w", err)
		}
		var buf bytes.Buffer
		if err := htmlTmpl.Execute(&buf, data); err != nil {
			return "", "", "", fmt.Errorf("failed to execute HTML template: %w", err)
		}
		htmlBody = buf.String()
	}

	// Render text body
	var textBody string
	if tmpl.BodyText != "" {
		// Use text/template for plain text to avoid HTML escaping
		textTmpl, err := template.New("text").Parse(tmpl.BodyText)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to parse text template: %w", err)
		}
		var buf bytes.Buffer
		if err := textTmpl.Execute(&buf, data); err != nil {
			return "", "", "", fmt.Errorf("failed to execute text template: %w", err)
		}
		textBody = buf.String()
	}

	return subject, htmlBody, textBody, nil
}

// renderPlaceholder uses simple {variable_name} replacement.
func (r *Renderer) renderPlaceholder(tmpl *models.EmailTemplate, vars map[string]string) (string, string, string, error) {
	subject := r.replacePlaceholders(tmpl.Subject, vars)
	htmlBody := r.replacePlaceholders(tmpl.BodyHTML, vars)
	textBody := r.replacePlaceholders(tmpl.BodyText, vars)
	return subject, htmlBody, textBody, nil
}

// renderRawHTML treats the template as raw HTML with {{.VarName}} substitution.
// This is similar to go_template but does not do HTML escaping of variables.
func (r *Renderer) renderRawHTML(tmpl *models.EmailTemplate, vars map[string]string) (string, string, string, error) {
	data := r.buildTemplateData(vars)

	subject, err := r.RenderSubject(tmpl.Subject, vars)
	if err != nil {
		subject = tmpl.Subject
	}

	// For raw HTML, we do simple string replacement for {{.VarName}} patterns
	// without HTML escaping (unlike go_template which escapes by default)
	htmlBody := r.replaceGoTemplateVars(tmpl.BodyHTML, data)
	textBody := r.replaceGoTemplateVars(tmpl.BodyText, data)

	return subject, htmlBody, textBody, nil
}

// buildTemplateData converts a map[string]string with snake_case keys to a
// map[string]interface{} with PascalCase keys for Go template compatibility.
func (r *Renderer) buildTemplateData(vars map[string]string) map[string]interface{} {
	data := make(map[string]interface{})
	for k, v := range vars {
		// Convert snake_case to PascalCase for Go template
		pascalKey := snakeToPascal(k)
		data[pascalKey] = v
		// Also keep the original key for flexibility
		data[k] = v
	}
	return data
}

// replacePlaceholders does simple {variable_name} replacement.
func (r *Renderer) replacePlaceholders(content string, vars map[string]string) string {
	result := content
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

// replaceGoTemplateVars does {{.VarName}} replacement without HTML escaping.
func (r *Renderer) replaceGoTemplateVars(content string, data map[string]interface{}) string {
	result := content
	for k, v := range data {
		result = strings.ReplaceAll(result, "{{."+k+"}}", fmt.Sprintf("%v", v))
	}
	return result
}

// snakeToPascal converts a snake_case string to PascalCase.
// e.g., "app_name" -> "AppName", "verification_link" -> "VerificationLink"
func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		result.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			result.WriteString(part[1:])
		}
	}
	return result.String()
}
