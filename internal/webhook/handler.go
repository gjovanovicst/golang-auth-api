package webhook

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

// Handler exposes webhook management endpoints.
// It serves both the Admin API (X-Admin-API-Key) and the App API (X-App-API-Key).
type Handler struct {
	Service *Service
}

// NewHandler creates a new webhook handler.
func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}

// toEndpointResponse converts a model to its DTO representation.
func toEndpointResponse(ep models.WebhookEndpoint) dto.WebhookEndpointResponse {
	return dto.WebhookEndpointResponse{
		ID:        ep.ID.String(),
		AppID:     ep.AppID.String(),
		EventType: ep.EventType,
		URL:       ep.URL,
		IsActive:  ep.IsActive,
		CreatedAt: ep.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: ep.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toDeliveryResponse(d models.WebhookDelivery) dto.WebhookDeliveryResponse {
	r := dto.WebhookDeliveryResponse{
		ID:           d.ID.String(),
		EndpointID:   d.EndpointID.String(),
		AppID:        d.AppID.String(),
		EventType:    d.EventType,
		Attempt:      d.Attempt,
		StatusCode:   d.StatusCode,
		ResponseBody: d.ResponseBody,
		LatencyMs:    d.LatencyMs,
		Success:      d.Success,
		ErrorMessage: d.ErrorMessage,
		CreatedAt:    d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if d.NextRetryAt != nil {
		s := d.NextRetryAt.UTC().Format("2006-01-02T15:04:05Z")
		r.NextRetryAt = &s
	}
	return r
}

func parsePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// ============================================================================
// Admin API endpoints (X-Admin-API-Key)
// ============================================================================

// AdminListEndpoints lists all webhook endpoints across all apps.
// @Summary List all webhook endpoints (admin)
// @Description Returns all registered webhook endpoints across all applications
// @Tags Webhooks
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.WebhookEndpointListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/webhooks [get]
func (h *Handler) AdminListEndpoints(c *gin.Context) {
	page, pageSize := parsePage(c)
	endpoints, total, err := h.Service.ListAllEndpoints(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list webhook endpoints"})
		return
	}

	resp := make([]dto.WebhookEndpointResponse, len(endpoints))
	for i, ep := range endpoints {
		resp[i] = toEndpointResponse(ep)
	}
	c.JSON(http.StatusOK, dto.WebhookEndpointListResponse{
		Endpoints: resp,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	})
}

// AdminCreateEndpoint registers a new webhook endpoint for an application.
// @Summary Create webhook endpoint (admin)
// @Description Register a new webhook endpoint for an application. Returns the secret only once.
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param app_id path string true "Application ID"
// @Param request body dto.CreateWebhookRequest true "Webhook endpoint details"
// @Success 201 {object} dto.CreateWebhookResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{app_id}/webhooks [post]
func (h *Handler) AdminCreateEndpoint(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app_id"})
		return
	}

	var req dto.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	ep, secret, svcErr := h.Service.RegisterEndpoint(appID, req.EventType, req.URL)
	if svcErr != nil {
		if isConflict(svcErr) {
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: svcErr.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: svcErr.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.CreateWebhookResponse{
		Endpoint: toEndpointResponse(*ep),
		Secret:   secret,
	})
}

// AdminListEndpointsByApp lists webhook endpoints for a specific app.
// @Summary List webhook endpoints for an app (admin)
// @Description Returns all webhook endpoints registered for the given application
// @Tags Webhooks
// @Produce json
// @Param app_id path string true "Application ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.WebhookEndpointListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{app_id}/webhooks [get]
func (h *Handler) AdminListEndpointsByApp(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app_id"})
		return
	}
	page, pageSize := parsePage(c)
	endpoints, total, svcErr := h.Service.ListEndpointsByApp(appID, page, pageSize)
	if svcErr != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list webhook endpoints"})
		return
	}

	resp := make([]dto.WebhookEndpointResponse, len(endpoints))
	for i, ep := range endpoints {
		resp[i] = toEndpointResponse(ep)
	}
	c.JSON(http.StatusOK, dto.WebhookEndpointListResponse{
		Endpoints: resp,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	})
}

// AdminToggleEndpoint enables or disables a webhook endpoint.
// @Summary Toggle webhook endpoint active state (admin)
// @Description Enable or disable a webhook endpoint
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param id path string true "Webhook Endpoint ID"
// @Param request body dto.ToggleWebhookRequest true "Active state"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/webhooks/{id}/toggle [put]
func (h *Handler) AdminToggleEndpoint(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid endpoint ID"})
		return
	}

	var req dto.ToggleWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.Service.SetEndpointActive(id, req.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to update endpoint"})
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Webhook endpoint updated"})
}

// AdminDeleteEndpoint soft-deletes a webhook endpoint.
// @Summary Delete webhook endpoint (admin)
// @Description Soft-delete a webhook endpoint
// @Tags Webhooks
// @Produce json
// @Param id path string true "Webhook Endpoint ID"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/webhooks/{id} [delete]
func (h *Handler) AdminDeleteEndpoint(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid endpoint ID"})
		return
	}
	if err := h.Service.DeleteEndpoint(id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to delete endpoint"})
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Webhook endpoint deleted"})
}

// AdminListDeliveriesByEndpoint returns delivery logs for a specific endpoint.
// @Summary List delivery logs for endpoint (admin)
// @Description Get paginated delivery history for a specific webhook endpoint
// @Tags Webhooks
// @Produce json
// @Param id path string true "Webhook Endpoint ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.WebhookDeliveryListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/webhooks/{id}/deliveries [get]
func (h *Handler) AdminListDeliveriesByEndpoint(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid endpoint ID"})
		return
	}
	page, pageSize := parsePage(c)
	deliveries, total, svcErr := h.Service.ListDeliveriesByEndpoint(id, page, pageSize)
	if svcErr != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to fetch delivery logs"})
		return
	}

	resp := make([]dto.WebhookDeliveryResponse, len(deliveries))
	for i, d := range deliveries {
		resp[i] = toDeliveryResponse(d)
	}
	c.JSON(http.StatusOK, dto.WebhookDeliveryListResponse{
		Deliveries: resp,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
	})
}

// AdminListDeliveriesByApp returns all delivery logs for an app.
// @Summary List all delivery logs for an app (admin)
// @Description Get paginated delivery history for all webhook endpoints of an application
// @Tags Webhooks
// @Produce json
// @Param app_id path string true "Application ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.WebhookDeliveryListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/apps/{app_id}/webhook-deliveries [get]
func (h *Handler) AdminListDeliveriesByApp(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app_id"})
		return
	}
	page, pageSize := parsePage(c)
	deliveries, total, svcErr := h.Service.ListDeliveriesByApp(appID, page, pageSize)
	if svcErr != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to fetch delivery logs"})
		return
	}

	resp := make([]dto.WebhookDeliveryResponse, len(deliveries))
	for i, d := range deliveries {
		resp[i] = toDeliveryResponse(d)
	}
	c.JSON(http.StatusOK, dto.WebhookDeliveryListResponse{
		Deliveries: resp,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
	})
}

// ============================================================================
// App API endpoints (X-App-API-Key)  — scoped to /app/:id/
// ============================================================================

// AppCreateEndpoint registers a new webhook endpoint via the App API.
// @Summary Create webhook endpoint (app API)
// @Description Register a new webhook endpoint for this application. Returns the secret only once.
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param request body dto.CreateWebhookRequest true "Webhook endpoint details"
// @Success 201 {object} dto.CreateWebhookResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Security AppApiKey
// @Router /app/{id}/webhooks [post]
func (h *Handler) AppCreateEndpoint(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app ID"})
		return
	}

	var req dto.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	ep, secret, svcErr := h.Service.RegisterEndpoint(appID, req.EventType, req.URL)
	if svcErr != nil {
		if isConflict(svcErr) {
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: svcErr.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: svcErr.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.CreateWebhookResponse{
		Endpoint: toEndpointResponse(*ep),
		Secret:   secret,
	})
}

// AppListEndpoints lists webhook endpoints for this application via the App API.
// @Summary List webhook endpoints (app API)
// @Description Returns all webhook endpoints for this application
// @Tags Webhooks
// @Produce json
// @Param id path string true "Application ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.WebhookEndpointListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Security AppApiKey
// @Router /app/{id}/webhooks [get]
func (h *Handler) AppListEndpoints(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app ID"})
		return
	}
	page, pageSize := parsePage(c)
	endpoints, total, svcErr := h.Service.ListEndpointsByApp(appID, page, pageSize)
	if svcErr != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list webhook endpoints"})
		return
	}

	resp := make([]dto.WebhookEndpointResponse, len(endpoints))
	for i, ep := range endpoints {
		resp[i] = toEndpointResponse(ep)
	}
	c.JSON(http.StatusOK, dto.WebhookEndpointListResponse{
		Endpoints: resp,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	})
}

// AppToggleEndpoint enables or disables a webhook endpoint via the App API.
// @Summary Toggle webhook endpoint (app API)
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param endpoint_id path string true "Webhook Endpoint ID"
// @Param request body dto.ToggleWebhookRequest true "Active state"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Security AppApiKey
// @Router /app/{id}/webhooks/{endpoint_id}/toggle [put]
func (h *Handler) AppToggleEndpoint(c *gin.Context) {
	_, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app ID"})
		return
	}

	endpointID, err := uuid.Parse(c.Param("endpoint_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid endpoint ID"})
		return
	}

	var req dto.ToggleWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.Service.SetEndpointActive(endpointID, req.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to update endpoint"})
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Webhook endpoint updated"})
}

// AppDeleteEndpoint soft-deletes a webhook endpoint via the App API.
// @Summary Delete webhook endpoint (app API)
// @Tags Webhooks
// @Produce json
// @Param id path string true "Application ID"
// @Param endpoint_id path string true "Webhook Endpoint ID"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Security AppApiKey
// @Router /app/{id}/webhooks/{endpoint_id} [delete]
func (h *Handler) AppDeleteEndpoint(c *gin.Context) {
	_, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app ID"})
		return
	}

	endpointID, err := uuid.Parse(c.Param("endpoint_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid endpoint ID"})
		return
	}

	if err := h.Service.DeleteEndpoint(endpointID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to delete endpoint"})
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Webhook endpoint deleted"})
}

// AppListDeliveries returns delivery history for an endpoint via the App API.
// @Summary List webhook delivery logs (app API)
// @Tags Webhooks
// @Produce json
// @Param id path string true "Application ID"
// @Param endpoint_id path string true "Webhook Endpoint ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.WebhookDeliveryListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Security AppApiKey
// @Router /app/{id}/webhooks/{endpoint_id}/deliveries [get]
func (h *Handler) AppListDeliveries(c *gin.Context) {
	_, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid app ID"})
		return
	}

	endpointID, err := uuid.Parse(c.Param("endpoint_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid endpoint ID"})
		return
	}

	page, pageSize := parsePage(c)
	deliveries, total, svcErr := h.Service.ListDeliveriesByEndpoint(endpointID, page, pageSize)
	if svcErr != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to fetch delivery logs"})
		return
	}

	resp := make([]dto.WebhookDeliveryResponse, len(deliveries))
	for i, d := range deliveries {
		resp[i] = toDeliveryResponse(d)
	}
	c.JSON(http.StatusOK, dto.WebhookDeliveryListResponse{
		Deliveries: resp,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
	})
}

// ============================================================================
// Helper
// ============================================================================

// isConflict checks if the error message indicates a unique-constraint conflict.
func isConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "duplicate") || contains(msg, "unique") || contains(msg, "23505")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
