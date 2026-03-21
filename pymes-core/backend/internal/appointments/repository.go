package appointments

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/backend/go/pagination"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/appointments/repository/models"
	appointmentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/appointments/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, from, to *time.Time, status, assigned string, limit int) ([]appointmentsdomain.Appointment, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 200})
	q := r.db.WithContext(ctx).Model(&models.AppointmentModel{}).Where("org_id = ?", orgID)
	if from != nil {
		q = q.Where("start_at >= ?", from.UTC())
	}
	if to != nil {
		q = q.Where("start_at <= ?", to.UTC())
	}
	if status = strings.TrimSpace(status); status != "" {
		q = q.Where("status = ?", status)
	}
	if assigned = strings.TrimSpace(assigned); assigned != "" {
		q = q.Where("assigned_to = ?", assigned)
	}
	var rows []models.AppointmentModel
	if err := q.Order("start_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]appointmentsdomain.Appointment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in appointmentsdomain.Appointment) (appointmentsdomain.Appointment, error) {
	row := models.AppointmentModel{
		ID:            in.ID,
		OrgID:         in.OrgID,
		CustomerID:    in.CustomerID,
		CustomerName:  strings.TrimSpace(in.CustomerName),
		CustomerPhone: strings.TrimSpace(in.CustomerPhone),
		Title:         strings.TrimSpace(in.Title),
		Description:   strings.TrimSpace(in.Description),
		Status:        normalizeStatus(in.Status),
		StartAt:       in.StartAt.UTC(),
		EndAt:         in.EndAt.UTC(),
		Duration:      in.Duration,
		Location:      strings.TrimSpace(in.Location),
		AssignedTo:    strings.TrimSpace(in.AssignedTo),
		Color:         colorOrDefault(in.Color),
		Notes:         strings.TrimSpace(in.Notes),
		Metadata:      mustJSON(in.Metadata),
		CreatedBy:     strings.TrimSpace(in.CreatedBy),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return appointmentsdomain.Appointment{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (appointmentsdomain.Appointment, error) {
	var row models.AppointmentModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error; err != nil {
		return appointmentsdomain.Appointment{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in appointmentsdomain.Appointment) (appointmentsdomain.Appointment, error) {
	row := models.AppointmentModel{
		ID:            in.ID,
		OrgID:         in.OrgID,
		CustomerID:    in.CustomerID,
		CustomerName:  strings.TrimSpace(in.CustomerName),
		CustomerPhone: strings.TrimSpace(in.CustomerPhone),
		Title:         strings.TrimSpace(in.Title),
		Description:   strings.TrimSpace(in.Description),
		Status:        normalizeStatus(in.Status),
		StartAt:       in.StartAt.UTC(),
		EndAt:         in.EndAt.UTC(),
		Duration:      in.Duration,
		Location:      strings.TrimSpace(in.Location),
		AssignedTo:    strings.TrimSpace(in.AssignedTo),
		Color:         colorOrDefault(in.Color),
		Notes:         strings.TrimSpace(in.Notes),
		Metadata:      mustJSON(in.Metadata),
		CreatedBy:     strings.TrimSpace(in.CreatedBy),
		CreatedAt:     in.CreatedAt.UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		return appointmentsdomain.Appointment{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Cancel(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.AppointmentModel{}).
		Where("org_id = ? AND id = ?", orgID, id).
		Updates(map[string]any{"status": "cancelled", "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func toDomain(row models.AppointmentModel) appointmentsdomain.Appointment {
	var metadata map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}
	return appointmentsdomain.Appointment{
		ID:            row.ID,
		OrgID:         row.OrgID,
		CustomerID:    row.CustomerID,
		CustomerName:  row.CustomerName,
		CustomerPhone: row.CustomerPhone,
		Title:         row.Title,
		Description:   row.Description,
		Status:        row.Status,
		StartAt:       row.StartAt,
		EndAt:         row.EndAt,
		Duration:      row.Duration,
		Location:      row.Location,
		AssignedTo:    row.AssignedTo,
		Color:         colorOrDefault(row.Color),
		Notes:         row.Notes,
		Metadata:      metadata,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mustJSON(v any) []byte {
	if v == nil {
		return []byte(`{}`)
	}
	b, _ := json.Marshal(v)
	return b
}

func colorOrDefault(v string) string {
	if strings.TrimSpace(v) == "" {
		return "#3B82F6"
	}
	return strings.TrimSpace(v)
}
