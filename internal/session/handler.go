package session

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/google/uuid"
)

// Handler handles HTTP requests for session management.
type Handler struct {
	Service *Service
}

// NewHandler creates a new session handler.
func NewHandler(s *Service) *Handler {
	return &Handler{Service: s}
}

// @Summary List active sessions
// @Description List all active sessions (devices/IPs) for the authenticated user
// @Tags Sessions
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.SessionListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /sessions [get]
func (h *Handler) ListSessions(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	appIDVal, exists := c.Get("appID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID not found in context"})
		return
	}

	// Get current session ID from context (set by AuthMiddleware)
	currentSessionID := ""
	if sid, exists := c.Get("sessionID"); exists {
		currentSessionID = sid.(string)
	}

	result, appErr := h.Service.ListSessions(appIDVal.(string), userID.(string), currentSessionID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, result)
}

// @Summary Revoke a specific session
// @Description Revoke a specific session by its ID (logout a specific device)
// @Tags Sessions
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /sessions/{id} [delete]
func (h *Handler) RevokeSession(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	appIDVal, exists := c.Get("appID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID not found in context"})
		return
	}

	sessionID := c.Param("id")
	if _, err := uuid.Parse(sessionID); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid session ID"})
		return
	}

	// Prevent revoking your own current session via this endpoint
	if sid, exists := c.Get("sessionID"); exists && sid.(string) == sessionID {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Cannot revoke current session. Use logout instead."})
		return
	}

	appErr := h.Service.RevokeSession(appIDVal.(string), userID.(string), sessionID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Session revoked successfully"})
}

// @Summary Revoke all other sessions
// @Description Revoke all sessions except the current one
// @Tags Sessions
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.MessageResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /sessions [delete]
func (h *Handler) RevokeAllSessions(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	appIDVal, exists := c.Get("appID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "App ID not found in context"})
		return
	}

	// Keep the current session alive
	currentSessionID := ""
	if sid, exists := c.Get("sessionID"); exists {
		currentSessionID = sid.(string)
	}

	appErr := h.Service.RevokeAllSessions(appIDVal.(string), userID.(string), currentSessionID)
	if appErr != nil {
		c.JSON(appErr.Code, dto.ErrorResponse{Error: appErr.Message})
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "All other sessions revoked successfully"})
}
