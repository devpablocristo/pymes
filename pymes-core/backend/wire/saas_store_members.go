package wire

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *pymesSaaSStore) UpsertTenantMember(ctx context.Context, tenantID, userID, role string) (tenantMemberDTO, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return tenantMemberDTO{}, err
	}
	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return tenantMemberDTO{}, err
	}
	var row pymesTenantMembershipRow
	tx := s.db.WithContext(ctx)
	err = tx.Where("tenant_id = ? AND user_id = ?", tenantUUID, userUUID).Preload("User").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = pymesTenantMembershipRow{
			ID:        uuid.New(),
			TenantID:  tenantUUID,
			UserID:    userUUID,
			Role:      normalizeTenantRole(role),
			Status:    "active",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&row).Error; err != nil {
			return tenantMemberDTO{}, err
		}
		if err := tx.Preload("User").Where("id = ?", row.ID).Take(&row).Error; err != nil {
			return tenantMemberDTO{}, err
		}
		return memberDTOFromRow(row), nil
	}
	if err != nil {
		return tenantMemberDTO{}, err
	}
	nextRole := normalizeTenantRole(role)
	if row.Role == "owner" && nextRole != "owner" {
		return tenantMemberDTO{}, domainerr.BusinessRule("owner role must be transferred before it can be changed")
	}
	row.Role = nextRole
	if err := tx.Model(&pymesTenantMembershipRow{}).Where("id = ?", row.ID).Updates(map[string]any{"role": row.Role, "status": "active", "removed_at": nil, "updated_at": time.Now().UTC()}).Error; err != nil {
		return tenantMemberDTO{}, err
	}
	row.Status = "active"
	return memberDTOFromRow(row), nil
}

func (s *pymesSaaSStore) ListTenantMembers(ctx context.Context, tenantID string) ([]tenantMemberDTO, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return nil, err
	}
	var rows []pymesTenantMembershipRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantUUID).
		Where("status = 'active'").
		Preload("User").
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]tenantMemberDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, memberDTOFromRow(row))
	}
	return items, nil
}

func (s *pymesSaaSStore) FindActiveMembershipRoleByExternalUser(ctx context.Context, tenantID, userExternalID string) (string, bool, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return "", false, err
	}
	var row struct {
		Role string
	}
	err = s.db.WithContext(ctx).
		Table("tenant_memberships AS om").
		Select("om.role").
		Joins("JOIN users u ON u.id = om.user_id").
		Where("om.tenant_id = ? AND om.status = 'active' AND u.external_id = ? AND u.deleted_at IS NULL", tenantUUID, strings.TrimSpace(userExternalID)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return normalizeTenantRole(row.Role), true, nil
}

func (s *pymesSaaSStore) LocalUserByExternalID(ctx context.Context, userExternalID string) (pymesUserRow, bool, error) {
	var row pymesUserRow
	err := s.db.WithContext(ctx).Where("external_id = ? AND deleted_at IS NULL", strings.TrimSpace(userExternalID)).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pymesUserRow{}, false, nil
	}
	if err != nil {
		return pymesUserRow{}, false, err
	}
	return row, true, nil
}

func (s *pymesSaaSStore) requireTenantOwner(ctx context.Context, tenantID, actorExternalID string) (pymesUserRow, error) {
	tenantUUID, err := uuid.Parse(strings.TrimSpace(tenantID))
	if err != nil {
		return pymesUserRow{}, domainerr.Validation("invalid tenant_id")
	}
	var user pymesUserRow
	err = s.db.WithContext(ctx).
		Table("users").
		Select("users.*").
		Joins("JOIN tenant_memberships om ON om.user_id = users.id").
		Where("users.external_id = ? AND users.deleted_at IS NULL", strings.TrimSpace(actorExternalID)).
		Where("om.tenant_id = ? AND om.role = 'owner' AND om.status = 'active'", tenantUUID).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pymesUserRow{}, domainerr.Forbidden("tenant owner privileges required")
	}
	if err != nil {
		return pymesUserRow{}, err
	}
	return user, nil
}

func normalizeTenantRole(role string) string {
	role = strings.TrimSpace(role)
	role = strings.TrimPrefix(role, "org:")
	switch role {
	case "owner":
		return "owner"
	case "admin":
		return "admin"
	default:
		return "member"
	}
}

func normalizeInviteRole(role string) string {
	role = normalizeTenantRole(role)
	if role == "owner" {
		return "admin"
	}
	return role
}

func clerkRoleFromTenantRole(role string) string {
	switch normalizeTenantRole(role) {
	case "admin", "owner":
		return "org:admin"
	default:
		return "org:member"
	}
}

func (s *pymesSaaSStore) acceptPendingInviteForWebhook(ctx context.Context, tenantUUID uuid.UUID, user pymesUserRow, email string) (tenantMemberDTO, bool, error) {
	email = normalizeEmail(email)
	if email == "" {
		return tenantMemberDTO{}, false, nil
	}
	var member tenantMemberDTO
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var invite pymesTenantInvitationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND email_normalized = ? AND status = 'pending' AND expires_at > now()", tenantUUID, email).
			Order("created_at ASC").
			Take(&invite).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return err
		}
		now := time.Now().UTC()
		row := pymesTenantMembershipRow{
			ID:        uuid.New(),
			TenantID:  tenantUUID,
			UserID:    user.ID,
			Role:      normalizeInviteRole(invite.Role),
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Model(&pymesTenantInvitationRow{}).
			Where("id = ?", invite.ID).
			Updates(map[string]any{
				"status":              "accepted",
				"accepted_by_user_id": user.ID,
				"accepted_at":         now,
				"updated_at":          now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Preload("User").Where("id = ?", row.ID).Take(&row).Error; err != nil {
			return err
		}
		member = memberDTOFromRow(row)
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantMemberDTO{}, false, nil
	}
	if err != nil {
		return tenantMemberDTO{}, false, err
	}
	return member, true, nil
}
