package admin

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/gjovanovicst/auth_api/web"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// GUIHandler serves HTML pages for the Admin GUI.
// Separate from Handler (which serves the JSON admin API).
type GUIHandler struct {
	AccountService   *AccountService
	DashboardService *DashboardService
	Repo             *Repository
	SettingsService  *SettingsService
	EmailService     *email.Service
}

// NewGUIHandler creates a new GUIHandler
func NewGUIHandler(accountService *AccountService, dashboardService *DashboardService, repo *Repository, settingsService *SettingsService, emailService *email.Service) *GUIHandler {
	return &GUIHandler{
		AccountService:   accountService,
		DashboardService: dashboardService,
		Repo:             repo,
		SettingsService:  settingsService,
		EmailService:     emailService,
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
			Error:    "Username and password are required.",
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
			Error:    "Invalid username or password.",
			Username: username,
			Redirect: redirect,
		}
		c.HTML(http.StatusUnauthorized, "login", data)
		return
	}

	// Create session
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
		ID              string
		Name            string
		Description     string
		TenantID        string
		TwoFAIssuerName string
		TwoFAEnabled    bool
		TwoFARequired   bool
		Tenants         []models.Tenant
		IsEdit          bool
	}
	c.HTML(http.StatusOK, "app_form", formData{
		TwoFAEnabled: true, // Default: 2FA enabled for new apps
		Tenants:      tenants,
	})
}

// AppCreate handles creating a new application.
// POST /gui/applications
func (h *GUIHandler) AppCreate(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	tenantID := c.PostForm("tenant_id")
	twoFAIssuerName := strings.TrimSpace(c.PostForm("two_fa_issuer_name"))
	twoFAEnabled := c.PostForm("two_fa_enabled") == "on"
	twoFARequired := c.PostForm("two_fa_required") == "on"

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
		TenantID:        parsedTenantID,
		Name:            name,
		Description:     description,
		TwoFAIssuerName: twoFAIssuerName,
		TwoFAEnabled:    twoFAEnabled,
		TwoFARequired:   twoFARequired,
	}
	if err := h.Repo.CreateApp(app); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to create application. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

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
		ID              string
		Name            string
		Description     string
		TenantID        string
		TwoFAIssuerName string
		TwoFAEnabled    bool
		TwoFARequired   bool
		Tenants         []models.Tenant
		IsEdit          bool
	}
	c.HTML(http.StatusOK, "app_form", formData{
		ID:              app.ID.String(),
		Name:            app.Name,
		Description:     app.Description,
		TenantID:        app.TenantID.String(),
		TwoFAIssuerName: app.TwoFAIssuerName,
		TwoFAEnabled:    app.TwoFAEnabled,
		TwoFARequired:   app.TwoFARequired,
		Tenants:         tenants,
		IsEdit:          true,
	})
}

// AppUpdate handles updating an application.
// PUT /gui/applications/:id
func (h *GUIHandler) AppUpdate(c *gin.Context) {
	id := c.Param("id")
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	twoFAIssuerName := strings.TrimSpace(c.PostForm("two_fa_issuer_name"))
	twoFAEnabled := c.PostForm("two_fa_enabled") == "on"
	twoFARequired := c.PostForm("two_fa_required") == "on"

	if name == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application name is required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if err := h.Repo.UpdateApp(id, name, description, twoFAIssuerName, twoFAEnabled, twoFARequired); err != nil {
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to update application. Please try again.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
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

	c.HTML(http.StatusOK, "user_detail", detail)
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

// ==================== Settings Management ====================

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
	}

	var items []serverItem
	for _, config := range allConfigs {
		appName := ""
		tenantName := ""
		if app, ok := appMap[config.AppID.String()]; ok {
			appName = app.Name
			tenantName = app.TenantName
		}
		items = append(items, serverItem{
			ID:          config.ID.String(),
			AppID:       config.AppID.String(),
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
		c.String(http.StatusInternalServerError,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Failed to load applications.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	if len(apps) == 0 {
		c.String(http.StatusOK,
			`<div class="alert alert-info alert-dismissible fade show" role="alert"><i class="bi bi-info-circle me-2"></i>No applications found. Create an application first.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
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

	if appIDStr == "" || smtpHost == "" || fromAddress == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Application, SMTP Host, and From Address are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid application ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	smtpPort, _ := strconv.Atoi(smtpPortStr)
	if smtpPort <= 0 {
		smtpPort = 587
	}

	if name == "" {
		name = "Default"
	}

	config := &models.EmailServerConfig{
		AppID:        appID,
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

	c.HTML(http.StatusOK, "email_server_form", gin.H{
		"IsEdit":       true,
		"ID":           found.ID.String(),
		"AppID":        found.AppID.String(),
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

	if appIDStr == "" || smtpHost == "" || fromAddress == "" {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">SMTP Host and From Address are required.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.String(http.StatusBadRequest,
			`<div class="alert alert-danger alert-dismissible fade show" role="alert">Invalid application ID.<button type="button" class="btn-close" data-bs-dismiss="alert"></button></div>`)
		return
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
		AppID:        appID,
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
		"IsEdit":   false,
		"IsActive": true,
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
		"IsEdit":         true,
		"ID":             emailType.ID.String(),
		"Code":           emailType.Code,
		"Name":           emailType.Name,
		"Description":    emailType.Description,
		"DefaultSubject": emailType.DefaultSubject,
		"IsSystem":       emailType.IsSystem,
		"IsActive":       emailType.IsActive,
		"Variables":      vars,
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

// parseVariablesFromForm parses variable rows from the dynamic form.
// Variables are submitted as var_name[], var_description[], var_required[] arrays.
func parseVariablesFromForm(c *gin.Context) []byte {
	names := c.PostFormArray("var_name[]")
	descriptions := c.PostFormArray("var_description[]")
	requireds := c.PostFormArray("var_required[]")

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
		vars = append(vars, models.EmailTypeVariable{
			Name:        name,
			Description: desc,
			Required:    required,
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
