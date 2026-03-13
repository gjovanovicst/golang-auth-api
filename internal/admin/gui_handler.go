package admin

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/bruteforce"
	"github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/geoip"
	logService "github.com/gjovanovicst/auth_api/internal/log"
	oidcpkg "github.com/gjovanovicst/auth_api/internal/oidc"
	"github.com/gjovanovicst/auth_api/internal/rbac"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/twofa"
	userimport "github.com/gjovanovicst/auth_api/internal/user"
	passkeypkg "github.com/gjovanovicst/auth_api/internal/webauthn"
	"github.com/gjovanovicst/auth_api/internal/webhook"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/gjovanovicst/auth_api/web"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// Brute-force form defaults — match the hardcoded defaults in bruteforce.ResolveConfig().
// These are used to pre-fill the GUI form fields so admins see the effective values.
const (
	bfDefaultLockoutEnabled   = true
	bfDefaultLockoutThreshold = "5"
	bfDefaultLockoutDurations = "15m,30m,1h,24h"
	bfDefaultLockoutWindow    = "15m"
	bfDefaultLockoutTierTTL   = "24h"

	bfDefaultDelayEnabled    = true
	bfDefaultDelayStartAfter = "2"
	bfDefaultDelayMaxSeconds = "16"
	bfDefaultDelayTierTTL    = "30m"

	bfDefaultCaptchaEnabled   = true
	bfDefaultCaptchaThreshold = "3"
	bfDefaultCaptchaSiteKey   = "6LeIxAcTAAAAAJcZVRqyHh71UMIEGNQ_MXjiZKhI"
)

// GUIHandler serves HTML pages for the Admin GUI.
// Separate from Handler (which serves the JSON admin API).
type GUIHandler struct {
	AccountService    *AccountService
	DashboardService  *DashboardService
	Repo              *Repository
	SettingsService   *SettingsService
	EmailService      *email.Service
	RBACService       *rbac.Service
	PasskeyService    *passkeypkg.Service
	IPRuleRepo        *geoip.IPRuleRepository        // IP rule repository (nil = IP rules disabled)
	IPRuleEvaluator   *geoip.IPRuleEvaluator         // IP rule evaluator for cache invalidation (nil = disabled)
	GeoIPService      *geoip.Service                 // GeoIP service for IP lookups (nil = disabled)
	BruteForceService *bruteforce.Service            // Brute-force protection service for account unlock (nil = disabled)
	WebhookService    *webhook.Service               // Webhook management service (nil = webhooks disabled)
	OIDCService       *oidcpkg.Service               // OIDC provider service (nil = OIDC disabled)
	TrustedDeviceRepo *twofa.TrustedDeviceRepository // Trusted device repository (nil = feature disabled)
}

// NewGUIHandler creates a new GUIHandler
func NewGUIHandler(accountService *AccountService, dashboardService *DashboardService, repo *Repository, settingsService *SettingsService, emailService *email.Service, rbacService *rbac.Service, passkeyService *passkeypkg.Service) *GUIHandler {
	return &GUIHandler{
		AccountService:   accountService,
		DashboardService: dashboardService,
		Repo:             repo,
		SettingsService:  settingsService,
		EmailService:     emailService,
		RBACService:      rbacService,
		PasskeyService:   passkeyService,
	}
}

// LoginPage renders the login form.
// GET /gui/login
func (h *GUIHandler) LoginPage(c *gin.Context) {
	data := web.TemplateData{
		Redirect: c.Query("redirect"),
	}
	c.HTML(http.StatusOK, "login", data)
}

// LoginSubmit handles login form submission.
// If the admin has 2FA enabled, it creates a temporary session and redirects
// to the 2FA verification page instead of creating a full session.
// POST /gui/login
func (h *GUIHandler) LoginSubmit(c *gin.Context) {
	// Check if rate limiter already blocked the request
	if errMsg, exists := c.Get(web.RateLimitErrorKey); exists {
		msg, _ := errMsg.(string)
		data := web.TemplateData{
			Error:    msg,
			Username: c.PostForm("username"),
			Redirect: c.PostForm("redirect"),
		}
		c.HTML(http.StatusTooManyRequests, "login", data)
		return
	}

	username := c.PostForm("username")
	password := c.PostForm("password")
	redirect := c.PostForm("redirect")

	// Validate input
	if username == "" || password == "" {
		data := web.TemplateData{
			Error:    "Username or email and password are required.",
			Username: username,
			Redirect: redirect,
		}
		c.HTML(http.StatusBadRequest, "login", data)
		return
	}

	// Authenticate
	account, err := h.AccountService.Authenticate(username, password)
	if err != nil {
		data := web.TemplateData{
			Error:    "Invalid username/email or password.",
			Username: username,
			Redirect: redirect,
		}
		c.HTML(http.StatusUnauthorized, "login", data)
		return
	}

	// Check if 2FA is enabled for this admin
	if account.TwoFAEnabled {
		// Create a temporary session (not a full session)
		tempToken, err := h.AccountService.Create2FATempSession(account.ID.String())
		if err != nil {
			data := web.TemplateData{
				Error:    "An internal error occurred. Please try again.",
				Username: username,
				Redirect: redirect,
			}
			c.HTML(http.StatusInternalServerError, "login", data)
			return
		}

		// For email-based 2FA, generate and send the code now
		if account.TwoFAMethod == "email" {
			if err := h.AccountService.GenerateAndSendEmail2FACode(account.ID.String()); err != nil {
				data := web.TemplateData{
					Error:    "Failed to send verification code. Please try again.",
					Username: username,
					Redirect: redirect,
				}
				c.HTML(http.StatusInternalServerError, "login", data)
				return
			}
		}

		// Build redirect URL to 2FA verification page
		redirectURL := fmt.Sprintf("/gui/2fa-verify?token=%s&method=%s", tempToken, account.TwoFAMethod)
		if redirect != "" && redirect != "/gui/login" {
			redirectURL += "&redirect=" + redirect
		}
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// No 2FA — create full session directly
	sessionID, err := h.AccountService.CreateSession(account.ID.String())
	if err != nil {
		data := web.TemplateData{
			Error:    "An internal error occurred. Please try again.",
			Username: username,
			Redirect: redirect,
		}
		c.HTML(http.StatusInternalServerError, "login", data)
		return
	}

	// Set session cookie
	maxAge := sessionMaxAgeSeconds()
	web.SetSessionCookie(c, sessionID, maxAge)

	// Clear rate limit counters on successful login (both Redis and in-memory fallback)
	_ = redis.ClearLoginAttempts(c.ClientIP())              // legacy admin:login_attempts keys
	_ = redis.ClearRateLimitKeys("gui:login", c.ClientIP()) // new rl:gui:login keys
	if web.ClearRateLimitFallback != nil {
		web.ClearRateLimitFallback("gui:login", c.ClientIP())
	}

	// Redirect to original page or dashboard
	if redirect != "" && redirect != "/gui/login" {
		c.Redirect(http.StatusFound, redirect)
		return
	}
	c.Redirect(http.StatusFound, "/gui/")
}

// Logout destroys the admin session and redirects to login.
// GET /gui/logout
func (h *GUIHandler) Logout(c *gin.Context) {
	sessionID, err := c.Cookie(web.AdminSessionCookie)
	if err == nil && sessionID != "" {
		_ = h.AccountService.Logout(sessionID)
	}

	// Clear cookie
	web.ClearSessionCookie(c)

	c.Redirect(http.StatusFound, "/gui/login")
}

// Dashboard renders the main dashboard page.
// GET /gui/
func (h *GUIHandler) Dashboard(c *gin.Context) {
	data := web.TemplateData{
		ActivePage:    "dashboard",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
	}
	c.HTML(http.StatusOK, "dashboard", data)
}

// DashboardStats returns the stats cards HTML fragment for HTMX.
// GET /gui/dashboard/stats
func (h *GUIHandler) DashboardStats(c *gin.Context) {
	stats, err := h.DashboardService.GetStats()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load dashboard stats.</div>`)
		return
	}
	c.HTML(http.StatusOK, "dashboard_stats", stats)
}

// DashboardActivity returns the recent activity table HTML fragment for HTMX.
// GET /gui/dashboard/activity
func (h *GUIHandler) DashboardActivity(c *gin.Context) {
	logs, err := h.DashboardService.GetRecentActivity(10)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load recent activity.</div>`)
		return
	}
	c.HTML(http.StatusOK, "dashboard_activity", logs)
}

// --- Tenant Management ---

// TenantPage renders the tenant management page.
// GET /gui/tenants
func (h *GUIHandler) TenantPage(c *gin.Context) {
	data := web.TemplateData{
		ActivePage:    "tenants",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
	}
	c.HTML(http.StatusOK, "tenants", data)
}

// TenantList returns the tenant table HTML fragment for HTMX.
// GET /gui/tenants/list
func (h *GUIHandler) TenantList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 10

	tenants, total, err := h.Repo.ListTenantsWithAppCount(page, pageSize)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load tenants.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	type tenantListData struct {
		Tenants    []TenantListItem
		Page       int
		TotalPages int
		Total      int64
	}

	c.HTML(http.StatusOK, "tenant_list", tenantListData{
		Tenants:    tenants,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
	})
}

// TenantCreateForm returns the empty create form HTML fragment for HTMX.
// GET /gui/tenants/new
func (h *GUIHandler) TenantCreateForm(c *gin.Context) {
	type formData struct {
		ID   string
		Name string
	}
	c.HTML(http.StatusOK, "tenant_form", formData{})
}

// TenantCreate handles creating a new tenant.
// POST /gui/tenants
func (h *GUIHandler) TenantCreate(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Tenant name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	tenant := &models.Tenant{Name: name}
	if err := h.Repo.CreateTenant(tenant); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create tenant. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "tenantListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Tenant created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// TenantEditForm returns the pre-filled edit form HTML fragment for HTMX.
// GET /gui/tenants/:id/edit
func (h *GUIHandler) TenantEditForm(c *gin.Context) {
	id := c.Param("id")
	tenant, err := h.Repo.GetTenantByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Tenant not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	type formData struct {
		ID   string
		Name string
	}
	c.HTML(http.StatusOK, "tenant_form", formData{
		ID:   tenant.ID.String(),
		Name: tenant.Name,
	})
}

// TenantUpdate handles updating a tenant.
// PUT /gui/tenants/:id
func (h *GUIHandler) TenantUpdate(c *gin.Context) {
	id := c.Param("id")
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Tenant name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if err := h.Repo.UpdateTenant(id, name); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update tenant. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "tenantListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Tenant updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// TenantDeleteConfirm returns the delete confirmation modal body for HTMX.
// GET /gui/tenants/:id/delete
func (h *GUIHandler) TenantDeleteConfirm(c *gin.Context) {
	id := c.Param("id")
	tenant, err := h.Repo.GetTenantByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Tenant not found.</div></div>`)
		return
	}

	type deleteData struct {
		ID   string
		Name string
	}
	c.HTML(http.StatusOK, "tenant_delete_confirm", deleteData{
		ID:   tenant.ID.String(),
		Name: tenant.Name,
	})
}

// TenantDelete handles deleting a tenant.
// DELETE /gui/tenants/:id
func (h *GUIHandler) TenantDelete(c *gin.Context) {
	id := c.Param("id")
	if err := h.Repo.DeleteTenant(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to delete tenant.</div>`)
		return
	}

	// Return a refreshed tenant list and trigger modal close
	c.Header("HX-Trigger", "tenantDeleted")

	// Re-fetch and render the updated tenant list
	page := 1
	pageSize := 10
	tenants, total, err := h.Repo.ListTenantsWithAppCount(page, pageSize)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Tenant deleted but failed to refresh list.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	type tenantListData struct {
		Tenants    []TenantListItem
		Page       int
		TotalPages int
		Total      int64
	}

	c.HTML(http.StatusOK, "tenant_list", tenantListData{
		Tenants:    tenants,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
	})
}

// TenantFormCancel returns an empty response to clear the form container.
// GET /gui/tenants/form-cancel
func (h *GUIHandler) TenantFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// --- Application Management ---

// AppPage renders the application management page.
// GET /gui/applications
func (h *GUIHandler) AppPage(c *gin.Context) {
	// Load all tenants for the filter dropdown
	tenants, err := h.Repo.ListAllTenants()
	if err != nil {
		tenants = nil // Degrade gracefully; filter just won't have options
	}

	data := web.TemplateData{
		ActivePage:    "applications",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
		Data:          tenants,
	}
	c.HTML(http.StatusOK, "applications", data)
}

// AppList returns the application table HTML fragment for HTMX.
// GET /gui/applications/list
func (h *GUIHandler) AppList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 10
	tenantID := c.Query("tenant_id")

	apps, total, err := h.Repo.ListAppsWithDetails(page, pageSize, tenantID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load applications.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	type appListData struct {
		Apps       []AppListItem
		Page       int
		TotalPages int
		Total      int64
		TenantID   string
	}

	c.HTML(http.StatusOK, "app_list", appListData{
		Apps:       apps,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
		TenantID:   tenantID,
	})
}

// AppCreateForm returns the empty create form HTML fragment for HTMX.
// GET /gui/applications/new
func (h *GUIHandler) AppCreateForm(c *gin.Context) {
	tenants, err := h.Repo.ListAllTenants()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load tenants.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	type formData struct {
		ID                  string
		Name                string
		Description         string
		FrontendURL         string
		TenantID            string
		TwoFAIssuerName     string
		TwoFAEnabled        bool
		TwoFARequired       bool
		Passkey2FAEnabled   bool
		PasskeyLoginEnabled bool
		MagicLinkEnabled    bool
		OIDCEnabled         bool
		Tenants             []models.Tenant
		IsEdit              bool
		// Brute-force overrides (nil = use global default)
		BfLockoutOverride  bool
		BfLockoutEnabled   bool
		BfLockoutThreshold string
		BfLockoutDurations string
		BfLockoutWindow    string
		BfLockoutTierTTL   string
		BfDelayOverride    bool
		BfDelayEnabled     bool
		BfDelayStartAfter  string
		BfDelayMaxSeconds  string
		BfDelayTierTTL     string
		BfCaptchaOverride  bool
		BfCaptchaEnabled   bool
		BfCaptchaSiteKey   string
		BfCaptchaThreshold string
		BfCaptchaHasSecret bool
	}
	c.HTML(http.StatusOK, "app_form", formData{
		TwoFAEnabled: true, // Default: 2FA enabled for new apps
		Tenants:      tenants,
		// Brute-force defaults (override toggles stay off, but fields show defaults)
		BfLockoutEnabled:   bfDefaultLockoutEnabled,
		BfLockoutThreshold: bfDefaultLockoutThreshold,
		BfLockoutDurations: bfDefaultLockoutDurations,
		BfLockoutWindow:    bfDefaultLockoutWindow,
		BfLockoutTierTTL:   bfDefaultLockoutTierTTL,
		BfDelayEnabled:     bfDefaultDelayEnabled,
		BfDelayStartAfter:  bfDefaultDelayStartAfter,
		BfDelayMaxSeconds:  bfDefaultDelayMaxSeconds,
		BfDelayTierTTL:     bfDefaultDelayTierTTL,
		BfCaptchaEnabled:   bfDefaultCaptchaEnabled,
		BfCaptchaThreshold: bfDefaultCaptchaThreshold,
		BfCaptchaSiteKey:   bfDefaultCaptchaSiteKey,
		BfCaptchaHasSecret: true, // System default secret key is configured
	})
}

// AppCreate handles creating a new application.
// POST /gui/applications
func (h *GUIHandler) AppCreate(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	frontendURL := strings.TrimSpace(c.PostForm("frontend_url"))
	tenantID := c.PostForm("tenant_id")
	twoFAIssuerName := strings.TrimSpace(c.PostForm("two_fa_issuer_name"))
	twoFAEnabled := c.PostForm("two_fa_enabled") == "on"
	twoFARequired := c.PostForm("two_fa_required") == "on"
	passkey2FAEnabled := c.PostForm("passkey_2fa_enabled") == "on"
	passkeyLoginEnabled := c.PostForm("passkey_login_enabled") == "on"
	magicLinkEnabled := c.PostForm("magic_link_enabled") == "on"
	sms2FAEnabled := c.PostForm("sms_2fa_enabled") == "on"
	trustedDeviceEnabled := c.PostForm("trusted_device_enabled") == "on"
	trustedDeviceMaxDays := 30
	if v, err := strconv.Atoi(c.PostForm("trusted_device_max_days")); err == nil && v > 0 {
		trustedDeviceMaxDays = v
	}

	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if tenantID == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Tenant is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	parsedTenantID, err := uuid.Parse(tenantID)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid tenant ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	app := &models.Application{
		TenantID:             parsedTenantID,
		Name:                 name,
		Description:          description,
		FrontendURL:          frontendURL,
		TwoFAIssuerName:      twoFAIssuerName,
		TwoFAEnabled:         twoFAEnabled,
		TwoFARequired:        twoFARequired,
		Passkey2FAEnabled:    passkey2FAEnabled,
		PasskeyLoginEnabled:  passkeyLoginEnabled,
		MagicLinkEnabled:     magicLinkEnabled,
		SMS2FAEnabled:        sms2FAEnabled,
		TrustedDeviceEnabled: trustedDeviceEnabled,
		TrustedDeviceMaxDays: trustedDeviceMaxDays,
	}

	// Brute-force lockout overrides
	if c.PostForm("bf_lockout_override") == "on" {
		bfLockoutEnabled := c.PostForm("bf_lockout_enabled") == "on"
		app.BfLockoutEnabled = &bfLockoutEnabled
		if v, err := strconv.Atoi(c.PostForm("bf_lockout_threshold")); err == nil {
			app.BfLockoutThreshold = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_lockout_durations")); v != "" {
			app.BfLockoutDurations = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_lockout_window")); v != "" {
			app.BfLockoutWindow = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_lockout_tier_ttl")); v != "" {
			app.BfLockoutTierTTL = &v
		}
	}

	// Brute-force progressive delay overrides
	if c.PostForm("bf_delay_override") == "on" {
		bfDelayEnabled := c.PostForm("bf_delay_enabled") == "on"
		app.BfDelayEnabled = &bfDelayEnabled
		if v, err := strconv.Atoi(c.PostForm("bf_delay_start_after")); err == nil {
			app.BfDelayStartAfter = &v
		}
		if v, err := strconv.Atoi(c.PostForm("bf_delay_max_seconds")); err == nil {
			app.BfDelayMaxSeconds = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_delay_tier_ttl")); v != "" {
			app.BfDelayTierTTL = &v
		}
	}

	// Brute-force CAPTCHA overrides
	if c.PostForm("bf_captcha_override") == "on" {
		bfCaptchaEnabled := c.PostForm("bf_captcha_enabled") == "on"
		app.BfCaptchaEnabled = &bfCaptchaEnabled
		if v := strings.TrimSpace(c.PostForm("bf_captcha_site_key")); v != "" {
			app.BfCaptchaSiteKey = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_captcha_secret_key")); v != "" {
			app.BfCaptchaSecretKey = &v
		}
		if v, err := strconv.Atoi(c.PostForm("bf_captcha_threshold")); err == nil {
			app.BfCaptchaThreshold = &v
		}
	}

	if err := h.Repo.CreateApp(app); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create application. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Seed default RBAC roles for the new application (non-fatal on error)
	_ = h.Repo.SeedDefaultRolesForApp(app.ID)

	c.Header("HX-Trigger", "appListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Application created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// AppEditForm returns the pre-filled edit form HTML fragment for HTMX.
// GET /gui/applications/:id/edit
func (h *GUIHandler) AppEditForm(c *gin.Context) {
	id := c.Param("id")
	app, err := h.Repo.GetAppByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	tenants, err := h.Repo.ListAllTenants()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load tenants.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	type formData struct {
		ID                   string
		Name                 string
		Description          string
		FrontendURL          string
		TenantID             string
		TwoFAIssuerName      string
		TwoFAEnabled         bool
		TwoFARequired        bool
		Passkey2FAEnabled    bool
		PasskeyLoginEnabled  bool
		MagicLinkEnabled     bool
		OIDCEnabled          bool
		SMS2FAEnabled        bool
		TrustedDeviceEnabled bool
		TrustedDeviceMaxDays int
		Tenants              []models.Tenant
		IsEdit               bool
		// Brute-force overrides
		BfLockoutOverride  bool
		BfLockoutEnabled   bool
		BfLockoutThreshold string
		BfLockoutDurations string
		BfLockoutWindow    string
		BfLockoutTierTTL   string
		BfDelayOverride    bool
		BfDelayEnabled     bool
		BfDelayStartAfter  string
		BfDelayMaxSeconds  string
		BfDelayTierTTL     string
		BfCaptchaOverride  bool
		BfCaptchaEnabled   bool
		BfCaptchaSiteKey   string
		BfCaptchaThreshold string
		BfCaptchaHasSecret bool
	}

	fd := formData{
		ID:                   app.ID.String(),
		Name:                 app.Name,
		Description:          app.Description,
		FrontendURL:          app.FrontendURL,
		TenantID:             app.TenantID.String(),
		TwoFAIssuerName:      app.TwoFAIssuerName,
		TwoFAEnabled:         app.TwoFAEnabled,
		TwoFARequired:        app.TwoFARequired,
		Passkey2FAEnabled:    app.Passkey2FAEnabled,
		PasskeyLoginEnabled:  app.PasskeyLoginEnabled,
		MagicLinkEnabled:     app.MagicLinkEnabled,
		OIDCEnabled:          app.OIDCEnabled,
		SMS2FAEnabled:        app.SMS2FAEnabled,
		TrustedDeviceEnabled: app.TrustedDeviceEnabled,
		TrustedDeviceMaxDays: app.TrustedDeviceMaxDays,
		Tenants:              tenants,
		IsEdit:               true,
	}

	// Pre-fill brute-force defaults so fields are never blank
	fd.BfLockoutEnabled = bfDefaultLockoutEnabled
	fd.BfLockoutThreshold = bfDefaultLockoutThreshold
	fd.BfLockoutDurations = bfDefaultLockoutDurations
	fd.BfLockoutWindow = bfDefaultLockoutWindow
	fd.BfLockoutTierTTL = bfDefaultLockoutTierTTL
	fd.BfDelayEnabled = bfDefaultDelayEnabled
	fd.BfDelayStartAfter = bfDefaultDelayStartAfter
	fd.BfDelayMaxSeconds = bfDefaultDelayMaxSeconds
	fd.BfDelayTierTTL = bfDefaultDelayTierTTL
	fd.BfCaptchaEnabled = bfDefaultCaptchaEnabled
	fd.BfCaptchaThreshold = bfDefaultCaptchaThreshold
	fd.BfCaptchaSiteKey = bfDefaultCaptchaSiteKey
	fd.BfCaptchaHasSecret = true // System default secret key is configured

	// Override with saved per-app values where non-nil
	if app.BfLockoutEnabled != nil || app.BfLockoutThreshold != nil || app.BfLockoutDurations != nil || app.BfLockoutWindow != nil || app.BfLockoutTierTTL != nil {
		fd.BfLockoutOverride = true
		if app.BfLockoutEnabled != nil {
			fd.BfLockoutEnabled = *app.BfLockoutEnabled
		}
		if app.BfLockoutThreshold != nil {
			fd.BfLockoutThreshold = strconv.Itoa(*app.BfLockoutThreshold)
		}
		if app.BfLockoutDurations != nil {
			fd.BfLockoutDurations = *app.BfLockoutDurations
		}
		if app.BfLockoutWindow != nil {
			fd.BfLockoutWindow = *app.BfLockoutWindow
		}
		if app.BfLockoutTierTTL != nil {
			fd.BfLockoutTierTTL = *app.BfLockoutTierTTL
		}
	}

	// Override with saved per-app delay values where non-nil
	if app.BfDelayEnabled != nil || app.BfDelayStartAfter != nil || app.BfDelayMaxSeconds != nil || app.BfDelayTierTTL != nil {
		fd.BfDelayOverride = true
		if app.BfDelayEnabled != nil {
			fd.BfDelayEnabled = *app.BfDelayEnabled
		}
		if app.BfDelayStartAfter != nil {
			fd.BfDelayStartAfter = strconv.Itoa(*app.BfDelayStartAfter)
		}
		if app.BfDelayMaxSeconds != nil {
			fd.BfDelayMaxSeconds = strconv.Itoa(*app.BfDelayMaxSeconds)
		}
		if app.BfDelayTierTTL != nil {
			fd.BfDelayTierTTL = *app.BfDelayTierTTL
		}
	}

	// Override with saved per-app CAPTCHA values where non-nil
	if app.BfCaptchaEnabled != nil || app.BfCaptchaSiteKey != nil || app.BfCaptchaSecretKey != nil || app.BfCaptchaThreshold != nil {
		fd.BfCaptchaOverride = true
		if app.BfCaptchaEnabled != nil {
			fd.BfCaptchaEnabled = *app.BfCaptchaEnabled
		}
		if app.BfCaptchaSiteKey != nil {
			fd.BfCaptchaSiteKey = *app.BfCaptchaSiteKey
		}
		if app.BfCaptchaSecretKey != nil && *app.BfCaptchaSecretKey != "" {
			fd.BfCaptchaHasSecret = true
		}
		if app.BfCaptchaThreshold != nil {
			fd.BfCaptchaThreshold = strconv.Itoa(*app.BfCaptchaThreshold)
		}
	}

	c.HTML(http.StatusOK, "app_form", fd)
}

// AppUpdate handles updating an application.
// PUT /gui/applications/:id
func (h *GUIHandler) AppUpdate(c *gin.Context) {
	id := c.Param("id")
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	frontendURL := strings.TrimSpace(c.PostForm("frontend_url"))
	twoFAIssuerName := strings.TrimSpace(c.PostForm("two_fa_issuer_name"))
	twoFAEnabled := c.PostForm("two_fa_enabled") == "on"
	twoFARequired := c.PostForm("two_fa_required") == "on"
	passkey2FAEnabled := c.PostForm("passkey_2fa_enabled") == "on"
	passkeyLoginEnabled := c.PostForm("passkey_login_enabled") == "on"
	magicLinkEnabled := c.PostForm("magic_link_enabled") == "on"
	oidcEnabled := c.PostForm("oidc_enabled") == "on"
	sms2FAEnabled := c.PostForm("sms_2fa_enabled") == "on"
	trustedDeviceEnabled := c.PostForm("trusted_device_enabled") == "on"
	trustedDeviceMaxDays := 30
	if v, err := strconv.Atoi(c.PostForm("trusted_device_max_days")); err == nil && v > 0 {
		trustedDeviceMaxDays = v
	}

	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Build brute-force settings
	var bf BruteForceAppSettings

	// Lockout overrides: if override toggle is on, read values; otherwise nil (clear overrides)
	if c.PostForm("bf_lockout_override") == "on" {
		bfLockoutEnabled := c.PostForm("bf_lockout_enabled") == "on"
		bf.LockoutEnabled = &bfLockoutEnabled
		if v, err := strconv.Atoi(c.PostForm("bf_lockout_threshold")); err == nil {
			bf.LockoutThreshold = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_lockout_durations")); v != "" {
			bf.LockoutDurations = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_lockout_window")); v != "" {
			bf.LockoutWindow = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_lockout_tier_ttl")); v != "" {
			bf.LockoutTierTTL = &v
		}
	}

	// Delay overrides
	if c.PostForm("bf_delay_override") == "on" {
		bfDelayEnabled := c.PostForm("bf_delay_enabled") == "on"
		bf.DelayEnabled = &bfDelayEnabled
		if v, err := strconv.Atoi(c.PostForm("bf_delay_start_after")); err == nil {
			bf.DelayStartAfter = &v
		}
		if v, err := strconv.Atoi(c.PostForm("bf_delay_max_seconds")); err == nil {
			bf.DelayMaxSeconds = &v
		}
		if v := strings.TrimSpace(c.PostForm("bf_delay_tier_ttl")); v != "" {
			bf.DelayTierTTL = &v
		}
	}

	// CAPTCHA overrides
	if c.PostForm("bf_captcha_override") == "on" {
		bfCaptchaEnabled := c.PostForm("bf_captcha_enabled") == "on"
		bf.CaptchaEnabled = &bfCaptchaEnabled
		if v := strings.TrimSpace(c.PostForm("bf_captcha_site_key")); v != "" {
			bf.CaptchaSiteKey = &v
		}
		// Only update secret key if a new value was provided (non-empty)
		if v := strings.TrimSpace(c.PostForm("bf_captcha_secret_key")); v != "" {
			bf.CaptchaSecretKey = &v
		}
		// If override is on but secret_key field is empty, keep existing (don't send nil)
		if v, err := strconv.Atoi(c.PostForm("bf_captcha_threshold")); err == nil {
			bf.CaptchaThreshold = &v
		}
	}
	// If override toggles are off, all bf fields remain nil -> clears overrides in DB

	if err := h.Repo.UpdateApp(id, name, description, frontendURL, twoFAIssuerName, twoFAEnabled, twoFARequired, passkey2FAEnabled, passkeyLoginEnabled, magicLinkEnabled, oidcEnabled, bf); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update application. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Update SMS and trusted device settings
	if err := h.Repo.UpdateAppSMSTrustedDevice(id, sms2FAEnabled, trustedDeviceEnabled, trustedDeviceMaxDays); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update SMS/trusted device settings.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "appListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Application updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// AppDeleteConfirm returns the delete confirmation modal body for HTMX.
// GET /gui/applications/:id/delete
func (h *GUIHandler) AppDeleteConfirm(c *gin.Context) {
	id := c.Param("id")
	app, err := h.Repo.GetAppByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Application not found.</div></div>`)
		return
	}

	type deleteData struct {
		ID   string
		Name string
	}
	c.HTML(http.StatusOK, "app_delete_confirm", deleteData{
		ID:   app.ID.String(),
		Name: app.Name,
	})
}

// AppDelete handles deleting an application.
// DELETE /gui/applications/:id
func (h *GUIHandler) AppDelete(c *gin.Context) {
	id := c.Param("id")
	if err := h.Repo.DeleteApp(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to delete application.</div>`)
		return
	}

	// Return a refreshed application list and trigger modal close
	c.Header("HX-Trigger", "appDeleted")

	// Re-fetch and render the updated application list
	page := 1
	pageSize := 10
	apps, total, err := h.Repo.ListAppsWithDetails(page, pageSize, "")
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Application deleted but failed to refresh list.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	type appListData struct {
		Apps       []AppListItem
		Page       int
		TotalPages int
		Total      int64
		TenantID   string
	}

	c.HTML(http.StatusOK, "app_list", appListData{
		Apps:       apps,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
	})
}

// AppFormCancel returns an empty response to clear the form container.
// GET /gui/applications/form-cancel
func (h *GUIHandler) AppFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// --- OAuth Config Management ---

// OAuthPage renders the OAuth config management page.
// GET /gui/oauth
func (h *GUIHandler) OAuthPage(c *gin.Context) {
	// Load all apps with tenant names for the filter dropdown
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		apps = nil // Degrade gracefully
	}

	data := web.TemplateData{
		ActivePage:    "oauth",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
		Data:          apps,
	}
	c.HTML(http.StatusOK, "oauth", data)
}

// OAuthList returns the OAuth config table HTML fragment for HTMX.
// GET /gui/oauth/list
func (h *GUIHandler) OAuthList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 10
	appID := c.Query("app_id")

	configs, total, err := h.Repo.ListOAuthConfigsWithDetails(page, pageSize, appID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load OAuth configurations.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	type oauthListData struct {
		Configs    []OAuthConfigListItem
		Page       int
		TotalPages int
		Total      int64
		AppID      string
	}

	c.HTML(http.StatusOK, "oauth_list", oauthListData{
		Configs:    configs,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
		AppID:      appID,
	})
}

// OAuthCreateForm returns the empty create form HTML fragment for HTMX.
// GET /gui/oauth/new
func (h *GUIHandler) OAuthCreateForm(c *gin.Context) {
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load applications.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	type formData struct {
		ID          string
		AppID       string
		Provider    string
		ClientID    string
		RedirectURL string
		IsEnabled   bool
		Apps        []AppWithTenant
		IsEdit      bool
	}
	c.HTML(http.StatusOK, "oauth_form", formData{
		IsEnabled: true, // Default to enabled for new configs
		Apps:      apps,
	})
}

// OAuthCreate handles creating a new OAuth config.
// POST /gui/oauth
func (h *GUIHandler) OAuthCreate(c *gin.Context) {
	appID := c.PostForm("app_id")
	provider := strings.TrimSpace(c.PostForm("provider"))
	clientID := strings.TrimSpace(c.PostForm("client_id"))
	clientSecret := strings.TrimSpace(c.PostForm("client_secret"))
	redirectURL := strings.TrimSpace(c.PostForm("redirect_url"))
	isEnabled := c.PostForm("is_enabled") == "true"

	if appID == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if provider == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Provider is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if clientID == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Client ID is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if clientSecret == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Client Secret is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if redirectURL == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Redirect URL is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	parsedAppID, err := uuid.Parse(appID)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid application ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	config := &models.OAuthProviderConfig{
		AppID:        parsedAppID,
		Provider:     provider,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		IsEnabled:    isEnabled,
	}
	if err := h.Repo.UpsertOAuthConfig(config); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create OAuth config. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "oauthListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">OAuth configuration created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// OAuthEditForm returns the pre-filled edit form HTML fragment for HTMX.
// GET /gui/oauth/:id/edit
func (h *GUIHandler) OAuthEditForm(c *gin.Context) {
	id := c.Param("id")
	config, err := h.Repo.GetOAuthConfigByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">OAuth config not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load applications.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	type formData struct {
		ID          string
		AppID       string
		Provider    string
		ClientID    string
		RedirectURL string
		IsEnabled   bool
		Apps        []AppWithTenant
		IsEdit      bool
	}
	c.HTML(http.StatusOK, "oauth_form", formData{
		ID:          config.ID.String(),
		AppID:       config.AppID.String(),
		Provider:    config.Provider,
		ClientID:    config.ClientID,
		RedirectURL: config.RedirectURL,
		IsEnabled:   config.IsEnabled,
		Apps:        apps,
		IsEdit:      true,
	})
}

// OAuthUpdate handles updating an OAuth config.
// PUT /gui/oauth/:id
func (h *GUIHandler) OAuthUpdate(c *gin.Context) {
	id := c.Param("id")
	clientID := strings.TrimSpace(c.PostForm("client_id"))
	clientSecret := strings.TrimSpace(c.PostForm("client_secret"))
	redirectURL := strings.TrimSpace(c.PostForm("redirect_url"))
	isEnabled := c.PostForm("is_enabled") == "true"

	if clientID == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Client ID is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if redirectURL == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Redirect URL is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if err := h.Repo.UpdateOAuthConfigByID(id, clientID, clientSecret, redirectURL, isEnabled); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update OAuth config. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "oauthListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">OAuth configuration updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// OAuthDeleteConfirm returns the delete confirmation modal body for HTMX.
// GET /gui/oauth/:id/delete
func (h *GUIHandler) OAuthDeleteConfirm(c *gin.Context) {
	id := c.Param("id")
	config, err := h.Repo.GetOAuthConfigByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">OAuth config not found.</div></div>`)
		return
	}

	// Get the app name for display
	app, _ := h.Repo.GetAppByID(config.AppID.String())
	appName := ""
	if app != nil {
		appName = app.Name
	}

	type deleteData struct {
		ID       string
		Provider string
		AppName  string
	}
	c.HTML(http.StatusOK, "oauth_delete_confirm", deleteData{
		ID:       config.ID.String(),
		Provider: config.Provider,
		AppName:  appName,
	})
}

// OAuthDelete handles deleting an OAuth config.
// DELETE /gui/oauth/:id
func (h *GUIHandler) OAuthDelete(c *gin.Context) {
	id := c.Param("id")
	if err := h.Repo.DeleteOAuthConfig(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to delete OAuth config.</div>`)
		return
	}

	// Return a refreshed list and trigger modal close
	c.Header("HX-Trigger", "oauthDeleted")

	page := 1
	pageSize := 10
	configs, total, err := h.Repo.ListOAuthConfigsWithDetails(page, pageSize, "")
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">OAuth config deleted but failed to refresh list.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	type oauthListData struct {
		Configs    []OAuthConfigListItem
		Page       int
		TotalPages int
		Total      int64
		AppID      string
	}

	c.HTML(http.StatusOK, "oauth_list", oauthListData{
		Configs:    configs,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
	})
}

// OAuthFormCancel returns an empty response to clear the form container.
// GET /gui/oauth/form-cancel
func (h *GUIHandler) OAuthFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// OAuthToggleEnabled toggles the IsEnabled flag for an OAuth config.
// PUT /gui/oauth/:id/toggle
func (h *GUIHandler) OAuthToggleEnabled(c *gin.Context) {
	id := c.Param("id")
	config, err := h.Repo.ToggleOAuthConfigEnabled(id)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<span class="badge bg-warning bg-opacity-10 text-warning">Error</span>`)
		return
	}

	// Return the updated toggle HTML fragment
	if config.IsEnabled {
		c.String(http.StatusOK,
			`<div id="toggle-`+id+`" hx-put="/gui/oauth/`+id+`/toggle" hx-target="#toggle-`+id+`" hx-swap="outerHTML" style="cursor: pointer;"><span class="badge bg-success bg-opacity-10 text-success"><i class="bi bi-check-circle-fill me-1"></i>On</span></div>`)
	} else {
		c.String(http.StatusOK,
			`<div id="toggle-`+id+`" hx-put="/gui/oauth/`+id+`/toggle" hx-target="#toggle-`+id+`" hx-swap="outerHTML" style="cursor: pointer;"><span class="badge bg-danger bg-opacity-10 text-danger"><i class="bi bi-x-circle-fill me-1"></i>Off</span></div>`)
	}
}

// --- Helpers ---

// getAdminUsername reads the admin username from the Gin context (set by GUIAuthMiddleware)
func getAdminUsername(c *gin.Context) string {
	if val, exists := c.Get(web.GUIAdminUsernameKey); exists {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// getAdminID reads the admin ID from the Gin context (set by GUIAuthMiddleware)
func getAdminID(c *gin.Context) string {
	if val, exists := c.Get(web.GUIAdminIDKey); exists {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// ============================================================
// User Management Handlers
// ============================================================

// UserPage renders the user management page with app filter dropdown
func (h *GUIHandler) UserPage(c *gin.Context) {
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "users", gin.H{
			"ActivePage": "users",
			"AdminUser":  getAdminUsername(c),
			"CSRFToken":  getCSRFToken(c),
			"Error":      "Failed to load applications",
		})
		return
	}

	c.HTML(http.StatusOK, "users", gin.H{
		"ActivePage": "users",
		"AdminUser":  getAdminUsername(c),
		"CSRFToken":  getCSRFToken(c),
		"Data":       apps,
	})
}

// UserList returns the paginated user list partial (HTMX fragment)
func (h *GUIHandler) UserList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 15

	appID := c.Query("app_id")
	search := c.Query("search")

	users, total, err := h.Repo.ListUsersWithDetails(page, pageSize, appID, search)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "user_list", gin.H{
			"Users": nil,
			"Error": "Failed to load users",
		})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.HTML(http.StatusOK, "user_list", gin.H{
		"Users":      users,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"AppID":      appID,
		"Search":     search,
	})
}

// UserDetail returns the user detail partial (HTMX fragment)
func (h *GUIHandler) UserDetail(c *gin.Context) {
	id := c.Param("id")

	detail, err := h.Repo.GetUserDetailByID(id)
	if err != nil {
		c.HTML(http.StatusNotFound, "user_detail", gin.H{
			"Error": "User not found",
		})
		return
	}

	// Load trusted devices if the feature is enabled
	if h.TrustedDeviceRepo != nil {
		userUUID, parseErr := uuid.Parse(id)
		if parseErr == nil {
			devices, devErr := h.TrustedDeviceRepo.FindAllForUser(userUUID)
			if devErr == nil {
				detail.TrustedDevices = devices
			}
		}
	}

	c.HTML(http.StatusOK, "user_detail", detail)
}

// UserRevokeTrustedDevice revokes a single trusted device for a user (admin action).
// DELETE /gui/users/:id/trusted-devices/:device_id
func (h *GUIHandler) UserRevokeTrustedDevice(c *gin.Context) {
	deviceIDStr := c.Param("device_id")
	if h.TrustedDeviceRepo == nil {
		c.String(http.StatusServiceUnavailable, "Trusted device feature is disabled.")
		return
	}
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid device ID.")
		return
	}
	if err := h.TrustedDeviceRepo.DeleteByID(deviceID); err != nil {
		c.String(http.StatusInternalServerError, "Failed to revoke trusted device.")
		return
	}
	c.String(http.StatusOK, `<span class="badge bg-success bg-opacity-10 text-success">Revoked</span>`)
}

// UserRevokeAllTrustedDevices revokes all trusted devices for a user across all apps (admin action).
// DELETE /gui/users/:id/trusted-devices
func (h *GUIHandler) UserRevokeAllTrustedDevices(c *gin.Context) {
	id := c.Param("id")
	if h.TrustedDeviceRepo == nil {
		c.String(http.StatusServiceUnavailable, "Trusted device feature is disabled.")
		return
	}
	userUUID, err := uuid.Parse(id)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid user ID.")
		return
	}
	// Fetch all devices across all apps and delete them one by one
	devices, err := h.TrustedDeviceRepo.FindAllForUser(userUUID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load trusted devices.")
		return
	}
	for _, d := range devices {
		_ = h.TrustedDeviceRepo.DeleteByID(d.ID)
	}
	c.Header("HX-Trigger", "trustedDevicesRevoked")
	c.String(http.StatusOK,
		`<div class="alert alert-success py-2 small">All trusted devices revoked successfully.</div>`)
}

// UserToggleActive toggles a user's IsActive flag and revokes tokens on deactivation (HTMX fragment)
func (h *GUIHandler) UserToggleActive(c *gin.Context) {
	id := c.Param("id")

	newActive, appID, err := h.Repo.ToggleUserActive(id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to toggle user status")
		return
	}

	// If user was deactivated, revoke all their tokens immediately
	if !newActive {
		// Blacklist all tokens for this user for 30 days
		maxTokenLifetime := 30 * 24 * time.Hour
		if rErr := redis.BlacklistAllUserTokens(appID, id, maxTokenLifetime); rErr != nil {
			// Log but don't fail the toggle
			fmt.Printf("Warning: Failed to blacklist tokens for deactivated user %s: %v\n", id, rErr)
		}
		// Revoke their current refresh token
		currentRefreshToken, rErr := redis.GetRefreshToken(appID, id)
		if rErr == nil && currentRefreshToken != "" {
			if rErr := redis.RevokeRefreshToken(appID, id, currentRefreshToken); rErr != nil {
				fmt.Printf("Warning: Failed to revoke refresh token for deactivated user %s: %v\n", id, rErr)
			}
		}
	}

	// Return the toggle badge HTML fragment.
	// HTMX outerHTML swap with hx-target="this" replaces whichever element was clicked.
	confirmMsg := "Reactivate this user?"
	toggleLabel := "Inactive"
	if newActive {
		confirmMsg = "Deactivate this user? Their sessions will be revoked immediately."
		toggleLabel = "Active"
	}

	// The HX-Trigger response header refreshes the list so both views stay in sync
	c.Header("HX-Trigger", "userListRefresh")

	if newActive {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(
			`<div hx-put="/gui/users/`+id+`/toggle"`+
				` hx-target="this"`+
				` hx-swap="outerHTML"`+
				` hx-confirm="`+confirmMsg+`"`+
				` style="cursor: pointer;"`+
				` title="Click to deactivate">`+
				`<span class="badge bg-success bg-opacity-10 text-success"><i class="bi bi-check-circle-fill me-1"></i>`+toggleLabel+`</span>`+
				`</div>`))
	} else {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(
			`<div hx-put="/gui/users/`+id+`/toggle"`+
				` hx-target="this"`+
				` hx-swap="outerHTML"`+
				` hx-confirm="`+confirmMsg+`"`+
				` style="cursor: pointer;"`+
				` title="Click to activate">`+
				`<span class="badge bg-danger bg-opacity-10 text-danger"><i class="bi bi-x-circle-fill me-1"></i>`+toggleLabel+`</span>`+
				`</div>`))
	}
}

// UserUnlock unlocks a locked user account (HTMX fragment).
// Clears DB lockout fields and resets all Redis brute-force counters.
// PUT /gui/users/:id/unlock
func (h *GUIHandler) UserUnlock(c *gin.Context) {
	id := c.Param("id")

	userEmail, appIDStr, err := h.Repo.UnlockUser(id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to unlock user account")
		return
	}

	// Reset Redis brute-force counters (lockout tier, delay tier, failure counter)
	if h.BruteForceService != nil {
		appID, parseErr := uuid.Parse(appIDStr)
		if parseErr == nil {
			userID, userParseErr := uuid.Parse(id)
			if userParseErr == nil {
				_ = h.BruteForceService.UnlockAccount(appID, userID, userEmail)
			}
		}
	}

	// Log the unlock event
	appID, parseErr := uuid.Parse(appIDStr)
	if parseErr == nil {
		adminUser := getAdminUsername(c)
		logService.LogAccountUnlocked(appID, uuid.Nil, "", "", map[string]interface{}{
			"email":         userEmail,
			"unlocked_by":   adminUser,
			"unlock_method": "admin_gui",
		})
	}

	// Return an inline success message — the HX-Trigger will refresh the detail view
	c.Header("HX-Trigger", "userDetailRefresh")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(
		`<span class="text-success"><i class="bi bi-unlock-fill me-1"></i>Account unlocked</span>`))
}

// ============================================================
// Activity Log Viewer
// ============================================================

// LogsPage renders the activity logs viewer page.
// GET /gui/logs
func (h *GUIHandler) LogsPage(c *gin.Context) {
	// Load filter dropdown data
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "activity_logs", gin.H{
			"ActivePage": "logs",
			"AdminUser":  getAdminUsername(c),
			"CSRFToken":  getCSRFToken(c),
			"Error":      "Failed to load applications",
		})
		return
	}

	eventTypes, err := h.Repo.ListDistinctEventTypes()
	if err != nil {
		eventTypes = []string{} // Non-critical, proceed with empty list
	}

	severities, err := h.Repo.ListDistinctSeverities()
	if err != nil {
		severities = []string{} // Non-critical, proceed with empty list
	}

	c.HTML(http.StatusOK, "activity_logs", gin.H{
		"ActivePage": "logs",
		"AdminUser":  getAdminUsername(c),
		"CSRFToken":  getCSRFToken(c),
		"Apps":       apps,
		"EventTypes": eventTypes,
		"Severities": severities,
	})
}

// LogList returns the paginated activity log list partial (HTMX fragment).
// GET /gui/logs/list
func (h *GUIHandler) LogList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 20

	eventType := c.Query("event_type")
	severity := c.Query("severity")
	appID := c.Query("app_id")
	search := c.Query("search")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	logs, total, err := h.Repo.ListActivityLogs(page, pageSize, eventType, severity, appID, search, startDate, endDate)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "activity_log_list", gin.H{
			"Logs":  nil,
			"Error": "Failed to load activity logs",
		})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.HTML(http.StatusOK, "activity_log_list", gin.H{
		"Logs":       logs,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"EventType":  eventType,
		"Severity":   severity,
		"AppID":      appID,
		"Search":     search,
		"StartDate":  startDate,
		"EndDate":    endDate,
	})
}

// LogDetail returns the activity log detail partial (HTMX fragment).
// GET /gui/logs/:id
func (h *GUIHandler) LogDetail(c *gin.Context) {
	id := c.Param("id")

	detail, err := h.Repo.GetActivityLogDetail(id)
	if err != nil {
		c.HTML(http.StatusNotFound, "activity_log_detail", gin.H{
			"Error": "Activity log not found",
		})
		return
	}

	c.HTML(http.StatusOK, "activity_log_detail", detail)
}

// LogExport streams activity logs as a downloadable CSV or JSON file.
// GET /gui/logs/export
func (h *GUIHandler) LogExport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	if format != "csv" && format != "json" {
		format = "csv"
	}

	eventType := c.Query("event_type")
	severity := c.Query("severity")
	appID := c.Query("app_id")
	search := c.Query("search")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	items, truncated, err := h.Repo.ExportActivityLogs(eventType, severity, appID, search, startDate, endDate)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to export activity logs")
		return
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	truncatedVal := "false"
	if truncated {
		truncatedVal = "true"
	}
	c.Header("X-Export-Truncated", truncatedVal)

	switch format {
	case "json":
		filename := fmt.Sprintf("activity_logs_%s.json", timestamp)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Writer.WriteHeader(http.StatusOK)

		type jsonExport struct {
			Data       []ActivityLogExportItem `json:"data"`
			Count      int                     `json:"count"`
			Truncated  bool                    `json:"truncated"`
			ExportedAt string                  `json:"exported_at"`
		}
		enc := json.NewEncoder(c.Writer)
		enc.SetIndent("", "  ")
		_ = enc.Encode(jsonExport{
			Data:       items,
			Count:      len(items),
			Truncated:  truncated,
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
		})

	default: // csv
		filename := fmt.Sprintf("activity_logs_%s.csv", timestamp)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Writer.WriteHeader(http.StatusOK)
		// Write UTF-8 BOM for Excel compatibility
		_, _ = c.Writer.Write([]byte("\xef\xbb\xbf"))
		writeAdminCSV(c.Writer, items)
	}
}

// writeAdminCSV encodes a slice of ActivityLogExportItem as CSV into w.
func writeAdminCSV(w io.Writer, items []ActivityLogExportItem) {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"id", "app_id", "app_name",
		"user_id", "user_email",
		"event_type", "severity",
		"timestamp", "ip_address", "user_agent",
		"is_anomaly",
	})
	for _, item := range items {
		_ = cw.Write([]string{
			item.ID.String(),
			item.AppID.String(),
			item.AppName,
			item.UserID.String(),
			item.UserEmail,
			item.EventType,
			item.Severity,
			item.Timestamp.UTC().Format(time.RFC3339),
			item.IPAddress,
			item.UserAgent,
			fmt.Sprintf("%t", item.IsAnomaly),
		})
	}
	cw.Flush()
}

// getCSRFToken reads the CSRF token from the Gin context (set by CSRFMiddleware)
func getCSRFToken(c *gin.Context) string {
	if val, exists := c.Get(web.CSRFTokenKey); exists {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// sessionMaxAgeSeconds returns the session cookie max age in seconds
func sessionMaxAgeSeconds() int {
	hours := viper.GetInt("ADMIN_SESSION_EXPIRATION_HOURS")
	if hours <= 0 {
		hours = 8
	}
	return hours * 3600
}

// ============================================================
// API Key Management Handlers
// ============================================================

// ApiKeysPage renders the API Keys management page.
// GET /gui/api-keys
func (h *GUIHandler) ApiKeysPage(c *gin.Context) {
	c.HTML(http.StatusOK, "api_keys", gin.H{
		"ActivePage":    "api-keys",
		"AdminUsername": getAdminUsername(c),
		"CSRFToken":     getCSRFToken(c),
	})
}

// ApiKeyList returns the paginated API key list partial (HTMX fragment).
// GET /gui/api-keys/list
func (h *GUIHandler) ApiKeyList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 20

	keyType := c.Query("key_type")

	keys, total, err := h.Repo.ListApiKeys(page, pageSize, keyType)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "api_key_list", gin.H{
			"Keys":  nil,
			"Error": "Failed to load API keys",
		})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.HTML(http.StatusOK, "api_key_list", gin.H{
		"Keys":       keys,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"KeyType":    keyType,
	})
}

// ApiKeyCreateForm returns the API key creation form HTML fragment.
// GET /gui/api-keys/new
func (h *GUIHandler) ApiKeyCreateForm(c *gin.Context) {
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load applications.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.HTML(http.StatusOK, "api_key_form", gin.H{
		"Apps": apps,
	})
}

// ApiKeyCreate handles creating a new API key.
// POST /gui/api-keys
func (h *GUIHandler) ApiKeyCreate(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	keyType := strings.TrimSpace(c.PostForm("key_type"))
	description := strings.TrimSpace(c.PostForm("description"))
	scopes := strings.TrimSpace(c.PostForm("scopes"))
	appIDStr := strings.TrimSpace(c.PostForm("app_id"))
	expiresAtStr := strings.TrimSpace(c.PostForm("expires_at"))

	// Validate required fields
	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	if keyType != KeyTypeAdmin && keyType != KeyTypeApp {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid key type. Must be "admin" or "app".<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// App keys require an application ID
	var appID *uuid.UUID
	var appName string
	if keyType == KeyTypeApp {
		if appIDStr == "" {
			c.String(http.StatusBadRequest,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application is required for app keys.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
		parsedID, err := uuid.Parse(appIDStr)
		if err != nil {
			c.String(http.StatusBadRequest,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid application ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
		appID = &parsedID

		// Look up app name for display in the "created" response
		app, err := h.Repo.GetAppByID(appIDStr)
		if err != nil {
			c.String(http.StatusBadRequest,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
		appName = app.Name
	}

	// Parse optional expiration
	var expiresAt *time.Time
	var expiresAtDisplay string
	if expiresAtStr != "" {
		t, err := time.Parse("2006-01-02T15:04", expiresAtStr)
		if err != nil {
			c.String(http.StatusBadRequest,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid expiration date format.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
		if t.Before(time.Now()) {
			c.String(http.StatusBadRequest,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Expiration date must be in the future.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
		expiresAt = &t
		expiresAtDisplay = t.Format("Jan 02, 2006 15:04")
	}

	// Generate the key
	rawKey, keyHash, keyPrefix, keySuffix, err := GenerateApiKey(keyType)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to generate API key. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Create the DB record
	apiKey := &models.ApiKey{
		KeyType:     keyType,
		Name:        name,
		Description: description,
		Scopes:      scopes,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		KeySuffix:   keySuffix,
		AppID:       appID,
		ExpiresAt:   expiresAt,
	}
	if err := h.Repo.CreateApiKey(apiKey); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create API key. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Clear the form and trigger list refresh
	c.Header("HX-Trigger", "apiKeyListRefresh")

	// Render the "key created" partial with the raw key (shown once)
	c.HTML(http.StatusOK, "api_key_created", gin.H{
		"RawKey":    rawKey,
		"Name":      name,
		"KeyType":   keyType,
		"Scopes":    scopes,
		"AppName":   appName,
		"ExpiresAt": expiresAtDisplay,
	})
}

// ApiKeyRevokeConfirm returns the revoke confirmation modal body.
// GET /gui/api-keys/:id/revoke
func (h *GUIHandler) ApiKeyRevokeConfirm(c *gin.Context) {
	id := c.Param("id")
	apiKey, err := h.Repo.GetApiKeyByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><p class="text-danger">API key not found.</p></div>`)
		return
	}

	c.HTML(http.StatusOK, "api_key_revoke_confirm", gin.H{
		"ID":        apiKey.ID,
		"Name":      apiKey.Name,
		"KeyType":   apiKey.KeyType,
		"KeyPrefix": apiKey.KeyPrefix,
		"KeySuffix": apiKey.KeySuffix,
	})
}

// ApiKeyRevoke handles revoking an API key.
// PUT /gui/api-keys/:id/revoke
func (h *GUIHandler) ApiKeyRevoke(c *gin.Context) {
	id := c.Param("id")
	if err := h.Repo.RevokeApiKey(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to revoke API key.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "apiKeyRevoked")

	// Re-render the list to show the updated state
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 20
	keyType := c.Query("key_type")

	keys, total, err := h.Repo.ListApiKeys(page, pageSize, keyType)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to refresh list.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.HTML(http.StatusOK, "api_key_list", gin.H{
		"Keys":       keys,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"KeyType":    keyType,
	})
}

// ApiKeyDeleteConfirm returns the delete confirmation modal body.
// GET /gui/api-keys/:id/delete
func (h *GUIHandler) ApiKeyDeleteConfirm(c *gin.Context) {
	id := c.Param("id")
	apiKey, err := h.Repo.GetApiKeyByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><p class="text-danger">API key not found.</p></div>`)
		return
	}

	c.HTML(http.StatusOK, "api_key_delete_confirm", gin.H{
		"ID":        apiKey.ID,
		"Name":      apiKey.Name,
		"KeyType":   apiKey.KeyType,
		"KeyPrefix": apiKey.KeyPrefix,
		"KeySuffix": apiKey.KeySuffix,
		"IsRevoked": apiKey.IsRevoked,
	})
}

// ApiKeyDelete handles permanently deleting an API key.
// DELETE /gui/api-keys/:id
func (h *GUIHandler) ApiKeyDelete(c *gin.Context) {
	id := c.Param("id")
	if err := h.Repo.DeleteApiKey(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to delete API key.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "apiKeyDeleted")

	// Re-render the list
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 20
	keyType := c.Query("key_type")

	keys, total, err := h.Repo.ListApiKeys(page, pageSize, keyType)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to refresh list.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.HTML(http.StatusOK, "api_key_list", gin.H{
		"Keys":       keys,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"KeyType":    keyType,
	})
}

// ApiKeyFormCancel clears the API key form.
// GET /gui/api-keys/form-cancel
func (h *GUIHandler) ApiKeyFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// ApiKeyEditForm returns the edit form partial for an existing API key.
// GET /gui/api-keys/:id/edit
func (h *GUIHandler) ApiKeyEditForm(c *gin.Context) {
	id := c.Param("id")
	apiKey, err := h.Repo.GetApiKeyByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">API key not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.HTML(http.StatusOK, "api_key_edit_form", gin.H{
		"ID":          apiKey.ID,
		"Name":        apiKey.Name,
		"Description": apiKey.Description,
		"Scopes":      apiKey.Scopes,
		"KeyType":     apiKey.KeyType,
		"KeyPrefix":   apiKey.KeyPrefix,
		"KeySuffix":   apiKey.KeySuffix,
	})
}

// ApiKeyUpdate handles updating name, description, and scopes for an existing API key.
// PUT /gui/api-keys/:id
func (h *GUIHandler) ApiKeyUpdate(c *gin.Context) {
	id := c.Param("id")
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	scopes := strings.TrimSpace(c.PostForm("scopes"))

	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if err := h.Repo.UpdateApiKeyScopes(id, name, description, scopes); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update API key.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "apiKeyListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">API key updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// ApiKeyUsagePage renders the full usage analytics page for a single API key.
// GET /gui/api-keys/:id/usage
func (h *GUIHandler) ApiKeyUsagePage(c *gin.Context) {
	id := c.Param("id")

	parsedID, err := uuid.Parse(id)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error", gin.H{"Error": "Invalid API key ID"})
		return
	}

	apiKey, err := h.Repo.GetApiKeyByID(id)
	if err != nil {
		c.HTML(http.StatusNotFound, "error", gin.H{"Error": "API key not found"})
		return
	}

	const days = 30
	points, err := h.Repo.GetApiKeyUsageSummary(parsedID, days)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"Error": "Failed to load usage data"})
		return
	}

	total, err := h.Repo.GetApiKeyTotalUsage(parsedID)
	if err != nil {
		total = 0
	}

	// Build label/count slices for Chart.js
	labels := make([]string, len(points))
	counts := make([]int64, len(points))
	for i, p := range points {
		labels[i] = p.PeriodDate.Format("Jan 02")
		counts[i] = p.RequestCount
	}

	c.HTML(http.StatusOK, "api_key_usage", gin.H{
		"ActivePage":    "api-keys",
		"AdminUsername": getAdminUsername(c),
		"CSRFToken":     getCSRFToken(c),
		"ApiKey":        apiKey,
		"Days":          days,
		"Labels":        labels,
		"Counts":        counts,
		"TotalRequests": total,
	})
}

// SettingsPage renders the system settings page with accordion categories.
// GET /gui/settings
func (h *GUIHandler) SettingsPage(c *gin.Context) {
	categories, err := h.SettingsService.ResolveAllByCategory()
	if err != nil {
		data := web.TemplateData{
			ActivePage:    "settings",
			AdminUsername: c.GetString(web.GUIAdminUsernameKey),
			CSRFToken:     c.GetString(web.CSRFTokenKey),
			FlashError:    "Failed to load settings: " + err.Error(),
		}
		c.HTML(http.StatusInternalServerError, "settings", data)
		return
	}

	data := web.TemplateData{
		ActivePage:    "settings",
		AdminUsername: c.GetString(web.GUIAdminUsernameKey),
		CSRFToken:     c.GetString(web.CSRFTokenKey),
		Data:          categories,
	}
	c.HTML(http.StatusOK, "settings", data)
}

// SettingsInfo returns the system info partial.
// GET /gui/settings/info
func (h *GUIHandler) SettingsInfo(c *gin.Context) {
	info := h.SettingsService.GetSystemInfo()
	c.HTML(http.StatusOK, "settings_info", info)
}

// SettingsSection returns the settings rows for a single category.
// GET /gui/settings/section/:category
func (h *GUIHandler) SettingsSection(c *gin.Context) {
	categorySlug := c.Param("category")
	category, err := h.SettingsService.ResolveCategorySettings(categorySlug)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger m-3 small">Failed to load settings: %s</div>`, err.Error())
		return
	}
	c.HTML(http.StatusOK, "settings_section", category)
}

// SettingUpdate saves a new value for a single setting.
// PUT /gui/settings/:key
func (h *GUIHandler) SettingUpdate(c *gin.Context) {
	key := c.Param("key")
	value := c.PostForm("value")

	// Validate the setting key exists
	def := GetSettingDefinition(key)
	if def == nil {
		c.Header("HX-Trigger", `{"settingError": {"message": "Unknown setting key."}}`)
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger small py-2 mb-0">Unknown setting key.</div>`)
		return
	}

	// Check if this setting is env-sourced (read-only)
	if getEnvValue(def.EnvVar) != "" {
		c.Header("HX-Trigger", `{"settingError": {"message": "Cannot override environment variable."}}`)
		c.String(http.StatusForbidden,
			`<div class="alert alert-warning small py-2 mb-0">Cannot override a setting controlled by environment variable.</div>`)
		return
	}

	// Save
	if err := h.SettingsService.UpdateSetting(key, value); err != nil {
		c.Header("HX-Trigger", fmt.Sprintf(`{"settingError": {"message": "%s"}}`, err.Error()))
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger small py-2 mb-0">%s</div>`, err.Error())
		return
	}

	// Re-resolve and return the updated row
	category, err := h.SettingsService.ResolveCategorySettings(def.Category)
	if err != nil {
		c.Header("HX-Trigger", "settingSaved")
		c.String(http.StatusOK, `<div class="alert alert-success small py-2 mb-0">Setting saved.</div>`)
		return
	}

	// Find the specific setting in the resolved list
	for _, s := range category.Settings {
		if s.Definition.Key == key {
			c.Header("HX-Trigger", "settingSaved")
			c.HTML(http.StatusOK, "settings_row", s)
			return
		}
	}

	// Fallback
	c.Header("HX-Trigger", "settingSaved")
	c.String(http.StatusOK, `<div class="alert alert-success small py-2 mb-0">Setting saved.</div>`)
}

// SettingReset removes the DB override for a setting (reverts to env/default).
// DELETE /gui/settings/:key
func (h *GUIHandler) SettingReset(c *gin.Context) {
	key := c.Param("key")

	def := GetSettingDefinition(key)
	if def == nil {
		c.Header("HX-Trigger", `{"settingError": {"message": "Unknown setting key."}}`)
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger small py-2 mb-0">Unknown setting key.</div>`)
		return
	}

	if err := h.SettingsService.ResetSetting(key); err != nil {
		c.Header("HX-Trigger", fmt.Sprintf(`{"settingError": {"message": "%s"}}`, err.Error()))
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger small py-2 mb-0">Failed to reset setting.</div>`)
		return
	}

	// Re-resolve and return the updated row
	category, err := h.SettingsService.ResolveCategorySettings(def.Category)
	if err != nil {
		c.Header("HX-Trigger", "settingReset")
		c.String(http.StatusOK, `<div class="alert alert-info small py-2 mb-0">Setting reset to default.</div>`)
		return
	}

	for _, s := range category.Settings {
		if s.Definition.Key == key {
			c.Header("HX-Trigger", "settingReset")
			c.HTML(http.StatusOK, "settings_row", s)
			return
		}
	}

	c.Header("HX-Trigger", "settingReset")
	c.String(http.StatusOK, `<div class="alert alert-info small py-2 mb-0">Setting reset to default.</div>`)
}

// ============================================================
// Email Server Config (SMTP) GUI Handlers
// ============================================================

// EmailServersPage renders the email server config management page.
// GET /gui/email-servers
func (h *GUIHandler) EmailServersPage(c *gin.Context) {
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		apps = nil
	}

	data := web.TemplateData{
		ActivePage:    "email-servers",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
		Data:          apps,
	}
	c.HTML(http.StatusOK, "email_servers", data)
}

// EmailServerList returns the email server config list partial (HTMX fragment).
// GET /gui/email-servers/list
func (h *GUIHandler) EmailServerList(c *gin.Context) {
	allConfigs, err := h.EmailService.GetAllServerConfigs()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load data.</div>`)
		return
	}

	// Build a map of app ID -> app info for display
	apps, _ := h.Repo.ListAllAppsWithTenantName()
	appMap := make(map[string]AppWithTenant)
	for _, app := range apps {
		appMap[app.ID.String()] = app
	}

	type serverItem struct {
		ID          string
		AppID       string
		AppName     string
		TenantName  string
		Name        string
		SMTPHost    string
		SMTPPort    int
		FromAddress string
		FromName    string
		UseTLS      bool
		IsDefault   bool
		IsActive    bool
		IsGlobal    bool
	}

	var items []serverItem
	for _, config := range allConfigs {
		appName := ""
		tenantName := ""
		appIDStr := ""
		isGlobal := config.AppID == nil
		if !isGlobal {
			appIDStr = config.AppID.String()
			if app, ok := appMap[appIDStr]; ok {
				appName = app.Name
				tenantName = app.TenantName
			}
		}
		items = append(items, serverItem{
			ID:          config.ID.String(),
			AppID:       appIDStr,
			AppName:     appName,
			TenantName:  tenantName,
			Name:        config.Name,
			SMTPHost:    config.SMTPHost,
			SMTPPort:    config.SMTPPort,
			FromAddress: config.FromAddress,
			FromName:    config.FromName,
			UseTLS:      config.UseTLS,
			IsDefault:   config.IsDefault,
			IsActive:    config.IsActive,
			IsGlobal:    isGlobal,
		})
	}

	c.HTML(http.StatusOK, "email_server_list", gin.H{
		"Configs": items,
	})
}

// EmailServerCreateForm returns the empty create form for email server config.
// GET /gui/email-servers/new
func (h *GUIHandler) EmailServerCreateForm(c *gin.Context) {
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		apps = nil // Non-fatal: global config can still be created without apps
	}

	c.HTML(http.StatusOK, "email_server_form", gin.H{
		"IsEdit":    false,
		"Apps":      apps,
		"Name":      "Default",
		"SMTPPort":  587,
		"UseTLS":    true,
		"IsDefault": true,
		"IsActive":  true,
	})
}

// EmailServerCreate handles creating a new email server config.
// POST /gui/email-servers
func (h *GUIHandler) EmailServerCreate(c *gin.Context) {
	appIDStr := c.PostForm("app_id")
	name := strings.TrimSpace(c.PostForm("name"))
	smtpHost := strings.TrimSpace(c.PostForm("smtp_host"))
	smtpPortStr := c.PostForm("smtp_port")
	smtpUsername := strings.TrimSpace(c.PostForm("smtp_username"))
	smtpPassword := c.PostForm("smtp_password")
	fromAddress := strings.TrimSpace(c.PostForm("from_address"))
	fromName := strings.TrimSpace(c.PostForm("from_name"))
	useTLS := c.PostForm("use_tls") == "true"
	isDefault := c.PostForm("is_default") == "true"
	isActive := c.PostForm("is_active") == "true"

	if smtpHost == "" || fromAddress == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">SMTP Host and From Address are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// app_id is optional: empty = global/system-level config
	var appIDPtr *uuid.UUID
	if appIDStr != "" {
		appID, err := uuid.Parse(appIDStr)
		if err != nil {
			c.String(http.StatusBadRequest,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid application ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
		appIDPtr = &appID
	}

	smtpPort, _ := strconv.Atoi(smtpPortStr)
	if smtpPort <= 0 {
		smtpPort = 587
	}

	if name == "" {
		name = "Default"
	}

	config := &models.EmailServerConfig{
		AppID:        appIDPtr,
		Name:         name,
		SMTPHost:     smtpHost,
		SMTPPort:     smtpPort,
		SMTPUsername: smtpUsername,
		SMTPPassword: smtpPassword,
		FromAddress:  fromAddress,
		FromName:     fromName,
		UseTLS:       useTLS,
		IsDefault:    isDefault,
		IsActive:     isActive,
	}

	if err := h.EmailService.SaveServerConfig(config); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to save SMTP config. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "emailServerListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">SMTP configuration created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// EmailServerEditForm returns the pre-filled edit form for an email server config.
// GET /gui/email-servers/:id/edit
func (h *GUIHandler) EmailServerEditForm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	found, err := h.EmailService.GetServerConfigByID(id)
	if err != nil || found == nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">SMTP config not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	apps, _ := h.Repo.ListAllAppsWithTenantName()

	appIDStr := ""
	if found.AppID != nil {
		appIDStr = found.AppID.String()
	}

	c.HTML(http.StatusOK, "email_server_form", gin.H{
		"IsEdit":       true,
		"ID":           found.ID.String(),
		"AppID":        appIDStr,
		"Name":         found.Name,
		"SMTPHost":     found.SMTPHost,
		"SMTPPort":     found.SMTPPort,
		"SMTPUsername": found.SMTPUsername,
		"FromAddress":  found.FromAddress,
		"FromName":     found.FromName,
		"UseTLS":       found.UseTLS,
		"IsDefault":    found.IsDefault,
		"IsActive":     found.IsActive,
		"Apps":         apps,
	})
}

// EmailServerUpdate handles updating an email server config.
// PUT /gui/email-servers/:id
func (h *GUIHandler) EmailServerUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid config ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Get existing config to preserve password if not provided
	existing, err := h.EmailService.GetServerConfigByID(id)
	if err != nil || existing == nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">SMTP config not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	appIDStr := c.PostForm("app_id")
	name := strings.TrimSpace(c.PostForm("name"))
	smtpHost := strings.TrimSpace(c.PostForm("smtp_host"))
	smtpPortStr := c.PostForm("smtp_port")
	smtpUsername := strings.TrimSpace(c.PostForm("smtp_username"))
	smtpPassword := c.PostForm("smtp_password")
	fromAddress := strings.TrimSpace(c.PostForm("from_address"))
	fromName := strings.TrimSpace(c.PostForm("from_name"))
	useTLS := c.PostForm("use_tls") == "true"
	isDefault := c.PostForm("is_default") == "true"
	isActive := c.PostForm("is_active") == "true"

	if smtpHost == "" || fromAddress == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">SMTP Host and From Address are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// app_id is optional: empty = global/system-level config
	var appIDPtr *uuid.UUID
	if appIDStr != "" {
		appID, err := uuid.Parse(appIDStr)
		if err != nil {
			c.String(http.StatusBadRequest,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid application ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
		appIDPtr = &appID
	}

	smtpPort, _ := strconv.Atoi(smtpPortStr)
	if smtpPort <= 0 {
		smtpPort = 587
	}

	if name == "" {
		name = "Default"
	}

	// Keep existing password if not provided
	if smtpPassword == "" {
		smtpPassword = existing.SMTPPassword
	}

	config := &models.EmailServerConfig{
		AppID:        appIDPtr,
		Name:         name,
		SMTPHost:     smtpHost,
		SMTPPort:     smtpPort,
		SMTPUsername: smtpUsername,
		SMTPPassword: smtpPassword,
		FromAddress:  fromAddress,
		FromName:     fromName,
		UseTLS:       useTLS,
		IsDefault:    isDefault,
		IsActive:     isActive,
	}
	config.ID = id

	if err := h.EmailService.SaveServerConfig(config); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update SMTP config.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "emailServerListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">SMTP configuration updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// EmailServerDeleteConfirm returns the delete confirmation modal body.
// GET /gui/email-servers/:id/delete
func (h *GUIHandler) EmailServerDeleteConfirm(c *gin.Context) {
	idStr := c.Param("id")
	appName := c.Query("app_name")
	configName := c.Query("config_name")

	c.HTML(http.StatusOK, "email_server_delete_confirm", gin.H{
		"ID":         idStr,
		"AppName":    appName,
		"ConfigName": configName,
	})
}

// EmailServerDelete handles deleting an email server config.
// DELETE /gui/email-servers/:id
func (h *GUIHandler) EmailServerDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger">Invalid config ID.</div>`)
		return
	}

	if err := h.EmailService.DeleteServerConfigByID(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to delete SMTP config.</div>`)
		return
	}

	c.Header("HX-Trigger", "emailServerDeleted")
	// Return refreshed list
	h.EmailServerList(c)
}

// EmailServerFormCancel clears the form container.
// GET /gui/email-servers/form-cancel
func (h *GUIHandler) EmailServerFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// EmailServerSendTest sends a test email for a given SMTP config.
// POST /gui/email-servers/:id/test
func (h *GUIHandler) EmailServerSendTest(c *gin.Context) {
	idStr := c.Param("id")
	toEmail := strings.TrimSpace(c.PostForm("to_email"))

	if toEmail == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show mb-0" role="alert"><i class="bi bi-exclamation-triangle me-2"></i>Please enter a recipient email address.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	configID, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show mb-0" role="alert"><i class="bi bi-exclamation-triangle me-2"></i>Invalid config ID. Please close this dialog and try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if err := h.EmailService.SendTestEmailWithConfigID(configID, toEmail); err != nil {
		friendlyMsg := formatSMTPError(err.Error())
		c.String(http.StatusOK,
			fmt.Sprintf(`<div class="alert alert-danger alert-dismissible fade show mb-0" role="alert"><i class="bi bi-exclamation-triangle me-2"></i><strong>Send failed:</strong> %s<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`, friendlyMsg))
		return
	}

	c.String(http.StatusOK,
		fmt.Sprintf(`<div class="alert alert-success alert-dismissible fade show mb-0" role="alert"><i class="bi bi-check-circle me-2"></i>Test email sent to <strong>%s</strong> successfully!</div>`, toEmail))
}

// resolveServerConfigDisplay resolves a server config ID to its display string and name.
func resolveServerConfigDisplay(h *GUIHandler, serverConfigID *uuid.UUID) (string, string) {
	if serverConfigID == nil {
		return "", ""
	}
	config, err := h.EmailService.GetServerConfigByID(*serverConfigID)
	if err != nil || config == nil {
		return serverConfigID.String(), "(unknown)"
	}
	return config.ID.String(), config.Name
}

// formatSMTPError translates raw SMTP error messages into user-friendly descriptions.
func formatSMTPError(rawErr string) string {
	lower := strings.ToLower(rawErr)

	switch {
	// Not configured
	case strings.Contains(lower, "not configured"):
		return rawErr

	// Authentication errors
	case strings.Contains(lower, "application-specific password required"):
		return "Gmail requires an App Password. Go to <a href=\"https://myaccount.google.com/apppasswords\" target=\"_blank\">Google App Passwords</a> to generate one, then use it as the SMTP password."
	case strings.Contains(lower, "535") || strings.Contains(lower, "authentication failed") || strings.Contains(lower, "invalid credentials") || strings.Contains(lower, "username and password not accepted"):
		return "Authentication failed. Please check your SMTP username and password."

	// Connection errors
	case strings.Contains(lower, "no such host") || strings.Contains(lower, "lookup"):
		return "SMTP host not found. Please check the hostname is correct."
	case strings.Contains(lower, "connection refused"):
		return "Connection refused. Please check the SMTP host and port are correct."
	case strings.Contains(lower, "connection timed out") || strings.Contains(lower, "i/o timeout"):
		return "Connection timed out. The SMTP server may be unreachable or the port may be blocked by a firewall."

	// TLS errors
	case strings.Contains(lower, "tls") || strings.Contains(lower, "certificate") || strings.Contains(lower, "x509"):
		return "TLS/SSL error. Try toggling the 'Use TLS' setting, or check if the port matches (587 for STARTTLS, 465 for SSL)."

	// Port / protocol mismatch
	case strings.Contains(lower, "eof") || strings.Contains(lower, "short response"):
		return "Unexpected response from server. This usually means a port/TLS mismatch &mdash; try port 587 with TLS enabled, or port 465 with TLS enabled (SSL)."

	// Sender / recipient errors
	case strings.Contains(lower, "550") || strings.Contains(lower, "sender rejected") || strings.Contains(lower, "relay"):
		return "The server rejected the sender address. Please check the 'From Address' is authorized for this SMTP account."

	default:
		// Truncate very long errors and clean up for display
		msg := rawErr
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return msg
	}
}

// ============================================================
// Email Templates GUI Handlers
// ============================================================

// EmailTemplatesPage renders the email templates management page.
// GET /gui/email-templates
func (h *GUIHandler) EmailTemplatesPage(c *gin.Context) {
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		apps = nil
	}

	emailTypes, err := h.EmailService.GetAllEmailTypes()
	if err != nil {
		emailTypes = nil
	}

	data := web.TemplateData{
		ActivePage:    "email-templates",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
		Data: gin.H{
			"Apps":       apps,
			"EmailTypes": emailTypes,
		},
	}
	c.HTML(http.StatusOK, "email_templates", data)
}

// EmailTemplateList returns the email template list partial (HTMX fragment).
// GET /gui/email-templates/list
func (h *GUIHandler) EmailTemplateList(c *gin.Context) {
	appIDStr := c.Query("app_id")
	scope := c.Query("scope") // "global" or "app" or ""

	type templateItem struct {
		ID               string
		AppID            string
		AppName          string
		EmailTypeCode    string
		EmailTypeName    string
		Name             string
		Subject          string
		TemplateEngine   string
		FromEmail        string
		FromName         string
		ServerConfigID   string
		ServerConfigName string
		IsActive         bool
		IsGlobal         bool
		HasDefault       bool
	}

	var items []templateItem

	if scope == "global" || (scope == "" && appIDStr == "") {
		// Show global default templates
		templates, err := h.EmailService.GetGlobalDefaultTemplates()
		if err == nil {
			for _, t := range templates {
				scID, scName := resolveServerConfigDisplay(h, t.ServerConfigID)
				items = append(items, templateItem{
					ID:               t.ID.String(),
					EmailTypeCode:    t.EmailType.Code,
					EmailTypeName:    t.EmailType.Name,
					Name:             t.Name,
					Subject:          t.Subject,
					TemplateEngine:   t.TemplateEngine,
					FromEmail:        t.FromEmail,
					FromName:         t.FromName,
					ServerConfigID:   scID,
					ServerConfigName: scName,
					IsActive:         t.IsActive,
					IsGlobal:         true,
					HasDefault:       email.GetDefaultTemplate(t.EmailType.Code) != nil,
				})
			}
		}
	}

	if appIDStr != "" {
		appID, err := uuid.Parse(appIDStr)
		if err == nil {
			templates, err := h.EmailService.GetTemplatesByApp(appID)
			if err == nil {
				// Find app name
				appName := ""
				apps, _ := h.Repo.ListAllAppsWithTenantName()
				for _, a := range apps {
					if a.ID == appID {
						appName = a.Name
						break
					}
				}
				for _, t := range templates {
					scID, scName := resolveServerConfigDisplay(h, t.ServerConfigID)
					items = append(items, templateItem{
						ID:               t.ID.String(),
						AppID:            appID.String(),
						AppName:          appName,
						EmailTypeCode:    t.EmailType.Code,
						EmailTypeName:    t.EmailType.Name,
						Name:             t.Name,
						Subject:          t.Subject,
						TemplateEngine:   t.TemplateEngine,
						FromEmail:        t.FromEmail,
						FromName:         t.FromName,
						ServerConfigID:   scID,
						ServerConfigName: scName,
						IsActive:         t.IsActive,
						IsGlobal:         false,
						HasDefault:       email.GetDefaultTemplate(t.EmailType.Code) != nil,
					})
				}
			}
		}
	}

	c.HTML(http.StatusOK, "email_template_list", gin.H{
		"Templates": items,
		"AppID":     appIDStr,
		"Scope":     scope,
	})
}

// EmailTemplateCreateForm returns the empty create form.
// GET /gui/email-templates/new
func (h *GUIHandler) EmailTemplateCreateForm(c *gin.Context) {
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		apps = nil
	}

	emailTypes, err := h.EmailService.GetAllEmailTypes()
	if err != nil {
		emailTypes = nil
	}

	serverConfigs, err := h.EmailService.GetAllServerConfigs()
	if err != nil {
		serverConfigs = nil
	}

	c.HTML(http.StatusOK, "email_template_form", gin.H{
		"IsEdit":         false,
		"Apps":           apps,
		"EmailTypes":     emailTypes,
		"ServerConfigs":  serverConfigs,
		"TemplateEngine": "go_template",
		"IsActive":       true,
	})
}

// EmailTemplateCreate handles creating a new email template.
// POST /gui/email-templates
func (h *GUIHandler) EmailTemplateCreate(c *gin.Context) {
	appIDStr := c.PostForm("app_id")
	emailTypeIDStr := c.PostForm("email_type_id")
	name := strings.TrimSpace(c.PostForm("name"))
	subject := strings.TrimSpace(c.PostForm("subject"))
	bodyHTML := c.PostForm("body_html")
	bodyText := c.PostForm("body_text")
	templateEngine := c.PostForm("template_engine")
	fromEmail := strings.TrimSpace(c.PostForm("from_email"))
	fromName := strings.TrimSpace(c.PostForm("from_name_override"))
	serverConfigIDStr := c.PostForm("server_config_id")
	isActive := c.PostForm("is_active") == "true"

	if emailTypeIDStr == "" || name == "" || subject == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Email type, name, and subject are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	emailTypeID, err := uuid.Parse(emailTypeIDStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid email type ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if templateEngine == "" {
		templateEngine = "go_template"
	}

	var serverConfigID *uuid.UUID
	if serverConfigIDStr != "" {
		parsed, err := uuid.Parse(serverConfigIDStr)
		if err == nil {
			serverConfigID = &parsed
		}
	}

	tmpl := &models.EmailTemplate{
		Name:           name,
		Subject:        subject,
		BodyHTML:       bodyHTML,
		BodyText:       bodyText,
		TemplateEngine: templateEngine,
		FromEmail:      fromEmail,
		FromName:       fromName,
		ServerConfigID: serverConfigID,
		IsActive:       isActive,
	}

	if appIDStr == "" {
		// Global default
		if err := h.EmailService.SaveGlobalTemplate(emailTypeID, tmpl); err != nil {
			c.String(http.StatusInternalServerError,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to save template.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
	} else {
		appID, err := uuid.Parse(appIDStr)
		if err != nil {
			c.String(http.StatusBadRequest,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid application ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
		if err := h.EmailService.SaveAppTemplate(appID, emailTypeID, tmpl); err != nil {
			c.String(http.StatusInternalServerError,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to save template.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
	}

	c.Header("HX-Trigger", "emailTemplateListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Email template created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// EmailTemplateEditForm returns the pre-filled edit form for an email template.
// GET /gui/email-templates/:id/edit
func (h *GUIHandler) EmailTemplateEditForm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid template ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	tmpl, err := h.EmailService.GetTemplateByID(id)
	if err != nil || tmpl == nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Template not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	apps, _ := h.Repo.ListAllAppsWithTenantName()
	emailTypes, _ := h.EmailService.GetAllEmailTypes()
	serverConfigs, _ := h.EmailService.GetAllServerConfigs()

	appIDStr := ""
	if tmpl.AppID != nil {
		appIDStr = tmpl.AppID.String()
	}

	serverConfigIDStr := ""
	if tmpl.ServerConfigID != nil {
		serverConfigIDStr = tmpl.ServerConfigID.String()
	}

	c.HTML(http.StatusOK, "email_template_form", gin.H{
		"IsEdit":         true,
		"ID":             tmpl.ID.String(),
		"AppID":          appIDStr,
		"EmailTypeID":    tmpl.EmailTypeID.String(),
		"Name":           tmpl.Name,
		"Subject":        tmpl.Subject,
		"BodyHTML":       tmpl.BodyHTML,
		"BodyText":       tmpl.BodyText,
		"TemplateEngine": tmpl.TemplateEngine,
		"FromEmail":      tmpl.FromEmail,
		"FromName":       tmpl.FromName,
		"ServerConfigID": serverConfigIDStr,
		"IsActive":       tmpl.IsActive,
		"Apps":           apps,
		"EmailTypes":     emailTypes,
		"ServerConfigs":  serverConfigs,
	})
}

// EmailTemplateUpdate handles updating an email template.
// PUT /gui/email-templates/:id
func (h *GUIHandler) EmailTemplateUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid template ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	tmpl, err := h.EmailService.GetTemplateByID(id)
	if err != nil || tmpl == nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Template not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	subject := strings.TrimSpace(c.PostForm("subject"))
	bodyHTML := c.PostForm("body_html")
	bodyText := c.PostForm("body_text")
	templateEngine := c.PostForm("template_engine")
	fromEmail := strings.TrimSpace(c.PostForm("from_email"))
	fromName := strings.TrimSpace(c.PostForm("from_name_override"))
	serverConfigIDStr := c.PostForm("server_config_id")
	isActive := c.PostForm("is_active") == "true"

	if name == "" || subject == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Name and subject are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	tmpl.Name = name
	tmpl.Subject = subject
	tmpl.BodyHTML = bodyHTML
	tmpl.BodyText = bodyText
	if templateEngine != "" {
		tmpl.TemplateEngine = templateEngine
	}
	tmpl.FromEmail = fromEmail
	tmpl.FromName = fromName
	if serverConfigIDStr != "" {
		parsed, err := uuid.Parse(serverConfigIDStr)
		if err == nil {
			tmpl.ServerConfigID = &parsed
		}
	} else {
		tmpl.ServerConfigID = nil
	}
	tmpl.IsActive = isActive

	if tmpl.AppID == nil {
		if err := h.EmailService.SaveGlobalTemplate(tmpl.EmailTypeID, tmpl); err != nil {
			c.String(http.StatusInternalServerError,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update template.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
	} else {
		if err := h.EmailService.SaveAppTemplate(*tmpl.AppID, tmpl.EmailTypeID, tmpl); err != nil {
			c.String(http.StatusInternalServerError,
				`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update template.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
			return
		}
	}

	c.Header("HX-Trigger", "emailTemplateListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Email template updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// EmailTemplateDeleteConfirm returns the delete confirmation modal body.
// GET /gui/email-templates/:id/delete
func (h *GUIHandler) EmailTemplateDeleteConfirm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="modal-body"><div class="alert alert-danger">Invalid template ID.</div></div>`)
		return
	}

	tmpl, err := h.EmailService.GetTemplateByID(id)
	if err != nil || tmpl == nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Template not found.</div></div>`)
		return
	}

	c.HTML(http.StatusOK, "email_template_delete_confirm", gin.H{
		"ID":            tmpl.ID.String(),
		"Name":          tmpl.Name,
		"EmailTypeName": tmpl.EmailType.Name,
	})
}

// EmailTemplateDelete handles deleting an email template.
// DELETE /gui/email-templates/:id
func (h *GUIHandler) EmailTemplateDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger">Invalid template ID.</div>`)
		return
	}

	if err := h.EmailService.DeleteTemplate(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to delete template.</div>`)
		return
	}

	c.Header("HX-Trigger", "emailTemplateDeleted")
	// Return refreshed list
	h.EmailTemplateList(c)
}

// EmailTemplateResetConfirm returns the reset confirmation modal body.
// GET /gui/email-templates/:id/reset
func (h *GUIHandler) EmailTemplateResetConfirm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="modal-body"><div class="alert alert-danger">Invalid template ID.</div></div>`)
		return
	}

	tmpl, err := h.EmailService.GetTemplateByID(id)
	if err != nil || tmpl == nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Template not found.</div></div>`)
		return
	}

	// Check that a hardcoded default exists for this email type
	if email.GetDefaultTemplate(tmpl.EmailType.Code) == nil {
		c.String(http.StatusBadRequest,
			`<div class="modal-body"><div class="alert alert-warning">No built-in default available for this email type.</div></div>`)
		return
	}

	c.HTML(http.StatusOK, "email_template_reset_confirm", gin.H{
		"ID":            tmpl.ID.String(),
		"Name":          tmpl.Name,
		"EmailTypeName": tmpl.EmailType.Name,
	})
}

// EmailTemplateReset resets a template's content to the built-in hardcoded default.
// POST /gui/email-templates/:id/reset
func (h *GUIHandler) EmailTemplateReset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger">Invalid template ID.</div>`)
		return
	}

	if err := h.EmailService.ResetTemplateToDefault(id); err != nil {
		c.String(http.StatusInternalServerError,
			fmt.Sprintf(`<div class="alert alert-danger alert-dismissible fade show" role="alert">%s<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`, err.Error()))
		return
	}

	c.Header("HX-Trigger", "emailTemplateReset, emailTemplateListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Template has been reset to the built-in default.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// EmailTemplateFormCancel clears the form container.
// GET /gui/email-templates/form-cancel
func (h *GUIHandler) EmailTemplateFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// EmailTemplatePreview renders a preview of the template.
// POST /gui/email-templates/preview
func (h *GUIHandler) EmailTemplatePreview(c *gin.Context) {
	subject := c.PostForm("subject")
	bodyHTML := c.PostForm("body_html")
	templateEngine := c.PostForm("template_engine")
	if templateEngine == "" {
		templateEngine = "go_template"
	}

	tmpl := &models.EmailTemplate{
		Subject:        subject,
		BodyHTML:       bodyHTML,
		TemplateEngine: templateEngine,
	}

	// Use sample variables for preview
	sampleVars := map[string]string{
		"app_name":           "My Application",
		"user_email":         "user@example.com",
		"user_name":          "John Doe",
		"verification_link":  "https://example.com/verify?token=abc123",
		"verification_token": "abc123",
		"reset_link":         "https://example.com/reset?token=xyz789",
		"code":               "123456",
		"expiration_minutes": "5",
		"change_time":        "2026-02-22 10:30:00 UTC",
	}

	renderedSubject, renderedHTML, _, err := h.EmailService.PreviewTemplate(tmpl, sampleVars)
	if err != nil {
		c.String(http.StatusOK,
			fmt.Sprintf(`<div class="alert alert-danger">Preview error: %s</div>`, err.Error()))
		return
	}

	c.String(http.StatusOK, fmt.Sprintf(`
<div class="card border-0 shadow-sm">
    <div class="card-header bg-light">
        <small class="text-muted">Subject:</small> <strong>%s</strong>
    </div>
    <div class="card-body p-0">
        <iframe srcdoc="%s" style="width:100%%;min-height:400px;border:none;" sandbox="allow-same-origin"></iframe>
    </div>
</div>`, renderedSubject, strings.ReplaceAll(strings.ReplaceAll(renderedHTML, `"`, `&quot;`), `<`, `&lt;`)))
}

// ============================================================
// Email Types GUI Handlers
// ============================================================

// EmailTypesPage renders the email types management page.
// GET /gui/email-types
func (h *GUIHandler) EmailTypesPage(c *gin.Context) {
	data := web.TemplateData{
		ActivePage:    "email-types",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
	}
	c.HTML(http.StatusOK, "email_types", data)
}

// EmailTypeList returns the email type list partial (HTMX fragment).
// GET /gui/email-types/list
func (h *GUIHandler) EmailTypeList(c *gin.Context) {
	types, err := h.EmailService.GetAllEmailTypes()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load email types.</div>`)
		return
	}

	// Parse variables JSON for each type to get count
	type emailTypeItem struct {
		ID             string
		Code           string
		Name           string
		Description    string
		DefaultSubject string
		IsSystem       bool
		IsActive       bool
		VarCount       int
	}

	var items []emailTypeItem
	for _, t := range types {
		varCount := 0
		if len(t.Variables) > 0 {
			var vars []models.EmailTypeVariable
			if err := json.Unmarshal(t.Variables, &vars); err == nil {
				varCount = len(vars)
			}
		}
		items = append(items, emailTypeItem{
			ID:             t.ID.String(),
			Code:           t.Code,
			Name:           t.Name,
			Description:    t.Description,
			DefaultSubject: t.DefaultSubject,
			IsSystem:       t.IsSystem,
			IsActive:       t.IsActive,
			VarCount:       varCount,
		})
	}

	c.HTML(http.StatusOK, "email_type_list", gin.H{
		"Types": items,
	})
}

// EmailTypeCreateForm returns the empty create form for email types.
// GET /gui/email-types/new
func (h *GUIHandler) EmailTypeCreateForm(c *gin.Context) {
	c.HTML(http.StatusOK, "email_type_form", gin.H{
		"IsEdit":             false,
		"IsActive":           true,
		"WellKnownVariables": email.WellKnownVariables,
	})
}

// EmailTypeCreate handles creating a new custom email type.
// POST /gui/email-types
func (h *GUIHandler) EmailTypeCreate(c *gin.Context) {
	code := strings.TrimSpace(c.PostForm("code"))
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	defaultSubject := strings.TrimSpace(c.PostForm("default_subject"))
	isActive := c.PostForm("is_active") == "true"

	if code == "" || name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Code and Name are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Check for duplicate code
	existing, _ := h.EmailService.GetEmailTypeByCode(code)
	if existing != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">An email type with this code already exists.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Parse variables from dynamic form rows
	varsJSON := parseVariablesFromForm(c)

	emailType := &models.EmailType{
		Code:           code,
		Name:           name,
		Description:    description,
		DefaultSubject: defaultSubject,
		Variables:      varsJSON,
		IsSystem:       false,
		IsActive:       isActive,
	}

	if err := h.EmailService.CreateEmailType(emailType); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create email type. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "emailTypeListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Email type created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// EmailTypeEditForm returns the pre-filled edit form for an email type.
// GET /gui/email-types/:id/edit
func (h *GUIHandler) EmailTypeEditForm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	emailType, err := h.EmailService.GetEmailTypeByID(id)
	if err != nil || emailType == nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Email type not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	// Parse variables JSON for the form
	var vars []models.EmailTypeVariable
	if len(emailType.Variables) > 0 {
		_ = json.Unmarshal(emailType.Variables, &vars)
	}

	c.HTML(http.StatusOK, "email_type_form", gin.H{
		"IsEdit":             true,
		"ID":                 emailType.ID.String(),
		"Code":               emailType.Code,
		"Name":               emailType.Name,
		"Description":        emailType.Description,
		"DefaultSubject":     emailType.DefaultSubject,
		"IsSystem":           emailType.IsSystem,
		"IsActive":           emailType.IsActive,
		"Variables":          vars,
		"WellKnownVariables": email.WellKnownVariables,
	})
}

// EmailTypeUpdate handles updating an email type.
// PUT /gui/email-types/:id
func (h *GUIHandler) EmailTypeUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	emailType, err := h.EmailService.GetEmailTypeByID(id)
	if err != nil || emailType == nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Email type not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	defaultSubject := strings.TrimSpace(c.PostForm("default_subject"))
	isActive := c.PostForm("is_active") == "true"

	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	emailType.Name = name
	emailType.Description = description
	emailType.DefaultSubject = defaultSubject
	emailType.IsActive = isActive
	emailType.Variables = parseVariablesFromForm(c)

	if err := h.EmailService.UpdateEmailType(emailType); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update email type.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "emailTypeListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Email type updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// EmailTypeDeleteConfirm returns the delete confirmation modal body.
// GET /gui/email-types/:id/delete
func (h *GUIHandler) EmailTypeDeleteConfirm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="modal-body"><div class="alert alert-danger">Invalid ID.</div></div>`)
		return
	}

	emailType, err := h.EmailService.GetEmailTypeByID(id)
	if err != nil || emailType == nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Email type not found.</div></div>`)
		return
	}

	if emailType.IsSystem {
		c.String(http.StatusBadRequest,
			`<div class="modal-body"><div class="alert alert-warning">System email types cannot be deleted.</div></div>`)
		return
	}

	c.HTML(http.StatusOK, "email_type_delete_confirm", gin.H{
		"ID":   emailType.ID.String(),
		"Code": emailType.Code,
		"Name": emailType.Name,
	})
}

// EmailTypeDelete handles deleting a custom email type.
// DELETE /gui/email-types/:id
func (h *GUIHandler) EmailTypeDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger">Invalid ID.</div>`)
		return
	}

	if err := h.EmailService.DeleteEmailType(id); err != nil {
		c.String(http.StatusBadRequest,
			fmt.Sprintf(`<div class="alert alert-danger">%s</div>`, err.Error()))
		return
	}

	c.Header("HX-Trigger", "emailTypeDeleted")
	// Return refreshed list
	h.EmailTypeList(c)
}

// EmailTypeFormCancel clears the email type form container.
// GET /gui/email-types/form-cancel
func (h *GUIHandler) EmailTypeFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// ============================================================================
// 2FA Login Verification Handlers
// ============================================================================

// TwoFAVerifyPage renders the 2FA verification form during login.
// The admin has already authenticated with password and received a temp token.
// GET /gui/2fa-verify
func (h *GUIHandler) TwoFAVerifyPage(c *gin.Context) {
	tempToken := c.Query("token")
	method := c.Query("method")
	redirect := c.Query("redirect")

	if tempToken == "" {
		c.Redirect(http.StatusFound, "/gui/login")
		return
	}

	// Validate the temp session still exists
	if _, err := h.AccountService.Validate2FATempSession(tempToken); err != nil {
		c.Redirect(http.StatusFound, "/gui/login?error=session_expired")
		return
	}

	data := web.TemplateData{
		TempToken:   tempToken,
		TwoFAMethod: method,
		Redirect:    redirect,
	}
	c.HTML(http.StatusOK, "2fa_verify", data)
}

// TwoFAVerifySubmit handles 2FA code submission during login.
// On success, creates a full session and redirects to the dashboard.
// POST /gui/2fa-verify
func (h *GUIHandler) TwoFAVerifySubmit(c *gin.Context) {
	tempToken := c.PostForm("temp_token")
	code := c.PostForm("code")
	method := c.PostForm("method")
	redirect := c.PostForm("redirect")
	useRecovery := c.PostForm("use_recovery") == "true"

	if tempToken == "" || code == "" {
		data := web.TemplateData{
			Error:       "Verification code is required.",
			TempToken:   tempToken,
			TwoFAMethod: method,
			Redirect:    redirect,
		}
		c.HTML(http.StatusBadRequest, "2fa_verify", data)
		return
	}

	// Validate temp session (without consuming it yet)
	account, err := h.AccountService.Validate2FATempSession(tempToken)
	if err != nil {
		c.Redirect(http.StatusFound, "/gui/login?error=session_expired")
		return
	}

	// Verify the code
	adminID := account.ID.String()
	if useRecovery {
		err = h.AccountService.VerifyRecoveryCode(adminID, code)
	} else if method == "email" {
		err = h.AccountService.VerifyEmail2FACode(adminID, code)
	} else {
		err = h.AccountService.VerifyTOTPCode(adminID, code)
	}

	if err != nil {
		data := web.TemplateData{
			Error:       "Invalid verification code. Please try again.",
			TempToken:   tempToken,
			TwoFAMethod: method,
			Redirect:    redirect,
		}
		c.HTML(http.StatusUnauthorized, "2fa_verify", data)
		return
	}

	// 2FA verified — consume temp session and create full session
	_, _ = h.AccountService.Consume2FATempSession(tempToken)

	sessionID, err := h.AccountService.CreateSession(adminID)
	if err != nil {
		data := web.TemplateData{
			Error:       "An internal error occurred. Please try again.",
			TempToken:   tempToken,
			TwoFAMethod: method,
			Redirect:    redirect,
		}
		c.HTML(http.StatusInternalServerError, "2fa_verify", data)
		return
	}

	// Set session cookie
	maxAge := sessionMaxAgeSeconds()
	web.SetSessionCookie(c, sessionID, maxAge)

	// Clear rate limit counters on successful login
	_ = redis.ClearLoginAttempts(c.ClientIP())
	_ = redis.ClearRateLimitKeys("gui:login", c.ClientIP())
	if web.ClearRateLimitFallback != nil {
		web.ClearRateLimitFallback("gui:login", c.ClientIP())
	}

	if redirect != "" && redirect != "/gui/login" {
		c.Redirect(http.StatusFound, redirect)
		return
	}
	c.Redirect(http.StatusFound, "/gui/")
}

// TwoFAResendEmail resends the 2FA email code during login verification.
// POST /gui/2fa-resend-email
func (h *GUIHandler) TwoFAResendEmail(c *gin.Context) {
	tempToken := c.PostForm("temp_token")

	account, err := h.AccountService.Validate2FATempSession(tempToken)
	if err != nil {
		c.String(http.StatusUnauthorized, `<div class="alert alert-danger py-2"><small>Session expired. Please log in again.</small></div>`)
		return
	}

	if err := h.AccountService.GenerateAndSendEmail2FACode(account.ID.String()); err != nil {
		c.String(http.StatusInternalServerError, `<div class="alert alert-danger py-2"><small>Failed to send code. Please try again.</small></div>`)
		return
	}

	c.String(http.StatusOK, `<div class="alert alert-success py-2"><small>A new code has been sent to your email.</small></div>`)
}

// ============================================================================
// My Account Handlers
// ============================================================================

// MyAccountData holds data for the My Account page template.
type MyAccountData struct {
	Email              string
	TwoFAEnabled       bool
	TwoFAMethod        string
	RecoveryCodesCount int
}

// MyAccountPage renders the "My Account" page with 2FA settings.
// GET /gui/my-account
func (h *GUIHandler) MyAccountPage(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)

	account, err := h.AccountService.Repo.GetByID(adminID)
	if err != nil {
		c.Redirect(http.StatusFound, "/gui/")
		return
	}

	// Count remaining recovery codes
	recoveryCount := 0
	if account.TwoFAEnabled && len(account.TwoFARecoveryCodes) > 0 {
		var codes []string
		if json.Unmarshal(account.TwoFARecoveryCodes, &codes) == nil {
			recoveryCount = len(codes)
		}
	}

	data := web.TemplateData{
		ActivePage:    "my-account",
		AdminUsername: c.GetString(web.GUIAdminUsernameKey),
		AdminID:       adminID,
		CSRFToken:     c.GetString(web.CSRFTokenKey),
		Data: MyAccountData{
			Email:              account.Email,
			TwoFAEnabled:       account.TwoFAEnabled,
			TwoFAMethod:        account.TwoFAMethod,
			RecoveryCodesCount: recoveryCount,
		},
	}
	c.HTML(http.StatusOK, "my_account", data)
}

// MyAccountUpdateEmail updates the admin's email address.
// POST /gui/my-account/email
func (h *GUIHandler) MyAccountUpdateEmail(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	email := strings.TrimSpace(c.PostForm("email"))

	if email == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>Email address is required.</small></div>`)
		return
	}

	if err := h.AccountService.UpdateEmail(adminID, email); err != nil {
		msg := "Failed to update email."
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			msg = "This email address is already in use."
		}
		c.String(http.StatusBadRequest,
			fmt.Sprintf(`<div class="alert alert-danger py-2"><small>%s</small></div>`, msg))
		return
	}

	c.String(http.StatusOK,
		fmt.Sprintf(`<div class="alert alert-success py-2"><small>Email updated to %s.</small></div>`, email))
}

// MyAccountChangePassword handles password changes.
// POST /gui/my-account/password
func (h *GUIHandler) MyAccountChangePassword(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if currentPassword == "" || newPassword == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>All password fields are required.</small></div>`)
		return
	}

	if len(newPassword) < 8 {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>New password must be at least 8 characters.</small></div>`)
		return
	}

	if newPassword != confirmPassword {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>New passwords do not match.</small></div>`)
		return
	}

	if err := h.AccountService.ChangePassword(adminID, currentPassword, newPassword); err != nil {
		c.String(http.StatusBadRequest,
			fmt.Sprintf(`<div class="alert alert-danger py-2"><small>%s</small></div>`, err.Error()))
		return
	}

	c.String(http.StatusOK,
		`<div class="alert alert-success py-2"><small>Password changed successfully.</small></div>`)
}

// MyAccount2FAGenerateTOTP generates a TOTP secret and returns the QR code partial.
// POST /gui/my-account/2fa/generate
func (h *GUIHandler) MyAccount2FAGenerateTOTP(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	username := c.GetString(web.GUIAdminUsernameKey)
	switching := c.PostForm("switching") == "true"

	setup, err := h.AccountService.GenerateTOTPSecret(adminID, username)
	if err != nil {
		c.String(http.StatusInternalServerError,
			fmt.Sprintf(`<div class="alert alert-danger py-2"><small>%s</small></div>`, err.Error()))
		return
	}

	data := web.TemplateData{
		CSRFToken: c.GetString(web.CSRFTokenKey),
		Data: map[string]interface{}{
			"Secret":     setup.Secret,
			"QRCodeData": fmt.Sprintf("data:image/png;base64,%s", base64Encode(setup.QRCodeData)),
			"Switching":  switching,
		},
	}
	c.HTML(http.StatusOK, "admin_2fa_setup", data)
}

// MyAccount2FAVerifyTOTP verifies the TOTP code during setup.
// POST /gui/my-account/2fa/verify-totp
func (h *GUIHandler) MyAccount2FAVerifyTOTP(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	code := strings.TrimSpace(c.PostForm("code"))
	switching := c.PostForm("switching") == "true"

	if code == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>Please enter the 6-digit code from your authenticator app.</small></div>`)
		return
	}

	if err := h.AccountService.VerifyTOTPSetup(adminID, code); err != nil {
		c.String(http.StatusBadRequest,
			fmt.Sprintf(`<div class="alert alert-danger py-2"><small>%s</small></div>`, err.Error()))
		return
	}

	// Verification succeeded — enable TOTP and return recovery codes
	recoveryCodes, err := h.AccountService.EnableTOTP(adminID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			fmt.Sprintf(`<div class="alert alert-danger py-2"><small>%s</small></div>`, err.Error()))
		return
	}

	data := web.TemplateData{
		CSRFToken: c.GetString(web.CSRFTokenKey),
		Data: map[string]interface{}{
			"RecoveryCodes": recoveryCodes,
			"Method":        "totp",
			"Switched":      switching,
		},
	}
	c.HTML(http.StatusOK, "admin_2fa_recovery", data)
}

// MyAccount2FAEnableEmail enables email-based 2FA.
// POST /gui/my-account/2fa/enable-email
func (h *GUIHandler) MyAccount2FAEnableEmail(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	switching := c.PostForm("switching") == "true"

	recoveryCodes, err := h.AccountService.EnableEmail2FA(adminID)
	if err != nil {
		c.String(http.StatusBadRequest,
			fmt.Sprintf(`<div class="alert alert-danger py-2"><small>%s</small></div>`, err.Error()))
		return
	}

	data := web.TemplateData{
		CSRFToken: c.GetString(web.CSRFTokenKey),
		Data: map[string]interface{}{
			"RecoveryCodes": recoveryCodes,
			"Method":        "email",
			"Switched":      switching,
		},
	}
	c.HTML(http.StatusOK, "admin_2fa_recovery", data)
}

// MyAccount2FADisable disables 2FA for the admin (requires password).
// POST /gui/my-account/2fa/disable
func (h *GUIHandler) MyAccount2FADisable(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	password := c.PostForm("password")

	if password == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>Password is required to disable 2FA.</small></div>`)
		return
	}

	if err := h.AccountService.Disable2FA(adminID, password); err != nil {
		c.String(http.StatusBadRequest,
			fmt.Sprintf(`<div class="alert alert-danger py-2"><small>%s</small></div>`, err.Error()))
		return
	}

	// Return refreshed 2FA status partial
	h.MyAccount2FAStatus(c)
}

// MyAccount2FAStatus returns the current 2FA status as an HTMX partial.
// GET /gui/my-account/2fa/status
func (h *GUIHandler) MyAccount2FAStatus(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)

	account, err := h.AccountService.Repo.GetByID(adminID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2"><small>Failed to load account.</small></div>`)
		return
	}

	recoveryCount := 0
	if account.TwoFAEnabled && len(account.TwoFARecoveryCodes) > 0 {
		var codes []string
		if json.Unmarshal(account.TwoFARecoveryCodes, &codes) == nil {
			recoveryCount = len(codes)
		}
	}

	data := web.TemplateData{
		CSRFToken: c.GetString(web.CSRFTokenKey),
		Data: MyAccountData{
			Email:              account.Email,
			TwoFAEnabled:       account.TwoFAEnabled,
			TwoFAMethod:        account.TwoFAMethod,
			RecoveryCodesCount: recoveryCount,
		},
	}
	c.HTML(http.StatusOK, "admin_2fa_status", data)
}

// MyAccount2FARegenerateCodes regenerates recovery codes (requires password).
// POST /gui/my-account/2fa/regenerate-codes
func (h *GUIHandler) MyAccount2FARegenerateCodes(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	password := c.PostForm("password")

	if password == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>Password is required to regenerate codes.</small></div>`)
		return
	}

	codes, err := h.AccountService.RegenerateRecoveryCodes(adminID, password)
	if err != nil {
		c.String(http.StatusBadRequest,
			fmt.Sprintf(`<div class="alert alert-danger py-2"><small>%s</small></div>`, err.Error()))
		return
	}

	data := web.TemplateData{
		CSRFToken: c.GetString(web.CSRFTokenKey),
		Data: map[string]interface{}{
			"RecoveryCodes": codes,
			"Regenerated":   true,
		},
	}
	c.HTML(http.StatusOK, "admin_2fa_recovery", data)
}

// base64Encode is a helper to encode bytes to base64 for inline images.
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// ============================================================
// Passkey Management (My Account - Admin self-service)
// ============================================================

// MyAccountPasskeyData holds passkey list data for the admin passkey status template.
type MyAccountPasskeyData struct {
	Passkeys []models.WebAuthnCredential
}

// MyAccountPasskeyStatus returns the current passkey status as an HTMX partial.
// GET /gui/my-account/passkeys/status
func (h *GUIHandler) MyAccountPasskeyStatus(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	adminUUID, err := uuid.Parse(adminID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2"><small>Invalid admin ID.</small></div>`)
		return
	}

	creds, appErr := h.PasskeyService.ListAdminCredentials(adminUUID)
	if appErr != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2"><small>Failed to load passkeys.</small></div>`)
		return
	}

	data := web.TemplateData{
		CSRFToken: c.GetString(web.CSRFTokenKey),
		Data: MyAccountPasskeyData{
			Passkeys: creds,
		},
	}
	c.HTML(http.StatusOK, "admin_passkey_status", data)
}

// MyAccountPasskeyBeginRegister starts the WebAuthn registration ceremony for the admin.
// POST /gui/my-account/passkeys/register/begin
func (h *GUIHandler) MyAccountPasskeyBeginRegister(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)

	account, err := h.AccountService.Repo.GetByID(adminID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load account")
		return
	}

	optionsJSON, appErr := h.PasskeyService.BeginAdminRegistration(account)
	if appErr != nil {
		c.String(appErr.Code, appErr.Message)
		return
	}

	c.Data(http.StatusOK, "application/json", optionsJSON)
}

// MyAccountPasskeyFinishRegister completes the WebAuthn registration ceremony for the admin.
// POST /gui/my-account/passkeys/register/finish
func (h *GUIHandler) MyAccountPasskeyFinishRegister(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)

	account, err := h.AccountService.Repo.GetByID(adminID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load account")
		return
	}

	credentialName := strings.TrimSpace(c.PostForm("name"))
	credentialJSON := c.PostForm("credential")
	if credentialJSON == "" {
		c.String(http.StatusBadRequest, "Missing credential data")
		return
	}

	appErr := h.PasskeyService.FinishAdminRegistration(account, credentialName, json.RawMessage(credentialJSON))
	if appErr != nil {
		c.String(appErr.Code, appErr.Message)
		return
	}

	c.String(http.StatusOK, "OK")
}

// MyAccountPasskeyDelete deletes a passkey for the current admin.
// DELETE /gui/my-account/passkeys/:id
func (h *GUIHandler) MyAccountPasskeyDelete(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	passkeyID := c.Param("id")

	adminUUID, err := uuid.Parse(adminID)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>Invalid admin ID.</small></div>`)
		return
	}

	credUUID, err := uuid.Parse(passkeyID)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2"><small>Invalid passkey ID.</small></div>`)
		return
	}

	appErr := h.PasskeyService.DeleteAdminCredential(adminUUID, credUUID)
	if appErr != nil {
		c.String(appErr.Code, fmt.Sprintf(
			`<div class="alert alert-danger py-2"><small>%s</small></div>`, appErr.Message))
		return
	}

	// Return refreshed passkey status
	h.MyAccountPasskeyStatus(c)
}

// MyAccountPasskeyRename renames a passkey for the current admin.
// POST /gui/my-account/passkeys/:id/rename
func (h *GUIHandler) MyAccountPasskeyRename(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	passkeyID := c.Param("id")
	newName := strings.TrimSpace(c.PostForm("name"))

	if newName == "" {
		c.String(http.StatusBadRequest, "Name is required")
		return
	}

	adminUUID, err := uuid.Parse(adminID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid admin ID")
		return
	}

	credUUID, err := uuid.Parse(passkeyID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid passkey ID")
		return
	}

	appErr := h.PasskeyService.RenameAdminCredential(adminUUID, credUUID, newName)
	if appErr != nil {
		c.String(appErr.Code, appErr.Message)
		return
	}

	c.String(http.StatusOK, "OK")
}

// ============================================================
// RBAC — Roles Management
// ============================================================

// RolesPage renders the roles management page.
// GET /gui/roles
func (h *GUIHandler) RolesPage(c *gin.Context) {
	apps, err := h.RBACService.Repo.ListAllApps()
	if err != nil {
		apps = nil
	}

	data := web.TemplateData{
		ActivePage:    "roles",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
		Data:          apps,
	}
	c.HTML(http.StatusOK, "roles", data)
}

// RoleList returns the role table HTML fragment for HTMX.
// GET /gui/roles/list?app_id=X
func (h *GUIHandler) RoleList(c *gin.Context) {
	appID := c.Query("app_id")
	if appID == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-warning">Please select an application.</div>`)
		return
	}

	roles, err := h.RBACService.GetRolesByAppID(appID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load roles.</div>`)
		return
	}

	type roleItem struct {
		ID              string
		Name            string
		Description     string
		IsSystem        bool
		PermissionCount int
		CreatedAt       time.Time
	}

	items := make([]roleItem, 0, len(roles))
	for _, r := range roles {
		items = append(items, roleItem{
			ID:              r.ID.String(),
			Name:            r.Name,
			Description:     r.Description,
			IsSystem:        r.IsSystem,
			PermissionCount: len(r.Permissions),
			CreatedAt:       r.CreatedAt,
		})
	}

	type roleListData struct {
		Roles []roleItem
	}

	c.HTML(http.StatusOK, "role_list", roleListData{Roles: items})
}

// RoleCreateForm returns the empty create form HTML fragment for HTMX.
// GET /gui/roles/new
func (h *GUIHandler) RoleCreateForm(c *gin.Context) {
	type formData struct {
		ID          string
		AppID       string
		Name        string
		Description string
	}
	// Try to read app_id from query string (set by JS reading the filter dropdown)
	appID := c.Query("app_id")
	c.HTML(http.StatusOK, "role_form", formData{AppID: appID})
}

// RoleCreate handles creating a new role.
// POST /gui/roles
func (h *GUIHandler) RoleCreate(c *gin.Context) {
	appID := strings.TrimSpace(c.PostForm("app_id"))
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))

	if appID == "" || name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application and role name are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if _, err := h.RBACService.CreateRole(appID, name, description); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create role. It may already exist.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Role created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// RoleEditForm returns the pre-filled edit form HTML fragment for HTMX.
// GET /gui/roles/:id/edit
func (h *GUIHandler) RoleEditForm(c *gin.Context) {
	id := c.Param("id")
	role, err := h.RBACService.GetRoleByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Role not found.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	type formData struct {
		ID          string
		AppID       string
		Name        string
		Description string
	}
	c.HTML(http.StatusOK, "role_form", formData{
		ID:          role.ID.String(),
		Name:        role.Name,
		Description: role.Description,
	})
}

// RoleUpdate handles updating a role.
// PUT /gui/roles/:id
func (h *GUIHandler) RoleUpdate(c *gin.Context) {
	id := c.Param("id")
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))

	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Role name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if err := h.RBACService.UpdateRole(id, name, description); err != nil {
		c.String(http.StatusInternalServerError,
			fmt.Sprintf(`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update role: %s<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`, err.Error()))
		return
	}

	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Role updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// RoleDeleteConfirm returns the delete confirmation modal body for HTMX.
// GET /gui/roles/:id/delete
func (h *GUIHandler) RoleDeleteConfirm(c *gin.Context) {
	id := c.Param("id")
	role, err := h.RBACService.GetRoleByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Role not found.</div></div>`)
		return
	}

	type deleteData struct {
		ID   string
		Name string
	}
	c.HTML(http.StatusOK, "role_delete_confirm", deleteData{
		ID:   role.ID.String(),
		Name: role.Name,
	})
}

// RoleDelete handles deleting a role and returns a refreshed role list.
// DELETE /gui/roles/:id
func (h *GUIHandler) RoleDelete(c *gin.Context) {
	id := c.Param("id")

	// Get the role first to know the app ID for refreshing the list
	role, err := h.RBACService.GetRoleByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger">Role not found.</div>`)
		return
	}
	appID := role.AppID.String()

	if err := h.RBACService.DeleteRole(id); err != nil {
		c.String(http.StatusInternalServerError,
			fmt.Sprintf(`<div class="alert alert-danger">Failed to delete role: %s</div>`, err.Error()))
		return
	}

	c.Header("HX-Trigger", "roleDeleted")

	// Re-fetch and render the updated role list
	roles, err := h.RBACService.GetRolesByAppID(appID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Role deleted but failed to refresh list.</div>`)
		return
	}

	type roleItem struct {
		ID              string
		Name            string
		Description     string
		IsSystem        bool
		PermissionCount int
		CreatedAt       time.Time
	}

	items := make([]roleItem, 0, len(roles))
	for _, r := range roles {
		items = append(items, roleItem{
			ID:              r.ID.String(),
			Name:            r.Name,
			Description:     r.Description,
			IsSystem:        r.IsSystem,
			PermissionCount: len(r.Permissions),
			CreatedAt:       r.CreatedAt,
		})
	}

	type roleListData struct {
		Roles []roleItem
	}

	c.HTML(http.StatusOK, "role_list", roleListData{Roles: items})
}

// RoleFormCancel clears the role form container.
// GET /gui/roles/form-cancel
func (h *GUIHandler) RoleFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// RolePermissions returns the permissions checkbox modal body for HTMX.
// GET /gui/roles/:id/permissions
func (h *GUIHandler) RolePermissions(c *gin.Context) {
	roleID := c.Param("id")

	role, err := h.RBACService.GetRoleByID(roleID)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Role not found.</div></div>`)
		return
	}

	allPermissions, err := h.RBACService.GetAllPermissions()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="modal-body"><div class="alert alert-danger">Failed to load permissions.</div></div>`)
		return
	}

	// Build a set of currently assigned permission IDs
	assigned := make(map[string]bool)
	for _, p := range role.Permissions {
		assigned[p.ID.String()] = true
	}

	type permItem struct {
		ID          string
		Resource    string
		Action      string
		Description string
		Checked     bool
	}

	items := make([]permItem, 0, len(allPermissions))
	for _, p := range allPermissions {
		items = append(items, permItem{
			ID:          p.ID.String(),
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
			Checked:     assigned[p.ID.String()],
		})
	}

	type permData struct {
		RoleID      string
		RoleName    string
		Permissions []permItem
	}

	c.HTML(http.StatusOK, "role_permissions", permData{
		RoleID:      role.ID.String(),
		RoleName:    role.Name,
		Permissions: items,
	})
}

// RolePermissionsUpdate saves the selected permissions for a role.
// PUT /gui/roles/:id/permissions
func (h *GUIHandler) RolePermissionsUpdate(c *gin.Context) {
	roleID := c.Param("id")
	permissionIDs := c.PostFormArray("permission_ids")

	if err := h.RBACService.SetRolePermissions(roleID, permissionIDs); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="modal-body"><div class="alert alert-danger">Failed to save permissions.</div></div>`)
		return
	}

	c.Header("HX-Trigger", "permissionsSaved")
	c.String(http.StatusOK,
		`<div class="modal-body"><div class="alert alert-success">Permissions saved successfully.</div></div><div class="modal-footer border-0"><button type="button" class="btn btn-outline-secondary btn-sm" data-bs-dismiss="modal">Close</button></div>`)
}

// ============================================================
// RBAC — Permissions Management
// ============================================================

// PermissionsPage renders the permissions management page.
// GET /gui/permissions
func (h *GUIHandler) PermissionsPage(c *gin.Context) {
	data := web.TemplateData{
		ActivePage:    "permissions",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
	}
	c.HTML(http.StatusOK, "permissions", data)
}

// PermissionList returns the permission table HTML fragment for HTMX.
// GET /gui/permissions/list
func (h *GUIHandler) PermissionList(c *gin.Context) {
	permissions, err := h.RBACService.GetAllPermissions()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load permissions.</div>`)
		return
	}

	type permListData struct {
		Permissions []models.Permission
	}

	c.HTML(http.StatusOK, "permission_list", permListData{Permissions: permissions})
}

// PermissionCreateForm returns the empty create form HTML fragment for HTMX.
// GET /gui/permissions/new
func (h *GUIHandler) PermissionCreateForm(c *gin.Context) {
	type formData struct {
		Resource    string
		Action      string
		Description string
	}
	c.HTML(http.StatusOK, "permission_form", formData{})
}

// PermissionCreate handles creating a new permission.
// POST /gui/permissions
func (h *GUIHandler) PermissionCreate(c *gin.Context) {
	resource := strings.TrimSpace(c.PostForm("resource"))
	action := strings.TrimSpace(c.PostForm("action"))
	description := strings.TrimSpace(c.PostForm("description"))

	if resource == "" || action == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Resource and action are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if _, err := h.RBACService.CreatePermission(resource, action, description); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create permission. It may already exist.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.Header("HX-Trigger", "permissionListRefresh")
	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Permission created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// PermissionFormCancel clears the permission form container.
// GET /gui/permissions/form-cancel
func (h *GUIHandler) PermissionFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// ============================================================
// RBAC — User Roles Management
// ============================================================

// UserRolesPage renders the user-roles management page.
// GET /gui/user-roles
func (h *GUIHandler) UserRolesPage(c *gin.Context) {
	apps, err := h.RBACService.Repo.ListAllApps()
	if err != nil {
		apps = nil
	}

	data := web.TemplateData{
		ActivePage:    "user-roles",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
		Data:          apps,
	}
	c.HTML(http.StatusOK, "user_roles", data)
}

// UserRoleList returns the user-role table HTML fragment for HTMX.
// GET /gui/user-roles/list?app_id=X&page=N
func (h *GUIHandler) UserRoleList(c *gin.Context) {
	appID := c.Query("app_id")
	if appID == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-warning">Please select an application.</div>`)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 20

	items, total, err := h.RBACService.Repo.GetUsersWithRoleInApp(appID, page, pageSize)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load user roles.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	type userRoleListData struct {
		Items      []rbac.UserRoleListItem
		AppID      string
		Page       int
		TotalPages int
		Total      int64
	}

	c.HTML(http.StatusOK, "user_role_list", userRoleListData{
		Items:      items,
		AppID:      appID,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
	})
}

// UserRoleCreateForm returns the assign role form HTML fragment for HTMX.
// GET /gui/user-roles/new
func (h *GUIHandler) UserRoleCreateForm(c *gin.Context) {
	apps, err := h.RBACService.Repo.ListAllApps()
	if err != nil {
		apps = nil
	}

	type formData struct {
		Apps []models.Application
	}
	c.HTML(http.StatusOK, "user_role_form", formData{Apps: apps})
}

// UserRoleCreate handles assigning a role to a user.
// POST /gui/user-roles
func (h *GUIHandler) UserRoleCreate(c *gin.Context) {
	appID := strings.TrimSpace(c.PostForm("app_id"))
	userID := strings.TrimSpace(c.PostForm("user_id"))
	roleID := strings.TrimSpace(c.PostForm("role_id"))

	if appID == "" || userID == "" || roleID == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application, user ID, and role are all required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if err := h.RBACService.AssignRoleToUser(userID, roleID, appID, nil); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to assign role. The user may already have this role.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	c.String(http.StatusOK,
		`<div class="alert alert-success alert-dismissible fade show" role="alert">Role assigned successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// UserRoleRolesForApp returns HTML <option> elements for the roles in an app.
// GET /gui/user-roles/roles-for-app?app_id=X
func (h *GUIHandler) UserRoleRolesForApp(c *gin.Context) {
	appID := c.Query("app_id")
	if appID == "" {
		c.String(http.StatusOK, `<option value="">-- Select App first --</option>`)
		return
	}

	roles, err := h.RBACService.GetRolesByAppID(appID)
	if err != nil {
		c.String(http.StatusOK, `<option value="">-- Error loading roles --</option>`)
		return
	}

	var sb strings.Builder
	sb.WriteString(`<option value="">-- Select Role --</option>`)
	for _, r := range roles {
		sb.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, r.ID.String(), r.Name))
	}
	c.String(http.StatusOK, sb.String())
}

// UserRoleSearchUsers returns a list of matching users as clickable HTML items.
// GET /gui/user-roles/search-users?app_id=X&q=term
func (h *GUIHandler) UserRoleSearchUsers(c *gin.Context) {
	appID := c.Query("app_id")
	q := strings.TrimSpace(c.Query("q"))

	if appID == "" {
		c.String(http.StatusOK, `<div class="list-group-item text-muted small">Select an application first.</div>`)
		return
	}
	if len(q) < 2 {
		c.String(http.StatusOK, `<div class="list-group-item text-muted small">Type at least 2 characters to search.</div>`)
		return
	}

	users, _, err := h.Repo.ListUsersWithDetails(1, 10, appID, q)
	if err != nil {
		c.String(http.StatusOK, `<div class="list-group-item text-danger small">Error searching users.</div>`)
		return
	}

	if len(users) == 0 {
		c.String(http.StatusOK, `<div class="list-group-item text-muted small">No users found.</div>`)
		return
	}

	var sb strings.Builder
	for _, u := range users {
		name := u.Email
		if u.Name != "" {
			name = u.Name + " &mdash; " + u.Email
		}
		sb.WriteString(fmt.Sprintf(
			`<a href="#" class="list-group-item list-group-item-action py-2 px-3" onclick="selectUser('%s','%s'); return false;">
				<div class="fw-semibold small">%s</div>
				<div class="text-muted font-monospace" style="font-size:.7rem">%s</div>
			</a>`,
			u.ID.String(), escapeHTML(u.Email), name, u.ID.String(),
		))
	}
	c.String(http.StatusOK, sb.String())
}

// escapeHTML is a minimal HTML entity escaper for dynamic string insertion.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// UserRoleRevokeConfirm returns the revoke confirmation modal body for HTMX.
// GET /gui/user-roles/revoke?user_id=X&role_id=X&app_id=X&user_email=X&role_name=X
func (h *GUIHandler) UserRoleRevokeConfirm(c *gin.Context) {
	type revokeData struct {
		UserID    string
		RoleID    string
		AppID     string
		UserEmail string
		RoleName  string
	}
	c.HTML(http.StatusOK, "user_role_revoke_confirm", revokeData{
		UserID:    c.Query("user_id"),
		RoleID:    c.Query("role_id"),
		AppID:     c.Query("app_id"),
		UserEmail: c.Query("user_email"),
		RoleName:  c.Query("role_name"),
	})
}

// UserRoleRevoke handles revoking a role from a user and returns a refreshed list.
// DELETE /gui/user-roles?user_id=X&role_id=X&app_id=X
func (h *GUIHandler) UserRoleRevoke(c *gin.Context) {
	userID := c.Query("user_id")
	roleID := c.Query("role_id")
	appID := c.Query("app_id")

	if userID == "" || roleID == "" || appID == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger">Missing required parameters.</div>`)
		return
	}

	if err := h.RBACService.RevokeRoleFromUser(userID, roleID, appID); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to revoke role.</div>`)
		return
	}

	c.Header("HX-Trigger", "roleRevoked")

	// Re-fetch and render the updated user-role list
	page := 1
	pageSize := 20
	items, total, err := h.RBACService.Repo.GetUsersWithRoleInApp(appID, page, pageSize)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Role revoked but failed to refresh list.</div>`)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	type userRoleListData struct {
		Items      []rbac.UserRoleListItem
		AppID      string
		Page       int
		TotalPages int
		Total      int64
	}

	c.HTML(http.StatusOK, "user_role_list", userRoleListData{
		Items:      items,
		AppID:      appID,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
	})
}

// UserRoleFormCancel clears the user-role form container.
// GET /gui/user-roles/form-cancel
func (h *GUIHandler) UserRoleFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// SocialAccountUnlinkConfirm returns a confirmation dialog partial for unlinking a social account.
// GET /gui/users/social-accounts/:id/unlink
func (h *GUIHandler) SocialAccountUnlinkConfirm(c *gin.Context) {
	id := c.Param("id")
	sa, err := h.Repo.GetSocialAccountByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Social account not found.</div></div>`)
		return
	}

	detail, err := h.Repo.GetUserDetailByID(sa.UserID.String())
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="modal-body"><div class="alert alert-danger">Failed to load user details.</div></div>`)
		return
	}

	count, err := h.Repo.CountSocialAccountsByUserID(sa.UserID.String())
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="modal-body"><div class="alert alert-danger">Failed to check social accounts.</div></div>`)
		return
	}

	isLockoutRisk := !detail.HasPassword && count == 1

	type confirmData struct {
		SocialAccountID string
		Provider        string
		Email           string
		UserEmail       string
		IsLockoutRisk   bool
	}
	c.HTML(http.StatusOK, "social_unlink_confirm", confirmData{
		SocialAccountID: sa.ID.String(),
		Provider:        sa.Provider,
		Email:           sa.Email,
		UserEmail:       detail.Email,
		IsLockoutRisk:   isLockoutRisk,
	})
}

// SocialAccountUnlink handles deleting (unlinking) a social account from a user.
// DELETE /gui/users/social-accounts/:id
func (h *GUIHandler) SocialAccountUnlink(c *gin.Context) {
	id := c.Param("id")
	sa, err := h.Repo.GetSocialAccountByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger">Social account not found.</div>`)
		return
	}

	userID := sa.UserID.String()

	// Lockout prevention: check if user has no password and this is their only social account
	detail, err := h.Repo.GetUserDetailByID(userID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to load user details.</div>`)
		return
	}

	count, err := h.Repo.CountSocialAccountsByUserID(userID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to check social accounts.</div>`)
		return
	}

	if !detail.HasPassword && count == 1 {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger">Cannot unlink the only social account when the user has no password set.</div>`)
		return
	}

	if err := h.Repo.DeleteSocialAccount(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to unlink social account.</div>`)
		return
	}

	// Trigger modal close via HX-Trigger
	c.Header("HX-Trigger", "socialAccountUnlinked")

	// Re-render the user detail with refreshed data
	refreshed, err := h.Repo.GetUserDetailByID(userID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Social account unlinked but failed to refresh user details.</div>`)
		return
	}

	c.HTML(http.StatusOK, "user_detail", refreshed)
}

// PasskeyDeleteConfirm returns a confirmation dialog partial for deleting a passkey.
// GET /gui/users/passkeys/:id/delete
func (h *GUIHandler) PasskeyDeleteConfirm(c *gin.Context) {
	id := c.Param("id")
	cred, err := h.Repo.GetWebAuthnCredentialByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="modal-body"><div class="alert alert-danger">Passkey not found.</div></div>`)
		return
	}

	if cred.UserID == nil {
		c.String(http.StatusBadRequest,
			`<div class="modal-body"><div class="alert alert-danger">This passkey is not associated with a regular user.</div></div>`)
		return
	}

	detail, err := h.Repo.GetUserDetailByID(cred.UserID.String())
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="modal-body"><div class="alert alert-danger">Failed to load user details.</div></div>`)
		return
	}

	type confirmData struct {
		PasskeyID   string
		PasskeyName string
		UserEmail   string
	}
	c.HTML(http.StatusOK, "passkey_delete_confirm", confirmData{
		PasskeyID:   cred.ID.String(),
		PasskeyName: cred.Name,
		UserEmail:   detail.Email,
	})
}

// PasskeyDelete handles deleting a WebAuthn passkey credential.
// DELETE /gui/users/passkeys/:id
func (h *GUIHandler) PasskeyDelete(c *gin.Context) {
	id := c.Param("id")
	cred, err := h.Repo.GetWebAuthnCredentialByID(id)
	if err != nil {
		c.String(http.StatusNotFound,
			`<div class="alert alert-danger">Passkey not found.</div>`)
		return
	}

	if cred.UserID == nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger">This passkey is not associated with a regular user.</div>`)
		return
	}

	userID := cred.UserID.String()

	if err := h.Repo.DeleteWebAuthnCredential(id); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Failed to delete passkey.</div>`)
		return
	}

	// Trigger modal close via HX-Trigger
	c.Header("HX-Trigger", "passkeyDeleted")

	// Re-render the user detail with refreshed data
	refreshed, err := h.Repo.GetUserDetailByID(userID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger">Passkey deleted but failed to refresh user details.</div>`)
		return
	}

	c.HTML(http.StatusOK, "user_detail", refreshed)
}

// parseVariablesFromForm parses variable rows from the dynamic form.
// Variables are submitted as var_name[], var_description[], var_required[],
// var_source[], and var_default_value[] arrays.
func parseVariablesFromForm(c *gin.Context) []byte {
	names := c.PostFormArray("var_name[]")
	descriptions := c.PostFormArray("var_description[]")
	requireds := c.PostFormArray("var_required[]")
	sources := c.PostFormArray("var_source[]")
	defaultValues := c.PostFormArray("var_default_value[]")

	var vars []models.EmailTypeVariable
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		desc := ""
		if i < len(descriptions) {
			desc = strings.TrimSpace(descriptions[i])
		}
		required := false
		if i < len(requireds) {
			required = requireds[i] == "true"
		}
		source := ""
		if i < len(sources) {
			source = strings.TrimSpace(sources[i])
		}
		defaultValue := ""
		if i < len(defaultValues) {
			defaultValue = strings.TrimSpace(defaultValues[i])
		}
		vars = append(vars, models.EmailTypeVariable{
			Name:         name,
			Description:  desc,
			Required:     required,
			Source:       source,
			DefaultValue: defaultValue,
		})
	}

	if len(vars) == 0 {
		return nil
	}

	data, err := json.Marshal(vars)
	if err != nil {
		return nil
	}
	return data
}

// ============================================================
// Passkey Login (Passwordless Admin Login via Discoverable Credentials)
// ============================================================

// PasskeyLoginBegin starts the WebAuthn discoverable login ceremony for admin passkey login.
// POST /gui/passkey-login/begin
func (h *GUIHandler) PasskeyLoginBegin(c *gin.Context) {
	// Check if rate limiter already blocked the request
	if errMsg, exists := c.Get(web.RateLimitErrorKey); exists {
		msg, _ := errMsg.(string)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": msg})
		return
	}

	optionsJSON, sessionID, appErr := h.PasskeyService.BeginAdminLogin()
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"options":    json.RawMessage(optionsJSON),
		"session_id": sessionID,
	})
}

// PasskeyLoginFinish completes the WebAuthn discoverable login ceremony for admin passkey login.
// On success, creates a full admin session (bypassing 2FA) and sets the session cookie.
// POST /gui/passkey-login/finish
func (h *GUIHandler) PasskeyLoginFinish(c *gin.Context) {
	// Check if rate limiter already blocked the request
	if errMsg, exists := c.Get(web.RateLimitErrorKey); exists {
		msg, _ := errMsg.(string)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": msg})
		return
	}

	var req struct {
		SessionID  string          `json:"session_id"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.SessionID == "" || len(req.Credential) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing session_id or credential"})
		return
	}

	adminID, appErr := h.PasskeyService.FinishAdminLogin(req.SessionID, req.Credential)
	if appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	// Update last login timestamp (best effort)
	_ = h.AccountService.Repo.UpdateLastLogin(adminID.String())

	// Create full session (passkey login bypasses 2FA entirely)
	sessionID, err := h.AccountService.CreateSession(adminID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Set session cookie
	maxAge := sessionMaxAgeSeconds()
	web.SetSessionCookie(c, sessionID, maxAge)

	// Clear rate limit counters on successful login
	_ = redis.ClearLoginAttempts(c.ClientIP())
	_ = redis.ClearRateLimitKeys("gui:passkey-login", c.ClientIP())
	if web.ClearRateLimitFallback != nil {
		web.ClearRateLimitFallback("gui:passkey-login", c.ClientIP())
	}

	c.JSON(http.StatusOK, gin.H{"redirect": "/gui/"})
}

// ============================================================
// Magic Link (My Account - Admin self-service toggle)
// ============================================================

// MyAccountMagicLinkData holds data for the admin magic link status template.
type MyAccountMagicLinkData struct {
	MagicLinkEnabled bool
}

// MyAccountMagicLinkStatus returns the current magic link status as an HTMX partial.
// GET /gui/my-account/magic-link/status
func (h *GUIHandler) MyAccountMagicLinkStatus(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)

	account, err := h.AccountService.Repo.GetByID(adminID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2"><small>Failed to load account.</small></div>`)
		return
	}

	data := web.TemplateData{
		CSRFToken: c.GetString(web.CSRFTokenKey),
		Data: MyAccountMagicLinkData{
			MagicLinkEnabled: account.MagicLinkEnabled,
		},
	}
	c.HTML(http.StatusOK, "admin_magic_link_status", data)
}

// MyAccountMagicLinkToggle enables or disables magic link login for the current admin.
// POST /gui/my-account/magic-link/toggle
func (h *GUIHandler) MyAccountMagicLinkToggle(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)

	account, err := h.AccountService.Repo.GetByID(adminID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2"><small>Failed to load account.</small></div>`)
		return
	}

	// Toggle the current state
	newState := !account.MagicLinkEnabled

	// Admin must have an email address to enable magic link login
	if newState && account.Email == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-warning py-2"><small>You must set an email address before enabling magic link login.</small></div>`)
		return
	}

	if err := h.AccountService.Repo.UpdateMagicLinkEnabled(adminID, newState); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2"><small>Failed to update magic link setting.</small></div>`)
		return
	}

	// Re-render the status partial with the updated state
	data := web.TemplateData{
		CSRFToken: c.GetString(web.CSRFTokenKey),
		Data: MyAccountMagicLinkData{
			MagicLinkEnabled: newState,
		},
	}
	c.HTML(http.StatusOK, "admin_magic_link_status", data)
}

// ============================================================
// Magic Link Login (Passwordless Admin Login via Email Link)
// ============================================================

// MagicLinkLoginRequest handles the magic link login request from the login page.
// It looks up the admin by email, checks if magic link is enabled, generates a token,
// sends the email, and returns a generic success message (to prevent email enumeration).
// POST /gui/magic-link-login
func (h *GUIHandler) MagicLinkLoginRequest(c *gin.Context) {
	// Check if rate limiter already blocked the request
	if errMsg, exists := c.Get(web.RateLimitErrorKey); exists {
		msg, _ := errMsg.(string)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": msg})
		return
	}

	email := strings.TrimSpace(c.PostForm("email"))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}

	// Always respond with success to prevent email enumeration.
	// Perform the lookup and send silently.
	account, err := h.AccountService.Repo.GetByEmail(email)
	if err == nil && account != nil && account.MagicLinkEnabled && account.Email != "" {
		// Generate magic link token
		magicToken := uuid.New().String()

		// Store in Redis with 10-minute expiration (invalidates any previous token)
		if storeErr := redis.SetAdminMagicLinkToken(account.ID.String(), magicToken, 10*time.Minute); storeErr == nil {
			// Build the magic link URL
			baseURL := viper.GetString("ADMIN_BASE_URL")
			if baseURL == "" {
				baseURL = fmt.Sprintf("%s://%s", schemeFromRequest(c), c.Request.Host)
			}
			magicLink := fmt.Sprintf("%s/gui/magic-link-login/verify?token=%s", baseURL, magicToken)

			// Send the email (best-effort — don't expose failures)
			_ = h.EmailService.SendAdminMagicLinkEmail(account.Email, magicLink, account.Username)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "If that email is associated with an admin account with magic link enabled, a login link has been sent."})
}

// MagicLinkLoginVerify handles the magic link verification callback.
// On success, creates a full admin session (bypassing 2FA like passkey login) and redirects.
// GET /gui/magic-link-login/verify
func (h *GUIHandler) MagicLinkLoginVerify(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.HTML(http.StatusBadRequest, "login", web.TemplateData{
			Error: "Invalid or missing magic link token.",
		})
		return
	}

	// Look up token in Redis
	adminID, err := redis.GetAdminMagicLinkToken(token)
	if err != nil || adminID == "" {
		c.HTML(http.StatusUnauthorized, "login", web.TemplateData{
			Error: "This magic link is invalid or has expired. Please request a new one.",
		})
		return
	}

	// Delete token immediately (single-use)
	_ = redis.DeleteAdminMagicLinkToken(token)

	// Verify the admin account still exists
	account, accErr := h.AccountService.Repo.GetByID(adminID)
	if accErr != nil || account == nil {
		c.HTML(http.StatusUnauthorized, "login", web.TemplateData{
			Error: "Account not found. Please contact an administrator.",
		})
		return
	}

	// Update last login timestamp (best effort)
	_ = h.AccountService.Repo.UpdateLastLogin(adminID)

	// Create full session (magic link login bypasses 2FA entirely, like passkey login)
	sessionID, sessErr := h.AccountService.CreateSession(adminID)
	if sessErr != nil {
		c.HTML(http.StatusInternalServerError, "login", web.TemplateData{
			Error: "Failed to create session. Please try again.",
		})
		return
	}

	// Set session cookie
	maxAge := sessionMaxAgeSeconds()
	web.SetSessionCookie(c, sessionID, maxAge)

	// Clear rate limit counters on successful login
	_ = redis.ClearRateLimitKeys("gui:magic-link", c.ClientIP())
	if web.ClearRateLimitFallback != nil {
		web.ClearRateLimitFallback("gui:magic-link", c.ClientIP())
	}

	// Redirect to admin dashboard
	c.Redirect(http.StatusFound, "/gui/")
}

// schemeFromRequest returns "https" or "http" based on the incoming request.
func schemeFromRequest(c *gin.Context) string {
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

// ============================================================
// Session Management
// ============================================================

// SessionsPage renders the session management page.
// GET /gui/sessions
func (h *GUIHandler) SessionsPage(c *gin.Context) {
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "sessions", gin.H{
			"ActivePage": "sessions",
			"AdminUser":  getAdminUsername(c),
			"CSRFToken":  getCSRFToken(c),
			"Error":      "Failed to load applications",
		})
		return
	}

	c.HTML(http.StatusOK, "sessions", gin.H{
		"ActivePage": "sessions",
		"AdminUser":  getAdminUsername(c),
		"CSRFToken":  getCSRFToken(c),
		"Apps":       apps,
	})
}

// sessionItem is a flattened struct for rendering in the session list template.
type sessionItem struct {
	SessionID           string
	UserID              string
	UserEmail           string
	AppID               string
	AppName             string
	IP                  string
	UserAgent           string
	CreatedAt           string
	LastActive          string
	CreatedAtFormatted  string
	LastActiveFormatted string
	Status              string // "active", "idle", or "stale"
	IdleMinutes         int    // minutes since last_active
}

// SessionList returns the paginated session list partial (HTMX fragment).
// GET /gui/sessions/list
func (h *GUIHandler) SessionList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 20

	filterAppID := c.Query("app_id")
	search := strings.ToLower(c.Query("search"))
	ipSearch := strings.ToLower(c.Query("ip"))

	// Determine which apps to query
	var appIDs []string
	var appNames map[string]string

	if filterAppID != "" {
		appIDs = []string{filterAppID}
	} else {
		apps, err := h.Repo.ListAllAppsWithTenantName()
		if err != nil {
			c.HTML(http.StatusInternalServerError, "session_list", gin.H{"Sessions": nil, "Error": "Failed to load apps"})
			return
		}
		for _, a := range apps {
			appIDs = append(appIDs, a.ID.String())
		}
	}

	// Fetch app names for display
	appNames, _ = h.Repo.GetAppNamesByIDs(appIDs)

	// Collect all sessions across selected apps
	var allSessions []map[string]string
	var allAppIDs []string // parallel array tracking which appID each session belongs to
	for _, appID := range appIDs {
		sessions, err := redis.GetAllSessionsForApp(appID)
		if err != nil {
			continue
		}
		for _, s := range sessions {
			s["app_id"] = appID
			allSessions = append(allSessions, s)
			allAppIDs = append(allAppIDs, appID)
		}
	}

	// Collect unique user IDs for batch email lookup
	userIDSet := make(map[string]bool)
	for _, s := range allSessions {
		if uid, ok := s["user_id"]; ok && uid != "" {
			userIDSet[uid] = true
		}
	}
	var userIDs []string
	for uid := range userIDSet {
		userIDs = append(userIDs, uid)
	}
	userEmails, _ := h.Repo.GetUserEmailsByIDs(userIDs)

	// Active threshold: treat sessions with last_active within this window as "active"
	activeThresholdMinutes := viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES")
	if activeThresholdMinutes <= 0 {
		activeThresholdMinutes = 15
	}

	// Build flattened items and apply in-memory filters
	var items []sessionItem
	for _, s := range allSessions {
		userID := s["user_id"]
		userEmail := userEmails[userID]
		ip := s["ip"]
		appID := s["app_id"]

		// Apply search filter (user email)
		if search != "" && !strings.Contains(strings.ToLower(userEmail), search) {
			continue
		}
		// Apply IP filter
		if ipSearch != "" && !strings.Contains(strings.ToLower(ip), ipSearch) {
			continue
		}

		item := sessionItem{
			SessionID:  s["session_id"],
			UserID:     userID,
			UserEmail:  userEmail,
			AppID:      appID,
			AppName:    appNames[appID],
			IP:         ip,
			UserAgent:  s["user_agent"],
			CreatedAt:  s["created_at"],
			LastActive: s["last_active"],
		}

		// Format timestamps for display
		if t, err := time.Parse(time.RFC3339, s["created_at"]); err == nil {
			item.CreatedAtFormatted = formatTimeAgo(t)
		} else {
			item.CreatedAtFormatted = s["created_at"]
		}
		if t, err := time.Parse(time.RFC3339, s["last_active"]); err == nil {
			item.LastActiveFormatted = formatTimeAgo(t)
			// Compute session status based on idle time
			idle := int(time.Since(t).Minutes())
			item.IdleMinutes = idle
			switch {
			case idle <= activeThresholdMinutes:
				item.Status = "active"
			case idle < 1440:
				item.Status = "idle"
			default:
				item.Status = "stale"
			}
		} else {
			item.LastActiveFormatted = s["last_active"]
			item.Status = "stale"
		}

		items = append(items, item)
	}

	// Sort by last_active descending (most recently active first)
	sortSessionsByLastActive(items)

	// Paginate
	total := len(items)

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pageItems := items[start:end]

	c.HTML(http.StatusOK, "session_list", gin.H{
		"Sessions":   pageItems,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"AppID":      filterAppID,
		"Search":     c.Query("search"),
		"IP":         c.Query("ip"),
	})
}

// SessionDetail returns the session detail partial (HTMX fragment).
// GET /gui/sessions/:app_id/:session_id/detail
func (h *GUIHandler) SessionDetail(c *gin.Context) {
	appID := c.Param("app_id")
	sessionID := c.Param("session_id")

	data, err := redis.GetSession(appID, sessionID)
	if err != nil {
		c.HTML(http.StatusNotFound, "session_detail", gin.H{
			"Error": "Session not found",
		})
		return
	}

	userID := data["user_id"]
	userEmails, _ := h.Repo.GetUserEmailsByIDs([]string{userID})
	appNames, _ := h.Repo.GetAppNamesByIDs([]string{appID})

	// Compute session status
	activeThresholdMinutes := viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES")
	if activeThresholdMinutes <= 0 {
		activeThresholdMinutes = 15
	}
	detailStatus := "stale"
	detailIdleMinutes := 0
	if t, err := time.Parse(time.RFC3339, data["last_active"]); err == nil {
		detailIdleMinutes = int(time.Since(t).Minutes())
		switch {
		case detailIdleMinutes <= activeThresholdMinutes:
			detailStatus = "active"
		case detailIdleMinutes < 1440:
			detailStatus = "idle"
		default:
			detailStatus = "stale"
		}
	}

	c.HTML(http.StatusOK, "session_detail", gin.H{
		"SessionID":   sessionID,
		"UserID":      userID,
		"UserEmail":   userEmails[userID],
		"AppID":       appID,
		"AppName":     appNames[appID],
		"IP":          data["ip"],
		"UserAgent":   data["user_agent"],
		"CreatedAt":   data["created_at"],
		"LastActive":  data["last_active"],
		"Status":      detailStatus,
		"IdleMinutes": detailIdleMinutes,
	})
}

// SessionRevoke deletes a single session (HTMX action).
// DELETE /gui/sessions/:app_id/:session_id
func (h *GUIHandler) SessionRevoke(c *gin.Context) {
	appID := c.Param("app_id")
	sessionID := c.Param("session_id")
	userID := c.Query("user_id")

	if userID == "" {
		// Try to get user_id from the session itself
		data, err := redis.GetSession(appID, sessionID)
		if err == nil {
			userID = data["user_id"]
		}
	}

	if err := redis.DeleteSession(appID, sessionID, userID); err != nil {
		c.String(http.StatusInternalServerError, "Failed to revoke session")
		return
	}

	// Immediately invalidate any live access tokens belonging to this user so the
	// client is forced out without waiting for the JWT to naturally expire.
	// We use a user-wide blacklist keyed to the access-token TTL; the middleware
	// checks this flag on every authenticated request and returns 401 when set.
	if userID != "" {
		accessTokenTTL := time.Duration(viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES")) * time.Minute
		if err := redis.BlacklistAllUserTokens(appID, userID, accessTokenTTL); err != nil {
			// Non-fatal: log the error but continue — the session hash is already gone so the
			// next request will still get a 401 via the SessionExists check.
			_ = err
		}
	}

	// Check if this was called from user detail page
	if c.Query("from_user_detail") == "1" {
		h.renderUserSessions(c, appID, userID)
		return
	}

	// Trigger list refresh and re-render
	c.Header("HX-Trigger", "sessionListRefresh")
	h.SessionList(c)
}

// SessionRevokeAllForUser revokes all sessions for a specific user (HTMX action).
// DELETE /gui/sessions/revoke-all-user
func (h *GUIHandler) SessionRevokeAllForUser(c *gin.Context) {
	appID := c.Query("app_id")
	userID := c.Query("user_id")

	if appID == "" || userID == "" {
		c.String(http.StatusBadRequest, "Missing app_id or user_id")
		return
	}

	if err := redis.DeleteAllUserSessions(appID, userID, ""); err != nil {
		c.String(http.StatusInternalServerError, "Failed to revoke sessions")
		return
	}

	// Blacklist all live access tokens for the user so they are forced out immediately.
	accessTokenTTL := time.Duration(viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES")) * time.Minute
	if err := redis.BlacklistAllUserTokens(appID, userID, accessTokenTTL); err != nil {
		_ = err // non-fatal
	}

	// Check if this was called from user detail page
	if c.Query("from_user_detail") == "1" {
		h.renderUserSessions(c, appID, userID)
		return
	}

	// Trigger list refresh and re-render
	c.Header("HX-Trigger", "sessionListRefresh")
	h.SessionList(c)
}

// UserSessions returns the user sessions partial for the user detail page.
// GET /gui/users/:id/sessions
func (h *GUIHandler) UserSessions(c *gin.Context) {
	userID := c.Param("id")
	appID := c.Query("app_id")

	if appID == "" {
		// Look up the user's app_id
		detail, err := h.Repo.GetUserDetailByID(userID)
		if err != nil {
			c.HTML(http.StatusNotFound, "user_sessions", gin.H{"Sessions": nil})
			return
		}
		appID = detail.AppID.String()
	}

	h.renderUserSessions(c, appID, userID)
}

// renderUserSessions is a helper that renders the user_sessions partial for a given user.
func (h *GUIHandler) renderUserSessions(c *gin.Context, appID, userID string) {
	sessionIDs, err := redis.GetUserSessionIDs(appID, userID)
	if err != nil {
		c.HTML(http.StatusOK, "user_sessions", gin.H{
			"Sessions": nil,
			"AppID":    appID,
			"UserID":   userID,
		})
		return
	}

	activeThresholdMinutes := viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES")
	if activeThresholdMinutes <= 0 {
		activeThresholdMinutes = 15
	}

	var items []sessionItem
	for _, sid := range sessionIDs {
		data, err := redis.GetSession(appID, sid)
		if err != nil {
			continue
		}

		item := sessionItem{
			SessionID:  sid,
			UserID:     userID,
			AppID:      appID,
			IP:         data["ip"],
			UserAgent:  data["user_agent"],
			CreatedAt:  data["created_at"],
			LastActive: data["last_active"],
		}

		if t, err := time.Parse(time.RFC3339, data["created_at"]); err == nil {
			item.CreatedAtFormatted = formatTimeAgo(t)
		} else {
			item.CreatedAtFormatted = data["created_at"]
		}
		if t, err := time.Parse(time.RFC3339, data["last_active"]); err == nil {
			item.LastActiveFormatted = formatTimeAgo(t)
			// Compute session status based on idle time
			idle := int(time.Since(t).Minutes())
			item.IdleMinutes = idle
			switch {
			case idle <= activeThresholdMinutes:
				item.Status = "active"
			case idle < 1440:
				item.Status = "idle"
			default:
				item.Status = "stale"
			}
		} else {
			item.LastActiveFormatted = data["last_active"]
			item.Status = "stale"
		}

		items = append(items, item)
	}

	sortSessionsByLastActive(items)

	c.HTML(http.StatusOK, "user_sessions", gin.H{
		"Sessions": items,
		"AppID":    appID,
		"UserID":   userID,
	})
}

// sortSessionsByLastActive sorts sessions by last_active time descending.
func sortSessionsByLastActive(items []sessionItem) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			ti, _ := time.Parse(time.RFC3339, items[i].LastActive)
			tj, _ := time.Parse(time.RFC3339, items[j].LastActive)
			if tj.After(ti) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// formatTimeAgo returns a human-readable relative time string.
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 02, 2006 15:04")
	}
}

// ============================================================================
// IP Rules Management
// ============================================================================

// IPRulePage renders the IP Rules management page.
// GET /gui/ip-rules
func (h *GUIHandler) IPRulePage(c *gin.Context) {
	apps, _ := h.Repo.ListAllAppsWithTenantName()

	c.HTML(http.StatusOK, "ip_rules", web.TemplateData{
		ActivePage:    "ip-rules",
		AdminUsername: getAdminUsername(c),
		AdminID:       getAdminID(c),
		CSRFToken:     getCSRFToken(c),
		Data:          apps,
	})
}

// IPRuleList renders the IP rules list (HTMX partial).
// GET /gui/ip-rules/list
func (h *GUIHandler) IPRuleList(c *gin.Context) {
	if h.IPRuleRepo == nil {
		c.String(http.StatusOK, `<div class="alert alert-warning">IP rules feature is not configured.</div>`)
		return
	}

	appIDStr := c.Query("app_id")
	if appIDStr == "" {
		c.String(http.StatusOK, `<div class="text-center py-5 text-muted"><i class="bi bi-funnel fs-1"></i><p class="mt-2 mb-0">Select an application above to view its IP rules.</p></div>`)
		return
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Invalid application ID.</div>`)
		return
	}

	rules, err := h.IPRuleRepo.ListAllByApp(appID)
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Failed to load IP rules.</div>`)
		return
	}

	type ruleListData struct {
		Rules []models.IPRule
		AppID string
		Total int
	}

	c.HTML(http.StatusOK, "ip_rule_list", ruleListData{
		Rules: rules,
		AppID: appIDStr,
		Total: len(rules),
	})
}

// IPRuleCreateForm renders the IP rule create form (HTMX partial).
// GET /gui/ip-rules/new
func (h *GUIHandler) IPRuleCreateForm(c *gin.Context) {
	appID := c.Query("app_id")

	type formData struct {
		IsEdit      bool
		AppID       string
		ID          string
		RuleType    string
		MatchType   string
		Value       string
		Description string
		IsActive    bool
	}

	c.HTML(http.StatusOK, "ip_rule_form", formData{
		IsEdit:   false,
		AppID:    appID,
		IsActive: true,
	})
}

// IPRuleCreate handles IP rule creation.
// POST /gui/ip-rules
func (h *GUIHandler) IPRuleCreate(c *gin.Context) {
	if h.IPRuleRepo == nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rules feature is not configured.</div>`)
		return
	}

	appIDStr := c.PostForm("app_id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Invalid application ID.</div>`)
		return
	}

	isActive := c.PostForm("is_active") == "on"

	rule := &models.IPRule{
		AppID:       appID,
		RuleType:    c.PostForm("rule_type"),
		MatchType:   c.PostForm("match_type"),
		Value:       strings.TrimSpace(c.PostForm("value")),
		Description: strings.TrimSpace(c.PostForm("description")),
		IsActive:    isActive,
	}

	if err := geoip.ValidateRule(rule); err != nil {
		c.String(http.StatusOK, fmt.Sprintf(`<div class="alert alert-danger">%s</div>`, escapeHTML(err.Error())))
		return
	}

	if err := h.IPRuleRepo.Create(rule); err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Failed to create IP rule.</div>`)
		return
	}

	if h.IPRuleEvaluator != nil {
		h.IPRuleEvaluator.InvalidateCache(appID)
	}

	c.Header("HX-Trigger", "ipRuleListRefresh")
	c.String(http.StatusOK, `<div class="alert alert-success alert-dismissible fade show"><i class="bi bi-check-circle me-2"></i>IP rule created successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// IPRuleEditForm renders the IP rule edit form (HTMX partial).
// GET /gui/ip-rules/:id/edit
func (h *GUIHandler) IPRuleEditForm(c *gin.Context) {
	if h.IPRuleRepo == nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rules feature is not configured.</div>`)
		return
	}

	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Invalid rule ID.</div>`)
		return
	}

	rule, err := h.IPRuleRepo.GetByID(ruleID)
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rule not found.</div>`)
		return
	}

	type formData struct {
		IsEdit      bool
		AppID       string
		ID          string
		RuleType    string
		MatchType   string
		Value       string
		Description string
		IsActive    bool
	}

	c.HTML(http.StatusOK, "ip_rule_form", formData{
		IsEdit:      true,
		AppID:       rule.AppID.String(),
		ID:          rule.ID.String(),
		RuleType:    rule.RuleType,
		MatchType:   rule.MatchType,
		Value:       rule.Value,
		Description: rule.Description,
		IsActive:    rule.IsActive,
	})
}

// IPRuleUpdate handles IP rule update.
// PUT /gui/ip-rules/:id
func (h *GUIHandler) IPRuleUpdate(c *gin.Context) {
	if h.IPRuleRepo == nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rules feature is not configured.</div>`)
		return
	}

	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Invalid rule ID.</div>`)
		return
	}

	rule, err := h.IPRuleRepo.GetByID(ruleID)
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rule not found.</div>`)
		return
	}

	rule.RuleType = c.PostForm("rule_type")
	rule.MatchType = c.PostForm("match_type")
	rule.Value = strings.TrimSpace(c.PostForm("value"))
	rule.Description = strings.TrimSpace(c.PostForm("description"))
	rule.IsActive = c.PostForm("is_active") == "on"

	if err := geoip.ValidateRule(rule); err != nil {
		c.String(http.StatusOK, fmt.Sprintf(`<div class="alert alert-danger">%s</div>`, escapeHTML(err.Error())))
		return
	}

	if err := h.IPRuleRepo.Update(rule); err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Failed to update IP rule.</div>`)
		return
	}

	if h.IPRuleEvaluator != nil {
		h.IPRuleEvaluator.InvalidateCache(rule.AppID)
	}

	c.Header("HX-Trigger", "ipRuleListRefresh")
	c.String(http.StatusOK, `<div class="alert alert-success alert-dismissible fade show"><i class="bi bi-check-circle me-2"></i>IP rule updated successfully.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
}

// IPRuleDeleteConfirm renders the IP rule delete confirmation (HTMX partial).
// GET /gui/ip-rules/:id/delete
func (h *GUIHandler) IPRuleDeleteConfirm(c *gin.Context) {
	if h.IPRuleRepo == nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rules feature is not configured.</div>`)
		return
	}

	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Invalid rule ID.</div>`)
		return
	}

	rule, err := h.IPRuleRepo.GetByID(ruleID)
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rule not found.</div>`)
		return
	}

	c.HTML(http.StatusOK, "ip_rule_delete_confirm", rule)
}

// IPRuleDelete handles IP rule deletion.
// DELETE /gui/ip-rules/:id
func (h *GUIHandler) IPRuleDelete(c *gin.Context) {
	if h.IPRuleRepo == nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rules feature is not configured.</div>`)
		return
	}

	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Invalid rule ID.</div>`)
		return
	}

	rule, err := h.IPRuleRepo.GetByID(ruleID)
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">IP rule not found.</div>`)
		return
	}

	appID := rule.AppID

	if err := h.IPRuleRepo.Delete(ruleID); err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Failed to delete IP rule.</div>`)
		return
	}

	if h.IPRuleEvaluator != nil {
		h.IPRuleEvaluator.InvalidateCache(appID)
	}

	c.Header("HX-Trigger", "ipRuleDeleted")

	// Return the updated list
	rules, _ := h.IPRuleRepo.ListAllByApp(appID)
	type ruleListData struct {
		Rules []models.IPRule
		AppID string
		Total int
	}
	c.HTML(http.StatusOK, "ip_rule_list", ruleListData{
		Rules: rules,
		AppID: appID.String(),
		Total: len(rules),
	})
}

// IPRuleFormCancel handles IP rule form cancellation (HTMX partial).
// GET /gui/ip-rules/form-cancel
func (h *GUIHandler) IPRuleFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// IPRuleCheckAccess checks IP access for testing purposes (HTMX partial).
// POST /gui/ip-rules/check
func (h *GUIHandler) IPRuleCheckAccess(c *gin.Context) {
	if h.IPRuleEvaluator == nil {
		c.String(http.StatusOK, `<div class="alert alert-warning">IP rules feature is not configured.</div>`)
		return
	}

	appIDStr := c.PostForm("app_id")
	ipAddress := strings.TrimSpace(c.PostForm("ip_address"))

	if appIDStr == "" || ipAddress == "" {
		c.String(http.StatusOK, `<div class="alert alert-warning">Please provide both an application and an IP address.</div>`)
		return
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.String(http.StatusOK, `<div class="alert alert-danger">Invalid application ID.</div>`)
		return
	}

	result := h.IPRuleEvaluator.EvaluateAccess(appID, ipAddress)

	// Build a nice result display
	var icon, alertClass, statusText string
	if result.Allowed {
		icon = "bi-check-circle-fill"
		alertClass = "alert-success"
		statusText = "Allowed"
	} else {
		icon = "bi-x-circle-fill"
		alertClass = "alert-danger"
		statusText = "Blocked"
	}

	locationInfo := ""
	if result.GeoInfo != nil {
		locationInfo = result.GeoInfo.String()
		if result.GeoInfo.Country != "" {
			locationInfo += " (" + result.GeoInfo.Country + ")"
		}
	}

	html := fmt.Sprintf(`<div class="alert %s alert-dismissible fade show">
		<i class="bi %s me-2"></i>
		<strong>%s</strong> &mdash; %s
		<br><small class="text-muted">Reason: %s%s</small>
		<button type="button" class="btn-close" data-bs-dismiss="alert"></button>
	</div>`, alertClass, icon, ipAddress, statusText, escapeHTML(result.Reason),
		func() string {
			if locationInfo != "" {
				return " | Location: " + escapeHTML(locationInfo)
			}
			return ""
		}())

	c.String(http.StatusOK, html)
}

// ============================================================================
// Webhook GUI handlers
// ============================================================================

// WebhookPage renders the webhooks management page.
// GET /gui/webhooks
func (h *GUIHandler) WebhookPage(c *gin.Context) {
	if h.WebhookService == nil {
		c.HTML(http.StatusServiceUnavailable, "error", gin.H{"Error": "Webhook service is not available"})
		return
	}
	c.HTML(http.StatusOK, "webhooks", gin.H{
		"ActivePage":    "webhooks",
		"AdminUsername": getAdminUsername(c),
		"CSRFToken":     getCSRFToken(c),
	})
}

// WebhookList returns the paginated webhook endpoints list partial (HTMX fragment).
// GET /gui/webhooks/list
func (h *GUIHandler) WebhookList(c *gin.Context) {
	if h.WebhookService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">Webhook service unavailable</div>`)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	appIDStr := c.Query("app_id")

	var endpoints []models.WebhookEndpoint
	var total int64
	var err error

	if appIDStr != "" {
		appID, parseErr := uuid.Parse(appIDStr)
		if parseErr != nil {
			c.String(http.StatusBadRequest, `<div class="alert alert-danger">Invalid app ID</div>`)
			return
		}
		endpoints, total, err = h.WebhookService.ListEndpointsByApp(appID, page, 20)
	} else {
		endpoints, total, err = h.WebhookService.ListAllEndpoints(page, 20)
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "webhook_list", gin.H{
			"Endpoints": nil,
			"Error":     "Failed to load webhook endpoints",
		})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(20)))
	apps, _ := h.Repo.ListAllAppsWithTenantName()

	c.HTML(http.StatusOK, "webhook_list", gin.H{
		"Endpoints":  endpoints,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"AppID":      appIDStr,
		"Apps":       apps,
		"CSRFToken":  getCSRFToken(c),
	})
}

// WebhookCreateForm returns the webhook creation form HTML fragment.
// GET /gui/webhooks/new
func (h *GUIHandler) WebhookCreateForm(c *gin.Context) {
	if h.WebhookService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">Webhook service unavailable</div>`)
		return
	}
	apps, err := h.Repo.ListAllAppsWithTenantName()
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load applications.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}
	c.HTML(http.StatusOK, "webhook_form", gin.H{
		"Apps":       apps,
		"EventTypes": models.ValidEventTypes,
		"CSRFToken":  getCSRFToken(c),
	})
}

// WebhookCreate handles creating a new webhook endpoint.
// POST /gui/webhooks
func (h *GUIHandler) WebhookCreate(c *gin.Context) {
	if h.WebhookService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">Webhook service unavailable</div>`)
		return
	}

	appIDStr := strings.TrimSpace(c.PostForm("app_id"))
	eventType := strings.TrimSpace(c.PostForm("event_type"))
	url := strings.TrimSpace(c.PostForm("url"))

	if appIDStr == "" || eventType == "" || url == "" {
		c.HTML(http.StatusBadRequest, "webhook_form", gin.H{
			"Error":      "App, event type, and URL are required",
			"EventTypes": models.ValidEventTypes,
			"CSRFToken":  getCSRFToken(c),
		})
		return
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.HTML(http.StatusBadRequest, "webhook_form", gin.H{
			"Error":      "Invalid application ID",
			"EventTypes": models.ValidEventTypes,
			"CSRFToken":  getCSRFToken(c),
		})
		return
	}

	_, secret, svcErr := h.WebhookService.RegisterEndpoint(appID, eventType, url)
	if svcErr != nil {
		apps, _ := h.Repo.ListAllAppsWithTenantName()
		c.HTML(http.StatusBadRequest, "webhook_form", gin.H{
			"Error":      svcErr.Error(),
			"Apps":       apps,
			"EventTypes": models.ValidEventTypes,
			"CSRFToken":  getCSRFToken(c),
		})
		return
	}

	// Show the secret once — it will never be shown again
	c.HTML(http.StatusOK, "webhook_created", gin.H{
		"Secret":    secret,
		"CSRFToken": getCSRFToken(c),
	})
}

// WebhookFormCancel returns an empty fragment to collapse the form.
// GET /gui/webhooks/form-cancel
func (h *GUIHandler) WebhookFormCancel(c *gin.Context) {
	c.String(http.StatusOK, "")
}

// WebhookDeleteConfirm renders the delete confirmation fragment.
// GET /gui/webhooks/:id/delete
func (h *GUIHandler) WebhookDeleteConfirm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest, `<div class="alert alert-danger">Invalid webhook ID</div>`)
		return
	}

	ep, svcErr := h.WebhookService.GetEndpoint(id)
	if svcErr != nil || ep == nil {
		c.String(http.StatusNotFound, `<div class="alert alert-danger">Webhook endpoint not found</div>`)
		return
	}

	c.HTML(http.StatusOK, "webhook_delete_confirm", gin.H{
		"Endpoint":  ep,
		"CSRFToken": getCSRFToken(c),
	})
}

// WebhookDelete deletes a webhook endpoint.
// DELETE /gui/webhooks/:id
func (h *GUIHandler) WebhookDelete(c *gin.Context) {
	if h.WebhookService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">Webhook service unavailable</div>`)
		return
	}
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest, `<div class="alert alert-danger">Invalid webhook ID</div>`)
		return
	}
	if err := h.WebhookService.DeleteEndpoint(id); err != nil {
		c.String(http.StatusInternalServerError, `<div class="alert alert-danger">Failed to delete webhook endpoint</div>`)
		return
	}
	c.String(http.StatusOK, "")
}

// WebhookToggle toggles the active state of a webhook endpoint.
// PUT /gui/webhooks/:id/toggle
func (h *GUIHandler) WebhookToggle(c *gin.Context) {
	if h.WebhookService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">Webhook service unavailable</div>`)
		return
	}
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest, `<div class="alert alert-danger">Invalid webhook ID</div>`)
		return
	}
	active := c.PostForm("active") == "true"
	if err := h.WebhookService.SetEndpointActive(id, active); err != nil {
		c.String(http.StatusInternalServerError, `<div class="alert alert-danger">Failed to update webhook endpoint</div>`)
		return
	}
	// Return updated badge
	if active {
		c.String(http.StatusOK, `<span class="badge bg-success">Active</span>`)
	} else {
		c.String(http.StatusOK, `<span class="badge bg-secondary">Inactive</span>`)
	}
}

// WebhookDeliveries renders the delivery log page for a webhook endpoint.
// GET /gui/webhooks/:id/deliveries
func (h *GUIHandler) WebhookDeliveries(c *gin.Context) {
	if h.WebhookService == nil {
		c.String(http.StatusServiceUnavailable, `<div class="alert alert-danger">Webhook service unavailable</div>`)
		return
	}
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.String(http.StatusBadRequest, `<div class="alert alert-danger">Invalid webhook ID</div>`)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	deliveries, total, svcErr := h.WebhookService.ListDeliveriesByEndpoint(id, page, 20)
	if svcErr != nil {
		c.HTML(http.StatusInternalServerError, "webhook_deliveries", gin.H{
			"Error": "Failed to load delivery history",
		})
		return
	}

	ep, _ := h.WebhookService.GetEndpoint(id)
	totalPages := int(math.Ceil(float64(total) / float64(20)))

	c.HTML(http.StatusOK, "webhook_deliveries", gin.H{
		"Endpoint":   ep,
		"Deliveries": deliveries,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"CSRFToken":  getCSRFToken(c),
	})
}

// ============================================================
// My Account — Backup Email
// ============================================================

// MyAccountBackupEmailStatus returns the current backup email status as an HTML partial.
// GET /gui/my-account/backup-email/status
func (h *GUIHandler) MyAccountBackupEmailStatus(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	account, err := h.AccountService.Repo.GetByID(adminID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2 small">Failed to load account.</div>`)
		return
	}
	c.HTML(http.StatusOK, "admin_backup_email_status", gin.H{
		"BackupEmail":         account.BackupEmail,
		"BackupEmailVerified": account.BackupEmailVerified,
		"CSRFToken":           c.GetString(web.CSRFTokenKey),
	})
}

// MyAccountSetBackupEmail sets (or updates) the backup email for the admin account.
// POST /gui/my-account/backup-email
func (h *GUIHandler) MyAccountSetBackupEmail(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)
	backupEmail := strings.TrimSpace(c.PostForm("backup_email"))

	if backupEmail == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger py-2 small">Backup email address is required.</div>`)
		return
	}

	if err := h.AccountService.Repo.SetBackupEmail(adminID, backupEmail); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2 small">Failed to update backup email.</div>`)
		return
	}

	c.String(http.StatusOK,
		fmt.Sprintf(`<div class="alert alert-success py-2 small">Backup email set to %s. Note: admin account backup email verification is not required.</div>`, backupEmail))
}

// MyAccountRemoveBackupEmail removes the backup email from the admin account.
// DELETE /gui/my-account/backup-email
func (h *GUIHandler) MyAccountRemoveBackupEmail(c *gin.Context) {
	adminID := c.GetString(web.GUIAdminIDKey)

	if err := h.AccountService.Repo.ClearBackupEmail(adminID); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2 small">Failed to remove backup email.</div>`)
		return
	}

	// Refresh the backup email status section
	c.Header("HX-Trigger", "backupEmailChanged")
	c.String(http.StatusOK,
		`<div class="alert alert-success py-2 small">Backup email removed.</div>`)
}

// ============================================================
// My Account — Trusted Devices (admin's own user-level devices)
// ============================================================

// MyAccountTrustedDevices lists the trusted devices belonging to the logged-in admin
// across all apps (the admin may also be a regular user in some apps).
// GET /gui/my-account/trusted-devices
func (h *GUIHandler) MyAccountTrustedDevices(c *gin.Context) {
	if h.TrustedDeviceRepo == nil {
		c.HTML(http.StatusOK, "admin_trusted_devices", gin.H{
			"Devices":   nil,
			"CSRFToken": c.GetString(web.CSRFTokenKey),
		})
		return
	}
	adminID := c.GetString(web.GUIAdminIDKey)
	adminUUID, err := uuid.Parse(adminID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid admin ID.")
		return
	}
	devices, err := h.TrustedDeviceRepo.FindAllForUser(adminUUID)
	if err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2 small">Failed to load trusted devices.</div>`)
		return
	}
	c.HTML(http.StatusOK, "admin_trusted_devices", gin.H{
		"Devices":   devices,
		"CSRFToken": c.GetString(web.CSRFTokenKey),
	})
}

// MyAccountRevokeTrustedDevice revokes one of the admin's own trusted devices.
// DELETE /gui/my-account/trusted-devices/:device_id
func (h *GUIHandler) MyAccountRevokeTrustedDevice(c *gin.Context) {
	if h.TrustedDeviceRepo == nil {
		c.String(http.StatusServiceUnavailable, "Trusted device feature is disabled.")
		return
	}
	deviceIDStr := c.Param("device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid device ID.")
		return
	}
	if err := h.TrustedDeviceRepo.DeleteByID(deviceID); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger py-2 small">Failed to revoke trusted device.</div>`)
		return
	}
	// Trigger a list refresh
	c.Header("HX-Trigger", "trustedDeviceRevoked")
	c.String(http.StatusOK, `<span class="badge bg-success bg-opacity-10 text-success">Revoked</span>`)
}

// ============================================================
// User Export / Import (Admin GUI)
// ============================================================

// UserExport streams all users as a downloadable CSV or JSON file.
// GET /gui/users/export?format=csv|json&app_id=&search=
func (h *GUIHandler) UserExport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	if format != "csv" && format != "json" {
		format = "csv"
	}
	appID := c.Query("app_id")
	search := c.Query("search")

	items, truncated, err := h.Repo.ExportUsers(appID, search)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to export users")
		return
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	truncatedVal := "false"
	if truncated {
		truncatedVal = "true"
	}
	c.Header("X-Export-Truncated", truncatedVal)

	switch format {
	case "json":
		filename := fmt.Sprintf("users_%s.json", timestamp)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Writer.WriteHeader(http.StatusOK)

		type jsonExport struct {
			Data       []UserExportItem `json:"data"`
			Count      int              `json:"count"`
			Truncated  bool             `json:"truncated"`
			ExportedAt string           `json:"exported_at"`
		}
		enc := json.NewEncoder(c.Writer)
		enc.SetIndent("", "  ")
		_ = enc.Encode(jsonExport{
			Data:       items,
			Count:      len(items),
			Truncated:  truncated,
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
		})

	default: // csv
		filename := fmt.Sprintf("users_%s.csv", timestamp)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Writer.WriteHeader(http.StatusOK)
		_, _ = c.Writer.Write([]byte("\xef\xbb\xbf")) // UTF-8 BOM for Excel compatibility
		writeUserCSVGUI(c.Writer, items)
	}
}

// UserImportModal returns the import modal partial for the Users page.
// GET /gui/users/import/modal
func (h *GUIHandler) UserImportModal(c *gin.Context) {
	appID := c.Query("app_id")
	csrfToken, _ := c.Get(web.CSRFTokenKey)
	c.HTML(http.StatusOK, "user_import_modal", gin.H{
		"AppID":     appID,
		"CSRFToken": csrfToken,
	})
}

// UserImport processes an uploaded CSV or JSON file and bulk-creates users.
// POST /gui/users/import  (multipart/form-data)
func (h *GUIHandler) UserImport(c *gin.Context) {
	renderErr := func(msg string) {
		c.HTML(http.StatusOK, "user_import_result", gin.H{
			"Result": dto.UserImportResult{
				Errors: []dto.UserImportRowError{{Error: msg}},
			},
			"HasErrors": true,
		})
	}

	appID := strings.TrimSpace(c.PostForm("app_id"))
	if appID == "" {
		renderErr("Application ID is required. Select an application before importing.")
		return
	}

	// Enforce 10 MB upload limit
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		renderErr("File too large or invalid form submission (max 10 MB)")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		renderErr("No file uploaded. Please select a CSV or JSON file.")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var rows []dto.UserImportRow
	var parseErrors []dto.UserImportRowError

	switch ext {
	case ".json":
		rows, parseErrors = userimport.ParseJSONImport(file)
	default: // .csv or unrecognised extension
		rows, parseErrors = userimport.ParseCSVImport(file)
	}

	result, err := h.Repo.ImportUsers(appID, rows)
	if err != nil {
		renderErr("Import failed: " + err.Error())
		return
	}

	// Prepend parse/validation errors (they come first so row numbers are meaningful)
	result.Errors = append(parseErrors, result.Errors...)
	result.Total += len(parseErrors)

	// Log a USER_CREATED activity event for each successfully imported user.
	// We use LogRegister because imported users are newly created accounts.
	appUUID, parseErr := uuid.Parse(appID)
	if parseErr == nil && result.Imported > 0 {
		ipAddress := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		// Re-query the just-created user IDs so we can attach them to log entries.
		// We do a best-effort lookup — logging failures are not fatal.
		var createdUsers []struct {
			ID    uuid.UUID
			Email string
		}
		emails := make([]string, 0, len(rows))
		for _, row := range rows {
			emails = append(emails, strings.ToLower(strings.TrimSpace(row.Email)))
		}
		if len(emails) > 0 {
			_ = h.Repo.DB.Model(&models.User{}).
				Select("id, email").
				Where("email IN ? AND app_id = ?", emails, appID).
				Scan(&createdUsers).Error
		}
		for _, u := range createdUsers {
			logService.LogRegister(appUUID, u.ID, ipAddress, userAgent, u.Email)
		}
	}

	c.HTML(http.StatusOK, "user_import_result", gin.H{
		"Result":    result,
		"HasErrors": len(result.Errors) > 0,
	})
}

// writeUserCSVGUI encodes a slice of UserExportItem as CSV rows into w.
// The first row is the header. Used exclusively by the GUI UserExport handler.
func writeUserCSVGUI(w io.Writer, items []UserExportItem) {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"id", "app_id", "email", "name", "first_name", "last_name",
		"locale", "email_verified", "is_active",
		"two_fa_enabled", "two_fa_method", "social_providers",
		"created_at", "updated_at",
	})
	for _, item := range items {
		_ = cw.Write([]string{
			item.ID.String(),
			item.AppID.String(),
			item.Email,
			item.Name,
			item.FirstName,
			item.LastName,
			item.Locale,
			fmt.Sprintf("%t", item.EmailVerified),
			fmt.Sprintf("%t", item.IsActive),
			fmt.Sprintf("%t", item.TwoFAEnabled),
			item.TwoFAMethod,
			item.SocialProviders,
			item.CreatedAt.UTC().Format(time.RFC3339),
			item.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	cw.Flush()
}
