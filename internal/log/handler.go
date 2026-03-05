package log

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
		// Authentication
		EventLogin,
		EventLoginFailed,
		EventLogout,
		EventRegister,
		EventTokenRefresh,

		// Password management
		EventPasswordChange,
		EventPasswordReset,

		// Email
		EventEmailVerify,
		EventEmailVerifyResend,
		EventEmailChange,

		// Two-factor authentication
		Event2FAEnable,
		Event2FADisable,
		Event2FALogin,
		EventRecoveryCodeUsed,
		EventRecoveryCodeGen,

		// Social authentication
		EventSocialLogin,
		EventSocialAccountLinked,
		EventSocialAccountUnlinked,

		// Passkey / WebAuthn
		EventPasskeyRegister,
		EventPasskeyDelete,
		EventPasskeyLogin,

		// Magic link
		EventMagicLinkRequested,
		EventMagicLinkLogin,
		EventMagicLinkFailed,

		// Profile & account
		EventProfileAccess,
		EventProfileUpdate,
		EventAccountDeletion,

		// Security events
		EventBruteForceDetected,
		EventIPBlocked,
	}

	c.JSON(http.StatusOK, gin.H{
		"event_types": eventTypes,
	})
}

// @Summary Export user activity logs
// @Description Export the authenticated user's activity logs as CSV or JSON (max 10,000 rows). Use the X-Export-Truncated response header to detect if the result was capped.
// @Tags Activity Logs
// @Security ApiKeyAuth
// @Produce json
// @Produce text/csv
// @Param format query string false "Export format: csv or json (default: json)" Enums(csv, json)
// @Param event_type query string false "Filter by event type"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Success 200 {object} dto.ActivityLogExportResponse "JSON export"
// @Success 200 {string} string "CSV export"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /activity-logs/export [get]
func (h *Handler) ExportUserActivityLogs(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.ActivityLogExportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if req.Format == "" {
		req.Format = "json"
	}

	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	logs, truncated, appErr := h.QueryService.ExportUserActivityLogs(userUUID, req)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	h.writeExportResponse(c, logs, truncated, req.Format)
}

// @Summary Export all activity logs (Admin)
// @Description Export all users' activity logs as CSV or JSON (max 10,000 rows). Use the X-Export-Truncated response header to detect if the result was capped.
// @Tags Activity Logs
// @Security ApiKeyAuth
// @Produce json
// @Produce text/csv
// @Param format query string false "Export format: csv or json (default: json)" Enums(csv, json)
// @Param event_type query string false "Filter by event type"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Success 200 {object} dto.ActivityLogExportResponse "JSON export"
// @Success 200 {string} string "CSV export"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /admin/activity-logs/export [get]
func (h *Handler) ExportAllActivityLogs(c *gin.Context) {
	var req dto.ActivityLogExportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if req.Format == "" {
		req.Format = "json"
	}

	logs, truncated, appErr := h.QueryService.ExportAllActivityLogs(req)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	h.writeExportResponse(c, logs, truncated, req.Format)
}

// writeExportResponse writes the export payload in the requested format (csv or json).
// It sets Content-Disposition and X-Export-Truncated headers on every response.
func (h *Handler) writeExportResponse(c *gin.Context, logs []dto.ActivityLogResponse, truncated bool, format string) {
	timestamp := time.Now().UTC().Format("20060102_150405")
	truncatedStr := "false"
	if truncated {
		truncatedStr = "true"
	}
	c.Header("X-Export-Truncated", truncatedStr)

	switch format {
	case "csv":
		filename := fmt.Sprintf("activity_logs_%s.csv", timestamp)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Header("Content-Type", "text/csv; charset=utf-8")

		// Write BOM so Excel auto-detects UTF-8
		c.Writer.WriteHeader(http.StatusOK)
		_, _ = c.Writer.Write([]byte("\xef\xbb\xbf"))

		if err := WriteCSV(c.Writer, logs); err != nil {
			// Headers already sent – log internally, cannot send JSON error
			_ = err
		}

	default: // json
		filename := fmt.Sprintf("activity_logs_%s.json", timestamp)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		resp := dto.ActivityLogExportResponse{
			Data:       logs,
			Count:      len(logs),
			Truncated:  truncated,
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
		}

		c.Writer.WriteHeader(http.StatusOK)
		c.Header("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(c.Writer)
		enc.SetIndent("", "  ")
		_ = enc.Encode(resp)
	}
}
