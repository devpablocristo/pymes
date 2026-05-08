package wire

import (
	"context"
	"fmt"
	"strings"
	"time"

	corepostgres "github.com/devpablocristo/core/databases/postgres/go"
	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *pymesSaaSStore) CreateTenantWithClerkOrganization(ctx context.Context, name, slug, clerkTenantID, ownerExternalID, ownerEmail, ownerName string, avatarURL *string) (string, string, string, pymesTenantAPIKeyRow, []string, error) {
	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(slug)
	clerkTenantID = strings.TrimSpace(clerkTenantID)
	ownerExternalID = strings.TrimSpace(ownerExternalID)
	if name == "" {
		return "", "", "", pymesTenantAPIKeyRow{}, nil, fmt.Errorf("name is required")
	}
	if slug == "" {
		slug = slugifyTenantName(name)
	}
	createdClerkOrg := false
	if clerkTenantID == "" {
		if s.clerk == nil {
			return "", "", "", pymesTenantAPIKeyRow{}, nil, domainerr.Unavailable("clerk backend client is not configured")
		}
		org, err := s.clerk.CreateOrganization(ctx, clerkCreateOrganizationInput{
			Name:      name,
			CreatedBy: ownerExternalID,
			PublicMetadata: map[string]any{
				"pymes_tenant_slug": slug,
			},
		})
		if err != nil {
			return "", "", "", pymesTenantAPIKeyRow{}, nil, err
		}
		clerkTenantID = strings.TrimSpace(org.ID)
		createdClerkOrg = true
	} else if s.clerk != nil && ownerExternalID != "" {
		ok, err := s.clerk.UserHasOrganizationMembership(ctx, clerkTenantID, ownerExternalID)
		if err != nil {
			return "", "", "", pymesTenantAPIKeyRow{}, nil, err
		}
		if !ok {
			return "", "", "", pymesTenantAPIKeyRow{}, nil, domainerr.Forbidden("user is not a member of the Clerk organization")
		}
	}
	tenantID, rawKey, key, scopes, err := s.CreateTenantWithOwner(ctx, name, slug, clerkTenantID, ownerExternalID, ownerEmail, ownerName, avatarURL)
	if err != nil {
		if createdClerkOrg && s.clerk != nil {
			_ = s.clerk.DeleteOrganization(ctx, clerkTenantID)
		}
		return "", "", "", pymesTenantAPIKeyRow{}, nil, err
	}
	return tenantID, clerkTenantID, rawKey, key, scopes, nil
}

func (s *pymesSaaSStore) CreateTenantWithOwner(ctx context.Context, name, slug, clerkTenantID, ownerExternalID, ownerEmail, ownerName string, avatarURL *string) (string, string, pymesTenantAPIKeyRow, []string, error) {
	now := time.Now().UTC()
	clerkTenantID = strings.TrimSpace(clerkTenantID)
	if clerkTenantID == "" {
		return "", "", pymesTenantAPIKeyRow{}, nil, domainerr.Validation("clerk organization is required")
	}
	if !strings.HasPrefix(clerkTenantID, "org_") {
		return "", "", pymesTenantAPIKeyRow{}, nil, domainerr.Validation("clerk organization id must start with org_")
	}
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
		if corepostgres.IsUniqueViolation(err) {
			return "", "", pymesTenantAPIKeyRow{}, nil, domainerr.Conflict("tenant slug already exists")
		}
		return "", "", pymesTenantAPIKeyRow{}, nil, err
	}
	return org.ID.String(), rawKey, key, scopes, nil
}
