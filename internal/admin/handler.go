package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

type Handler struct {
	Repo         *Repository
	EmailService *email.Service
}

func NewHandler(r *Repository, emailService *email.Service) *Handler {
	return &Handler{Repo: r, EmailService: emailService}
}

// CreateTenant creates a new tenant
// @Summary Create a new tenant
// @Description Register a new tenant organization in the system
// @Tags Admin
// @Accept json
// @Produce json
// @Param   tenant  body      dto.CreateTenantRequest  true  "Tenant Creation Data"
// @Success 201 {object} dto.TenantResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/tenants [post]
func (h *Handler) CreateTenant(c *gin.Context) {
	var req dto.CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	tenant := &models.Tenant{
		Name: req.Name,
	}

	if err := h.Repo.CreateTenant(tenant); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to create tenant"})
		return
	}

	c.JSON(http.StatusCreated, dto.TenantResponse{
		ID:        tenant.ID,
		Name:      tenant.Name,
		CreatedAt: tenant.CreatedAt,
		UpdatedAt: tenant.UpdatedAt,
	})
}

// ListTenants lists all tenants with pagination
// @Summary List all tenants
// @Description Retrieve a paginated list of all tenants
// @Tags Admin
// @Accept json
// @Produce json
// @Param   page       query     int     false  "Page number" default(1)
// @Param   page_size  query     int     false  "Page size" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/tenants [get]
func (h *Handler) ListTenants(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	tenants, total, err := h.Repo.ListTenants(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list tenants"})
		return
	}

	var response []dto.TenantResponse
	for _, t := range tenants {
		response = append(response, dto.TenantResponse{
			ID:        t.ID,
			Name:      t.Name,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        response,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// CreateApp creates a new application for a tenant
// @Summary Create a new application
// @Description Register a new application under a specific tenant
// @Tags Admin
// @Accept json
// @Produce json
// @Param   app  body      dto.CreateAppRequest  true  "Application Creation Data"
// @Success 201 {object} dto.AppResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps [post]
func (h *Handler) CreateApp(c *gin.Context) {
	var req dto.CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid Tenant ID"})
		return
	}

	app := &models.Application{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := h.Repo.CreateApp(app); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to create application"})
		return
	}

	c.JSON(http.StatusCreated, dto.AppResponse{
		ID:          app.ID,
		TenantID:    app.TenantID,
		Name:        app.Name,
		Description: app.Description,
		CreatedAt:   app.CreatedAt,
		UpdatedAt:   app.UpdatedAt,
	})
}

// GetAppDetails retrieves app details including OAuth configs
// @Summary Get application details
// @Description Retrieve details of a specific application including OAuth configurations
// @Tags Admin
// @Accept json
// @Produce json
// @Param   id   path      string  true  "Application ID"
// @Success 200 {object} dto.AppResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{id} [get]
func (h *Handler) GetAppDetails(c *gin.Context) {
	appID := c.Param("id")
	app, err := h.Repo.GetAppByID(appID)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Application not found"})
		return
	}

	// Simple mapping for now, ideally we'd map OAuth configs too in DTO
	c.JSON(http.StatusOK, dto.AppResponse{
		ID:          app.ID,
		TenantID:    app.TenantID,
		Name:        app.Name,
		Description: app.Description,
		CreatedAt:   app.CreatedAt,
		UpdatedAt:   app.UpdatedAt,
	})
}

// UpsertOAuthConfig creates or updates OAuth configuration for an app
// @Summary Set OAuth configuration
// @Description Configure OAuth provider credentials (Google, GitHub, etc.) for an application
// @Tags Admin
// @Accept json
// @Produce json
// @Param   id      path      string                      true  "Application ID"
// @Param   config  body      dto.UpsertOAuthConfigRequest true  "OAuth Config Data"
// @Success 200 {object} dto.OAuthConfigResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{id}/oauth-config [post]
func (h *Handler) UpsertOAuthConfig(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	var req dto.UpsertOAuthConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	config := &models.OAuthProviderConfig{
		AppID:        appID,
		Provider:     req.Provider,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURL:  req.RedirectURL,
		IsEnabled:    true,
	}

	if err := h.Repo.UpsertOAuthConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to save OAuth config"})
		return
	}

	c.JSON(http.StatusOK, dto.OAuthConfigResponse{
		ID:          config.ID,
		AppID:       config.AppID,
		Provider:    config.Provider,
		ClientID:    config.ClientID,
		RedirectURL: config.RedirectURL,
		IsEnabled:   config.IsEnabled,
		CreatedAt:   config.CreatedAt,
		UpdatedAt:   config.UpdatedAt,
	})
}

// ============================================================================
// Email Type Management
// ============================================================================

// ListEmailTypes returns all email types
// @Summary List all email types
// @Description Retrieve all registered email types
// @Tags Admin - Email
// @Produce json
// @Success 200 {array} dto.EmailTypeResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-types [get]
func (h *Handler) ListEmailTypes(c *gin.Context) {
	types, err := h.EmailService.GetAllEmailTypes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list email types"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": types})
}

// GetEmailType returns a single email type by code
// @Summary Get email type by code
// @Description Retrieve a specific email type by its code
// @Tags Admin - Email
// @Produce json
// @Param code path string true "Email Type Code"
// @Success 200 {object} dto.EmailTypeResponse
// @Failure 404 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-types/{code} [get]
func (h *Handler) GetEmailType(c *gin.Context) {
	code := c.Param("code")
	emailType, err := h.EmailService.GetEmailTypeByCode(code)
	if err != nil || emailType == nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Email type not found"})
		return
	}
	c.JSON(http.StatusOK, emailType)
}

// CreateEmailType creates a new custom email type
// @Summary Create a custom email type
// @Description Register a new custom email type for use in templates
// @Tags Admin - Email
// @Accept json
// @Produce json
// @Param emailType body dto.CreateEmailTypeRequest true "Email Type Data"
// @Success 201 {object} dto.EmailTypeResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-types [post]
func (h *Handler) CreateEmailType(c *gin.Context) {
	var req dto.CreateEmailTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if req.Code == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Code and name are required"})
		return
	}

	// Check for duplicate code
	existing, _ := h.EmailService.GetEmailTypeByCode(req.Code)
	if existing != nil {
		c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "An email type with this code already exists"})
		return
	}

	// Marshal variables to JSON
	var varsJSON []byte
	if len(req.Variables) > 0 {
		var err error
		varsJSON, err = json.Marshal(req.Variables)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid variables format"})
			return
		}
	}

	emailType := &models.EmailType{
		Code:           req.Code,
		Name:           req.Name,
		Description:    req.Description,
		DefaultSubject: req.DefaultSubject,
		Variables:      varsJSON,
		IsSystem:       false,
		IsActive:       true,
	}

	if err := h.EmailService.CreateEmailType(emailType); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to create email type"})
		return
	}

	c.JSON(http.StatusCreated, emailType)
}

// UpdateEmailType updates an existing email type
// @Summary Update an email type
// @Description Update an existing email type's name, description, subject, or variables
// @Tags Admin - Email
// @Accept json
// @Produce json
// @Param id path string true "Email Type ID"
// @Param emailType body dto.UpdateEmailTypeRequest true "Email Type Update Data"
// @Success 200 {object} dto.EmailTypeResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-types/{id} [put]
func (h *Handler) UpdateEmailType(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid email type ID"})
		return
	}

	emailType, err := h.EmailService.GetEmailTypeByID(id)
	if err != nil || emailType == nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Email type not found"})
		return
	}

	var req dto.UpdateEmailTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if req.Name != "" {
		emailType.Name = req.Name
	}
	if req.Description != "" {
		emailType.Description = req.Description
	}
	if req.DefaultSubject != "" {
		emailType.DefaultSubject = req.DefaultSubject
	}
	if req.Variables != nil {
		varsJSON, err := json.Marshal(req.Variables)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid variables format"})
			return
		}
		emailType.Variables = varsJSON
	}
	if req.IsActive != nil {
		emailType.IsActive = *req.IsActive
	}

	if err := h.EmailService.UpdateEmailType(emailType); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to update email type"})
		return
	}

	c.JSON(http.StatusOK, emailType)
}

// DeleteEmailType deletes a custom email type
// @Summary Delete a custom email type
// @Description Delete a custom email type (system types cannot be deleted)
// @Tags Admin - Email
// @Produce json
// @Param id path string true "Email Type ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-types/{id} [delete]
func (h *Handler) DeleteEmailType(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid email type ID"})
		return
	}

	if err := h.EmailService.DeleteEmailType(id); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email type deleted successfully"})
}

// SendCustomEmail sends an email of a specific type to a recipient
// @Summary Send an email
// @Description Send an email of the specified type using app's SMTP config and templates
// @Tags Admin - Email
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param request body dto.SendEmailRequest true "Send Email Data"
// @Success 200 {object} dto.SendEmailResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{id}/send-email [post]
func (h *Handler) SendCustomEmail(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	var req dto.SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if req.TypeCode == "" || req.ToEmail == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "type_code and to_email are required"})
		return
	}

	// Verify the email type exists and is active
	emailType, err := h.EmailService.GetEmailTypeByCode(req.TypeCode)
	if err != nil || emailType == nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Email type not found: " + req.TypeCode})
		return
	}
	if !emailType.IsActive {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Email type is not active: " + req.TypeCode})
		return
	}

	// Validate required variables (only enforce "explicit" source variables;
	// "user" and "setting" source variables are auto-resolved by the pipeline)
	if len(emailType.Variables) > 0 {
		var typeVars []models.EmailTypeVariable
		if err := json.Unmarshal(emailType.Variables, &typeVars); err == nil {
			for _, v := range typeVars {
				if v.Required && v.Source == models.VarSourceExplicit {
					if val, ok := req.Variables[v.Name]; !ok || val == "" {
						c.JSON(http.StatusBadRequest, dto.ErrorResponse{
							Error: "Missing required variable: " + v.Name,
						})
						return
					}
				}
			}
		}
	}

	vars := req.Variables
	if vars == nil {
		vars = make(map[string]string)
	}

	if err := h.EmailService.SendEmail(appID, req.TypeCode, req.ToEmail, vars); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to send email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.SendEmailResponse{
		Message:  "Email sent successfully",
		TypeCode: req.TypeCode,
		ToEmail:  req.ToEmail,
	})
}

// ============================================================================
// Email Server Config Management (App-scoped - legacy endpoints)
// ============================================================================

// GetEmailServerConfig returns the default SMTP config for an app
// @Summary Get default SMTP config for application
// @Description Retrieve the default SMTP server configuration for an application
// @Tags Admin - Email
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} dto.EmailServerConfigResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{id}/email-config [get]
func (h *Handler) GetEmailServerConfig(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	config, err := h.EmailService.GetServerConfig(appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to get email config"})
		return
	}
	if config == nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "No SMTP configuration found for this application"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// SaveEmailServerConfig creates or updates SMTP config for an app (legacy app-scoped)
// @Summary Save SMTP config for application
// @Description Create or update an SMTP server configuration for an application
// @Tags Admin - Email
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param config body dto.EmailServerConfigRequest true "SMTP Config"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{id}/email-config [put]
func (h *Handler) SaveEmailServerConfig(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	var req dto.EmailServerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	config := &models.EmailServerConfig{
		AppID:        appID,
		Name:         req.Name,
		SMTPHost:     req.SMTPHost,
		SMTPPort:     req.SMTPPort,
		SMTPUsername: req.SMTPUsername,
		SMTPPassword: req.SMTPPassword,
		FromAddress:  req.FromAddress,
		FromName:     req.FromName,
		UseTLS:       req.UseTLS,
		IsDefault:    req.IsDefault,
		IsActive:     req.IsActive,
	}

	if err := h.EmailService.SaveServerConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to save email config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email server configuration saved successfully"})
}

// DeleteEmailServerConfig removes all SMTP configs for an app (legacy app-scoped)
// @Summary Delete SMTP configs for application
// @Description Remove all SMTP server configurations for an application (falls back to global)
// @Tags Admin - Email
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{id}/email-config [delete]
func (h *Handler) DeleteEmailServerConfig(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	if err := h.EmailService.DeleteServerConfig(appID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to delete email config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email server configuration deleted"})
}

// ============================================================================
// Email Server Config Management (Config-level multi-config endpoints)
// ============================================================================

// ListAllEmailServerConfigs returns all SMTP configs across all apps
// @Summary List all SMTP configs
// @Description Retrieve all SMTP server configurations across all applications
// @Tags Admin - Email Servers
// @Produce json
// @Success 200 {array} dto.EmailServerConfigResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-servers [get]
func (h *Handler) ListAllEmailServerConfigs(c *gin.Context) {
	configs, err := h.EmailService.GetAllServerConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list email server configs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// GetEmailServerConfigByID returns a single SMTP config by its ID
// @Summary Get SMTP config by ID
// @Description Retrieve a specific SMTP server configuration by its ID
// @Tags Admin - Email Servers
// @Produce json
// @Param id path string true "Config ID"
// @Success 200 {object} dto.EmailServerConfigResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-servers/{id} [get]
func (h *Handler) GetEmailServerConfigByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid config ID"})
		return
	}

	config, err := h.EmailService.GetServerConfigByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to get email server config"})
		return
	}
	if config == nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Email server config not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// ListEmailServerConfigsByApp returns all SMTP configs for a specific app
// @Summary List SMTP configs for application
// @Description Retrieve all SMTP server configurations for a specific application
// @Tags Admin - Email Servers
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {array} dto.EmailServerConfigResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{id}/email-servers [get]
func (h *Handler) ListEmailServerConfigsByApp(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	configs, err := h.EmailService.GetServerConfigsByApp(appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list email server configs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// CreateEmailServerConfig creates a new SMTP config
// @Summary Create SMTP config
// @Description Create a new SMTP server configuration for an application
// @Tags Admin - Email Servers
// @Accept json
// @Produce json
// @Param app_id query string true "Application ID"
// @Param config body dto.EmailServerConfigRequest true "SMTP Config"
// @Success 201 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-servers [post]
func (h *Handler) CreateEmailServerConfig(c *gin.Context) {
	appIDStr := c.Query("app_id")
	if appIDStr == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "app_id query parameter is required"})
		return
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	var req dto.EmailServerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	config := &models.EmailServerConfig{
		AppID:        appID,
		Name:         req.Name,
		SMTPHost:     req.SMTPHost,
		SMTPPort:     req.SMTPPort,
		SMTPUsername: req.SMTPUsername,
		SMTPPassword: req.SMTPPassword,
		FromAddress:  req.FromAddress,
		FromName:     req.FromName,
		UseTLS:       req.UseTLS,
		IsDefault:    req.IsDefault,
		IsActive:     req.IsActive,
	}

	if err := h.EmailService.SaveServerConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to create email server config"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Email server configuration created successfully", "id": config.ID.String()})
}

// UpdateEmailServerConfigByID updates an existing SMTP config by ID
// @Summary Update SMTP config
// @Description Update an existing SMTP server configuration by its ID
// @Tags Admin - Email Servers
// @Accept json
// @Produce json
// @Param id path string true "Config ID"
// @Param config body dto.EmailServerConfigRequest true "SMTP Config"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-servers/{id} [put]
func (h *Handler) UpdateEmailServerConfigByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid config ID"})
		return
	}

	// Fetch existing config to get the AppID and preserve password if not provided
	existing, err := h.EmailService.GetServerConfigByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to get email server config"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Email server config not found"})
		return
	}

	var req dto.EmailServerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	existing.Name = req.Name
	existing.SMTPHost = req.SMTPHost
	existing.SMTPPort = req.SMTPPort
	existing.SMTPUsername = req.SMTPUsername
	if req.SMTPPassword != "" {
		existing.SMTPPassword = req.SMTPPassword
	}
	existing.FromAddress = req.FromAddress
	existing.FromName = req.FromName
	existing.UseTLS = req.UseTLS
	existing.IsDefault = req.IsDefault
	existing.IsActive = req.IsActive

	if err := h.EmailService.SaveServerConfig(existing); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to update email server config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email server configuration updated successfully"})
}

// DeleteEmailServerConfigByID removes a single SMTP config by its ID
// @Summary Delete SMTP config by ID
// @Description Remove a specific SMTP server configuration by its ID
// @Tags Admin - Email Servers
// @Produce json
// @Param id path string true "Config ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-servers/{id} [delete]
func (h *Handler) DeleteEmailServerConfigByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid config ID"})
		return
	}

	if err := h.EmailService.DeleteServerConfigByID(id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to delete email server config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email server configuration deleted"})
}

// SendTestEmailByConfigID sends a test email using a specific SMTP config
// @Summary Send test email by config ID
// @Description Send a test email to verify a specific SMTP configuration
// @Tags Admin - Email Servers
// @Accept json
// @Produce json
// @Param id path string true "Config ID"
// @Param request body dto.EmailTestRequest true "Test Email Data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-servers/{id}/test [post]
func (h *Handler) SendTestEmailByConfigID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid config ID"})
		return
	}

	var req dto.EmailTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.EmailService.SendTestEmailWithConfigID(id, req.ToEmail); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to send test email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test email sent successfully"})
}

// ============================================================================
// Email Template Management
// ============================================================================

// ListEmailTemplates returns all templates for an app or global defaults
// @Summary List email templates
// @Description Retrieve email templates for a specific app or global defaults
// @Tags Admin - Email
// @Produce json
// @Param app_id query string false "Application ID (omit for global defaults)"
// @Success 200 {array} dto.EmailTemplateResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-templates [get]
func (h *Handler) ListEmailTemplates(c *gin.Context) {
	appIDStr := c.Query("app_id")

	if appIDStr == "" {
		templates, err := h.EmailService.GetGlobalDefaultTemplates()
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list templates"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": templates})
		return
	}

	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	templates, err := h.EmailService.GetTemplatesByApp(appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": templates})
}

// GetEmailTemplate returns a single template by ID
// @Summary Get email template
// @Description Retrieve a specific email template by ID
// @Tags Admin - Email
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} dto.EmailTemplateResponse
// @Failure 404 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-templates/{id} [get]
func (h *Handler) GetEmailTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid template ID"})
		return
	}

	tmpl, err := h.EmailService.GetTemplateByID(id)
	if err != nil || tmpl == nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Template not found"})
		return
	}

	c.JSON(http.StatusOK, tmpl)
}

// SaveEmailTemplate creates or updates an email template
// @Summary Save email template
// @Description Create or update an email template for a specific app or as global default
// @Tags Admin - Email
// @Accept json
// @Produce json
// @Param app_id query string false "Application ID (omit for global default)"
// @Param email_type_id query string true "Email Type ID"
// @Param template body dto.EmailTemplateRequest true "Template Data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-templates [post]
func (h *Handler) SaveEmailTemplate(c *gin.Context) {
	appIDStr := c.Query("app_id")
	emailTypeIDStr := c.Query("email_type_id")

	if emailTypeIDStr == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "email_type_id is required"})
		return
	}

	emailTypeID, err := uuid.Parse(emailTypeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid email_type_id"})
		return
	}

	var req dto.EmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	tmpl := &models.EmailTemplate{
		Name:           req.Name,
		Subject:        req.Subject,
		BodyHTML:       req.BodyHTML,
		BodyText:       req.BodyText,
		TemplateEngine: req.TemplateEngine,
		IsActive:       req.IsActive,
	}

	if appIDStr == "" {
		// Global default
		if err := h.EmailService.SaveGlobalTemplate(emailTypeID, tmpl); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to save template"})
			return
		}
	} else {
		appID, err := uuid.Parse(appIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app_id"})
			return
		}
		if err := h.EmailService.SaveAppTemplate(appID, emailTypeID, tmpl); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to save template"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email template saved successfully"})
}

// DeleteEmailTemplate removes an email template
// @Summary Delete email template
// @Description Remove an email template by ID
// @Tags Admin - Email
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-templates/{id} [delete]
func (h *Handler) DeleteEmailTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid template ID"})
		return
	}

	if err := h.EmailService.DeleteTemplate(id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email template deleted"})
}

// PreviewEmailTemplate renders a template with sample data
// @Summary Preview email template
// @Description Render a template with sample variables for preview
// @Tags Admin - Email
// @Accept json
// @Produce json
// @Param preview body dto.EmailPreviewRequest true "Preview Data"
// @Success 200 {object} dto.EmailPreviewResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/email-templates/preview [post]
func (h *Handler) PreviewEmailTemplate(c *gin.Context) {
	var req dto.EmailPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	tmpl := &models.EmailTemplate{
		Subject:        req.Subject,
		BodyHTML:       req.BodyHTML,
		BodyText:       req.BodyText,
		TemplateEngine: req.TemplateEngine,
	}

	subject, htmlBody, textBody, err := h.EmailService.PreviewTemplate(tmpl, req.Variables)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to preview template: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.EmailPreviewResponse{
		Subject:  subject,
		BodyHTML: htmlBody,
		BodyText: textBody,
	})
}

// SendTestEmail sends a test email using an app's default SMTP configuration
// @Summary Send test email (app-scoped)
// @Description Send a test email to verify the default SMTP configuration for an application
// @Tags Admin - Email
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param request body dto.EmailTestRequest true "Test Email Data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{id}/email-test [post]
func (h *Handler) SendTestEmail(c *gin.Context) {
	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid App ID"})
		return
	}

	var req dto.EmailTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.EmailService.SendTestEmail(appID, req.ToEmail); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to send test email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test email sent successfully"})
}

// ListWellKnownVariables returns the list of all variables the system can auto-resolve.
// @Summary List well-known email template variables
// @Description Returns all variables that the system can automatically resolve from user profiles, app settings, or that must be passed explicitly. Use this as a reference when adding variables to email types.
// @Tags Admin - Email
// @Produce json
// @Success 200 {array} dto.EmailTypeVariableResponse
// @Security AdminApiKey
// @Router /admin/email-variables [get]
func (h *Handler) ListWellKnownVariables(c *gin.Context) {
	wellKnown := h.EmailService.GetWellKnownVariables()

	response := make([]dto.EmailTypeVariableResponse, len(wellKnown))
	for i, v := range wellKnown {
		response[i] = dto.EmailTypeVariableResponse{
			Name:         v.Name,
			Description:  v.Description,
			Required:     v.Required,
			DefaultValue: v.DefaultValue,
			Source:       v.Source,
		}
	}

	c.JSON(http.StatusOK, response)
}
