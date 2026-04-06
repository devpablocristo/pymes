package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	dashboarddomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/dashboard/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/authz"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	ListWidgets(ctx context.Context) ([]dashboarddomain.WidgetDefinition, error)
	LoadSalesSummary(ctx context.Context, orgID uuid.UUID) (dashboarddomain.SalesSummaryData, error)
	LoadCashflowSummary(ctx context.Context, orgID uuid.UUID) (dashboarddomain.CashflowSummaryData, error)
	LoadQuotesPipeline(ctx context.Context, orgID uuid.UUID) (dashboarddomain.QuotesPipelineData, error)
	LoadLowStock(ctx context.Context, orgID uuid.UUID) (dashboarddomain.LowStockData, error)
	LoadRecentSales(ctx context.Context, orgID uuid.UUID) (dashboarddomain.RecentSalesData, error)
	LoadTopProducts(ctx context.Context, orgID uuid.UUID) (dashboarddomain.TopProductsData, error)
	LoadTopServices(ctx context.Context, orgID uuid.UUID) (dashboarddomain.TopServicesData, error)
	LoadBillingStatus(ctx context.Context, orgID uuid.UUID) (dashboarddomain.BillingStatusData, error)
	LoadAuditActivity(ctx context.Context, orgID uuid.UUID) (dashboarddomain.AuditActivityData, error)
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) GetWidgetData(ctx context.Context, viewer dashboarddomain.Viewer, rawContext, endpointKey string) (any, error) {
	contextKey := normalizeContext(rawContext)
	widget, err := u.findWidgetByEndpoint(ctx, viewer, contextKey, endpointKey)
	if err != nil {
		return nil, err
	}
	if viewer.OrgID == uuid.Nil {
		return nil, fmt.Errorf("org_id is required: %w", httperrors.ErrBadInput)
	}

	switch widget.WidgetKey {
	case "sales.summary":
		return u.repo.LoadSalesSummary(ctx, viewer.OrgID)
	case "cashflow.summary":
		return u.repo.LoadCashflowSummary(ctx, viewer.OrgID)
	case "quotes.pipeline":
		return u.repo.LoadQuotesPipeline(ctx, viewer.OrgID)
	case "inventory.low_stock":
		return u.repo.LoadLowStock(ctx, viewer.OrgID)
	case "sales.recent":
		return u.repo.LoadRecentSales(ctx, viewer.OrgID)
	case "products.top":
		return u.repo.LoadTopProducts(ctx, viewer.OrgID)
	case "services.top":
		return u.repo.LoadTopServices(ctx, viewer.OrgID)
	case "billing.subscription":
		return u.repo.LoadBillingStatus(ctx, viewer.OrgID)
	case "audit.activity":
		return u.repo.LoadAuditActivity(ctx, viewer.OrgID)
	default:
		return nil, fmt.Errorf("widget not implemented: %w", httperrors.ErrNotFound)
	}
}

func (u *Usecases) allowedWidgets(ctx context.Context, viewer dashboarddomain.Viewer, contextKey string) ([]dashboarddomain.WidgetDefinition, error) {
	widgets, err := u.repo.ListWidgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list widgets: %w", err)
	}
	allowed := make([]dashboarddomain.WidgetDefinition, 0, len(widgets))
	for _, widget := range widgets {
		if widgetAllowed(widget, viewer, contextKey) {
			allowed = append(allowed, widget)
		}
	}
	sort.SliceStable(allowed, func(i, j int) bool {
		return allowed[i].Title < allowed[j].Title
	})
	return allowed, nil
}

func (u *Usecases) findWidgetByEndpoint(ctx context.Context, viewer dashboarddomain.Viewer, contextKey, endpointKey string) (dashboarddomain.WidgetDefinition, error) {
	widgets, err := u.allowedWidgets(ctx, viewer, contextKey)
	if err != nil {
		return dashboarddomain.WidgetDefinition{}, err
	}
	needle := strings.Trim(strings.ToLower(strings.TrimSpace(endpointKey)), "/")
	for _, widget := range widgets {
		candidate := strings.Trim(strings.TrimPrefix(strings.ToLower(widget.DataEndpoint), "/v1/dashboard-data/"), "/")
		if candidate == needle {
			return widget, nil
		}
	}
	return dashboarddomain.WidgetDefinition{}, fmt.Errorf("widget not found: %w", httperrors.ErrNotFound)
}

func widgetAllowed(widget dashboarddomain.WidgetDefinition, viewer dashboarddomain.Viewer, contextKey string) bool {
	if !strings.EqualFold(widget.Status, "active") && widget.Status != "" {
		return false
	}
	if len(widget.SupportedContexts) > 0 && !containsFold(widget.SupportedContexts, contextKey) {
		return false
	}
	if len(widget.AllowedRoles) > 0 {
		role := strings.TrimSpace(viewer.Role)
		if role != "" {
			if !containsFold(widget.AllowedRoles, role) {
				return false
			}
		} else if len(viewer.Scopes) == 0 {
			return false
		}
	}
	if len(widget.RequiredScopes) > 0 && !isPrivilegedRole(viewer.Role) && !intersectsFold(widget.RequiredScopes, viewer.Scopes) {
		return false
	}
	return true
}

func normalizeContext(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "home"
	}
	return value
}

func containsFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(needle)) {
			return true
		}
	}
	return false
}

func intersectsFold(left, right []string) bool {
	for _, value := range left {
		if containsFold(right, value) {
			return true
		}
	}
	return false
}

func isPrivilegedRole(role string) bool {
	return authz.IsPrivilegedRole(role)
}
