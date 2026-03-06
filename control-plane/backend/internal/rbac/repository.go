package rbac

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/rbac/repository/models"
	rbacdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/rbac/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

type permissionRow struct {
	Resource string `gorm:"column:resource"`
	Action   string `gorm:"column:action"`
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListRoles(orgID uuid.UUID) ([]rbacdomain.Role, error) {
	var rows []models.RoleModel
	if err := r.db.Where("org_id = ?", orgID).Order("is_system DESC, name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]rbacdomain.Role, 0, len(rows))
	for _, row := range rows {
		perms, err := r.loadRolePermissions(row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, roleToDomain(row, perms))
	}
	return out, nil
}

func (r *Repository) GetRole(orgID, roleID uuid.UUID) (rbacdomain.Role, error) {
	var row models.RoleModel
	if err := r.db.Where("org_id = ? AND id = ?", orgID, roleID).First(&row).Error; err != nil {
		return rbacdomain.Role{}, err
	}
	perms, err := r.loadRolePermissions(row.ID)
	if err != nil {
		return rbacdomain.Role{}, err
	}
	return roleToDomain(row, perms), nil
}

func (r *Repository) CreateRole(in rbacdomain.Role) (rbacdomain.Role, error) {
	now := time.Now().UTC()
	m := models.RoleModel{
		ID:          uuid.New(),
		OrgID:       in.OrgID,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		IsSystem:    in.IsSystem,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.db.Create(&m).Error; err != nil {
		return rbacdomain.Role{}, err
	}
	if err := r.replaceRolePermissions(m.ID, in.Permissions); err != nil {
		return rbacdomain.Role{}, err
	}
	perms, err := r.loadRolePermissions(m.ID)
	if err != nil {
		return rbacdomain.Role{}, err
	}
	return roleToDomain(m, perms), nil
}

func (r *Repository) UpdateRole(in rbacdomain.Role) (rbacdomain.Role, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"description": strings.TrimSpace(in.Description),
		"updated_at":  now,
	}
	if strings.TrimSpace(in.Name) != "" {
		updates["name"] = strings.TrimSpace(in.Name)
	}
	result := r.db.Model(&models.RoleModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if result.Error != nil {
		return rbacdomain.Role{}, result.Error
	}
	if result.RowsAffected == 0 {
		return rbacdomain.Role{}, gorm.ErrRecordNotFound
	}
	if err := r.replaceRolePermissions(in.ID, in.Permissions); err != nil {
		return rbacdomain.Role{}, err
	}
	return r.GetRole(in.OrgID, in.ID)
}

func (r *Repository) DeleteRole(orgID, roleID uuid.UUID) error {
	result := r.db.Where("org_id = ? AND id = ?", orgID, roleID).Delete(&models.RoleModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) AssignRole(orgID, roleID, userID uuid.UUID, assignedBy string) error {
	if ok, err := r.isOrgMember(orgID, userID); err != nil {
		return err
	} else if !ok {
		return gorm.ErrRecordNotFound
	}

	now := time.Now().UTC()
	var existing models.UserRoleModel
	err := r.db.Where("org_id = ? AND user_id = ?", orgID, userID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.Create(&models.UserRoleModel{
				OrgID:      orgID,
				UserID:     userID,
				RoleID:     roleID,
				AssignedBy: assignedBy,
				AssignedAt: now,
			}).Error
		}
		return err
	}

	return r.db.Model(&models.UserRoleModel{}).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Updates(map[string]any{"role_id": roleID, "assigned_by": assignedBy, "assigned_at": now}).Error
}

func (r *Repository) RemoveRole(orgID, roleID, userID uuid.UUID) error {
	result := r.db.Where("org_id = ? AND role_id = ? AND user_id = ?", orgID, roleID, userID).Delete(&models.UserRoleModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) IsSystemRole(orgID, roleID uuid.UUID) (bool, error) {
	var row models.RoleModel
	if err := r.db.Select("is_system").Where("org_id = ? AND id = ?", orgID, roleID).First(&row).Error; err != nil {
		return false, err
	}
	return row.IsSystem, nil
}

func (r *Repository) GetUserPermissions(orgID, userID uuid.UUID) (map[string]map[string]bool, error) {
	if ok, err := r.isOrgAdmin(orgID, userID); err != nil {
		return nil, err
	} else if ok {
		return map[string]map[string]bool{"*": {"*": true}}, nil
	}

	var rows []permissionRow
	err := r.db.Table("user_roles ur").
		Select("rp.resource, rp.action").
		Joins("JOIN role_permissions rp ON rp.role_id = ur.role_id").
		Where("ur.org_id = ? AND ur.user_id = ?", orgID, userID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToPermissionMap(rows), nil
}

func (r *Repository) GetActorPermissions(orgID uuid.UUID, actor string) (map[string]map[string]bool, error) {
	userID, ok, err := r.findActorUserID(orgID, actor)
	if err != nil {
		return nil, err
	}
	if !ok {
		return map[string]map[string]bool{}, nil
	}
	return r.GetUserPermissions(orgID, userID)
}

func (r *Repository) findActorUserID(orgID uuid.UUID, actor string) (uuid.UUID, bool, error) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return uuid.Nil, false, nil
	}
	if id, err := uuid.Parse(actor); err == nil {
		if ok, err := r.isOrgMember(orgID, id); err != nil {
			return uuid.Nil, false, err
		} else if ok {
			return id, true, nil
		}
	}

	type userRow struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	var row userRow
	err := r.db.Table("users u").
		Select("u.id").
		Joins("JOIN org_members om ON om.user_id = u.id AND om.org_id = ?", orgID).
		Where("u.external_id = ?", actor).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return uuid.Nil, false, err
	}
	if row.ID == uuid.Nil {
		return uuid.Nil, false, nil
	}
	return row.ID, true, nil
}

func (r *Repository) isOrgAdmin(orgID, userID uuid.UUID) (bool, error) {
	type row struct {
		Role string `gorm:"column:role"`
	}
	var out row
	err := r.db.Table("org_members").Select("role").Where("org_id = ? AND user_id = ?", orgID, userID).Limit(1).Scan(&out).Error
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(out.Role), "admin"), nil
}

func (r *Repository) isOrgMember(orgID, userID uuid.UUID) (bool, error) {
	type row struct {
		ID uuid.UUID `gorm:"column:user_id"`
	}
	var out row
	err := r.db.Table("org_members").
		Select("user_id").
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Limit(1).
		Scan(&out).Error
	if err != nil {
		return false, err
	}
	return out.ID != uuid.Nil, nil
}

func (r *Repository) loadRolePermissions(roleID uuid.UUID) ([]rbacdomain.Permission, error) {
	var rows []models.RolePermissionModel
	if err := r.db.Where("role_id = ?", roleID).Order("resource, action").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]rbacdomain.Permission, 0, len(rows))
	for _, row := range rows {
		out = append(out, rbacdomain.Permission{Resource: row.Resource, Action: row.Action})
	}
	return out, nil
}

func (r *Repository) replaceRolePermissions(roleID uuid.UUID, perms []rbacdomain.Permission) error {
	if err := r.db.Where("role_id = ?", roleID).Delete(&models.RolePermissionModel{}).Error; err != nil {
		return err
	}
	seen := make(map[string]struct{})
	for _, p := range perms {
		resource := strings.TrimSpace(p.Resource)
		action := strings.TrimSpace(p.Action)
		if resource == "" || action == "" {
			continue
		}
		key := resource + ":" + action
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := r.db.Create(&models.RolePermissionModel{
			ID:       uuid.New(),
			RoleID:   roleID,
			Resource: resource,
			Action:   action,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func roleToDomain(in models.RoleModel, perms []rbacdomain.Permission) rbacdomain.Role {
	return rbacdomain.Role{
		ID:          in.ID,
		OrgID:       in.OrgID,
		Name:        in.Name,
		Description: in.Description,
		IsSystem:    in.IsSystem,
		Permissions: perms,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
	}
}

func rowsToPermissionMap(rows []permissionRow) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	for _, row := range rows {
		resource := strings.TrimSpace(row.Resource)
		action := strings.TrimSpace(row.Action)
		if resource == "" || action == "" {
			continue
		}
		if _, ok := out[resource]; !ok {
			out[resource] = make(map[string]bool)
		}
		out[resource][action] = true
	}
	return out
}
