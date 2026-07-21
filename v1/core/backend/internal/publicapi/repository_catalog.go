package publicapi

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/devpablocristo/platform/http/go/pagination"
	"github.com/google/uuid"

	schedulingdomain "github.com/devpablocristo/platform/features/scheduling/go/domain"
)

// ListPublicServices mantiene el shape compacto usado por el adapter externo schedulingpublichttp.
func (r *Repository) ListPublicServices(ctx context.Context, orgID uuid.UUID, limit int) ([]PublicService, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	if items, ok, err := r.listSchedulingPublicServices(ctx, orgID, limit); err != nil {
		return nil, err
	} else if ok {
		return items, nil
	}

	var rows []PublicService
	err := r.db.WithContext(ctx).
		Table("services").
		Select("id, name, 'service' as type, description, '' as unit, sale_price as price, currency").
		Where("org_id = ? AND archived_at IS NULL AND is_active = true", orgID).
		Order("name ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) listSchedulingPublicServices(ctx context.Context, orgID uuid.UUID, limit int) ([]PublicService, bool, error) {
	if r.scheduling == nil {
		return nil, false, nil
	}
	services, err := r.scheduling.ListServices(ctx, orgID)
	if err != nil {
		return nil, false, err
	}
	active := make([]PublicService, 0, len(services))
	for _, service := range services {
		if !service.Active {
			continue
		}
		// Catch-all services (used by the SMB owner from the internal calendar
		// to anote ad-hoc bookings) must never appear in the public catalog —
		// clients booking through PublicSchedulingFlow should only see real
		// catalog services with meaningful names and durations.
		if isCatchAllService(service.Metadata) {
			continue
		}
		unit := "booking"
		if service.FulfillmentMode == schedulingdomain.FulfillmentModeQueue {
			unit = "ticket"
		}
		active = append(active, PublicService{
			ID:          service.ID,
			Name:        service.Name,
			Type:        string(service.FulfillmentMode),
			Description: service.Description,
			Unit:        unit,
			Price:       0,
			Currency:    "",
		})
	}
	if len(active) == 0 {
		return nil, false, nil
	}
	sort.Slice(active, func(i, j int) bool { return strings.ToLower(active[i].Name) < strings.ToLower(active[j].Name) })
	if len(active) > limit {
		active = active[:limit]
	}
	return active, true, nil
}

// PublicServiceCatalogItem es el shape rico expuesto a verticales y al storefront
// para listar el catálogo de servicios desde public.services.
type PublicServiceCatalogItem struct {
	ID                     uuid.UUID      `json:"id"`
	Code                   string         `json:"code"`
	Name                   string         `json:"name"`
	Description            string         `json:"description"`
	CategoryCode           string         `json:"category_code"`
	SalePrice              float64        `json:"sale_price"`
	Currency               string         `json:"currency"`
	TaxRate                *float64       `json:"tax_rate,omitempty"`
	DefaultDurationMinutes *int           `json:"default_duration_minutes,omitempty"`
	Metadata               map[string]any `json:"metadata"`
}

type publicServiceCatalogRow struct {
	ID                     uuid.UUID
	Code                   string
	Name                   string
	Description            string
	CategoryCode           string
	SalePrice              float64
	Currency               string
	TaxRate                *float64
	DefaultDurationMinutes *int `gorm:"column:default_duration_minutes"`
	Metadata               []byte
}

// ListPublicServiceCatalog lee el catálogo rico desde public.services con filtros
// opcionales por metadata.vertical / metadata.segment y un search por nombre/código.
func (r *Repository) ListPublicServiceCatalog(ctx context.Context, orgID uuid.UUID, vertical, segment, search string, limit int) ([]PublicServiceCatalogItem, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})

	q := r.db.WithContext(ctx).
		Table("services").
		Select(`id, code, name, description, category_code, sale_price, currency,
			tax_rate, default_duration_minutes, metadata`).
		Where("org_id = ? AND archived_at IS NULL AND is_active = true", orgID)

	if v := strings.TrimSpace(vertical); v != "" {
		q = q.Where("metadata->>'vertical' = ?", v)
	}
	if s := strings.TrimSpace(segment); s != "" {
		q = q.Where("metadata->>'segment' = ?", s)
	}
	if s := strings.TrimSpace(search); s != "" {
		like := "%" + s + "%"
		q = q.Where("(name ILIKE ? OR description ILIKE ? OR code ILIKE ?)", like, like, like)
	}

	var rows []publicServiceCatalogRow
	if err := q.Order("name ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]PublicServiceCatalogItem, 0, len(rows))
	for _, row := range rows {
		metadata := map[string]any{}
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &metadata)
		}
		out = append(out, PublicServiceCatalogItem{
			ID:                     row.ID,
			Code:                   row.Code,
			Name:                   row.Name,
			Description:            row.Description,
			CategoryCode:           row.CategoryCode,
			SalePrice:              row.SalePrice,
			Currency:               row.Currency,
			TaxRate:                row.TaxRate,
			DefaultDurationMinutes: row.DefaultDurationMinutes,
			Metadata:               metadata,
		})
	}
	return out, nil
}
