package wire

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type tenantSummaryDTO struct {
	ID         string `json:"id"`
	Slug       string `json:"slug,omitempty"`
	Name       string `json:"name"`
	ClerkOrgID string `json:"clerk_org_id,omitempty"`
	Role       string `json:"role"`
}

func (s *pymesSaaSStore) ListTenantsForUser(ctx context.Context, userExternalID string) ([]tenantSummaryDTO, error) {
	var rows []struct {
		ID         uuid.UUID
		Name       string
		Slug       *string
		ExternalID *string
		ClerkOrgID *string
		Role       string
	}
	if err := s.db.WithContext(ctx).
		Table("tenants AS t").
		Select("t.id, t.name, t.slug, t.external_id, t.clerk_org_id, tm.role").
		Joins("JOIN tenant_memberships tm ON tm.tenant_id = t.id AND tm.status = 'active'").
		Joins("JOIN users u ON u.id = tm.user_id AND u.deleted_at IS NULL").
		Where("u.external_id = ?", strings.TrimSpace(userExternalID)).
		Order("t.name ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]tenantSummaryDTO, 0, len(rows))
	for _, row := range rows {
		clerkTenantID := ""
		if row.ClerkOrgID != nil {
			clerkTenantID = strings.TrimSpace(*row.ClerkOrgID)
		}
		if clerkTenantID == "" && row.ExternalID != nil && strings.HasPrefix(strings.TrimSpace(*row.ExternalID), "org_") {
			clerkTenantID = strings.TrimSpace(*row.ExternalID)
		}
		slug := ""
		if row.Slug != nil {
			slug = strings.TrimSpace(*row.Slug)
		}
		items = append(items, tenantSummaryDTO{
			ID:         row.ID.String(),
			Slug:       slug,
			Name:       row.Name,
			ClerkOrgID: clerkTenantID,
			Role:       normalizeTenantRole(row.Role),
		})
	}
	return items, nil
}

func (s *pymesSaaSStore) UpdateTenantMemberRole(ctx context.Context, tenantID, userID, role string) (tenantMemberDTO, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return tenantMemberDTO{}, domainerr.Validation("invalid tenant_id")
	}
	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return tenantMemberDTO{}, domainerr.Validation("invalid user_id")
	}
	role = normalizeInviteRole(role)
	var row pymesTenantMembershipRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND status = 'active'", tenantUUID, userUUID).
		Preload("User").
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tenantMemberDTO{}, domainerr.NotFound("member not found")
		}
		return tenantMemberDTO{}, err
	}
	if row.Role == "owner" {
		return tenantMemberDTO{}, domainerr.BusinessRule("owner role must be transferred before it can be changed")
	}
	if err := s.db.WithContext(ctx).Model(&pymesTenantMembershipRow{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{"role": role, "updated_at": time.Now().UTC()}).Error; err != nil {
		return tenantMemberDTO{}, err
	}
	row.Role = role
	return memberDTOFromRow(row), nil
}

func (s *pymesSaaSStore) RemoveTenantMember(ctx context.Context, tenantID, userID string) error {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return domainerr.Validation("invalid tenant_id")
	}
	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return domainerr.Validation("invalid user_id")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row pymesTenantMembershipRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ? AND status = 'active'", tenantUUID, userUUID).
			Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domainerr.NotFound("member not found")
			}
			return err
		}
		if row.Role == "owner" {
			return domainerr.BusinessRule("owner must transfer ownership before removal")
		}
		now := time.Now().UTC()
		return tx.Model(&pymesTenantMembershipRow{}).Where("id = ?", row.ID).Updates(map[string]any{
			"status":     "removed",
			"removed_at": now,
			"updated_at": now,
		}).Error
	})
}

func (s *pymesSaaSStore) TransferTenantOwnership(ctx context.Context, tenantID, actorExternalID, nextOwnerUserID string) error {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return domainerr.Validation("invalid tenant_id")
	}
	nextOwnerUUID, err := uuid.Parse(strings.TrimSpace(nextOwnerUserID))
	if err != nil {
		return domainerr.Validation("invalid user_id")
	}
	actor, err := s.requireTenantOwner(ctx, tenantID, actorExternalID)
	if err != nil {
		return err
	}
	if actor.ID == nextOwnerUUID {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current pymesTenantMembershipRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ? AND role = 'owner' AND status = 'active'", tenantUUID, actor.ID).
			Take(&current).Error; err != nil {
			return err
		}
		var next pymesTenantMembershipRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ? AND status = 'active'", tenantUUID, nextOwnerUUID).
			Take(&next).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domainerr.NotFound("next owner member not found")
			}
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&pymesTenantMembershipRow{}).Where("id = ?", current.ID).Updates(map[string]any{"role": "admin", "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&pymesTenantMembershipRow{}).Where("id = ?", next.ID).Updates(map[string]any{"role": "owner", "updated_at": now}).Error
	})
}

var slugUnsafeChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyTenantName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = slugUnsafeChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "tenant"
	}
	return name
}
