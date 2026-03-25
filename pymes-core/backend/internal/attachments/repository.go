package attachments

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/utils/go/pagination"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/attachments/repository/models"
	attachmentdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/attachments/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, in attachmentdomain.Attachment) (attachmentdomain.Attachment, error) {
	row := models.AttachmentModel{ID: in.ID, OrgID: in.OrgID, AttachableType: in.AttachableType, AttachableID: in.AttachableID, FileName: in.FileName, ContentType: in.ContentType, SizeBytes: in.SizeBytes, StorageKey: in.StorageKey, UploadedBy: in.UploadedBy, CreatedAt: in.CreatedAt}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return attachmentdomain.Attachment{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (attachmentdomain.Attachment, error) {
	var row models.AttachmentModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return attachmentdomain.Attachment{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Delete(&models.AttachmentModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ListByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]attachmentdomain.Attachment, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	var rows []models.AttachmentModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND attachable_type = ? AND attachable_id = ?", orgID, entityType, entityID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]attachmentdomain.Attachment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func toDomain(row models.AttachmentModel) attachmentdomain.Attachment {
	return attachmentdomain.Attachment{ID: row.ID, OrgID: row.OrgID, AttachableType: row.AttachableType, AttachableID: row.AttachableID, FileName: row.FileName, ContentType: row.ContentType, SizeBytes: row.SizeBytes, StorageKey: row.StorageKey, UploadedBy: row.UploadedBy, CreatedAt: row.CreatedAt}
}
