package wire

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetOrgNameByOrgUUID devuelve el nombre legible de `orgs` para el UUID interno (tenant_id / org_id del token).
func (s *pymesSaaSStore) GetOrgNameByOrgUUID(ctx context.Context, orgID string) (string, bool, error) {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return "", false, nil
	}
	id, err := uuid.Parse(orgID)
	if err != nil {
		return "", false, nil
	}
	var row pymesOrgRow
	err = s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	name := strings.TrimSpace(row.Name)
	if name == "" {
		return "", false, nil
	}
	return name, true, nil
}

func (s *pymesSaaSStore) FindOrgIDByExternalID(ctx context.Context, externalID string) (string, bool, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return "", false, nil
	}
	if orgID, ok, err := s.findOrgIDByUUID(ctx, externalID); ok || err != nil {
		return orgID, ok, err
	}
	if orgID, ok, err := s.findOrgIDByExternalRef(ctx, externalID); ok || err != nil {
		return orgID, ok, err
	}
	if !shouldAutoProvisionOrg(externalID) {
		return "", false, nil
	}
	orgID, err := s.autoProvisionOrgForVerifiedExternalID(ctx, externalID)
	if err != nil {
		return "", false, err
	}
	return orgID, true, nil
}

func (s *pymesSaaSStore) findOrgIDByUUID(ctx context.Context, externalID string) (string, bool, error) {
	id, err := uuid.Parse(strings.TrimSpace(externalID))
	if err != nil {
		return "", false, nil
	}
	var row pymesOrgRow
	err = s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.ID.String(), true, nil
}

func (s *pymesSaaSStore) findOrgIDByExternalRef(ctx context.Context, externalID string) (string, bool, error) {
	var row pymesOrgRow
	err := s.db.WithContext(ctx).
		Where("external_id = ?", strings.TrimSpace(externalID)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.ID.String(), true, nil
}

func shouldAutoProvisionOrg(externalID string) bool {
	return strings.HasPrefix(strings.TrimSpace(externalID), "org_")
}

func (s *pymesSaaSStore) autoProvisionOrgForVerifiedExternalID(ctx context.Context, externalID string) (string, error) {
	// Clerk emite org_id tipo org_...; si el webhook aún no materializó la fila local,
	// provisionamos una org mínima porque el JWT/API key ya fue verificado aguas arriba.
	return s.UpsertOrg(ctx, strings.TrimSpace(externalID), "Organization")
}

func (s *pymesSaaSStore) UpsertOrg(ctx context.Context, externalID, orgName string) (string, error) {
	externalID = strings.TrimSpace(externalID)
	orgName = strings.TrimSpace(orgName)
	var row pymesOrgRow
	err := s.db.WithContext(ctx).Where("external_id = ?", externalID).Take(&row).Error
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = pymesOrgRow{
			ID:         uuid.New(),
			Name:       orgName,
			CreatedAt:  now,
			UpdatedAt:  now,
			ExternalID: stringPtr(externalID),
		}
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return "", err
		}
		if err := s.ensureTenantSettings(ctx, row.ID); err != nil {
			return "", err
		}
		return row.ID.String(), nil
	}
	if err != nil {
		return "", err
	}
	row.Name = orgName
	row.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return "", err
	}
	if err := s.ensureTenantSettings(ctx, row.ID); err != nil {
		return "", err
	}
	return row.ID.String(), nil
}

func (s *pymesSaaSStore) SyncOrganization(ctx context.Context, orgExternalID, orgName string) (string, error) {
	return s.UpsertOrg(ctx, orgExternalID, orgName)
}
