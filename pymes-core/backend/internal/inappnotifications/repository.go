package inappnotifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	coredomain "github.com/devpablocristo/core/notifications/go/inbox/usecases/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications/repository/models"
)

var ErrNotFound = errors.New("in_app_notification not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserIDByExternalID(externalID string) (uuid.UUID, bool) {
	var row struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	err := r.db.Table("users").
		Select("id").
		Where("external_id = ? AND deleted_at IS NULL", externalID).
		First(&row).Error
	if err != nil {
		return uuid.Nil, false
	}
	return row.ID, true
}

func (r *Repository) GetOnlyUserIDByOrg(orgID uuid.UUID) (uuid.UUID, bool) {
	var rows []struct {
		UserID uuid.UUID `gorm:"column:user_id"`
	}
	err := r.db.Table("org_members AS om").
		Select("om.user_id").
		Joins("JOIN users AS u ON u.id = om.user_id").
		Where("om.org_id = ? AND u.deleted_at IS NULL", orgID).
		Order("om.created_at ASC, om.user_id ASC").
		Limit(2).
		Find(&rows).Error
	if err != nil {
		return uuid.Nil, false
	}
	if len(rows) != 1 {
		return uuid.Nil, false
	}
	return rows[0].UserID, true
}

func (r *Repository) ListUserIDsByOrg(orgID uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		UserID uuid.UUID `gorm:"column:user_id"`
	}
	err := r.db.Table("org_members AS om").
		Select("DISTINCT om.user_id").
		Joins("JOIN users AS u ON u.id = om.user_id").
		Where("om.org_id = ? AND u.deleted_at IS NULL", orgID).
		Order("om.user_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.UserID)
	}
	return out, nil
}

func (r *Repository) ListOrgIDsWithUsers() ([]uuid.UUID, error) {
	var rows []struct {
		OrgID uuid.UUID `gorm:"column:org_id"`
	}
	err := r.db.Table("org_members AS om").
		Select("DISTINCT om.org_id").
		Joins("JOIN users AS u ON u.id = om.user_id").
		Where("u.deleted_at IS NULL").
		Order("om.org_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.OrgID)
	}
	return out, nil
}

func (r *Repository) ListForRecipient(ctx context.Context, tenantID, recipientID string, limit int) ([]coredomain.Notification, error) {
	orgID, userID, err := parseRecipientScope(tenantID, recipientID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var rows []models.InAppNotificationModel
	err = r.db.WithContext(ctx).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]coredomain.Notification, 0, len(rows))
	for _, row := range rows {
		metadata := row.ChatContext
		if len(metadata) == 0 {
			metadata = []byte("{}")
		}
		out = append(out, coredomain.Notification{
			ID:          row.ID.String(),
			TenantID:    row.OrgID.String(),
			RecipientID: row.UserID.String(),
			Title:       row.Title,
			Body:        row.Body,
			Kind:        row.Kind,
			EntityType:  row.EntityType,
			EntityID:    row.EntityID,
			Metadata:    json.RawMessage(metadata),
			ReadAt:      row.ReadAt,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out, nil
}

func (r *Repository) CountUnread(ctx context.Context, tenantID, recipientID string) (int64, error) {
	orgID, userID, err := parseRecipientScope(tenantID, recipientID)
	if err != nil {
		return 0, err
	}
	var n int64
	err = r.db.WithContext(ctx).Model(&models.InAppNotificationModel{}).
		Where("org_id = ? AND user_id = ? AND read_at IS NULL", orgID, userID).
		Count(&n).Error
	return n, err
}

func (r *Repository) Append(ctx context.Context, notification coredomain.Notification) (coredomain.Notification, error) {
	orgID, userID, err := parseRecipientScope(notification.TenantID, notification.RecipientID)
	if err != nil {
		return coredomain.Notification{}, err
	}
	id, err := parseOrNewUUID(notification.TenantID+":"+notification.RecipientID, notification.ID)
	if err != nil {
		return coredomain.Notification{}, err
	}
	existing, ok, err := r.findByID(ctx, id)
	if err != nil {
		return coredomain.Notification{}, err
	}
	if ok {
		return existing, nil
	}
	createdAt := notification.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	metadata := normalizeMetadata(notification.Metadata)
	row := models.InAppNotificationModel{
		ID:          id,
		OrgID:       orgID,
		UserID:      userID,
		Title:       notification.Title,
		Body:        notification.Body,
		Kind:        notification.Kind,
		EntityType:  notification.EntityType,
		EntityID:    notification.EntityID,
		ChatContext: metadata,
		ReadAt:      nil,
		CreatedAt:   createdAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		existing, ok, lookupErr := r.findByID(ctx, id)
		if lookupErr == nil && ok {
			return existing, nil
		}
		return coredomain.Notification{}, err
	}
	return rowToDomain(row), nil
}

// MarkRead marca leída si pertenece a org y usuario; devuelve el timestamp aplicado.
func (r *Repository) MarkRead(ctx context.Context, tenantID, recipientID, notificationID string, readAt time.Time) (time.Time, error) {
	orgID, userID, err := parseRecipientScope(tenantID, recipientID)
	if err != nil {
		return time.Time{}, err
	}
	notifID, err := uuid.Parse(notificationID)
	if err != nil {
		return time.Time{}, fmt.Errorf("notification_id: %w", err)
	}
	if readAt.IsZero() {
		readAt = time.Now().UTC()
	}
	res := r.db.WithContext(ctx).Model(&models.InAppNotificationModel{}).
		Where("id = ? AND org_id = ? AND user_id = ?", notifID, orgID, userID).
		Update("read_at", readAt)
	if res.Error != nil {
		return time.Time{}, res.Error
	}
	if res.RowsAffected == 0 {
		return time.Time{}, ErrNotFound
	}
	return readAt, nil
}

func (r *Repository) ResolveApprovalNotifications(ctx context.Context, tenantID, approvalID, requestID string, readAt time.Time) (int64, error) {
	approvalID = strings.TrimSpace(approvalID)
	requestID = strings.TrimSpace(requestID)
	if approvalID == "" && requestID == "" {
		return 0, fmt.Errorf("approval lookup: %w", ErrNotFound)
	}
	if readAt.IsZero() {
		readAt = time.Now().UTC()
	}
	query := r.db.WithContext(ctx).Model(&models.InAppNotificationModel{}).
		Where("kind = ? AND entity_type = ? AND read_at IS NULL", approvalNotificationKind, approvalNotificationEntityType)
	if strings.TrimSpace(tenantID) != "" {
		orgID, err := uuid.Parse(strings.TrimSpace(tenantID))
		if err != nil {
			return 0, fmt.Errorf("tenant_id: %w", err)
		}
		query = query.Where("org_id = ?", orgID)
	}
	switch {
	case approvalID != "" && requestID != "":
		query = query.Where("(entity_id = ? OR chat_context -> 'approval' ->> 'request_id' = ?)", approvalID, requestID)
	case approvalID != "":
		query = query.Where("entity_id = ?", approvalID)
	default:
		query = query.Where("chat_context -> 'approval' ->> 'request_id' = ?", requestID)
	}
	result := query.Update("read_at", readAt)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func parseRecipientScope(tenantID, recipientID string) (uuid.UUID, uuid.UUID, error) {
	orgID, err := uuid.Parse(tenantID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("tenant_id: %w", err)
	}
	userID, err := uuid.Parse(recipientID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("recipient_id: %w", err)
	}
	return orgID, userID, nil
}

func parseOrNewUUID(scope, raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err == nil {
		return id, nil
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(scope+":"+raw)), nil
}

func normalizeMetadata(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

func (r *Repository) findByID(ctx context.Context, id uuid.UUID) (coredomain.Notification, bool, error) {
	var row models.InAppNotificationModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return coredomain.Notification{}, false, nil
	}
	if err != nil {
		return coredomain.Notification{}, false, err
	}
	return rowToDomain(row), true, nil
}

func rowToDomain(row models.InAppNotificationModel) coredomain.Notification {
	metadata := row.ChatContext
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}
	return coredomain.Notification{
		ID:          row.ID.String(),
		TenantID:    row.OrgID.String(),
		RecipientID: row.UserID.String(),
		Title:       row.Title,
		Body:        row.Body,
		Kind:        row.Kind,
		EntityType:  row.EntityType,
		EntityID:    row.EntityID,
		Metadata:    json.RawMessage(metadata),
		ReadAt:      row.ReadAt,
		CreatedAt:   row.CreatedAt,
	}
}
