package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

type Handler struct {
	Repo *Repository
}

func NewHandler(r *Repository) *Handler {
	return &Handler{Repo: r}
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
