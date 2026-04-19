// Package suppliers implements persistence for supplier operations.
package suppliers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	utils "github.com/devpablocristo/core/validate/go/stringutil"
	supplierdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/suppliers/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
	Tag    string
	Sort   string
	Order  string
}

type supplierPartyRow struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	DisplayName string `gorm:"column:display_name"`
	Email       string
	Phone       string
	Address     []byte `gorm:"column:address"`
	TaxID       string `gorm:"column:tax_id"`
	Notes       string
	Tags        pq.StringArray `gorm:"type:text[];column:tags"`
	Metadata    []byte         `gorm:"column:metadata"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at"`
	DeletedAt   *time.Time     `gorm:"column:deleted_at"`
	ContactName string         `gorm:"column:contact_name"`
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]supplierdomain.Supplier, int64, bool, *uuid.UUID, error) {
	limit := pagination.NormalizeLimit(p.Limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})

	q := r.baseQuery(ctx, p.OrgID)
	if tag := strings.TrimSpace(p.Tag); tag != "" {
		q = q.Where("? = ANY(p.tags)", tag)
	}
	if s := strings.TrimSpace(p.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(p.display_name ILIKE ? OR p.email ILIKE ? OR p.phone ILIKE ? OR p.tax_id ILIKE ? OR COALESCE(pr.metadata->>'contact_name', '') ILIKE ?)", like, like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, false, nil, err
	}
	order := "desc"
	if strings.EqualFold(strings.TrimSpace(p.Order), "asc") {
		order = "asc"
	}
	if p.After != nil && *p.After != uuid.Nil {
		if order == "asc" {
			q = q.Where("p.id > ?", *p.After)
		} else {
			q = q.Where("p.id < ?", *p.After)
		}
	}
	var rows []supplierPartyRow
	if err := q.Order("p.id " + order).Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]supplierdomain.Supplier, 0, len(rows))
	for _, row := range rows {
		out = append(out, supplierFromPartyRow(row))
	}
	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) Create(ctx context.Context, in supplierdomain.Supplier) (supplierdomain.Supplier, error) {
	addr, _ := json.Marshal(in.Address)
	metadata := defaultMetadata(in.Metadata)
	if strings.TrimSpace(in.ContactName) != "" {
		metadata["contact_name"] = strings.TrimSpace(in.ContactName)
	}
	meta, _ := json.Marshal(metadata)
	partyID := uuid.New()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("parties").Create(map[string]any{
			"id":           partyID,
			"org_id":       in.OrgID,
			"party_type":   "organization",
			"display_name": strings.TrimSpace(in.Name),
			"email":        strings.TrimSpace(in.Email),
			"phone":        strings.TrimSpace(in.Phone),
			"address":      addr,
			"tax_id":       strings.TrimSpace(in.TaxID),
			"notes":        strings.TrimSpace(in.Notes),
			"tags":         pq.StringArray(utils.NormalizeTags(in.Tags)),
			"metadata":     meta,
			"created_at":   time.Now().UTC(),
			"updated_at":   time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (party_id) DO UPDATE SET legal_name = EXCLUDED.legal_name, trade_name = EXCLUDED.trade_name, tax_condition = EXCLUDED.tax_condition
		`, partyID, strings.TrimSpace(in.Name), strings.TrimSpace(in.Name), stringValue(metadata, "tax_condition")).Error; err != nil {
			return err
		}
		roleMetadata, _ := json.Marshal(map[string]any{"contact_name": strings.TrimSpace(in.ContactName)})
		if err := tx.Exec(`
			INSERT INTO party_roles (id, party_id, org_id, role, is_active, metadata, created_at)
			VALUES (?, ?, ?, 'supplier', true, ?::jsonb, now())
			ON CONFLICT (party_id, org_id, role) DO UPDATE SET is_active = true, metadata = EXCLUDED.metadata
		`, uuid.New(), partyID, in.OrgID, string(roleMetadata)).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return supplierdomain.Supplier{}, err
	}
	return r.GetByID(ctx, in.OrgID, partyID)
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (supplierdomain.Supplier, error) {
	var row supplierPartyRow
	err := r.baseQuery(ctx, orgID).Where("p.id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return supplierdomain.Supplier{}, gorm.ErrRecordNotFound
		}
		return supplierdomain.Supplier{}, err
	}
	return supplierFromPartyRow(row), nil
}

func (r *Repository) Update(ctx context.Context, in supplierdomain.Supplier) (supplierdomain.Supplier, error) {
	addr, _ := json.Marshal(in.Address)
	metadata := defaultMetadata(in.Metadata)
	if strings.TrimSpace(in.ContactName) != "" {
		metadata["contact_name"] = strings.TrimSpace(in.ContactName)
	}
	meta, _ := json.Marshal(metadata)
	roleMetadata, _ := json.Marshal(map[string]any{"contact_name": strings.TrimSpace(in.ContactName)})
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("parties").
			Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
			Updates(map[string]any{
				"party_type":   "organization",
				"display_name": strings.TrimSpace(in.Name),
				"email":        strings.TrimSpace(in.Email),
				"phone":        strings.TrimSpace(in.Phone),
				"address":      addr,
				"tax_id":       strings.TrimSpace(in.TaxID),
				"notes":        strings.TrimSpace(in.Notes),
				"tags":         pq.StringArray(utils.NormalizeTags(in.Tags)),
				"metadata":     meta,
				"updated_at":   time.Now().UTC(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Exec(`
			INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (party_id) DO UPDATE SET legal_name = EXCLUDED.legal_name, trade_name = EXCLUDED.trade_name, tax_condition = EXCLUDED.tax_condition
		`, in.ID, strings.TrimSpace(in.Name), strings.TrimSpace(in.Name), stringValue(metadata, "tax_condition")).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			UPDATE party_roles SET metadata = ?::jsonb, is_active = true WHERE org_id = ? AND party_id = ? AND role = 'supplier'
		`, string(roleMetadata), in.OrgID, in.ID).Error; err != nil {
			return err
		}
		in.Metadata = metadata
		return nil
	}); err != nil {
		return supplierdomain.Supplier{}, err
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("parties").
			Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
			Updates(map[string]any{"deleted_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()")})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (r *Repository) ListArchived(ctx context.Context, orgID uuid.UUID) ([]supplierdomain.Supplier, error) {
	var rows []supplierPartyRow
	err := r.db.WithContext(ctx).
		Table("parties p").
		Select(`
			p.id,
			p.org_id,
			p.display_name,
			p.email,
			p.phone,
			p.address,
			p.tax_id,
			p.notes,
			p.tags,
			p.metadata,
			p.created_at,
			p.updated_at,
			p.deleted_at,
			COALESCE(pr.metadata->>'contact_name', p.metadata->>'contact_name', '') AS contact_name
		`).
		Joins("JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id AND pr.role = 'supplier'").
		Where("p.org_id = ? AND p.deleted_at IS NOT NULL", orgID).
		Order("p.updated_at DESC").
		Limit(200).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]supplierdomain.Supplier, 0, len(rows))
	for _, row := range rows {
		out = append(out, supplierFromPartyRow(row))
	}
	return out, nil
}

func (r *Repository) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("parties").
			Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
			Updates(map[string]any{"deleted_at": nil, "updated_at": gorm.Expr("now()")})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *Repository) HardDelete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Verificar que está archivado antes de eliminar permanentemente.
		var count int64
		if err := tx.Table("parties").
			Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}

		// Soltar referencias anulables que guardan snapshot del nombre proveedor.
		for _, table := range []string{
			"purchases",
			"quotes",
			"sales",
			"scheduling_bookings",
			"scheduling_queue_tickets",
			"scheduling_waitlist_entries",
			"whatsapp_messages",
			"payment_preferences",
			"recurring_expenses",
		} {
			if err := tx.Exec("UPDATE "+table+" SET party_id = NULL WHERE party_id = ? AND org_id = ?", id, orgID).Error; err != nil {
				return err
			}
		}

		// La cuenta corriente del proveedor es una dependencia dura del party:
		// en hard delete también se elimina, junto con sus movimientos por cascade.
		if err := tx.Exec("DELETE FROM accounts WHERE party_id = ? AND org_id = ? AND type = 'payable'", id, orgID).Error; err != nil {
			return err
		}

		// Eliminar roles y extensiones del party.
		if err := tx.Exec("DELETE FROM party_roles WHERE party_id = ? AND org_id = ?", id, orgID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM party_organizations WHERE party_id = ?", id).Error; err != nil {
			return err
		}

		// Eliminar el party.
		return tx.Table("parties").
			Where("org_id = ? AND id = ?", orgID, id).
			Delete(nil).Error
	})
}

func (r *Repository) baseQuery(ctx context.Context, orgID uuid.UUID) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("parties p").
		Select(`
			p.id,
			p.org_id,
			p.display_name,
			p.email,
			p.phone,
			p.address,
			p.tax_id,
			p.notes,
			p.tags,
			p.metadata,
			p.created_at,
			p.updated_at,
			p.deleted_at,
			COALESCE(pr.metadata->>'contact_name', p.metadata->>'contact_name', '') AS contact_name
		`).
		Joins("JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id AND pr.role = 'supplier' AND pr.is_active = true").
		Where("p.org_id = ? AND p.deleted_at IS NULL", orgID)
}

func supplierFromPartyRow(row supplierPartyRow) supplierdomain.Supplier {
	addr := supplierdomain.Address{}
	_ = json.Unmarshal(row.Address, &addr)
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return supplierdomain.Supplier{
		ID:          row.ID,
		OrgID:       row.OrgID,
		Name:        row.DisplayName,
		TaxID:       row.TaxID,
		Email:       row.Email,
		Phone:       row.Phone,
		Address:     addr,
		ContactName: row.ContactName,
		Notes:       row.Notes,
		Tags:        append([]string(nil), row.Tags...),
		Metadata:    meta,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	}
}

func defaultMetadata(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
