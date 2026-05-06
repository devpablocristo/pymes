package wire

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *pymesSaaSStore) CreateOrgWithDefaultKey(ctx context.Context, name, slug, actor string) (string, string, pymesAPIKeyRow, []string, error) {
	now := time.Now().UTC()
	org := pymesOrgRow{
		ID:         uuid.New(),
		Name:       strings.TrimSpace(name),
		CreatedAt:  now,
		UpdatedAt:  now,
		ExternalID: stringPtr(strings.TrimSpace(slug)),
		Slug:       stringPtr(strings.TrimSpace(slug)),
	}
	if org.Name == "" {
		return "", "", pymesAPIKeyRow{}, nil, fmt.Errorf("name is required")
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return "", "", pymesAPIKeyRow{}, nil, err
	}
	key := pymesAPIKeyRow{
		ID:         uuid.New(),
		OrgID:      org.ID,
		Name:       "default",
		APIKeyHash: keyHash,
		KeyPrefix:  keyPrefix,
		CreatedAt:  now,
		CreatedBy:  stringPtr(strings.TrimSpace(actor)),
	}
	scopes := normalizeScopes(nil, s.defaultKeyScopes)
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			return err
		}
		settings := pymesTenantSettingsRow{
			OrgID:          org.ID,
			PlanCode:       "starter",
			BillingStatus:  "trialing",
			Status:         "active",
			HardLimits:     mustJSONBytes(defaultSaaSHardLimits()),
			HardLimitsJSON: mustJSONBytes(defaultSaaSHardLimits()),
			CreatedAt:      now,
			UpdatedAt:      now,
			UpdatedBy:      stringPtr(strings.TrimSpace(actor)),
		}
		if err := tx.Save(&settings).Error; err != nil {
			return err
		}
		if err := tx.Create(&key).Error; err != nil {
			return err
		}
		return s.replaceKeyScopesTx(ctx, tx, key.ID, scopes)
	}); err != nil {
		return "", "", pymesAPIKeyRow{}, nil, err
	}
	return org.ID.String(), rawKey, key, scopes, nil
}
