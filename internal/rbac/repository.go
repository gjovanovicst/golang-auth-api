package rbac

import (
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles RBAC data access operations.
type Repository struct {
	DB *gorm.DB
}

// NewRepository creates a new RBAC repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// ============================================================
// Role Operations
// ============================================================

// GetRolesByAppID returns all roles for an application.
func (r *Repository) GetRolesByAppID(appID string) ([]models.Role, error) {
	var roles []models.Role
	err := r.DB.Where("app_id = ?", appID).
		Preload("Permissions").
		Order("is_system DESC, name ASC").
		Find(&roles).Error
	return roles, err
}

// GetRoleByID returns a single role with its permissions.
func (r *Repository) GetRoleByID(id string) (*models.Role, error) {
	var role models.Role
	if err := r.DB.Preload("Permissions").First(&role, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// GetRoleByName returns a role by app ID and name.
func (r *Repository) GetRoleByName(appID, name string) (*models.Role, error) {
	var role models.Role
	if err := r.DB.Where("app_id = ? AND name = ?", appID, name).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// CreateRole creates a new role.
func (r *Repository) CreateRole(role *models.Role) error {
	return r.DB.Create(role).Error
}

// UpdateRole updates a role's name and description.
func (r *Repository) UpdateRole(id string, name, description string) error {
	return r.DB.Model(&models.Role{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"name":        name,
			"description": description,
		}).Error
}

// DeleteRole deletes a role by ID. Returns error if role is a system role.
func (r *Repository) DeleteRole(id string) error {
	var role models.Role
	if err := r.DB.First(&role, "id = ?", id).Error; err != nil {
		return err
	}
	if role.IsSystem {
		return gorm.ErrInvalidData // System roles cannot be deleted
	}
	// Delete role-permission and user-role associations (cascaded by FK), then the role
	return r.DB.Delete(&role).Error
}

// ============================================================
// Permission Operations
// ============================================================

// GetAllPermissions returns all permissions ordered by resource and action.
func (r *Repository) GetAllPermissions() ([]models.Permission, error) {
	var permissions []models.Permission
	err := r.DB.Order("resource ASC, action ASC").Find(&permissions).Error
	return permissions, err
}

// GetPermissionByID returns a single permission by ID.
func (r *Repository) GetPermissionByID(id string) (*models.Permission, error) {
	var perm models.Permission
	if err := r.DB.First(&perm, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &perm, nil
}

// CreatePermission creates a new permission.
func (r *Repository) CreatePermission(perm *models.Permission) error {
	return r.DB.Create(perm).Error
}

// GetPermissionsByRoleID returns all permissions for a specific role.
func (r *Repository) GetPermissionsByRoleID(roleID string) ([]models.Permission, error) {
	var role models.Role
	if err := r.DB.Preload("Permissions").First(&role, "id = ?", roleID).Error; err != nil {
		return nil, err
	}
	return role.Permissions, nil
}

// SetRolePermissions replaces all permissions for a role.
func (r *Repository) SetRolePermissions(roleID string, permissionIDs []string) error {
	var role models.Role
	if err := r.DB.First(&role, "id = ?", roleID).Error; err != nil {
		return err
	}

	var permissions []models.Permission
	if len(permissionIDs) > 0 {
		if err := r.DB.Where("id IN ?", permissionIDs).Find(&permissions).Error; err != nil {
			return err
		}
	}

	return r.DB.Model(&role).Association("Permissions").Replace(permissions)
}

// ============================================================
// User-Role Operations
// ============================================================

// GetUserRoles returns all roles assigned to a user in a specific application.
func (r *Repository) GetUserRoles(appID, userID string) ([]models.Role, error) {
	var userRoles []models.UserRole
	err := r.DB.Where("app_id = ? AND user_id = ?", appID, userID).
		Preload("Role").
		Preload("Role.Permissions").
		Find(&userRoles).Error
	if err != nil {
		return nil, err
	}

	roles := make([]models.Role, 0, len(userRoles))
	for _, ur := range userRoles {
		roles = append(roles, ur.Role)
	}
	return roles, nil
}

// GetUserRoleNames returns just the role names for a user in an application.
func (r *Repository) GetUserRoleNames(appID, userID string) ([]string, error) {
	var names []string
	err := r.DB.Model(&models.UserRole{}).
		Select("roles.name").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.app_id = ? AND user_roles.user_id = ?", appID, userID).
		Pluck("roles.name", &names).Error
	return names, err
}

// GetUserPermissions returns all unique permission strings for a user in an application.
// Returns permissions as "resource:action" strings.
func (r *Repository) GetUserPermissions(appID, userID string) ([]string, error) {
	var permissions []struct {
		Resource string
		Action   string
	}
	err := r.DB.Table("user_roles").
		Select("DISTINCT permissions.resource, permissions.action").
		Joins("JOIN role_permissions ON role_permissions.role_id = user_roles.role_id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("user_roles.app_id = ? AND user_roles.user_id = ?", appID, userID).
		Scan(&permissions).Error
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(permissions))
	for _, p := range permissions {
		result = append(result, p.Resource+":"+p.Action)
	}
	return result, nil
}

// AssignRoleToUser assigns a role to a user. Returns error if already assigned.
func (r *Repository) AssignRoleToUser(userID, roleID, appID string, assignedBy *uuid.UUID) error {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	parsedRoleID, err := uuid.Parse(roleID)
	if err != nil {
		return err
	}
	parsedAppID, err := uuid.Parse(appID)
	if err != nil {
		return err
	}

	userRole := models.UserRole{
		UserID:     parsedUserID,
		RoleID:     parsedRoleID,
		AppID:      parsedAppID,
		AssignedBy: assignedBy,
	}
	return r.DB.Create(&userRole).Error
}

// RevokeRoleFromUser removes a role assignment from a user.
func (r *Repository) RevokeRoleFromUser(userID, roleID string) error {
	return r.DB.Where("user_id = ? AND role_id = ?", userID, roleID).
		Delete(&models.UserRole{}).Error
}

// GetUsersWithRoleInApp returns all user-role assignments for an app, with user info.
func (r *Repository) GetUsersWithRoleInApp(appID string, page, pageSize int) ([]UserRoleListItem, int64, error) {
	var items []UserRoleListItem
	var total int64

	countQuery := r.DB.Model(&models.UserRole{}).Where("user_roles.app_id = ?", appID)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := r.DB.Model(&models.UserRole{}).
		Select(`user_roles.user_id, user_roles.role_id, user_roles.app_id,
			user_roles.assigned_at,
			users.email as user_email, users.name as user_name,
			roles.name as role_name`).
		Joins("JOIN users ON users.id = user_roles.user_id").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.app_id = ?", appID).
		Order("users.email ASC, roles.name ASC").
		Offset(offset).Limit(pageSize).
		Scan(&items).Error

	return items, total, err
}

// UserRoleListItem represents a user-role assignment for list views.
type UserRoleListItem struct {
	UserID     uuid.UUID `json:"user_id"`
	RoleID     uuid.UUID `json:"role_id"`
	AppID      uuid.UUID `json:"app_id"`
	UserEmail  string    `json:"user_email"`
	UserName   string    `json:"user_name"`
	RoleName   string    `json:"role_name"`
	AssignedAt string    `json:"assigned_at"`
}

// GetUserRolesForUserInApp returns all role assignments for a specific user in an app.
func (r *Repository) GetUserRolesForUserInApp(appID, userID string) ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.DB.Where("app_id = ? AND user_id = ?", appID, userID).
		Preload("Role").
		Find(&userRoles).Error
	return userRoles, err
}

// ListAllApps returns all applications (id, name) for dropdown selects.
func (r *Repository) ListAllApps() ([]models.Application, error) {
	var apps []models.Application
	err := r.DB.Select("id, name").Order("name ASC").Find(&apps).Error
	return apps, err
}
