package wire

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *pymesSaaSStore) CreateOrgWithDefaultKey(ctx context.Context, name, slug, actor string) (string, string, pymesTenantAPIKeyRow, []string, error) {
	return s.CreateTenantWithOwner(ctx, name, slug, strings.TrimSpace(slug), actor, "", "", nil)
}

func (s *pymesSaaSStore) CreateTenantWithOwner(ctx context.Context, name, slug, clerkTenantID, ownerExternalID, ownerEmail, ownerName string, avatarURL *string) (string, string, pymesTenantAPIKeyRow, []string, error) {
	now := time.Now().UTC()
	clerkTenantID = strings.TrimSpace(clerkTenantID)
	org := pymesTenantRow{
		ID:         uuid.New(),
		Name:       strings.TrimSpace(name),
		CreatedAt:  now,
		UpdatedAt:  now,
		ExternalID: stringPtr(clerkTenantID),
		ClerkOrgID: stringPtr(clerkTenantID),
		Slug:       stringPtr(strings.TrimSpace(slug)),
	}
	if org.Name == "" {
		return "", "", pymesTenantAPIKeyRow{}, nil, fmt.Errorf("name is required")
	}
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return "", "", pymesTenantAPIKeyRow{}, nil, err
	}
	key := pymesTenantAPIKeyRow{
		ID:         uuid.New(),
		TenantID:   org.ID,
		Name:       "default",
		APIKeyHash: keyHash,
		KeyPrefix:  keyPrefix,
		CreatedAt:  now,
		CreatedBy:  stringPtr(strings.TrimSpace(ownerExternalID)),
	}
	scopes := normalizeScopes(nil, s.defaultKeyScopes)
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			return err
		}
		if strings.TrimSpace(ownerExternalID) != "" {
			user, err := s.upsertUserTx(ctx, tx, ownerExternalID, ownerEmail, ownerName, avatarURL)
			if err != nil {
				return err
			}
			member := pymesTenantMembershipRow{
				ID:        uuid.New(),
				TenantID:  org.ID,
				UserID:    user.ID,
				Role:      "owner",
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		}
		settings := pymesTenantSettingsRow{
			TenantID:       org.ID,
			PlanCode:       "starter",
			BillingStatus:  "trialing",
			Status:         "active",
			HardLimits:     mustJSONBytes(defaultSaaSHardLimits()),
			HardLimitsJSON: mustJSONBytes(defaultSaaSHardLimits()),
			CreatedAt:      now,
			UpdatedAt:      now,
			UpdatedBy:      stringPtr(strings.TrimSpace(ownerExternalID)),
		}
		if err := tx.Create(&settings).Error; err != nil {
			return err
		}
		if err := tx.Create(&key).Error; err != nil {
			return err
		}
		return s.replaceKeyScopesTx(ctx, tx, key.ID, scopes)
	}); err != nil {
		return "", "", pymesTenantAPIKeyRow{}, nil, err
	}
	return org.ID.String(), rawKey, key, scopes, nil
}
