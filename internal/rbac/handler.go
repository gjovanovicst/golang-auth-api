package rbac

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/google/uuid"
)

// Handler serves RBAC admin API endpoints (JSON).
type Handler struct {
	Service *Service
}

// NewHandler creates a new RBAC handler.
func NewHandler(service *Service) *Handler {
	return &Handler{Service: service}
}

// ============================================================
// Role Endpoints
// ============================================================

// ListRoles returns all roles for an application.
// @Summary List roles for an application
// @Description Retrieve all roles (including permissions) for a specific application
// @Tags Admin - RBAC
// @Produce json
// @Param app_id query string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/roles [get]
func (h *Handler) ListRoles(c *gin.Context) {
	appID := c.Query("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "app_id query parameter is required"})
		return
	}

	roles, err := h.Service.GetRolesByAppID(appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list roles"})
		return
	}

	var response []dto.RoleResponse
	for _, r := range roles {
		roleResp := dto.RoleResponse{
			ID:          r.ID,
			AppID:       r.AppID,
			Name:        r.Name,
			Description: r.Description,
			IsSystem:    r.IsSystem,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		}
		for _, p := range r.Permissions {
			roleResp.Permissions = append(roleResp.Permissions, dto.PermissionResponse{
				ID:          p.ID,
				Resource:    p.Resource,
				Action:      p.Action,
				Description: p.Description,
			})
		}
		response = append(response, roleResp)
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// GetRole returns a single role by ID.
// @Summary Get role by ID
// @Description Retrieve a specific role with its permissions
// @Tags Admin - RBAC
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} dto.RoleResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/roles/{id} [get]
func (h *Handler) GetRole(c *gin.Context) {
	id := c.Param("id")
	role, err := h.Service.GetRoleByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Role not found"})
		return
	}

	resp := dto.RoleResponse{
		ID:          role.ID,
		AppID:       role.AppID,
		Name:        role.Name,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}
	for _, p := range role.Permissions {
		resp.Permissions = append(resp.Permissions, dto.PermissionResponse{
			ID:          p.ID,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
		})
	}

	c.JSON(http.StatusOK, resp)
}

// CreateRole creates a new custom role.
// @Summary Create a new role
// @Description Create a custom role for an application
// @Tags Admin - RBAC
// @Accept json
// @Produce json
// @Param role body dto.CreateRoleRequest true "Role Data"
// @Success 201 {object} dto.RoleResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/roles [post]
func (h *Handler) CreateRole(c *gin.Context) {
	var req dto.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	role, err := h.Service.CreateRole(req.AppID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to create role: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.RoleResponse{
		ID:          role.ID,
		AppID:       role.AppID,
		Name:        role.Name,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	})
}

// UpdateRole updates a role's name and description.
// @Summary Update a role
// @Description Update a role's name and description (system roles cannot be renamed)
// @Tags Admin - RBAC
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param role body dto.UpdateRoleRequest true "Role Update Data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/roles/{id} [put]
func (h *Handler) UpdateRole(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.Service.UpdateRole(id, req.Name, req.Description); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to update role: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role updated successfully"})
}

// DeleteRole deletes a non-system role.
// @Summary Delete a role
// @Description Delete a custom role (system roles cannot be deleted)
// @Tags Admin - RBAC
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/roles/{id} [delete]
func (h *Handler) DeleteRole(c *gin.Context) {
	id := c.Param("id")

	if err := h.Service.DeleteRole(id); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Failed to delete role: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role deleted successfully"})
}

// SetRolePermissions replaces all permissions for a role.
// @Summary Set role permissions
// @Description Replace all permissions for a role with the specified permission IDs
// @Tags Admin - RBAC
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param permissions body dto.SetRolePermissionsRequest true "Permission IDs"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/roles/{id}/permissions [put]
func (h *Handler) SetRolePermissions(c *gin.Context) {
	id := c.Param("id")

	var req dto.SetRolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.Service.SetRolePermissions(id, req.PermissionIDs); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to set permissions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role permissions updated successfully"})
}

// ============================================================
// Permission Endpoints
// ============================================================

// ListPermissions returns all available permissions.
// @Summary List all permissions
// @Description Retrieve all available permissions in the system
// @Tags Admin - RBAC
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/permissions [get]
func (h *Handler) ListPermissions(c *gin.Context) {
	permissions, err := h.Service.GetAllPermissions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list permissions"})
		return
	}

	var response []dto.PermissionResponse
	for _, p := range permissions {
		response = append(response, dto.PermissionResponse{
			ID:          p.ID,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// CreatePermission creates a new custom permission.
// @Summary Create a permission
// @Description Create a new custom permission (resource:action pair)
// @Tags Admin - RBAC
// @Accept json
// @Produce json
// @Param permission body dto.CreatePermissionRequest true "Permission Data"
// @Success 201 {object} dto.PermissionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/permissions [post]
func (h *Handler) CreatePermission(c *gin.Context) {
	var req dto.CreatePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	perm, err := h.Service.CreatePermission(req.Resource, req.Action, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to create permission: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.PermissionResponse{
		ID:          perm.ID,
		Resource:    perm.Resource,
		Action:      perm.Action,
		Description: perm.Description,
	})
}

// ============================================================
// User-Role Assignment Endpoints
// ============================================================

// ListUserRoles returns all user-role assignments for an application.
// @Summary List user-role assignments
// @Description Retrieve all user-role assignments for a specific application with pagination
// @Tags Admin - RBAC
// @Produce json
// @Param app_id query string true "Application ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/user-roles [get]
func (h *Handler) ListUserRoles(c *gin.Context) {
	appID := c.Query("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "app_id query parameter is required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	items, total, err := h.Service.Repo.GetUsersWithRoleInApp(appID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to list user-role assignments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        items,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// AssignRole assigns a role to a user.
// @Summary Assign a role to a user
// @Description Assign a role to a user within an application
// @Tags Admin - RBAC
// @Accept json
// @Produce json
// @Param app_id query string true "Application ID"
// @Param assignment body dto.AssignRoleRequest true "Assignment Data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/user-roles [post]
func (h *Handler) AssignRole(c *gin.Context) {
	appID := c.Query("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "app_id query parameter is required"})
		return
	}

	var req dto.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	// Use admin's context as the assigner if available
	var assignedBy *uuid.UUID
	if adminID, exists := c.Get("admin_id"); exists {
		if id, ok := adminID.(string); ok {
			parsed, err := uuid.Parse(id)
			if err == nil {
				assignedBy = &parsed
			}
		}
	}

	if err := h.Service.AssignRoleToUser(req.UserID, req.RoleID, appID, assignedBy); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to assign role: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role assigned successfully"})
}

// RevokeRole revokes a role from a user.
// @Summary Revoke a role from a user
// @Description Remove a role assignment from a user within an application
// @Tags Admin - RBAC
// @Accept json
// @Produce json
// @Param app_id query string true "Application ID"
// @Param user_id query string true "User ID"
// @Param role_id query string true "Role ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/user-roles [delete]
func (h *Handler) RevokeRole(c *gin.Context) {
	appID := c.Query("app_id")
	userID := c.Query("user_id")
	roleID := c.Query("role_id")

	if appID == "" || userID == "" || roleID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "app_id, user_id, and role_id query parameters are required"})
		return
	}

	if err := h.Service.RevokeRoleFromUser(userID, roleID, appID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to revoke role: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role revoked successfully"})
}

// GetUserRoles returns the roles assigned to a specific user in an app.
// @Summary Get user's roles
// @Description Retrieve all roles assigned to a specific user in an application
// @Tags Admin - RBAC
// @Produce json
// @Param app_id query string true "Application ID"
// @Param user_id query string true "User ID"
// @Success 200 {object} dto.UserRolesResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security AdminApiKey
// @Router /admin/rbac/user-roles/user [get]
func (h *Handler) GetUserRoles(c *gin.Context) {
	appID := c.Query("app_id")
	userID := c.Query("user_id")

	if appID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "app_id and user_id query parameters are required"})
		return
	}

	roles, err := h.Service.GetUserRoles(appID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to get user roles"})
		return
	}

	parsedUserID, _ := uuid.Parse(userID)
	parsedAppID, _ := uuid.Parse(appID)

	var roleResponses []dto.RoleResponse
	for _, r := range roles {
		roleResp := dto.RoleResponse{
			ID:          r.ID,
			AppID:       r.AppID,
			Name:        r.Name,
			Description: r.Description,
			IsSystem:    r.IsSystem,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		}
		for _, p := range r.Permissions {
			roleResp.Permissions = append(roleResp.Permissions, dto.PermissionResponse{
				ID:          p.ID,
				Resource:    p.Resource,
				Action:      p.Action,
				Description: p.Description,
			})
		}
		roleResponses = append(roleResponses, roleResp)
	}

	c.JSON(http.StatusOK, dto.UserRolesResponse{
		UserID: parsedUserID,
		AppID:  parsedAppID,
		Roles:  roleResponses,
	})
}
