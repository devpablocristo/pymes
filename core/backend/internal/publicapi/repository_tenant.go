package publicapi

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type tenantResolveByIDRow struct {
	ID uuid.UUID `gorm:"column:id"`
}

type tenantResolveBySlugRow struct {
	ID uuid.UUID
}

type businessInfoRow struct {
	OrgID             uuid.UUID
	Name              string
	Slug              string
	BusinessName      string
	BusinessAddress   string
	BusinessPhone     string
	BusinessEmail     string
	SchedulingEnabled bool
}

func (r *Repository) ResolveOrgID(ctx context.Context, ref string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return uuid.Nil, ErrTenantNotFound
	}

	if parsed, err := uuid.Parse(trimmed); err == nil {
		var row tenantResolveByIDRow
		err = r.db.WithContext(ctx).
			Table("orgs").
			Select("id").
			Where("id = ?", parsed).
			Take(&row).Error
		if err == nil {
			return row.ID, nil
		}
	}

	var row tenantResolveBySlugRow
	err := r.db.WithContext(ctx).
		Table("orgs").
		Select("id").
		Where("slug = ?", trimmed).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, ErrTenantNotFound
		}
		return uuid.Nil, err
	}
	return row.ID, nil
}

func (r *Repository) GetBusinessInfo(ctx context.Context, orgID uuid.UUID) (BusinessInfo, error) {
	var row businessInfoRow

	err := r.db.WithContext(ctx).
		Table("orgs o").
		Select(`
			o.id as org_id,
			o.name,
			o.slug,
			COALESCE(ts.business_name, '') as business_name,
			COALESCE(ts.business_address, '') as business_address,
			COALESCE(ts.business_phone, '') as business_phone,
			COALESCE(ts.business_email, '') as business_email,
			COALESCE(ts.scheduling_enabled, false) as scheduling_enabled
		`).
		Joins("LEFT JOIN org_settings ts ON ts.org_id = o.id").
		Where("o.id = ?", orgID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return BusinessInfo{}, ErrTenantNotFound
		}
		return BusinessInfo{}, err
	}

	businessName := strings.TrimSpace(row.BusinessName)
	if businessName == "" {
		businessName = row.Name
	}

	return BusinessInfo{
		OrgID:             row.OrgID,
		Name:              row.Name,
		Slug:              row.Slug,
		BusinessName:      businessName,
		BusinessAddress:   row.BusinessAddress,
		BusinessPhone:     row.BusinessPhone,
		BusinessEmail:     row.BusinessEmail,
		SchedulingEnabled: row.SchedulingEnabled,
	}, nil
}
