package customers

import (
	"context"
	"encoding/json"
	"errors"
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
	Tags        pq.StringArray `gorm:"column:tags"`
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
	err := r.baseQuery(ctx, orgID).Where("p.id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return customerdomain.Customer{}, gorm.ErrRecordNotFound
		}
		return customerdomain.Customer{}, err
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
		return upsertCustomerExtension(ctx, tx, in.ID, partyType, strings.TrimSpace(in.Name), defaultMetadata(in.Metadata))
	}); err != nil {
		return customerdomain.Customer{}, err
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Table("parties").
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Updates(map[string]any{"deleted_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
	if err := r.db.WithContext(ctx).
		Table("sales").
		Select("id, number, status, payment_method, total, currency, created_at").
		Where("org_id = ? AND party_id = ?", orgID, customerID).
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
		Select("p.*").
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
