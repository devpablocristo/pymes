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

func (r *tenantMembershipResolver) FindActiveMembershipRole(ctx context.Context, tenantID, actor string) (string, bool, error) {
	if r == nil || r.db == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actor) == "" {
		return "", false, nil
	}
	var row struct {
		Role string
	}
	err := r.db.WithContext(ctx).
		Table("tenant_memberships AS tm").
		Select("tm.role").
		Joins("JOIN users u ON u.id = tm.user_id").
		Where("tm.tenant_id = ? AND tm.status = 'active' AND u.external_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(actor)).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return "", false, err
	}
	role := strings.TrimSpace(row.Role)
	return role, role != "", nil
}
