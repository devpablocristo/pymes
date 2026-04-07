package pymescore

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescoreops"
)

func (c *Client) GetService(ctx context.Context, orgID, serviceID string) (map[string]any, error) {
	return pymescoreops.GetService(ctx, c.Client, orgID, serviceID)
}

// CoreService es el shape mínimo del catálogo público de servicios servido por
// pymes-core (`/v1/public/:org_id/catalog/services`).
type CoreService struct {
	ID                     string         `json:"id"`
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

// ListPublicServices llama al endpoint público del catálogo de servicios en pymes-core
// con filtros opcionales por vertical y segment (que viven en services.metadata).
func (c *Client) ListPublicServices(ctx context.Context, orgRef, vertical, segment, search string) ([]CoreService, error) {
	q := url.Values{}
	if vertical != "" {
		q.Set("vertical", vertical)
	}
	if segment != "" {
		q.Set("segment", segment)
	}
	if search != "" {
		q.Set("search", search)
	}
	path := fmt.Sprintf("/v1/public/%s/catalog/services", url.PathEscape(orgRef))
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	raw, err := c.Get(ctx, path, "")
	if err != nil {
		return nil, err
	}
	rawItems, _ := raw["items"].([]any)
	out := make([]CoreService, 0, len(rawItems))
	for _, entry := range rawItems {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, coreServiceFromMap(m))
	}
	return out, nil
}

func coreServiceFromMap(m map[string]any) CoreService {
	svc := CoreService{
		ID:           stringFromMap(m, "id"),
		Code:         stringFromMap(m, "code"),
		Name:         stringFromMap(m, "name"),
		Description:  stringFromMap(m, "description"),
		CategoryCode: stringFromMap(m, "category_code"),
		SalePrice:    floatFromMap(m, "sale_price"),
		Currency:     stringFromMap(m, "currency"),
	}
	if v, ok := m["tax_rate"].(float64); ok {
		svc.TaxRate = &v
	}
	if v, ok := m["default_duration_minutes"].(float64); ok {
		minutes := int(v)
		svc.DefaultDurationMinutes = &minutes
	}
	if md, ok := m["metadata"].(map[string]any); ok {
		svc.Metadata = md
	} else {
		svc.Metadata = map[string]any{}
	}
	return svc
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func floatFromMap(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}
