package wire

import (
	"context"
	"errors"
	"strings"
	"time"

	saasuserdomain "github.com/devpablocristo/core/saas/go/users/usecases/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *pymesSaaSStore) UpsertOrgMember(ctx context.Context, orgID, userID, role string) (saasuserdomain.OrgMember, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	var row pymesOrgMemberRow
	tx := s.db.WithContext(ctx)
	err = tx.Where("org_id = ? AND user_id = ?", orgUUID, userUUID).Preload("User").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = pymesOrgMemberRow{
			ID:        uuid.New(),
			OrgID:     orgUUID,
			UserID:    userUUID,
			Role:      strings.TrimSpace(role),
			CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&row).Error; err != nil {
			return saasuserdomain.OrgMember{}, err
		}
		if err := tx.Preload("User").Where("id = ?", row.ID).Take(&row).Error; err != nil {
			return saasuserdomain.OrgMember{}, err
		}
		return memberDomainFromRow(row), nil
	}
	if err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	row.Role = strings.TrimSpace(role)
	if err := tx.Model(&pymesOrgMemberRow{}).Where("id = ?", row.ID).Update("role", row.Role).Error; err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	row.Role = strings.TrimSpace(role)
	return memberDomainFromRow(row), nil
}

func (s *pymesSaaSStore) SyncMembership(ctx context.Context, orgID, userExternalID, email, name string, avatarURL *string, role string) (saasuserdomain.OrgMember, error) {
	user, err := s.UpsertUser(ctx, userExternalID, email, name, avatarURL)
	if err != nil {
		return saasuserdomain.OrgMember{}, err
	}
	return s.UpsertOrgMember(ctx, orgID, user.ID, role)
}

func (s *pymesSaaSStore) ListOrgMembers(ctx context.Context, orgID string) ([]saasuserdomain.OrgMember, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, err
	}
	var rows []pymesOrgMemberRow
	if err := s.db.WithContext(ctx).
		Where("org_id = ?", orgUUID).
		Preload("User").
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]saasuserdomain.OrgMember, 0, len(rows))
	for _, row := range rows {
		items = append(items, memberDomainFromRow(row))
	}
	return items, nil
}

func (s *pymesSaaSStore) RemoveMembership(ctx context.Context, userExternalID, orgExternalID, orgName string) error {
	var user pymesUserRow
	if err := s.db.WithContext(ctx).
		Where("external_id = ?", strings.TrimSpace(userExternalID)).
		Take(&user).Error; err != nil {
		return err
	}
	query := s.db.WithContext(ctx).Table("org_members AS om").
		Joins("JOIN orgs o ON o.id = om.org_id").
		Where("om.user_id = ?", user.ID)
	if value := strings.TrimSpace(orgExternalID); value != "" {
		query = query.Where("o.external_id = ?", value)
	} else if value := strings.TrimSpace(orgName); value != "" {
		query = query.Where("o.name = ?", value)
	}
	return query.Delete(&pymesOrgMemberRow{}).Error
}
