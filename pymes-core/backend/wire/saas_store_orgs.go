package wire

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetTenantNameByID devuelve el nombre legible del tenant para el UUID interno.
func (s *pymesSaaSStore) GetTenantNameByID(ctx context.Context, tenantID string) (string, bool, error) {
	name, _, ok, err := s.GetTenantNameSlugByID(ctx, tenantID)
	return name, ok, err
}

// GetTenantNameSlugByID devuelve nombre y slug del tenant para el UUID interno.
func (s *pymesSaaSStore) GetTenantNameSlugByID(ctx context.Context, tenantID string) (string, string, bool, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return "", "", false, nil
	}
	id, err := uuid.Parse(tenantID)
	if err != nil {
		return "", "", false, nil
	}
	var row pymesTenantRow
	err = s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	name := strings.TrimSpace(row.Name)
	slug := ""
	if row.Slug != nil {
		slug = strings.TrimSpace(*row.Slug)
	}
	if name == "" && slug == "" {
		return "", "", false, nil
	}
	return name, slug, true, nil
}

func (s *pymesSaaSStore) ResolveTenantIDByExternalRef(ctx context.Context, ref string) (string, bool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false, nil
	}
	if tenantID, ok, err := s.findTenantIDByUUID(ctx, ref); ok || err != nil {
		return tenantID, ok, err
	}
	if tenantID, ok, err := s.findTenantIDByExternalRef(ctx, ref); ok || err != nil {
		return tenantID, ok, err
	}
	return "", false, nil
}

func (s *pymesSaaSStore) findTenantIDByUUID(ctx context.Context, ref string) (string, bool, error) {
	id, err := uuid.Parse(strings.TrimSpace(ref))
	if err != nil {
		return "", false, nil
	}
	var row pymesTenantRow
	err = s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.ID.String(), true, nil
}

func (s *pymesSaaSStore) findTenantIDByExternalRef(ctx context.Context, ref string) (string, bool, error) {
	var row pymesTenantRow
	ref = strings.TrimSpace(ref)
	err := s.db.WithContext(ctx).
		Where("clerk_org_id = ? OR external_id = ? OR slug = ?", ref, ref, ref).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.ID.String(), true, nil
}
