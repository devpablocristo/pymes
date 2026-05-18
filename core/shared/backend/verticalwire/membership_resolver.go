package verticalwire

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

type tenantMembershipResolver struct {
	db *gorm.DB
}

func NewTenantMembershipResolver(db *gorm.DB) *tenantMembershipResolver {
	return &tenantMembershipResolver{db: db}
}

func (r *tenantMembershipResolver) FindActiveMembershipRole(ctx context.Context, orgID, actor string) (string, bool, error) {
	if r == nil || r.db == nil || strings.TrimSpace(orgID) == "" || strings.TrimSpace(actor) == "" {
		return "", false, nil
	}
	var row struct {
		Role string
	}
	err := r.db.WithContext(ctx).
		Table("org_members AS tm").
		Select("tm.role").
		Joins("JOIN users u ON u.id = tm.user_id").
		Where("tm.org_id = ? AND tm.status = 'active' AND u.external_id = ?", strings.TrimSpace(orgID), strings.TrimSpace(actor)).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return "", false, err
	}
	role := strings.TrimSpace(row.Role)
	return role, role != "", nil
}
