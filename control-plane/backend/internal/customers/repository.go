// Package customers implements persistence for customer operations.
package customers

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	customerdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/customers/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/utils"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
	Type   string
	Tag    string
	Sort   string
	Order  string
}

type customerPartyRow struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	PartyType   string `gorm:"column:party_type"`
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
}

func (r *Repository) List(ctx context.Context, p ListParams) ([]customerdomain.Customer, int64, bool, *uuid.UUID, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := r.baseQuery(ctx, p.OrgID)
	if t := strings.TrimSpace(p.Type); t != "" {
		q = q.Where("p.party_type = ?", mapCustomerType(t))
	}
	if tag := strings.TrimSpace(p.Tag); tag != "" {
		q = q.Where("? = ANY(p.tags)", tag)
	}
	if s := strings.TrimSpace(p.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(p.display_name ILIKE ? OR p.email ILIKE ? OR p.phone ILIKE ? OR p.tax_id ILIKE ?)", like, like, like, like)
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

	var rows []customerPartyRow
	if err := q.Order("p.id " + order).Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]customerdomain.Customer, 0, len(rows))
	for _, row := range rows {
		out = append(out, customerFromPartyRow(row))
	}

	var next *uuid.UUID
	if hasMore && len(rows) > 0 {
		v := rows[len(rows)-1].ID
		next = &v
	}
	return out, total, hasMore, next, nil
}

func (r *Repository) Create(ctx context.Context, in customerdomain.Customer) (customerdomain.Customer, error) {
	addr, _ := json.Marshal(in.Address)
	meta, _ := json.Marshal(defaultMetadata(in.Metadata))
	partyType := mapCustomerType(in.Type)
	partyID := uuid.New()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("parties").Create(map[string]any{
			"id":           partyID,
			"org_id":       in.OrgID,
			"party_type":   partyType,
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
		if err := upsertCustomerExtension(ctx, tx, partyID, partyType, strings.TrimSpace(in.Name), defaultMetadata(in.Metadata)); err != nil {
			return err
		}
		if err := upsertLegacyCustomer(ctx, tx, customerdomain.Customer{
			ID:       partyID,
			OrgID:    in.OrgID,
			Type:     in.Type,
			Name:     in.Name,
			TaxID:    in.TaxID,
			Email:    in.Email,
			Phone:    in.Phone,
			Address:  in.Address,
			Notes:    in.Notes,
			Tags:     in.Tags,
			Metadata: in.Metadata,
		}); err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO party_roles (id, party_id, org_id, role, is_active, metadata, created_at)
			VALUES (?, ?, ?, 'customer', true, '{}'::jsonb, now())
			ON CONFLICT (party_id, org_id, role) DO UPDATE SET is_active = true
		`, uuid.New(), partyID, in.OrgID).Error
	}); err != nil {
		return customerdomain.Customer{}, err
	}
	return r.GetByID(ctx, in.OrgID, partyID)
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (customerdomain.Customer, error) {
	var row customerPartyRow
	err := r.baseQuery(ctx, orgID).Where("p.id = ?", id).Limit(1).Scan(&row).Error
	if err != nil {
		return customerdomain.Customer{}, err
	}
	if row.ID == uuid.Nil {
		return customerdomain.Customer{}, gorm.ErrRecordNotFound
	}
	return customerFromPartyRow(row), nil
}

func (r *Repository) Update(ctx context.Context, in customerdomain.Customer) (customerdomain.Customer, error) {
	addr, _ := json.Marshal(in.Address)
	meta, _ := json.Marshal(defaultMetadata(in.Metadata))
	partyType := mapCustomerType(in.Type)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("parties").
			Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
			Updates(map[string]any{
				"party_type":   partyType,
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
		if err := upsertCustomerExtension(ctx, tx, in.ID, partyType, strings.TrimSpace(in.Name), defaultMetadata(in.Metadata)); err != nil {
			return err
		}
		return upsertLegacyCustomer(ctx, tx, in)
	}); err != nil {
		return customerdomain.Customer{}, err
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
		return softDeleteLegacyCustomer(ctx, tx, orgID, id)
	}); err != nil {
		return err
	}
	return nil
}

func (r *Repository) ListArchived(ctx context.Context, orgID uuid.UUID) ([]customerdomain.Customer, error) {
	var rows []customerPartyRow
	err := r.db.WithContext(ctx).
		Table("parties p").
		Select(`p.id, p.org_id, p.party_type, p.display_name, p.email, p.phone, p.address, p.tax_id, p.notes, p.tags, p.metadata, p.created_at, p.updated_at, p.deleted_at`).
		Joins("JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id AND pr.role = 'customer'").
		Where("p.org_id = ? AND p.deleted_at IS NOT NULL", orgID).
		Order("p.updated_at DESC").
		Limit(200).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]customerdomain.Customer, 0, len(rows))
	for _, row := range rows {
		out = append(out, customerFromPartyRow(row))
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
		exists, err := legacyCustomerTableExists(ctx, tx)
		if err != nil || !exists {
			return err
		}
		return tx.Table("customers").
			Where("org_id = ? AND id = ?", orgID, id).
			Updates(map[string]any{"deleted_at": nil, "updated_at": gorm.Expr("now()")}).Error
	})
}

func (r *Repository) HardDelete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Verify it's archived before hard-deleting.
		var count int64
		if err := tx.Table("parties").
			Where("org_id = ? AND id = ? AND deleted_at IS NOT NULL", orgID, id).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}

		// Nullify FK references in dependent tables.
		for _, table := range []string{"quotes", "sales", "returns", "credit_notes", "appointments"} {
			if has, _ := r.tableHasColumn(ctx, table, "customer_id"); has {
				tx.Exec("UPDATE "+table+" SET customer_id = NULL WHERE customer_id = ? AND org_id = ?", id, orgID)
			}
			if has, _ := r.tableHasColumn(ctx, table, "party_id"); has {
				tx.Exec("UPDATE "+table+" SET party_id = NULL WHERE party_id = ? AND org_id = ?", id, orgID)
			}
		}

		// Nullify account references.
		if has, _ := r.tableHasColumn(ctx, "accounts", "party_id"); has {
			tx.Exec("UPDATE accounts SET party_id = NULL WHERE party_id = ? AND org_id = ?", id, orgID)
		}

		// Delete legacy customer row first (has FK from other tables too).
		exists, err := legacyCustomerTableExists(ctx, tx)
		if err != nil {
			return err
		}
		if exists {
			tx.Table("customers").Where("org_id = ? AND id = ?", orgID, id).Delete(nil)
		}

		// Delete party extensions and roles.
		tx.Exec("DELETE FROM party_roles WHERE party_id = ? AND org_id = ?", id, orgID)
		tx.Exec("DELETE FROM party_persons WHERE party_id = ?", id)
		tx.Exec("DELETE FROM party_organizations WHERE party_id = ?", id)

		// Delete the party itself.
		return tx.Table("parties").
			Where("org_id = ? AND id = ?", orgID, id).
			Delete(nil).Error
	})
}

func (r *Repository) ListSales(ctx context.Context, orgID, customerID uuid.UUID) ([]customerdomain.SaleHistoryItem, error) {
	type row struct {
		ID            uuid.UUID
		Number        string
		Status        string
		PaymentMethod string `gorm:"column:payment_method"`
		Total         float64
		Currency      string
		CreatedAt     time.Time `gorm:"column:created_at"`
	}
	var rows []row
	customerIDColumn, err := r.salesCustomerIDColumn(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).
		Table("sales").
		Select("id, number, status, payment_method, total, currency, created_at").
		Where("org_id = ? AND "+customerIDColumn+" = ?", orgID, customerID).
		Order("created_at DESC").
		Limit(200).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]customerdomain.SaleHistoryItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, customerdomain.SaleHistoryItem{ID: row.ID, Number: row.Number, Status: row.Status, PaymentMethod: row.PaymentMethod, Total: row.Total, Currency: row.Currency, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (r *Repository) baseQuery(ctx context.Context, orgID uuid.UUID) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("parties p").
		Select(`
			p.id,
			p.org_id,
			p.party_type,
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
			p.deleted_at
		`).
		Joins("JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id AND pr.role = 'customer' AND pr.is_active = true").
		Where("p.org_id = ? AND p.deleted_at IS NULL", orgID)
}

func upsertCustomerExtension(ctx context.Context, tx *gorm.DB, partyID uuid.UUID, partyType, name string, metadata map[string]any) error {
	if partyType == "person" {
		first, last := splitName(name)
		if err := tx.Exec("DELETE FROM party_organizations WHERE party_id = ?", partyID).Error; err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO party_persons (party_id, first_name, last_name)
			VALUES (?, ?, ?)
			ON CONFLICT (party_id) DO UPDATE SET first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name
		`, partyID, first, last).Error
	}
	if err := tx.Exec("DELETE FROM party_persons WHERE party_id = ?", partyID).Error; err != nil {
		return err
	}
	return tx.Exec(`
		INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (party_id) DO UPDATE SET legal_name = EXCLUDED.legal_name, trade_name = EXCLUDED.trade_name, tax_condition = EXCLUDED.tax_condition
	`, partyID, name, name, stringValue(metadata, "tax_condition")).Error
}

func customerFromPartyRow(row customerPartyRow) customerdomain.Customer {
	addr := customerdomain.Address{}
	_ = json.Unmarshal(row.Address, &addr)
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return customerdomain.Customer{
		ID:        row.ID,
		OrgID:     row.OrgID,
		Type:      unmapCustomerType(row.PartyType),
		Name:      row.DisplayName,
		TaxID:     row.TaxID,
		Email:     row.Email,
		Phone:     row.Phone,
		Address:   addr,
		Notes:     row.Notes,
		Tags:      append([]string(nil), row.Tags...),
		Metadata:  meta,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		DeletedAt: row.DeletedAt,
	}
}

func mapCustomerType(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "company") {
		return "organization"
	}
	return "person"
}

func unmapCustomerType(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "organization") {
		return "company"
	}
	return "person"
}

func defaultMetadata(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

func splitName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
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

func upsertLegacyCustomer(ctx context.Context, tx *gorm.DB, in customerdomain.Customer) error {
	exists, err := legacyCustomerTableExists(ctx, tx)
	if err != nil || !exists {
		return err
	}
	addr, _ := json.Marshal(in.Address)
	meta, _ := json.Marshal(defaultMetadata(in.Metadata))
	return tx.Exec(`
		INSERT INTO customers (
			id, org_id, type, name, tax_id, email, phone, address, notes, tags, metadata, created_at, updated_at, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?, ?::jsonb, now(), now(), NULL)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			name = EXCLUDED.name,
			tax_id = EXCLUDED.tax_id,
			email = EXCLUDED.email,
			phone = EXCLUDED.phone,
			address = EXCLUDED.address,
			notes = EXCLUDED.notes,
			tags = EXCLUDED.tags,
			metadata = EXCLUDED.metadata,
			updated_at = now(),
			deleted_at = NULL
	`, in.ID, in.OrgID, strings.TrimSpace(in.Type), strings.TrimSpace(in.Name), strings.TrimSpace(in.TaxID), strings.TrimSpace(in.Email), strings.TrimSpace(in.Phone), string(addr), strings.TrimSpace(in.Notes), pq.StringArray(utils.NormalizeTags(in.Tags)), string(meta)).Error
}

func softDeleteLegacyCustomer(ctx context.Context, tx *gorm.DB, orgID, id uuid.UUID) error {
	exists, err := legacyCustomerTableExists(ctx, tx)
	if err != nil || !exists {
		return err
	}
	return tx.Table("customers").
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Updates(map[string]any{"deleted_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()")}).Error
}

func legacyCustomerTableExists(ctx context.Context, tx *gorm.DB) (bool, error) {
	var exists bool
	err := tx.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = current_schema()
			  AND c.relname = 'customers'
			  AND c.relkind = 'r'
		)
	`).Scan(&exists).Error
	return exists, err
}

func (r *Repository) salesCustomerIDColumn(ctx context.Context) (string, error) {
	hasPartyID, err := r.tableHasColumn(ctx, "sales", "party_id")
	if err != nil {
		return "", err
	}
	if hasPartyID {
		return "party_id", nil
	}
	return "customer_id", nil
}

func (r *Repository) tableHasColumn(ctx context.Context, tableName, columnName string) (bool, error) {
	var exists bool
	err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = ?
			  AND column_name = ?
		)
	`, tableName, columnName).Scan(&exists).Error
	return exists, err
}
