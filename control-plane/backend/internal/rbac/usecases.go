package rbac

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	rbacdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/rbac/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/internal/shared/httperrors"
)

const permissionCacheTTL = 5 * time.Minute

type RepositoryPort interface {
	ListRoles(orgID uuid.UUID) ([]rbacdomain.Role, error)
	GetRole(orgID, roleID uuid.UUID) (rbacdomain.Role, error)
	CreateRole(in rbacdomain.Role) (rbacdomain.Role, error)
	UpdateRole(in rbacdomain.Role) (rbacdomain.Role, error)
	DeleteRole(orgID, roleID uuid.UUID) error
	AssignRole(orgID, roleID, userID uuid.UUID, assignedBy string) error
	RemoveRole(orgID, roleID, userID uuid.UUID) error
	IsSystemRole(orgID, roleID uuid.UUID) (bool, error)
	GetUserPermissions(orgID, userID uuid.UUID) (map[string]map[string]bool, error)
	GetActorPermissions(orgID uuid.UUID, actor string) (map[string]map[string]bool, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type cacheEntry struct {
	permissions map[string]map[string]bool
	expiresAt   time.Time
}

type permCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
	cache *permCache
}

func NewUsecases(repo RepositoryPort, audit AuditPort) *Usecases {
	return &Usecases{
		repo:  repo,
		audit: audit,
		cache: &permCache{items: make(map[string]cacheEntry)},
	}
}

func (u *Usecases) ListRoles(ctx context.Context, orgID string) ([]rbacdomain.Role, error) {
	_ = ctx
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	return u.repo.ListRoles(orgUUID)
}

func (u *Usecases) GetRole(ctx context.Context, orgID, roleID string) (rbacdomain.Role, error) {
	_ = ctx
	orgUUID, roleUUID, err := parseOrgRoleIDs(orgID, roleID)
	if err != nil {
		return rbacdomain.Role{}, err
	}
	out, err := u.repo.GetRole(orgUUID, roleUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return rbacdomain.Role{}, fmt.Errorf("role not found: %w", httperrors.ErrNotFound)
		}
		return rbacdomain.Role{}, err
	}
	return out, nil
}

func (u *Usecases) CreateRole(ctx context.Context, orgID, actor, name, description string, perms []rbacdomain.Permission) (rbacdomain.Role, error) {
	_ = ctx
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return rbacdomain.Role{}, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return rbacdomain.Role{}, fmt.Errorf("name is required: %w", httperrors.ErrBadInput)
	}
	cleanPerms := normalizePermissions(perms)
	if len(cleanPerms) == 0 {
		return rbacdomain.Role{}, fmt.Errorf("permissions are required: %w", httperrors.ErrBadInput)
	}

	out, err := u.repo.CreateRole(rbacdomain.Role{
		OrgID:       orgUUID,
		Name:        strings.ToLower(name),
		Description: strings.TrimSpace(description),
		Permissions: cleanPerms,
		IsSystem:    false,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return rbacdomain.Role{}, fmt.Errorf("role already exists: %w", httperrors.ErrConflict)
		}
		return rbacdomain.Role{}, err
	}
	u.invalidateAllCache()
	if u.audit != nil {
		u.audit.Log(ctx, orgID, actor, "rbac.role.created", "role", out.ID.String(), map[string]any{"name": out.Name})
	}
	return out, nil
}

func (u *Usecases) UpdateRole(ctx context.Context, orgID, roleID, actor string, description *string, permissions []rbacdomain.Permission) (rbacdomain.Role, error) {
	orgUUID, roleUUID, err := parseOrgRoleIDs(orgID, roleID)
	if err != nil {
		return rbacdomain.Role{}, err
	}

	system, err := u.repo.IsSystemRole(orgUUID, roleUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return rbacdomain.Role{}, fmt.Errorf("role not found: %w", httperrors.ErrNotFound)
		}
		return rbacdomain.Role{}, err
	}
	if system {
		return rbacdomain.Role{}, fmt.Errorf("system roles cannot be modified: %w", httperrors.ErrForbidden)
	}

	current, err := u.repo.GetRole(orgUUID, roleUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return rbacdomain.Role{}, fmt.Errorf("role not found: %w", httperrors.ErrNotFound)
		}
		return rbacdomain.Role{}, err
	}
	if description != nil {
		current.Description = strings.TrimSpace(*description)
	}
	if permissions != nil {
		current.Permissions = normalizePermissions(permissions)
	}
	if len(current.Permissions) == 0 {
		return rbacdomain.Role{}, fmt.Errorf("permissions are required: %w", httperrors.ErrBadInput)
	}

	updated, err := u.repo.UpdateRole(current)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return rbacdomain.Role{}, fmt.Errorf("role not found: %w", httperrors.ErrNotFound)
		}
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return rbacdomain.Role{}, fmt.Errorf("role already exists: %w", httperrors.ErrConflict)
		}
		return rbacdomain.Role{}, err
	}
	u.invalidateAllCache()
	if u.audit != nil {
		u.audit.Log(ctx, orgID, actor, "rbac.role.updated", "role", updated.ID.String(), map[string]any{"name": updated.Name})
	}
	return updated, nil
}

func (u *Usecases) DeleteRole(ctx context.Context, orgID, roleID, actor string) error {
	orgUUID, roleUUID, err := parseOrgRoleIDs(orgID, roleID)
	if err != nil {
		return err
	}
	system, err := u.repo.IsSystemRole(orgUUID, roleUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("role not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if system {
		return fmt.Errorf("system roles cannot be deleted: %w", httperrors.ErrForbidden)
	}
	if err := u.repo.DeleteRole(orgUUID, roleUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("role not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	u.invalidateAllCache()
	if u.audit != nil {
		u.audit.Log(ctx, orgID, actor, "rbac.role.deleted", "role", roleID, map[string]any{})
	}
	return nil
}

func (u *Usecases) AssignRole(ctx context.Context, orgID, roleID, userID, actor string) error {
	orgUUID, roleUUID, userUUID, err := parseIDs(orgID, roleID, userID)
	if err != nil {
		return err
	}
	system, err := u.repo.IsSystemRole(orgUUID, roleUUID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		return fmt.Errorf("role not found: %w", httperrors.ErrNotFound)
	}
	if !system {
		if _, err := u.repo.GetRole(orgUUID, roleUUID); err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("role not found: %w", httperrors.ErrNotFound)
			}
			return err
		}
	}

	if err := u.repo.AssignRole(orgUUID, roleUUID, userUUID, actor); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user or role not found in org: %w", httperrors.ErrNotFound)
		}
		return err
	}
	u.invalidateAllCache()
	if u.audit != nil {
		u.audit.Log(ctx, orgID, actor, "rbac.role.assigned", "user", userID, map[string]any{"role_id": roleID})
	}
	return nil
}

func (u *Usecases) RemoveRole(ctx context.Context, orgID, roleID, userID, actor string) error {
	orgUUID, roleUUID, userUUID, err := parseIDs(orgID, roleID, userID)
	if err != nil {
		return err
	}
	if err := u.repo.RemoveRole(orgUUID, roleUUID, userUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("role assignment not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	u.invalidateAllCache()
	if u.audit != nil {
		u.audit.Log(ctx, orgID, actor, "rbac.role.unassigned", "user", userID, map[string]any{"role_id": roleID})
	}
	return nil
}

func (u *Usecases) EffectivePermissions(ctx context.Context, orgID, userID string) (map[string][]string, error) {
	_ = ctx
	orgUUID, userUUID, err := parseOrgUserIDs(orgID, userID)
	if err != nil {
		return nil, err
	}
	m, err := u.repo.GetUserPermissions(orgUUID, userUUID)
	if err != nil {
		return nil, err
	}
	return permissionMapToResponse(m), nil
}

func (u *Usecases) HasPermission(ctx context.Context, orgID, actor, role string, scopes []string, authMethod, resource, action string) bool {
	resource = strings.TrimSpace(resource)
	action = strings.TrimSpace(action)
	if resource == "" || action == "" {
		return false
	}

	if isAdminRole(role) {
		return true
	}
	if allowsByScopes(scopes, resource, action) {
		return true
	}

	if strings.EqualFold(strings.TrimSpace(authMethod), "api_key") {
		return false
	}

	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return false
	}

	permissions, err := u.getCachedPermissions(ctx, orgUUID, actor)
	if err != nil {
		return false
	}
	if allowsByMap(permissions, resource, action) {
		return true
	}

	return memberDefaultAllows(resource, action)
}

func (u *Usecases) getCachedPermissions(ctx context.Context, orgID uuid.UUID, actor string) (map[string]map[string]bool, error) {
	cacheKey := orgID.String() + ":" + strings.TrimSpace(actor)
	now := time.Now()

	u.cache.mu.RLock()
	entry, ok := u.cache.items[cacheKey]
	u.cache.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.permissions, nil
	}

	permissions, err := u.repo.GetActorPermissions(orgID, actor)
	if err != nil {
		return nil, err
	}

	u.cache.mu.Lock()
	u.cache.items[cacheKey] = cacheEntry{permissions: permissions, expiresAt: now.Add(permissionCacheTTL)}
	u.cache.mu.Unlock()

	return permissions, nil
}

func (u *Usecases) invalidateAllCache() {
	u.cache.mu.Lock()
	u.cache.items = make(map[string]cacheEntry)
	u.cache.mu.Unlock()
}

func normalizePermissions(perms []rbacdomain.Permission) []rbacdomain.Permission {
	out := make([]rbacdomain.Permission, 0, len(perms))
	seen := make(map[string]struct{})
	for _, p := range perms {
		resource := strings.ToLower(strings.TrimSpace(p.Resource))
		action := strings.ToLower(strings.TrimSpace(p.Action))
		if resource == "" || action == "" {
			continue
		}
		k := resource + ":" + action
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, rbacdomain.Permission{Resource: resource, Action: action})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Resource == out[j].Resource {
			return out[i].Action < out[j].Action
		}
		return out[i].Resource < out[j].Resource
	})
	return out
}

func allowsByMap(permissions map[string]map[string]bool, resource, action string) bool {
	resource = strings.ToLower(strings.TrimSpace(resource))
	action = strings.ToLower(strings.TrimSpace(action))
	if permissions == nil {
		return false
	}
	if v, ok := permissions["*"]; ok {
		if v["*"] || v[action] {
			return true
		}
	}
	if v, ok := permissions[resource]; ok {
		if v["*"] || v[action] {
			return true
		}
	}
	return false
}

func allowsByScopes(scopes []string, resource, action string) bool {
	resource = strings.ToLower(strings.TrimSpace(resource))
	action = strings.ToLower(strings.TrimSpace(action))
	for _, s := range scopes {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "*:*" || s == "admin:*" || s == "admin:console:write" {
			return true
		}
		if s == resource+":"+action || s == resource+":*" {
			return true
		}
	}
	return false
}

func memberDefaultAllows(resource, action string) bool {
	if action != "read" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "customers", "suppliers", "products", "inventory", "sales", "quotes":
		return true
	default:
		return false
	}
}

func isAdminRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "admin" || role == "org:admin"
}

func permissionMapToResponse(m map[string]map[string]bool) map[string][]string {
	out := make(map[string][]string)
	for resource, actions := range m {
		vals := make([]string, 0, len(actions))
		for action, allowed := range actions {
			if allowed {
				vals = append(vals, action)
			}
		}
		sort.Strings(vals)
		out[resource] = vals
	}
	return out
}

func parseOrgRoleIDs(orgID, roleID string) (uuid.UUID, uuid.UUID, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	roleUUID, err := uuid.Parse(strings.TrimSpace(roleID))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid role_id: %w", httperrors.ErrBadInput)
	}
	return orgUUID, roleUUID, nil
}

func parseIDs(orgID, roleID, userID string) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	orgUUID, roleUUID, err := parseOrgRoleIDs(orgID, roleID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("invalid user_id: %w", httperrors.ErrBadInput)
	}
	return orgUUID, roleUUID, userUUID, nil
}

func parseOrgUserIDs(orgID, userID string) (uuid.UUID, uuid.UUID, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid user_id: %w", httperrors.ErrBadInput)
	}
	return orgUUID, userUUID, nil
}
