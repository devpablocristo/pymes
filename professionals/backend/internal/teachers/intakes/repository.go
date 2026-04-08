package intakes

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/intakes/repository/models"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/intakes/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, orgID uuid.UUID) ([]domain.Intake, error) {
	var rows []models.IntakeModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Intake, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Intake) (domain.Intake, error) {
	payload, _ := json.Marshal(in.Payload)
	row := models.IntakeModel{
		ID:              uuid.New(),
		OrgID:           in.OrgID,
		BookingID:   in.BookingID,
		ProfileID:       in.ProfileID,
		CustomerPartyID: in.CustomerPartyID,
		ServiceID:       in.ServiceID,
		Status:          in.Status,
		Payload:         payload,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Intake{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Intake, error) {
	var row models.IntakeModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Intake{}, gorm.ErrRecordNotFound
		}
		return domain.Intake{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Intake) (domain.Intake, error) {
	payload, _ := json.Marshal(in.Payload)
	updates := map[string]any{
		"booking_id":    in.BookingID,
		"customer_party_id": in.CustomerPartyID,
		"service_id":        in.ServiceID,
		"status":            in.Status,
		"payload":           payload,
		"updated_at":        time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.IntakeModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.Intake{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Intake{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func toDomain(row models.IntakeModel) domain.Intake {
	payload := map[string]any{}
	if len(row.Payload) > 0 {
		_ = json.Unmarshal(row.Payload, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return domain.Intake{
		ID:              row.ID,
		OrgID:           row.OrgID,
		BookingID:   row.BookingID,
		ProfileID:       row.ProfileID,
		CustomerPartyID: row.CustomerPartyID,
		ServiceID:       coalesceServiceReference(row.ServiceID),
		Status:          row.Status,
		Payload:         payload,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func coalesceServiceReference(primary *uuid.UUID) *uuid.UUID {
	if primary != nil && *primary != uuid.Nil {
		value := *primary
		return &value
	}
	return nil
}
