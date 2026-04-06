package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/repository/models"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/sessions/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID     uuid.UUID
	ProfileID *uuid.UUID
	Status    string
	From      *time.Time
	To        *time.Time
	Limit     int
	After     *uuid.UUID
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.Session, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})

	q := r.db.WithContext(ctx).Model(&models.SessionModel{}).Where("org_id = ?", p.OrgID)
	if p.ProfileID != nil {
		q = q.Where("profile_id = ?", *p.ProfileID)
	}
	if s := strings.TrimSpace(p.Status); s != "" {
		q = q.Where("status = ?", s)
	}
	if p.From != nil {
		q = q.Where("created_at >= ?", *p.From)
	}
	if p.To != nil {
		q = q.Where("created_at <= ?", *p.To)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}

	if p.After != nil && *p.After != uuid.Nil {
		q = q.Where("id < ?", *p.After)
	}
	q = q.Order("id DESC")

	var rows []models.SessionModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSessionDomain(row))
	}

	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Session) (domain.Session, error) {
	meta, _ := json.Marshal(in.Metadata)
	row := models.SessionModel{
		ID:              uuid.New(),
		OrgID:           in.OrgID,
		BookingID:       in.BookingID,
		ProfileID:       in.ProfileID,
		CustomerPartyID: in.CustomerPartyID,
		ServiceID:       in.ServiceID,
		Status:          in.Status,
		StartedAt:       in.StartedAt,
		EndedAt:         in.EndedAt,
		Summary:         in.Summary,
		Metadata:        meta,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Session{}, err
	}
	return toSessionDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Session, error) {
	var row models.SessionModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, gorm.ErrRecordNotFound
		}
		return domain.Session{}, err
	}
	return toSessionDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in domain.Session) (domain.Session, error) {
	meta, _ := json.Marshal(in.Metadata)
	updates := map[string]any{
		"status":     in.Status,
		"started_at": in.StartedAt,
		"ended_at":   in.EndedAt,
		"summary":    in.Summary,
		"metadata":   meta,
		"updated_at": time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.SessionModel{}).
		Where("org_id = ? AND id = ?", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return domain.Session{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Session{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) BookingSessionExists(ctx context.Context, orgID, bookingID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.SessionModel{}).
		Where("org_id = ? AND booking_id = ?", orgID, bookingID).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) CreateNote(ctx context.Context, in domain.SessionNote) (domain.SessionNote, error) {
	row := models.SessionNoteModel{
		ID:        uuid.New(),
		OrgID:     in.OrgID,
		SessionID: in.SessionID,
		NoteType:  in.NoteType,
		Title:     in.Title,
		Body:      in.Body,
		CreatedBy: in.CreatedBy,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.SessionNote{}, err
	}
	return domain.SessionNote{
		ID:        row.ID,
		OrgID:     row.OrgID,
		SessionID: row.SessionID,
		NoteType:  row.NoteType,
		Title:     row.Title,
		Body:      row.Body,
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
	}, nil
}

func toSessionDomain(row models.SessionModel) domain.Session {
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return domain.Session{
		ID:              row.ID,
		OrgID:           row.OrgID,
		BookingID:       row.BookingID,
		ProfileID:       row.ProfileID,
		CustomerPartyID: row.CustomerPartyID,
		ServiceID:       coalesceServiceReference(row.ServiceID),
		Status:          row.Status,
		StartedAt:       row.StartedAt,
		EndedAt:         row.EndedAt,
		Summary:         row.Summary,
		Metadata:        meta,
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
