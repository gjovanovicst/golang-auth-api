//go:build integration

package email

import (
	"testing"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

// =============================================================================
// Email Integration Tests
// =============================================================================
//
// These tests exercise the email system's template resolution, rendering, and
// sending pipeline. They run against hardcoded defaults (no database required).
//
// Run with:
//   go test -v -tags=integration ./internal/email/...
//
// For tests against a real SMTP server, set these environment variables:
//   SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD,
//   SMTP_FROM_ADDRESS, TEST_EMAIL
//
// Without SMTP config, emails are logged to stdout (dev mode).
// =============================================================================

// sampleAppID is a fake UUID used for testing (no DB lookup needed).
var sampleAppID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// newTestService creates an email service with nil repo (hardcoded defaults only).
func newTestService() *Service {
	return NewService(nil, nil)
}

// sampleVariables returns a complete set of test variables for all email types.
func sampleVariables() map[string]map[string]string {
	return map[string]map[string]string{
		TypeEmailVerification: {
			VarVerificationLink:  "https://example.com/verify-email?token=test-token-abc123",
			VarVerificationToken: "test-token-abc123",
			VarAppName:           "Integration Test App",
			VarUserEmail:         "testuser@example.com",
			VarUserName:          "Test User",
		},
		TypePasswordReset: {
			VarResetLink:         "https://example.com/reset-password?token=reset-xyz789",
			VarExpirationMinutes: "60",
			VarAppName:           "Integration Test App",
			VarUserEmail:         "testuser@example.com",
			VarUserName:          "Test User",
		},
		TypeTwoFACode: {
			VarCode:              "847293",
			VarExpirationMinutes: "5",
			VarAppName:           "Integration Test App",
			VarUserEmail:         "testuser@example.com",
			VarUserName:          "Test User",
		},
		TypeWelcome: {
			VarAppName:   "Integration Test App",
			VarUserEmail: "testuser@example.com",
			VarUserName:  "Test User",
		},
		TypeAccountDeactivated: {
			VarAppName:   "Integration Test App",
			VarUserEmail: "testuser@example.com",
			VarUserName:  "Test User",
		},
		TypePasswordChanged: {
			VarChangeTime: "2026-02-25 14:30:00 UTC",
			VarAppName:    "Integration Test App",
			VarUserEmail:  "testuser@example.com",
			VarUserName:   "Test User",
		},
	}
}

// allEmailTypes returns the 6 system email type codes.
func allEmailTypes() []string {
	return []string{
		TypeEmailVerification,
		TypePasswordReset,
		TypeTwoFACode,
		TypeWelcome,
		TypeAccountDeactivated,
		TypePasswordChanged,
	}
}

// =============================================================================
// Test: Default Template Resolution
// =============================================================================

// TestDefaultTemplateResolution verifies that all 6 hardcoded default templates
// exist and have valid content (subject, HTML body, text body, engine).
func TestDefaultTemplateResolution(t *testing.T) {
	for _, typeCode := range allEmailTypes() {
		t.Run(typeCode, func(t *testing.T) {
			tmpl := GetDefaultTemplate(typeCode)
			if tmpl == nil {
				t.Fatalf("GetDefaultTemplate(%q) returned nil", typeCode)
			}

			if tmpl.Name == "" {
				t.Errorf("template Name is empty for %q", typeCode)
			}
			if tmpl.Subject == "" {
				t.Errorf("template Subject is empty for %q", typeCode)
			}
			if tmpl.BodyHTML == "" {
				t.Errorf("template BodyHTML is empty for %q", typeCode)
			}
			if tmpl.BodyText == "" {
				t.Errorf("template BodyText is empty for %q", typeCode)
			}
			if tmpl.TemplateEngine == "" {
				t.Errorf("template TemplateEngine is empty for %q", typeCode)
			}

			t.Logf("OK: %s - Name=%q, Subject=%q, Engine=%s, HTML=%d bytes, Text=%d bytes",
				typeCode, tmpl.Name, tmpl.Subject, tmpl.TemplateEngine,
				len(tmpl.BodyHTML), len(tmpl.BodyText))
		})
	}
}

// TestDefaultTemplateUnknownType verifies that an unknown type code returns nil.
func TestDefaultTemplateUnknownType(t *testing.T) {
	tmpl := GetDefaultTemplate("nonexistent_type")
	if tmpl != nil {
		t.Errorf("expected nil for unknown type, got: %+v", tmpl)
	}
}

// =============================================================================
// Test: Template Rendering (all 3 engines)
// =============================================================================

// TestRenderAllDefaultTemplates renders all 6 default templates with sample
// variables and verifies the output contains expected content.
func TestRenderAllDefaultTemplates(t *testing.T) {
	renderer := NewRenderer()
	samples := sampleVariables()

	for _, typeCode := range allEmailTypes() {
		t.Run(typeCode, func(t *testing.T) {
			tmpl := GetDefaultTemplate(typeCode)
			if tmpl == nil {
				t.Fatalf("no default template for %q", typeCode)
			}

			vars := samples[typeCode]
			subject, htmlBody, textBody, err := renderer.RenderTemplate(tmpl, vars)
			if err != nil {
				t.Fatalf("RenderTemplate failed for %q: %v", typeCode, err)
			}

			if subject == "" {
				t.Errorf("rendered subject is empty")
			}
			if htmlBody == "" {
				t.Errorf("rendered HTML body is empty")
			}
			if textBody == "" {
				t.Errorf("rendered text body is empty")
			}

			// Verify app_name was substituted
			appName := vars[VarAppName]
			if appName != "" {
				assertContains(t, "subject or HTML", subject+htmlBody, appName)
			}

			// Type-specific content checks
			switch typeCode {
			case TypeEmailVerification:
				assertContains(t, "HTML", htmlBody, vars[VarVerificationLink])
				assertContains(t, "text", textBody, vars[VarVerificationLink])
			case TypePasswordReset:
				assertContains(t, "HTML", htmlBody, vars[VarResetLink])
				assertContains(t, "HTML", htmlBody, vars[VarExpirationMinutes])
			case TypeTwoFACode:
				assertContains(t, "HTML", htmlBody, vars[VarCode])
				assertContains(t, "HTML", htmlBody, vars[VarExpirationMinutes])
			case TypePasswordChanged:
				assertContains(t, "HTML", htmlBody, vars[VarChangeTime])
			}

			t.Logf("OK: Rendered %s - subject=%q, html=%d bytes, text=%d bytes",
				typeCode, subject, len(htmlBody), len(textBody))
		})
	}
}

// TestRendererGoTemplate tests the go_template engine specifically.
func TestRendererGoTemplate(t *testing.T) {
	renderer := NewRenderer()
	tmpl := &models.EmailTemplate{
		Subject:        "Hello {{.UserName}} from {{.AppName}}",
		BodyHTML:       "<h1>Welcome {{.UserName}}</h1><p>Email: {{.UserEmail}}</p>",
		BodyText:       "Welcome {{.UserName}}, email: {{.UserEmail}}",
		TemplateEngine: models.TemplateEngineGoTemplate,
	}

	vars := map[string]string{
		"user_name":  "Alice",
		"app_name":   "TestApp",
		"user_email": "alice@example.com",
	}

	subject, html, text, err := renderer.RenderTemplate(tmpl, vars)
	if err != nil {
		t.Fatalf("RenderTemplate (go_template) failed: %v", err)
	}

	assertContains(t, "subject", subject, "Alice")
	assertContains(t, "subject", subject, "TestApp")
	assertContains(t, "HTML", html, "Welcome Alice")
	assertContains(t, "HTML", html, "alice@example.com")
	assertContains(t, "text", text, "Welcome Alice")

	t.Logf("go_template: subject=%q", subject)
}

// TestRendererGoTemplateHTMLEscaping verifies that go_template escapes HTML in variables.
func TestRendererGoTemplateHTMLEscaping(t *testing.T) {
	renderer := NewRenderer()
	tmpl := &models.EmailTemplate{
		Subject:        "Test",
		BodyHTML:       "<p>Content: {{.UserInput}}</p>",
		BodyText:       "Content: {{.UserInput}}",
		TemplateEngine: models.TemplateEngineGoTemplate,
	}

	vars := map[string]string{
		"user_input": "<script>alert('xss')</script>",
	}

	_, html, _, err := renderer.RenderTemplate(tmpl, vars)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	// go_template should HTML-escape the script tag
	assertNotContains(t, "HTML", html, "<script>")
	t.Logf("go_template HTML escaping: %s", html)
}

// TestRendererPlaceholder tests the placeholder engine ({var_name} syntax).
func TestRendererPlaceholder(t *testing.T) {
	renderer := NewRenderer()
	tmpl := &models.EmailTemplate{
		Subject:        "Hello {user_name} from {app_name}",
		BodyHTML:       "<h1>Welcome {user_name}</h1><p>Email: {user_email}</p>",
		BodyText:       "Welcome {user_name}, email: {user_email}",
		TemplateEngine: models.TemplateEnginePlaceholder,
	}

	vars := map[string]string{
		"user_name":  "Bob",
		"app_name":   "PlaceholderApp",
		"user_email": "bob@example.com",
	}

	subject, html, text, err := renderer.RenderTemplate(tmpl, vars)
	if err != nil {
		t.Fatalf("RenderTemplate (placeholder) failed: %v", err)
	}

	assertContains(t, "subject", subject, "Bob")
	assertContains(t, "subject", subject, "PlaceholderApp")
	assertContains(t, "HTML", html, "Welcome Bob")
	assertContains(t, "HTML", html, "bob@example.com")
	assertContains(t, "text", text, "Welcome Bob")

	t.Logf("placeholder: subject=%q", subject)
}

// TestRendererPlaceholderNoEscaping verifies placeholder engine does NOT escape HTML.
func TestRendererPlaceholderNoEscaping(t *testing.T) {
	renderer := NewRenderer()
	tmpl := &models.EmailTemplate{
		Subject:        "Test",
		BodyHTML:       "<p>Content: {user_input}</p>",
		BodyText:       "Content: {user_input}",
		TemplateEngine: models.TemplateEnginePlaceholder,
	}

	vars := map[string]string{
		"user_input": "<b>Bold</b>",
	}

	_, html, _, err := renderer.RenderTemplate(tmpl, vars)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	// placeholder should NOT escape HTML
	assertContains(t, "HTML", html, "<b>Bold</b>")
	t.Logf("placeholder no-escaping: %s", html)
}

// TestRendererRawHTML tests the raw_html engine ({{.VarName}} without escaping).
func TestRendererRawHTML(t *testing.T) {
	renderer := NewRenderer()
	tmpl := &models.EmailTemplate{
		Subject:        "Hello {{.UserName}} from {{.AppName}}",
		BodyHTML:       "<h1>Welcome {{.UserName}}</h1><p>Content: {{.HtmlContent}}</p>",
		BodyText:       "Welcome {{.UserName}}",
		TemplateEngine: models.TemplateEngineRawHTML,
	}

	vars := map[string]string{
		"user_name":    "Charlie",
		"app_name":     "RawApp",
		"html_content": "<b>Bold Text</b>",
	}

	subject, html, text, err := renderer.RenderTemplate(tmpl, vars)
	if err != nil {
		t.Fatalf("RenderTemplate (raw_html) failed: %v", err)
	}

	assertContains(t, "subject", subject, "Charlie")
	assertContains(t, "subject", subject, "RawApp")
	assertContains(t, "HTML", html, "Welcome Charlie")
	// raw_html should NOT escape HTML content
	assertContains(t, "HTML", html, "<b>Bold Text</b>")
	assertContains(t, "text", text, "Welcome Charlie")

	t.Logf("raw_html: subject=%q", subject)
}

// =============================================================================
// Test: Send All Email Types (via service, dev mode)
// =============================================================================

// TestSendAllEmailTypes sends all 6 email types through the service layer.
// Without a real SMTP config, emails are logged to stdout (dev mode).
// Check the test output for "EMAIL (DEVELOPMENT/FALLBACK MODE)" log lines.
func TestSendAllEmailTypes(t *testing.T) {
	svc := newTestService()
	samples := sampleVariables()

	for _, typeCode := range allEmailTypes() {
		t.Run(typeCode, func(t *testing.T) {
			vars := samples[typeCode]
			err := svc.SendEmail(sampleAppID, typeCode, "test@example.com", vars)
			if err != nil {
				t.Fatalf("SendEmail(%q) failed: %v", typeCode, err)
			}
			t.Logf("OK: Sent %s (dev mode - check stdout for email content)", typeCode)
		})
	}
}

// TestSendVerificationEmailHelper tests the typed helper method.
func TestSendVerificationEmailHelper(t *testing.T) {
	svc := newTestService()
	err := svc.SendVerificationEmail(sampleAppID, "verify@example.com", "token-123", nil)
	if err != nil {
		t.Fatalf("SendVerificationEmail failed: %v", err)
	}
	t.Log("OK: SendVerificationEmail (dev mode)")
}

// TestSendPasswordResetEmailHelper tests the typed helper method.
func TestSendPasswordResetEmailHelper(t *testing.T) {
	svc := newTestService()
	err := svc.SendPasswordResetEmail(sampleAppID, "reset@example.com", "https://example.com/reset?token=abc", nil)
	if err != nil {
		t.Fatalf("SendPasswordResetEmail failed: %v", err)
	}
	t.Log("OK: SendPasswordResetEmail (dev mode)")
}

// TestSend2FACodeEmailHelper tests the typed helper method.
func TestSend2FACodeEmailHelper(t *testing.T) {
	svc := newTestService()
	err := svc.Send2FACodeEmail(sampleAppID, "2fa@example.com", "123456", nil)
	if err != nil {
		t.Fatalf("Send2FACodeEmail failed: %v", err)
	}
	t.Log("OK: Send2FACodeEmail (dev mode)")
}

// TestSendWelcomeEmailHelper tests the typed helper method.
func TestSendWelcomeEmailHelper(t *testing.T) {
	svc := newTestService()
	err := svc.SendWelcomeEmail(sampleAppID, "welcome@example.com", nil)
	if err != nil {
		t.Fatalf("SendWelcomeEmail failed: %v", err)
	}
	t.Log("OK: SendWelcomeEmail (dev mode)")
}

// TestSendAccountDeactivatedEmailHelper tests the typed helper method.
func TestSendAccountDeactivatedEmailHelper(t *testing.T) {
	svc := newTestService()
	err := svc.SendAccountDeactivatedEmail(sampleAppID, "deactivated@example.com", nil)
	if err != nil {
		t.Fatalf("SendAccountDeactivatedEmail failed: %v", err)
	}
	t.Log("OK: SendAccountDeactivatedEmail (dev mode)")
}

// TestSendPasswordChangedEmailHelper tests the typed helper method.
func TestSendPasswordChangedEmailHelper(t *testing.T) {
	svc := newTestService()
	err := svc.SendPasswordChangedEmail(sampleAppID, "changed@example.com", "2026-02-25 14:30:00 UTC", nil)
	if err != nil {
		t.Fatalf("SendPasswordChangedEmail failed: %v", err)
	}
	t.Log("OK: SendPasswordChangedEmail (dev mode)")
}

// =============================================================================
// Test: Preview Templates (via service)
// =============================================================================

// TestPreviewAllTemplates renders each default template via the PreviewTemplate
// method with sample data and verifies non-empty output.
func TestPreviewAllTemplates(t *testing.T) {
	svc := newTestService()
	samples := sampleVariables()

	for _, typeCode := range allEmailTypes() {
		t.Run(typeCode, func(t *testing.T) {
			tmpl := GetDefaultTemplate(typeCode)
			if tmpl == nil {
				t.Fatalf("no default template for %q", typeCode)
			}

			vars := samples[typeCode]
			subject, htmlBody, textBody, err := svc.PreviewTemplate(tmpl, vars)
			if err != nil {
				t.Fatalf("PreviewTemplate(%q) failed: %v", typeCode, err)
			}

			if subject == "" {
				t.Error("preview subject is empty")
			}
			if htmlBody == "" {
				t.Error("preview HTML body is empty")
			}
			if textBody == "" {
				t.Error("preview text body is empty")
			}

			t.Logf("OK: Preview %s - subject=%q, html=%d bytes, text=%d bytes",
				typeCode, subject, len(htmlBody), len(textBody))
		})
	}
}

// =============================================================================
// Test: Variable Resolution (without DB)
// =============================================================================

// TestVariableResolverWithoutDB verifies the resolver works with nil DB.
// Only explicit variables and the toEmail fallback should be available.
func TestVariableResolverWithoutDB(t *testing.T) {
	resolver := NewVariableResolver(nil)

	explicitVars := map[string]string{
		VarVerificationLink: "https://example.com/verify",
		VarAppName:          "Explicit App Name",
	}

	resolved := resolver.ResolveVariables(
		sampleAppID,
		TypeEmailVerification,
		"fallback@example.com",
		nil,
		explicitVars,
	)

	// Explicit vars should be present
	if resolved[VarVerificationLink] != "https://example.com/verify" {
		t.Errorf("expected verification_link to be set, got: %q", resolved[VarVerificationLink])
	}

	// Explicit app_name should override the default
	if resolved[VarAppName] != "Explicit App Name" {
		t.Errorf("expected app_name='Explicit App Name', got: %q", resolved[VarAppName])
	}

	// user_email should fall back to toEmail parameter
	if resolved[VarUserEmail] != "fallback@example.com" {
		t.Errorf("expected user_email='fallback@example.com', got: %q", resolved[VarUserEmail])
	}

	t.Logf("Resolved variables: %v", resolved)
}

// TestVariableResolverExplicitOverridesAll verifies that explicit vars always win.
func TestVariableResolverExplicitOverridesAll(t *testing.T) {
	resolver := NewVariableResolver(nil)

	resolved := resolver.ResolveVariables(
		sampleAppID,
		TypeWelcome,
		"original@example.com",
		nil,
		map[string]string{
			VarUserEmail: "override@example.com",
			VarAppName:   "Override App",
		},
	)

	if resolved[VarUserEmail] != "override@example.com" {
		t.Errorf("expected user_email='override@example.com', got: %q", resolved[VarUserEmail])
	}
	if resolved[VarAppName] != "Override App" {
		t.Errorf("expected app_name='Override App', got: %q", resolved[VarAppName])
	}
}

// =============================================================================
// Test: Snake-to-Pascal conversion
// =============================================================================

// TestSnakeToPascal verifies the key conversion used by the go_template engine.
func TestSnakeToPascal(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"app_name", "AppName"},
		{"user_email", "UserEmail"},
		{"verification_link", "VerificationLink"},
		{"code", "Code"},
		{"expiration_minutes", "ExpirationMinutes"},
		{"change_time", "ChangeTime"},
		{"first_name", "FirstName"},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			result := snakeToPascal(tc.input)
			if result != tc.expected {
				t.Errorf("snakeToPascal(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// Test: Well-Known Variables Registry
// =============================================================================

// TestWellKnownVariablesCompleteness verifies all expected variables are registered.
func TestWellKnownVariablesCompleteness(t *testing.T) {
	expectedVars := []string{
		VarUserEmail, VarUserName, VarFirstName, VarLastName, VarLocale, VarProfilePicture,
		VarAppName, VarFrontendURL,
		VarVerificationLink, VarVerificationToken, VarResetLink, VarCode, VarExpirationMinutes, VarChangeTime,
	}

	varMap := make(map[string]bool)
	for _, v := range WellKnownVariables {
		varMap[v.Name] = true
	}

	for _, name := range expectedVars {
		if !varMap[name] {
			t.Errorf("expected well-known variable %q not found in WellKnownVariables", name)
		}
	}

	t.Logf("WellKnownVariables has %d entries, all %d expected variables present",
		len(WellKnownVariables), len(expectedVars))
}

// TestWellKnownVariablesSources verifies variable source annotations are valid.
func TestWellKnownVariablesSources(t *testing.T) {
	validSources := map[string]bool{
		"user":     true,
		"setting":  true,
		"explicit": true,
		"":         true, // empty is allowed
	}

	for _, v := range WellKnownVariables {
		if !validSources[v.Source] {
			t.Errorf("variable %q has invalid source %q", v.Name, v.Source)
		}
		if v.Description == "" {
			t.Errorf("variable %q has empty description", v.Name)
		}
	}
}

// =============================================================================
// Test: SMTP Config Resolution (without DB)
// =============================================================================

// TestSMTPConfigResolutionNoDB verifies that without a DB, we get an empty config
// (which triggers dev mode in the sender).
func TestSMTPConfigResolutionNoDB(t *testing.T) {
	config := ResolveGlobalSMTPConfig()
	if config.Host != "" {
		t.Errorf("expected empty Host, got: %q", config.Host)
	}
	if config.Port != 0 {
		t.Errorf("expected Port=0, got: %d", config.Port)
	}
	t.Log("OK: Global SMTP config is empty (triggers dev mode)")
}

// =============================================================================
// Test: Sender dev mode behavior
// =============================================================================

// TestSenderDevMode verifies that Send() succeeds in dev mode (empty SMTP config).
func TestSenderDevMode(t *testing.T) {
	sender := NewSender()

	// Empty config -> dev mode (logs to stdout)
	err := sender.Send(SMTPConfig{}, "test@example.com", "Test Subject", "<p>Hello</p>", "Hello")
	if err != nil {
		t.Fatalf("Send in dev mode should not fail, got: %v", err)
	}
	t.Log("OK: Sender dev mode (check stdout for logged email)")
}

// TestSenderDevModeExampleHost verifies dev mode with smtp.example.com host.
func TestSenderDevModeExampleHost(t *testing.T) {
	sender := NewSender()

	config := SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	err := sender.Send(config, "test@example.com", "Test Subject", "<p>Hello</p>", "Hello")
	if err != nil {
		t.Fatalf("Send with example host should trigger dev mode, got: %v", err)
	}
	t.Log("OK: smtp.example.com triggers dev mode")
}

// TestSenderTestModeRequiresSMTP verifies that SendTest() returns an error
// when no SMTP is configured (unlike Send() which falls back to dev mode).
func TestSenderTestModeRequiresSMTP(t *testing.T) {
	sender := NewSender()

	// Empty config
	err := sender.SendTest(SMTPConfig{}, "test@example.com", "Test", "<p>Hi</p>", "Hi")
	if err == nil {
		t.Fatal("SendTest with empty config should return error")
	}
	t.Logf("OK: SendTest correctly rejected empty config: %v", err)

	// smtp.example.com
	err = sender.SendTest(SMTPConfig{Host: "smtp.example.com"}, "test@example.com", "Test", "<p>Hi</p>", "Hi")
	if err == nil {
		t.Fatal("SendTest with example host should return error")
	}
	t.Logf("OK: SendTest correctly rejected smtp.example.com: %v", err)
}

// =============================================================================
// Helpers
// =============================================================================

func assertContains(t *testing.T, label, haystack, needle string) {
	t.Helper()
	if needle == "" {
		return
	}
	if len(haystack) == 0 {
		t.Errorf("%s: content is empty, expected to contain %q", label, needle)
		return
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return
		}
	}
	t.Errorf("%s: expected to contain %q, but it doesn't (content length: %d)", label, needle, len(haystack))
}

func assertNotContains(t *testing.T, label, haystack, needle string) {
	t.Helper()
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			t.Errorf("%s: should NOT contain %q, but it does", label, needle)
			return
		}
	}
}
