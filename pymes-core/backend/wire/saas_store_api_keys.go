package wire

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *pymesSaaSStore) FindPrincipalByAPIKeyHash(ctx context.Context, apiKeyHash string) (tenantAPIKeyPrincipal, string, error) {
	var key pymesTenantAPIKeyRow
	err := s.db.WithContext(ctx).
		Where("api_key_hash = ?", strings.TrimSpace(apiKeyHash)).
		Take(&key).Error
	if err != nil {
		return tenantAPIKeyPrincipal{}, "", err
	}
	scopes, err := s.loadKeyScopes(ctx, key.ID)
	if err != nil {
		return tenantAPIKeyPrincipal{}, "", err
	}
	return tenantAPIKeyPrincipal{
		TenantID: key.TenantID.String(),
		Scopes:   scopes,
	}, key.ID.String(), nil
}

func (s *pymesSaaSStore) ListAPIKeys(ctx context.Context, tenantID string) ([]tenantAPIKeyDTO, error) {
	rows, err := s.listAPIKeyRows(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	items := make([]tenantAPIKeyDTO, 0, len(rows))
	for _, row := range rows {
		scopes, err := s.loadKeyScopes(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, tenantAPIKeyDTO{
			ID:        row.ID.String(),
			TenantID:  row.TenantID.String(),
			Name:      row.Name,
			Scopes:    scopes,
			CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}

func (s *pymesSaaSStore) CreateAPIKey(ctx context.Context, tenantID, name string, scopes []string) (createdTenantAPIKey, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return createdTenantAPIKey{}, err
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return createdTenantAPIKey{}, err
	}
	key := pymesTenantAPIKeyRow{
		ID:         uuid.New(),
		TenantID:   tenantUUID,
		Name:       strings.TrimSpace(name),
		APIKeyHash: keyHash,
		KeyPrefix:  keyPrefix,
		CreatedAt:  time.Now().UTC(),
	}
	if key.Name == "" {
		key.Name = "api-key"
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&key).Error; err != nil {
			return err
		}
		return s.replaceKeyScopesTx(ctx, tx, key.ID, normalizeScopes(scopes, s.defaultKeyScopes))
	}); err != nil {
		return createdTenantAPIKey{}, err
	}
	keyScopes, err := s.loadKeyScopes(ctx, key.ID)
	if err != nil {
		return createdTenantAPIKey{}, err
	}
	return createdTenantAPIKey{
		APIKey: tenantAPIKeyDTO{
			ID:        key.ID.String(),
			TenantID:  key.TenantID.String(),
			Name:      key.Name,
			Scopes:    keyScopes,
			CreatedAt: key.CreatedAt,
		},
		Secret: rawKey,
	}, nil
}

func (s *pymesSaaSStore) DeleteAPIKey(ctx context.Context, tenantID, keyID string) error {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return err
	}
	keyUUID, err := uuid.Parse(strings.TrimSpace(keyID))
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", keyUUID, tenantUUID).
		Delete(&pymesTenantAPIKeyRow{}).Error
}

func (s *pymesSaaSStore) RotateAPIKey(ctx context.Context, tenantID, keyID string) (rotatedTenantAPIKey, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return rotatedTenantAPIKey{}, err
	}
	keyUUID, err := uuid.Parse(strings.TrimSpace(keyID))
	if err != nil {
		return rotatedTenantAPIKey{}, err
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return rotatedTenantAPIKey{}, err
	}
	var row pymesTenantAPIKeyRow
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", keyUUID, tenantUUID).
		Take(&row).Error; err != nil {
		return rotatedTenantAPIKey{}, err
	}
	now := time.Now().UTC()
	row.APIKeyHash = keyHash
	row.KeyPrefix = keyPrefix
	row.RotatedAt = &now
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return rotatedTenantAPIKey{}, err
	}
	scopes, err := s.loadKeyScopes(ctx, row.ID)
	if err != nil {
		return rotatedTenantAPIKey{}, err
	}
	return rotatedTenantAPIKey{
		APIKey: tenantAPIKeyDTO{
			ID:        row.ID.String(),
			TenantID:  row.TenantID.String(),
			Name:      row.Name,
			Scopes:    scopes,
			CreatedAt: row.CreatedAt,
		},
		Secret: rawKey,
	}, nil
}

func (s *pymesSaaSStore) listAPIKeyRows(ctx context.Context, tenantID string) ([]pymesTenantAPIKeyRow, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return nil, err
	}
	var rows []pymesTenantAPIKeyRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantUUID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *pymesSaaSStore) loadKeyScopes(ctx context.Context, keyID uuid.UUID) ([]string, error) {
	var scopes []string
	if err := s.db.WithContext(ctx).
		Table("tenant_api_key_scopes").
		Where("api_key_id = ?", keyID).
		Order("scope ASC").
		Pluck("scope", &scopes).Error; err != nil {
		return nil, err
	}
	sort.Strings(scopes)
	return scopes, nil
}

func (s *pymesSaaSStore) replaceKeyScopesTx(_ context.Context, tx *gorm.DB, keyID uuid.UUID, scopes []string) error {
	if err := tx.Where("api_key_id = ?", keyID).Delete(&pymesTenantAPIKeyScopeRow{}).Error; err != nil {
		return err
	}
	for _, scope := range scopes {
		row := pymesTenantAPIKeyScopeRow{
			ID:       uuid.New(),
			APIKeyID: keyID,
			Scope:    scope,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
