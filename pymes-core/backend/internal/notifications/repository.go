package notifications

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/notifications/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/notifications/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByExternalID(externalID string) (uuid.UUID, string, bool) {
	var row struct {
		ID    uuid.UUID `gorm:"column:id"`
		Email string    `gorm:"column:email"`
	}
	err := r.db.Table("users").
		Select("id, email").
		Where("external_id = ? AND deleted_at IS NULL", externalID).
		First(&row).Error
	if err != nil {
		return uuid.Nil, "", false
	}
	return row.ID, row.Email, true
}

func (r *Repository) ListMembers(orgID uuid.UUID) []Member {
	type memberRow struct {
		UserID uuid.UUID `gorm:"column:user_id"`
		Email  string    `gorm:"column:email"`
		Role   string    `gorm:"column:role"`
	}

	var rows []memberRow
	r.db.Table("org_members").
		Select("org_members.user_id, users.email, org_members.role").
		Joins("JOIN users ON users.id = org_members.user_id").
		Where("org_members.org_id = ? AND users.deleted_at IS NULL", orgID).
		Find(&rows)

	result := make([]Member, 0, len(rows))
	for _, row := range rows {
		result = append(result, Member(row))
	}
	return result
}

func (r *Repository) GetPreferences(userID uuid.UUID) []domain.Preference {
	var rows []models.NotificationPreferenceModel
	r.db.Where("user_id = ?", userID).
		Order("notification_type, channel").
		Find(&rows)

	result := make([]domain.Preference, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.Preference{
			UserID:           row.UserID,
			NotificationType: row.NotificationType,
			Channel:          row.Channel,
			Enabled:          row.Enabled,
		})
	}
	return result
}

// IsNotificationEnabled returns true when the user has not set a row (default opt-in)
// or when the stored preference is enabled.
func (r *Repository) IsNotificationEnabled(userID uuid.UUID, notifType, channel string) bool {
	var m models.NotificationPreferenceModel
	err := r.db.Where("user_id = ? AND notification_type = ? AND channel = ?", userID, notifType, channel).
		First(&m).Error
	if err != nil {
		return true
	}
	return m.Enabled
}

func (r *Repository) UpsertPreference(userID uuid.UUID, notifType, channel string, enabled bool) domain.Preference {
	var m models.NotificationPreferenceModel
	result := r.db.Where("user_id = ? AND notification_type = ? AND channel = ?", userID, notifType, channel).First(&m)

	now := time.Now().UTC()
	if result.Error != nil {
		m = models.NotificationPreferenceModel{
			ID:               uuid.New(),
			UserID:           userID,
			NotificationType: notifType,
			Channel:          channel,
			Enabled:          enabled,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		r.db.Create(&m)
	} else {
		r.db.Model(&m).Updates(map[string]any{
			"enabled":    enabled,
			"updated_at": now,
		})
		m.Enabled = enabled
		m.UpdatedAt = now
	}

	return domain.Preference{
		UserID:           m.UserID,
		NotificationType: m.NotificationType,
		Channel:          m.Channel,
		Enabled:          m.Enabled,
	}
}

func (r *Repository) HasLogByDedupKey(key string) bool {
	var count int64
	r.db.Model(&models.NotificationLogModel{}).
		Where("dedup_key = ?", key).
		Count(&count)
	return count > 0
}

func (r *Repository) CreateLog(entry domain.Log) {
	m := models.NotificationLogModel{
		ID:                entry.ID,
		OrgID:             entry.OrgID,
		UserID:            entry.UserID,
		NotificationType:  entry.NotificationType,
		Channel:           entry.Channel,
		Status:            entry.Status,
		ProviderMessageID: entry.ProviderMessageID,
		DedupKey:          entry.DedupKey,
		CreatedAt:         entry.CreatedAt,
	}
	r.db.Create(&m)
}
