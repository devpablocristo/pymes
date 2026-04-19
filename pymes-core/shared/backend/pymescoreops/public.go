package pymescoreops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
)

type AvailabilityParams struct {
	Date       string
	Duration   int
	BranchID   string
	ServiceID  string
	ResourceID string
}

func GetBusinessInfo(ctx context.Context, client *pymescorehttp.Client, orgRef string) (map[string]any, error) {
	return client.Get(ctx, fmt.Sprintf("/v1/public/%s/info", url.PathEscape(orgRef)), "")
}

func GetAvailability(ctx context.Context, client *pymescorehttp.Client, orgRef string, params AvailabilityParams) (map[string]any, error) {
	path := fmt.Sprintf("/v1/public/%s/availability", url.PathEscape(orgRef))
	query := url.Values{}
	if params.Date != "" {
		query.Set("date", params.Date)
	}
	if params.Duration > 0 {
		query.Set("duration", fmt.Sprintf("%d", params.Duration))
	}
	if params.BranchID != "" {
		query.Set("branch_id", params.BranchID)
	}
	if params.ServiceID != "" {
		query.Set("service_id", params.ServiceID)
	}
	if params.ResourceID != "" {
		query.Set("resource_id", params.ResourceID)
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return client.Get(ctx, path, "")
}

func BookScheduling(ctx context.Context, client *pymescorehttp.Client, orgRef string, payload map[string]any) (map[string]any, error) {
	return client.Post(ctx, fmt.Sprintf("/v1/public/%s/book", url.PathEscape(orgRef)), "", payload)
}

// CoreService is the minimal shape exposed by the public service catalog.
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

func ListPublicServices(ctx context.Context, client *pymescorehttp.Client, orgRef, vertical, segment, search string) ([]CoreService, error) {
	query := url.Values{}
	if vertical != "" {
		query.Set("vertical", vertical)
	}
	if segment != "" {
		query.Set("segment", segment)
	}
	if search != "" {
		query.Set("search", search)
	}
	path := fmt.Sprintf("/v1/public/%s/catalog/services", url.PathEscape(orgRef))
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	raw, err := client.Get(ctx, path, "")
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
