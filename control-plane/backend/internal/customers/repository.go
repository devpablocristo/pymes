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

	"github.com/devpablocristo/pymes/control-plane/backend/internal/customers/repository/models"
	customerdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/customers/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/utils"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

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

func (r *Repository) List(ctx context.Context, p ListParams) ([]customerdomain.Customer, int64, bool, *uuid.UUID, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := r.db.WithContext(ctx).
		Model(&models.CustomerModel{}).
		Where("org_id = ? AND deleted_at IS NULL", p.OrgID)

	if t := strings.TrimSpace(p.Type); t != "" {
		q = q.Where("type = ?", t)
	}
	if tag := strings.TrimSpace(p.Tag); tag != "" {
		q = q.Where("? = ANY(tags)", tag)
	}
	if s := strings.TrimSpace(p.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(name ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR tax_id ILIKE ?)", like, like, like, like)
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
			q = q.Where("id > ?", *p.After)
		} else {
			q = q.Where("id < ?", *p.After)
		}
	}

	q = q.Order("id " + order)

	var rows []models.CustomerModel
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, false, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	out := make([]customerdomain.Customer, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
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
	meta, _ := json.Marshal(in.Metadata)
	row := models.CustomerModel{
		ID:        uuid.New(),
		OrgID:     in.OrgID,
		Type:      strings.TrimSpace(in.Type),
		Name:      strings.TrimSpace(in.Name),
		TaxID:     strings.TrimSpace(in.TaxID),
		Email:     strings.TrimSpace(in.Email),
		Phone:     strings.TrimSpace(in.Phone),
		Address:   addr,
		Notes:     strings.TrimSpace(in.Notes),
		Tags:      pq.StringArray(utils.NormalizeTags(in.Tags)),
		Metadata:  meta,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if row.Type == "" {
		row.Type = "person"
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return customerdomain.Customer{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (customerdomain.Customer, error) {
	var row models.CustomerModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return customerdomain.Customer{}, gorm.ErrRecordNotFound
		}
		return customerdomain.Customer{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, in customerdomain.Customer) (customerdomain.Customer, error) {
	addr, _ := json.Marshal(in.Address)
	meta, _ := json.Marshal(in.Metadata)
	updates := map[string]any{
		"type":       strings.TrimSpace(in.Type),
		"name":       strings.TrimSpace(in.Name),
		"tax_id":     strings.TrimSpace(in.TaxID),
		"email":      strings.TrimSpace(in.Email),
		"phone":      strings.TrimSpace(in.Phone),
		"address":    addr,
		"notes":      strings.TrimSpace(in.Notes),
		"tags":       pq.StringArray(utils.NormalizeTags(in.Tags)),
		"metadata":   meta,
		"updated_at": time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).Model(&models.CustomerModel{}).
		Where("org_id = ? AND id = ? AND deleted_at IS NULL", in.OrgID, in.ID).
		Updates(updates)
	if res.Error != nil {
		return customerdomain.Customer{}, res.Error
	}
	if res.RowsAffected == 0 {
		return customerdomain.Customer{}, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, in.OrgID, in.ID)
}

func (r *Repository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.CustomerModel{}).
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
		Where("org_id = ? AND customer_id = ?", orgID, customerID).
		Order("created_at DESC").
		Limit(200).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]customerdomain.SaleHistoryItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, customerdomain.SaleHistoryItem{
			ID:            r.ID,
			Number:        r.Number,
			Status:        r.Status,
			PaymentMethod: r.PaymentMethod,
			Total:         r.Total,
			Currency:      r.Currency,
			CreatedAt:     r.CreatedAt,
		})
	}
	return out, nil
}

func toDomain(row models.CustomerModel) customerdomain.Customer {
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
		Type:      row.Type,
		Name:      row.Name,
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

