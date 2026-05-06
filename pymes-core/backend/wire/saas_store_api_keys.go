package wire

import (
	"context"
	"sort"
	"strings"
	"time"

	saasorgdomain "github.com/devpablocristo/core/saas/go/org/usecases/domain"
	saasuserdomain "github.com/devpablocristo/core/saas/go/users/usecases/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *pymesSaaSStore) FindPrincipalByAPIKeyHash(ctx context.Context, apiKeyHash string) (saasorgdomain.Principal, string, error) {
	var key pymesAPIKeyRow
	err := s.db.WithContext(ctx).
		Where("api_key_hash = ?", strings.TrimSpace(apiKeyHash)).
		Take(&key).Error
	if err != nil {
		return saasorgdomain.Principal{}, "", err
	}
	scopes, err := s.loadKeyScopes(ctx, key.ID)
	if err != nil {
		return saasorgdomain.Principal{}, "", err
	}
	return saasorgdomain.Principal{
		TenantID: key.OrgID.String(),
		Scopes:   scopes,
	}, key.ID.String(), nil
}

func (s *pymesSaaSStore) ListAPIKeys(ctx context.Context, orgID string) ([]saasuserdomain.APIKey, error) {
	rows, err := s.listAPIKeyRows(ctx, orgID)
	if err != nil {
		return nil, err
	}
	items := make([]saasuserdomain.APIKey, 0, len(rows))
	for _, row := range rows {
		scopes, err := s.loadKeyScopes(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, saasuserdomain.APIKey{
			ID:        row.ID.String(),
			OrgID:     row.OrgID.String(),
			Name:      row.Name,
			Scopes:    scopes,
			CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}

func (s *pymesSaaSStore) CreateAPIKey(ctx context.Context, orgID, name string, scopes []string) (saasuserdomain.CreatedAPIKey, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return saasuserdomain.CreatedAPIKey{}, err
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return saasuserdomain.CreatedAPIKey{}, err
	}
	key := pymesAPIKeyRow{
		ID:         uuid.New(),
		OrgID:      orgUUID,
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
		return saasuserdomain.CreatedAPIKey{}, err
	}
	keyScopes, err := s.loadKeyScopes(ctx, key.ID)
	if err != nil {
		return saasuserdomain.CreatedAPIKey{}, err
	}
	return saasuserdomain.CreatedAPIKey{
		APIKey: saasuserdomain.APIKey{
			ID:        key.ID.String(),
			OrgID:     key.OrgID.String(),
			Name:      key.Name,
			Scopes:    keyScopes,
			CreatedAt: key.CreatedAt,
		},
		Secret: rawKey,
	}, nil
}

func (s *pymesSaaSStore) DeleteAPIKey(ctx context.Context, orgID, keyID string) error {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return err
	}
	keyUUID, err := uuid.Parse(strings.TrimSpace(keyID))
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).
		Where("id = ? AND org_id = ?", keyUUID, orgUUID).
		Delete(&pymesAPIKeyRow{}).Error
}

func (s *pymesSaaSStore) RotateAPIKey(ctx context.Context, orgID, keyID string) (saasuserdomain.RotatedAPIKey, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	keyUUID, err := uuid.Parse(strings.TrimSpace(keyID))
	if err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	var row pymesAPIKeyRow
	if err := s.db.WithContext(ctx).
		Where("id = ? AND org_id = ?", keyUUID, orgUUID).
		Take(&row).Error; err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	now := time.Now().UTC()
	row.APIKeyHash = keyHash
	row.KeyPrefix = keyPrefix
	row.RotatedAt = &now
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	scopes, err := s.loadKeyScopes(ctx, row.ID)
	if err != nil {
		return saasuserdomain.RotatedAPIKey{}, err
	}
	return saasuserdomain.RotatedAPIKey{
		APIKey: saasuserdomain.APIKey{
			ID:        row.ID.String(),
			OrgID:     row.OrgID.String(),
			Name:      row.Name,
			Scopes:    scopes,
			CreatedAt: row.CreatedAt,
		},
		Secret: rawKey,
	}, nil
}

func (s *pymesSaaSStore) listAPIKeyRows(ctx context.Context, orgID string) ([]pymesAPIKeyRow, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, err
	}
	var rows []pymesAPIKeyRow
	if err := s.db.WithContext(ctx).
		Where("org_id = ?", orgUUID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *pymesSaaSStore) loadKeyScopes(ctx context.Context, keyID uuid.UUID) ([]string, error) {
	var scopes []string
	if err := s.db.WithContext(ctx).
		Table("org_api_key_scopes").
		Where("api_key_id = ?", keyID).
		Order("scope ASC").
		Pluck("scope", &scopes).Error; err != nil {
		return nil, err
	}
	sort.Strings(scopes)
	return scopes, nil
}

func (s *pymesSaaSStore) replaceKeyScopesTx(_ context.Context, tx *gorm.DB, keyID uuid.UUID, scopes []string) error {
	if err := tx.Where("api_key_id = ?", keyID).Delete(&pymesAPIKeyScopeRow{}).Error; err != nil {
		return err
	}
	for _, scope := range scopes {
		row := pymesAPIKeyScopeRow{
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
