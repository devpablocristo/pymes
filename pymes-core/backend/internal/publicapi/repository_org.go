package publicapi

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type orgResolveByIDRow struct {
	ID uuid.UUID `gorm:"column:id"`
}

type orgResolveBySlugRow struct {
	ID uuid.UUID
}

type businessInfoRow struct {
	TenantID          uuid.UUID
	Name              string
	Slug              string
	BusinessName      string
	BusinessAddress   string
	BusinessPhone     string
	BusinessEmail     string
	SchedulingEnabled bool
}

func (r *Repository) ResolveTenantID(ctx context.Context, ref string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return uuid.Nil, ErrOrgNotFound
	}

	if parsed, err := uuid.Parse(trimmed); err == nil {
		var row orgResolveByIDRow
		err = r.db.WithContext(ctx).
			Table("tenants").
			Select("id").
			Where("id = ?", parsed).
			Take(&row).Error
		if err == nil {
			return row.ID, nil
		}
	}

	var row orgResolveBySlugRow
	err := r.db.WithContext(ctx).
		Table("tenants").
		Select("id").
		Where("slug = ?", trimmed).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, ErrOrgNotFound
		}
		return uuid.Nil, err
	}
	return row.ID, nil
}

func (r *Repository) ResolveOrgID(ctx context.Context, ref string) (uuid.UUID, error) {
	return r.ResolveTenantID(ctx, ref)
}

func (r *Repository) GetBusinessInfo(ctx context.Context, tenantID uuid.UUID) (BusinessInfo, error) {
	var row businessInfoRow

	err := r.db.WithContext(ctx).
		Table("tenants o").
		Select(`
			o.id as tenant_id,
			o.name,
			o.slug,
			COALESCE(ts.business_name, '') as business_name,
			COALESCE(ts.business_address, '') as business_address,
			COALESCE(ts.business_phone, '') as business_phone,
			COALESCE(ts.business_email, '') as business_email,
			COALESCE(ts.scheduling_enabled, false) as scheduling_enabled
		`).
		Joins("LEFT JOIN tenant_settings ts ON ts.tenant_id = o.id").
		Where("o.id = ?", tenantID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return BusinessInfo{}, ErrOrgNotFound
		}
		return BusinessInfo{}, err
	}

	businessName := strings.TrimSpace(row.BusinessName)
	if businessName == "" {
		businessName = row.Name
	}

	return BusinessInfo{
		TenantID:          row.TenantID,
		Name:              row.Name,
		Slug:              row.Slug,
		BusinessName:      businessName,
		BusinessAddress:   row.BusinessAddress,
		BusinessPhone:     row.BusinessPhone,
		BusinessEmail:     row.BusinessEmail,
		SchedulingEnabled: row.SchedulingEnabled,
	}, nil
}
