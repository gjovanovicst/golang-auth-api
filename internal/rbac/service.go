package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	redispkg "github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// CachedAccess holds the cached RBAC data for a user in an application.
type CachedAccess struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// Service handles RBAC business logic including caching.
type Service struct {
	Repo *Repository
}

// NewService creates a new RBAC service.
func NewService(repo *Repository) *Service {
	return &Service{Repo: repo}
}

// cacheKey returns the Redis key for a user's RBAC cache.
func cacheKey(appID, userID string) string {
	return fmt.Sprintf("rbac:%s:%s", appID, userID)
}

// cacheTTL returns the RBAC cache TTL (matches access token lifetime).
func cacheTTL() time.Duration {
	minutes := viper.GetInt("ACCESS_TOKEN_EXPIRATION_MINUTES")
	if minutes <= 0 {
		minutes = 15
	}
	return time.Minute * time.Duration(minutes)
}

// ============================================================
// Role Resolution (the core RBAC check)
// ============================================================

// ResolveUserAccess returns the user's roles and permissions for an application.
// Resolution order: Redis cache → Database (populates cache on miss).
func (s *Service) ResolveUserAccess(appID, userID string) (*CachedAccess, error) {
	// 1. Try Redis cache
	cached, err := s.getFromCache(appID, userID)
	if err == nil && cached != nil {
		return cached, nil
	}

	// 2. Cache miss — fetch from DB
	roles, err := s.Repo.GetUserRoleNames(appID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user roles: %w", err)
	}

	permissions, err := s.Repo.GetUserPermissions(appID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user permissions: %w", err)
	}

	access := &CachedAccess{
		Roles:       roles,
		Permissions: permissions,
	}

	// 3. Populate cache (non-blocking, log errors)
	if err := s.setCache(appID, userID, access); err != nil {
		log.Printf("RBAC cache write error for %s/%s: %v", appID, userID, err)
	}

	return access, nil
}

// HasRole checks if a user has any of the specified roles.
func (s *Service) HasRole(appID, userID string, requiredRoles ...string) (bool, error) {
	access, err := s.ResolveUserAccess(appID, userID)
	if err != nil {
		return false, err
	}

	required := make(map[string]bool, len(requiredRoles))
	for _, r := range requiredRoles {
		required[r] = true
	}

	for _, role := range access.Roles {
		if required[role] {
			return true, nil
		}
	}
	return false, nil
}

// HasPermission checks if a user has a specific permission.
func (s *Service) HasPermission(appID, userID, resource, action string) (bool, error) {
	access, err := s.ResolveUserAccess(appID, userID)
	if err != nil {
		return false, err
	}

	target := resource + ":" + action
	for _, perm := range access.Permissions {
		if perm == target {
			return true, nil
		}
	}
	return false, nil
}

// ============================================================
// Role CRUD (with cache invalidation)
// ============================================================

// GetRolesByAppID returns all roles for an application.
func (s *Service) GetRolesByAppID(appID string) ([]models.Role, error) {
	return s.Repo.GetRolesByAppID(appID)
}

// GetRoleByID returns a single role with permissions.
func (s *Service) GetRoleByID(id string) (*models.Role, error) {
	return s.Repo.GetRoleByID(id)
}

// CreateRole creates a new custom role.
func (s *Service) CreateRole(appID, name, description string) (*models.Role, error) {
	parsedAppID, err := uuid.Parse(appID)
	if err != nil {
		return nil, fmt.Errorf("invalid app ID: %w", err)
	}

	role := &models.Role{
		AppID:       parsedAppID,
		Name:        name,
		Description: description,
		IsSystem:    false,
	}
	if err := s.Repo.CreateRole(role); err != nil {
		return nil, err
	}
	return role, nil
}

// UpdateRole updates a role's name and description.
func (s *Service) UpdateRole(id, name, description string) error {
	role, err := s.Repo.GetRoleByID(id)
	if err != nil {
		return err
	}
	if role.IsSystem && name != role.Name {
		return fmt.Errorf("cannot rename system role")
	}
	return s.Repo.UpdateRole(id, name, description)
}

// DeleteRole deletes a non-system role and invalidates affected user caches.
func (s *Service) DeleteRole(id string) error {
	role, err := s.Repo.GetRoleByID(id)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return fmt.Errorf("cannot delete system role")
	}

	// Find affected users before deletion
	appID := role.AppID.String()
	userRoles, _, err := s.Repo.GetUsersWithRoleInApp(appID, 1, 10000)
	if err != nil {
		return err
	}

	if err := s.Repo.DeleteRole(id); err != nil {
		return err
	}

	// Invalidate caches for affected users
	for _, ur := range userRoles {
		if ur.RoleID.String() == id {
			s.InvalidateCache(appID, ur.UserID.String())
		}
	}
	return nil
}

// ============================================================
// Permission Operations
// ============================================================

// GetAllPermissions returns all available permissions.
func (s *Service) GetAllPermissions() ([]models.Permission, error) {
	return s.Repo.GetAllPermissions()
}

// CreatePermission creates a new custom permission.
func (s *Service) CreatePermission(resource, action, description string) (*models.Permission, error) {
	perm := &models.Permission{
		Resource:    resource,
		Action:      action,
		Description: description,
	}
	if err := s.Repo.CreatePermission(perm); err != nil {
		return nil, err
	}
	return perm, nil
}

// SetRolePermissions replaces all permissions for a role and invalidates affected caches.
func (s *Service) SetRolePermissions(roleID string, permissionIDs []string) error {
	role, err := s.Repo.GetRoleByID(roleID)
	if err != nil {
		return err
	}

	if err := s.Repo.SetRolePermissions(roleID, permissionIDs); err != nil {
		return err
	}

	// Invalidate caches for all users who have this role
	s.invalidateCacheForRole(role.AppID.String(), roleID)
	return nil
}

// ============================================================
// User-Role Assignment (with cache invalidation)
// ============================================================

// AssignRoleToUser assigns a role to a user and invalidates cache.
func (s *Service) AssignRoleToUser(userID, roleID, appID string, assignedBy *uuid.UUID) error {
	if err := s.Repo.AssignRoleToUser(userID, roleID, appID, assignedBy); err != nil {
		return err
	}
	s.InvalidateCache(appID, userID)
	return nil
}

// AssignDefaultRole assigns the "member" role to a user in an application.
// This is intended to be called on user registration and social account creation.
// Non-fatal: logs a warning and returns nil if the role doesn't exist yet.
func (s *Service) AssignDefaultRole(appID, userID string) error {
	role, err := s.Repo.GetRoleByName(appID, "member")
	if err != nil {
		log.Printf("Warning: 'member' role not found for app %s, skipping default role assignment: %v", appID, err)
		return nil
	}
	if err := s.Repo.AssignRoleToUser(userID, role.ID.String(), appID, nil); err != nil {
		log.Printf("Warning: failed to assign default 'member' role to user %s in app %s: %v", userID, appID, err)
		return nil
	}
	s.InvalidateCache(appID, userID)
	return nil
}

// RevokeRoleFromUser removes a role from a user and invalidates cache.
func (s *Service) RevokeRoleFromUser(userID, roleID, appID string) error {
	if err := s.Repo.RevokeRoleFromUser(userID, roleID); err != nil {
		return err
	}
	s.InvalidateCache(appID, userID)
	return nil
}

// GetUserRoles returns the roles assigned to a user in an application.
func (s *Service) GetUserRoles(appID, userID string) ([]models.Role, error) {
	return s.Repo.GetUserRoles(appID, userID)
}

// GetUserRoleNames returns just the role names for a user in an application.
func (s *Service) GetUserRoleNames(appID, userID string) ([]string, error) {
	return s.Repo.GetUserRoleNames(appID, userID)
}

// ============================================================
// Redis Cache Management
// ============================================================

func (s *Service) getFromCache(appID, userID string) (*CachedAccess, error) {
	ctx := context.Background()
	key := cacheKey(appID, userID)

	data, err := redispkg.Rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	var access CachedAccess
	if err := json.Unmarshal([]byte(data), &access); err != nil {
		return nil, err
	}
	return &access, nil
}

func (s *Service) setCache(appID, userID string, access *CachedAccess) error {
	ctx := context.Background()
	key := cacheKey(appID, userID)

	data, err := json.Marshal(access)
	if err != nil {
		return err
	}

	return redispkg.Rdb.Set(ctx, key, string(data), cacheTTL()).Err()
}

// InvalidateCache removes the RBAC cache for a user in an application.
func (s *Service) InvalidateCache(appID, userID string) {
	ctx := context.Background()
	key := cacheKey(appID, userID)
	if err := redispkg.Rdb.Del(ctx, key).Err(); err != nil {
		log.Printf("RBAC cache invalidation error for %s/%s: %v", appID, userID, err)
	}
}

// invalidateCacheForRole invalidates caches for all users who have a given role.
func (s *Service) invalidateCacheForRole(appID, roleID string) {
	userRoles, _, err := s.Repo.GetUsersWithRoleInApp(appID, 1, 10000)
	if err != nil {
		log.Printf("RBAC: failed to find users for role %s cache invalidation: %v", roleID, err)
		return
	}
	for _, ur := range userRoles {
		if ur.RoleID.String() == roleID {
			s.InvalidateCache(appID, ur.UserID.String())
		}
	}
}
