package inappnotifications

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inappnotifications/usecases/domain"
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

func (r *Repository) ListForUser(orgID, userID uuid.UUID, limit int) ([]domain.InAppNotification, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var rows []models.InAppNotificationModel
	err := r.db.Where("org_id = ? AND user_id = ?", orgID, userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.InAppNotification, 0, len(rows))
	for _, row := range rows {
		ctx := row.ChatContext
		if len(ctx) == 0 {
			ctx = []byte("{}")
		}
		out = append(out, domain.InAppNotification{
			ID:          row.ID,
			OrgID:       row.OrgID,
			UserID:      row.UserID,
			Title:       row.Title,
			Body:        row.Body,
			Kind:        row.Kind,
			EntityType:  row.EntityType,
			EntityID:    row.EntityID,
			ChatContext: json.RawMessage(ctx),
			ReadAt:      row.ReadAt,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out, nil
}

func (r *Repository) CountUnread(orgID, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.Model(&models.InAppNotificationModel{}).
		Where("org_id = ? AND user_id = ? AND read_at IS NULL", orgID, userID).
		Count(&n).Error
	return n, err
}

// MarkRead marca leída si pertenece a org y usuario; devuelve el timestamp aplicado.
func (r *Repository) MarkRead(orgID, userID, notifID uuid.UUID) (time.Time, error) {
	now := time.Now().UTC()
	res := r.db.Model(&models.InAppNotificationModel{}).
		Where("id = ? AND org_id = ? AND user_id = ?", notifID, orgID, userID).
		Update("read_at", now)
	if res.Error != nil {
		return time.Time{}, res.Error
	}
	if res.RowsAffected == 0 {
		return time.Time{}, ErrNotFound
	}
	return now, nil
}
