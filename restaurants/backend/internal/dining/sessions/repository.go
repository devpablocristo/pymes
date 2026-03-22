package sessions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	tablemodels "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/tables/repository/models"
	"github.com/devpablocristo/pymes/restaurants/backend/internal/dining/sessions/repository/models"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/sessions/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, p ListParams) ([]domain.TableSessionListItem, int64, error) {
	type row struct {
		models.TableSessionModel
		TableCode string `gorm:"column:table_code"`
		AreaName  string `gorm:"column:area_name"`
	}
	base := r.db.WithContext(ctx).Table("restaurant.table_sessions AS s").
		Joins("JOIN restaurant.dining_tables t ON t.id = s.table_id AND t.org_id = s.org_id").
		Joins("JOIN restaurant.dining_areas a ON a.id = t.area_id AND a.org_id = s.org_id").
		Where("s.org_id = ?", p.OrgID)
	if p.OpenOnly {
		base = base.Where("s.closed_at IS NULL")
	}
	if p.TableID != nil && *p.TableID != uuid.Nil {
		base = base.Where("s.table_id = ?", *p.TableID)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []row
	if err := base.Session(&gorm.Session{}).
		Select("s.*, t.code AS table_code, a.name AS area_name").
		Order("s.opened_at DESC").
		Limit(200).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]domain.TableSessionListItem, 0, len(rows))
	for _, rw := range rows {
		out = append(out, domain.TableSessionListItem{
			TableSession: domain.TableSession{
				ID:         rw.ID,
				OrgID:      rw.OrgID,
				TableID:    rw.TableID,
				GuestCount: rw.GuestCount,
				PartyLabel: rw.PartyLabel,
				Notes:      rw.Notes,
				OpenedAt:   rw.OpenedAt,
				ClosedAt:   rw.ClosedAt,
				CreatedAt:  rw.CreatedAt,
				UpdatedAt:  rw.UpdatedAt,
			},
			TableCode: rw.TableCode,
			AreaName:  rw.AreaName,
		})
	}
	return out, total, nil
}

func (r *Repository) OpenSession(ctx context.Context, orgID, tableID uuid.UUID, guestCount int, partyLabel, notes string) (domain.TableSession, error) {
	var out domain.TableSession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t tablemodels.DiningTableModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("org_id = ? AND id = ?", orgID, tableID).
			First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("dining table not found: %w", httperrors.ErrNotFound)
			}
			return err
		}
		if t.Status != "available" && t.Status != "reserved" {
			return fmt.Errorf("table not available for seating: %w", httperrors.ErrConflict)
		}
		var openCount int64
		if err := tx.Model(&models.TableSessionModel{}).
			Where("table_id = ? AND closed_at IS NULL", tableID).
			Count(&openCount).Error; err != nil {
			return err
		}
		if openCount > 0 {
			return fmt.Errorf("table already has an open session: %w", httperrors.ErrConflict)
		}
		now := time.Now().UTC()
		row := models.TableSessionModel{
			ID:         uuid.New(),
			OrgID:      orgID,
			TableID:    tableID,
			GuestCount: guestCount,
			PartyLabel: partyLabel,
			Notes:      notes,
			OpenedAt:   now,
			ClosedAt:   nil,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Model(&tablemodels.DiningTableModel{}).
			Where("org_id = ? AND id = ?", orgID, tableID).
			Updates(map[string]any{"status": "occupied", "updated_at": now}).Error; err != nil {
			return err
		}
		out = toDomain(row)
		return nil
	})
	return out, err
}

func (r *Repository) CloseSession(ctx context.Context, orgID, sessionID uuid.UUID) (domain.TableSession, error) {
	var out domain.TableSession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var s models.TableSessionModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("org_id = ? AND id = ?", orgID, sessionID).
			First(&s).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
			}
			return err
		}
		if s.ClosedAt != nil {
			return fmt.Errorf("session already closed: %w", httperrors.ErrConflict)
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.TableSessionModel{}).
			Where("org_id = ? AND id = ?", orgID, sessionID).
			Updates(map[string]any{"closed_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&tablemodels.DiningTableModel{}).
			Where("org_id = ? AND id = ?", orgID, s.TableID).
			Updates(map[string]any{"status": "available", "updated_at": now}).Error; err != nil {
			return err
		}
		s.ClosedAt = &now
		s.UpdatedAt = now
		out = toDomain(s)
		return nil
	})
	return out, err
}

func toDomain(row models.TableSessionModel) domain.TableSession {
	return domain.TableSession{
		ID:         row.ID,
		OrgID:      row.OrgID,
		TableID:    row.TableID,
		GuestCount: row.GuestCount,
		PartyLabel: row.PartyLabel,
		Notes:      row.Notes,
		OpenedAt:   row.OpenedAt,
		ClosedAt:   row.ClosedAt,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
