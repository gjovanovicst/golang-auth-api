package log

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/google/uuid"
)

type Handler struct {
	QueryService *QueryService
}

func NewHandler(queryService *QueryService) *Handler {
	return &Handler{QueryService: queryService}
}

// @Summary Get user activity logs
// @Description Retrieve the authenticated user's activity logs with pagination and filtering
// @Tags Activity Logs
// @Security ApiKeyAuth
// @Produce json
// @Param page query int false "Page number (default: 1)" minimum(1)
// @Param limit query int false "Items per page (default: 20, max: 100)" minimum(1) maximum(100)
// @Param event_type query string false "Filter by event type"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Success 200 {object} dto.ActivityLogListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /activity-logs [get]
func (h *Handler) GetUserActivityLogs(c *gin.Context) {
	// Get user ID from authentication middleware
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	// Parse query parameters
	var req dto.ActivityLogListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	// Parse user ID
	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	// Get activity logs
	response, appErr := h.QueryService.ListUserActivityLogs(userUUID, req)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get activity log by ID
// @Description Retrieve a specific activity log by ID (users can only access their own logs)
// @Tags Activity Logs
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Activity Log ID"
// @Success 200 {object} dto.ActivityLogResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /activity-logs/{id} [get]
func (h *Handler) GetActivityLogByID(c *gin.Context) {
	// Get user ID from authentication middleware
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	// Parse log ID from URL
	logIDStr := c.Param("id")
	logID, err := uuid.Parse(logIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid activity log ID"})
		return
	}

	// Parse user ID
	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	// Get activity log
	response, appErr := h.QueryService.GetActivityLogByID(logID, userUUID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get all activity logs (Admin)
// @Description Retrieve all users' activity logs with pagination and filtering (admin access required)
// @Tags Activity Logs
// @Security ApiKeyAuth
// @Produce json
// @Param page query int false "Page number (default: 1)" minimum(1)
// @Param limit query int false "Items per page (default: 20, max: 100)" minimum(1) maximum(100)
// @Param event_type query string false "Filter by event type"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Success 200 {object} dto.ActivityLogListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /admin/activity-logs [get]
func (h *Handler) GetAllActivityLogs(c *gin.Context) {
	// Note: In a real application, you would check for admin role here
	// For now, this is a placeholder for future role-based access control

	// Parse query parameters
	var req dto.ActivityLogListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	// Get activity logs
	response, appErr := h.QueryService.ListAllActivityLogs(req)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get available event types
// @Description Retrieve list of available activity log event types for filtering
// @Tags Activity Logs
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} map[string][]string
// @Failure 401 {object} dto.ErrorResponse
// @Router /activity-logs/event-types [get]
func (h *Handler) GetEventTypes(c *gin.Context) {
	eventTypes := []string{
		EventLogin,
		EventLogout,
		EventRegister,
		EventPasswordChange,
		EventPasswordReset,
		EventEmailVerify,
		Event2FAEnable,
		Event2FADisable,
		Event2FALogin,
		EventTokenRefresh,
		EventSocialLogin,
		EventProfileAccess,
		EventRecoveryCodeUsed,
		EventRecoveryCodeGen,
	}

	c.JSON(http.StatusOK, gin.H{
		"event_types": eventTypes,
	})
}
