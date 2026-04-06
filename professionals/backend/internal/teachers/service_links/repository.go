package service_links

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/service_links/repository/models"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/service_links/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListByProfile(ctx context.Context, orgID, profileID uuid.UUID) ([]domain.ServiceLink, error) {
	var rows []models.ServiceLinkModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND profile_id = ?", orgID, profileID).
		Order("display_order ASC, created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.ServiceLink, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) ReplaceForProfile(ctx context.Context, orgID, profileID uuid.UUID, links []domain.ServiceLink) ([]domain.ServiceLink, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("org_id = ? AND profile_id = ?", orgID, profileID).
			Delete(&models.ServiceLinkModel{}).Error; err != nil {
			return err
		}
		for _, link := range links {
			meta, _ := json.Marshal(link.Metadata)
			row := models.ServiceLinkModel{
				ID:                uuid.New(),
				OrgID:             orgID,
				ProfileID:         profileID,
				ServiceID:         link.ServiceID,
				PublicDescription: link.PublicDescription,
				DisplayOrder:      link.DisplayOrder,
				IsFeatured:        link.IsFeatured,
				Metadata:          meta,
				CreatedAt:         time.Now().UTC(),
				UpdatedAt:         time.Now().UTC(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.ListByProfile(ctx, orgID, profileID)
}

func (r *Repository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.ServiceLink, error) {
	var rows []models.ServiceLinkModel
	err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("display_order ASC, created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.ServiceLink, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func toDomain(row models.ServiceLinkModel) domain.ServiceLink {
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return domain.ServiceLink{
		ID:                row.ID,
		OrgID:             row.OrgID,
		ProfileID:         row.ProfileID,
		ServiceID:         row.ServiceID,
		PublicDescription: row.PublicDescription,
		DisplayOrder:      row.DisplayOrder,
		IsFeatured:        row.IsFeatured,
		Metadata:          meta,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}
