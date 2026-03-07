package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	dashboarddomain "github.com/devpablocristo/pymes/control-plane/backend/internal/dashboard/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type RepositoryPort interface {
	ListWidgets(ctx context.Context) ([]dashboarddomain.WidgetDefinition, error)
	GetDefaultLayout(ctx context.Context, context string) (dashboarddomain.DefaultLayout, error)
	GetUserLayout(ctx context.Context, actor, context string) (dashboarddomain.UserLayout, bool, error)
	SaveUserLayout(ctx context.Context, in dashboarddomain.UserLayout) error
	ResetUserLayout(ctx context.Context, actor, context string) error
	ResolveUserID(ctx context.Context, actor string) (*uuid.UUID, error)
	LoadSalesSummary(ctx context.Context, orgID uuid.UUID) (dashboarddomain.SalesSummaryData, error)
	LoadCashflowSummary(ctx context.Context, orgID uuid.UUID) (dashboarddomain.CashflowSummaryData, error)
	LoadQuotesPipeline(ctx context.Context, orgID uuid.UUID) (dashboarddomain.QuotesPipelineData, error)
	LoadLowStock(ctx context.Context, orgID uuid.UUID) (dashboarddomain.LowStockData, error)
	LoadRecentSales(ctx context.Context, orgID uuid.UUID) (dashboarddomain.RecentSalesData, error)
	LoadTopProducts(ctx context.Context, orgID uuid.UUID) (dashboarddomain.TopProductsData, error)
	LoadBillingStatus(ctx context.Context, orgID uuid.UUID) (dashboarddomain.BillingStatusData, error)
	LoadAuditActivity(ctx context.Context, orgID uuid.UUID) (dashboarddomain.AuditActivityData, error)
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) Get(ctx context.Context, viewer dashboarddomain.Viewer, rawContext string) (dashboarddomain.Dashboard, error) {
	contextKey := normalizeContext(rawContext)
	defaults, err := u.repo.GetDefaultLayout(ctx, contextKey)
	if err != nil {
		return dashboarddomain.Dashboard{}, fmt.Errorf("load default layout: %w", err)
	}

	allowedWidgets, err := u.allowedWidgets(ctx, viewer, contextKey)
	if err != nil {
		return dashboarddomain.Dashboard{}, err
	}
	widgetMap := indexWidgets(allowedWidgets)

	items := sanitizeLayout(defaults.Items, widgetMap)
	layout := dashboarddomain.DashboardLayout{
		Source:    "default",
		LayoutKey: defaults.LayoutKey,
		Version:   1,
		Items:     sortLayoutItems(items),
	}

	if viewer.Actor != "" {
		userLayout, ok, err := u.repo.GetUserLayout(ctx, viewer.Actor, contextKey)
		if err != nil {
			return dashboarddomain.Dashboard{}, fmt.Errorf("load user layout: %w", err)
		}
		if ok {
			userItems := sanitizeLayout(userLayout.Items, widgetMap)
			if len(userItems) > 0 {
				layout.Source = "user"
				layout.LayoutKey = nonEmpty(userLayout.LastAppliedDefaultLayoutKey, defaults.LayoutKey)
				layout.Version = maxInt(userLayout.LayoutVersion, 1)
				layout.Items = sortLayoutItems(userItems)
			}
		}
	}

	return dashboarddomain.Dashboard{
		Context:          contextKey,
		Layout:           layout,
		AvailableWidgets: allowedWidgets,
	}, nil
}

func (u *Usecases) ListWidgets(ctx context.Context, viewer dashboarddomain.Viewer, rawContext string) (dashboarddomain.WidgetCatalog, error) {
	contextKey := normalizeContext(rawContext)
	widgets, err := u.allowedWidgets(ctx, viewer, contextKey)
	if err != nil {
		return dashboarddomain.WidgetCatalog{}, err
	}
	return dashboarddomain.WidgetCatalog{Context: contextKey, Items: widgets}, nil
}

func (u *Usecases) Save(ctx context.Context, in dashboarddomain.SaveDashboardInput) (dashboarddomain.Dashboard, error) {
	contextKey := normalizeContext(in.Context)
	if strings.TrimSpace(in.Viewer.Actor) == "" {
		return dashboarddomain.Dashboard{}, fmt.Errorf("actor is required: %w", httperrors.ErrBadInput)
	}
	defaults, err := u.repo.GetDefaultLayout(ctx, contextKey)
	if err != nil {
		return dashboarddomain.Dashboard{}, fmt.Errorf("load default layout: %w", err)
	}
	allowedWidgets, err := u.allowedWidgets(ctx, in.Viewer, contextKey)
	if err != nil {
		return dashboarddomain.Dashboard{}, err
	}
	widgetMap := indexWidgets(allowedWidgets)
	validatedItems, err := validateLayout(in.Items, widgetMap)
	if err != nil {
		return dashboarddomain.Dashboard{}, err
	}

	version := 1
	if existing, ok, err := u.repo.GetUserLayout(ctx, in.Viewer.Actor, contextKey); err != nil {
		return dashboarddomain.Dashboard{}, fmt.Errorf("load user layout: %w", err)
	} else if ok {
		version = existing.LayoutVersion + 1
	}

	userID, err := u.repo.ResolveUserID(ctx, in.Viewer.Actor)
	if err != nil {
		return dashboarddomain.Dashboard{}, fmt.Errorf("resolve user id: %w", err)
	}

	if err := u.repo.SaveUserLayout(ctx, dashboarddomain.UserLayout{
		UserID:                      userID,
		UserActor:                   in.Viewer.Actor,
		Context:                     contextKey,
		LayoutVersion:               version,
		Items:                       validatedItems,
		LastAppliedDefaultLayoutKey: defaults.LayoutKey,
	}); err != nil {
		return dashboarddomain.Dashboard{}, fmt.Errorf("save user layout: %w", err)
	}
	return u.Get(ctx, in.Viewer, contextKey)
}

func (u *Usecases) Reset(ctx context.Context, viewer dashboarddomain.Viewer, rawContext string) (dashboarddomain.Dashboard, error) {
	contextKey := normalizeContext(rawContext)
	if strings.TrimSpace(viewer.Actor) == "" {
		return dashboarddomain.Dashboard{}, fmt.Errorf("actor is required: %w", httperrors.ErrBadInput)
	}
	if err := u.repo.ResetUserLayout(ctx, viewer.Actor, contextKey); err != nil {
		return dashboarddomain.Dashboard{}, fmt.Errorf("reset user layout: %w", err)
	}
	return u.Get(ctx, viewer, contextKey)
}

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

func validateLayout(items []dashboarddomain.LayoutItem, allowed map[string]dashboarddomain.WidgetDefinition) ([]dashboarddomain.LayoutItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("layout items are required: %w", httperrors.ErrBadInput)
	}
	validated := make([]dashboarddomain.LayoutItem, 0, len(items))
	seenInstances := make(map[string]struct{}, len(items))
	for idx, item := range items {
		widgetKey := strings.TrimSpace(item.WidgetKey)
		instanceID := strings.TrimSpace(item.InstanceID)
		if instanceID == "" {
			return nil, fmt.Errorf("instance_id is required: %w", httperrors.ErrBadInput)
		}
		if _, exists := seenInstances[instanceID]; exists {
			return nil, fmt.Errorf("duplicate instance_id %q: %w", instanceID, httperrors.ErrBadInput)
		}
		seenInstances[instanceID] = struct{}{}

		widget, ok := allowed[widgetKey]
		if !ok {
			return nil, fmt.Errorf("widget %q is not allowed: %w", widgetKey, httperrors.ErrForbidden)
		}
		if item.W < widget.MinW || item.W > widget.MaxW || item.H < widget.MinH || item.H > widget.MaxH {
			return nil, fmt.Errorf("widget %q has invalid size: %w", widgetKey, httperrors.ErrBadInput)
		}
		clean := item
		clean.WidgetKey = widgetKey
		clean.InstanceID = instanceID
		clean.X = maxInt(item.X, 0)
		clean.Y = maxInt(item.Y, 0)
		clean.OrderHint = idx
		if clean.Settings == nil {
			clean.Settings = map[string]any{}
		}
		validated = append(validated, clean)
	}
	return sortLayoutItems(validated), nil
}

func sanitizeLayout(items []dashboarddomain.LayoutItem, allowed map[string]dashboarddomain.WidgetDefinition) []dashboarddomain.LayoutItem {
	sanitized := make([]dashboarddomain.LayoutItem, 0, len(items))
	seenInstances := make(map[string]struct{}, len(items))
	for idx, item := range items {
		widget, ok := allowed[strings.TrimSpace(item.WidgetKey)]
		if !ok {
			continue
		}
		instanceID := strings.TrimSpace(item.InstanceID)
		if instanceID == "" {
			continue
		}
		if _, exists := seenInstances[instanceID]; exists {
			continue
		}
		seenInstances[instanceID] = struct{}{}

		clean := item
		clean.WidgetKey = widget.WidgetKey
		clean.InstanceID = instanceID
		clean.W = clampInt(clean.W, widget.MinW, widget.MaxW, widget.DefaultSize.W)
		clean.H = clampInt(clean.H, widget.MinH, widget.MaxH, widget.DefaultSize.H)
		clean.X = maxInt(clean.X, 0)
		clean.Y = maxInt(clean.Y, 0)
		if clean.Settings == nil {
			clean.Settings = map[string]any{}
		}
		if clean.OrderHint < 0 {
			clean.OrderHint = idx
		}
		sanitized = append(sanitized, clean)
	}
	return sanitized
}

func sortLayoutItems(items []dashboarddomain.LayoutItem) []dashboarddomain.LayoutItem {
	copied := append([]dashboarddomain.LayoutItem(nil), items...)
	sort.SliceStable(copied, func(i, j int) bool {
		if copied[i].Pinned != copied[j].Pinned {
			return copied[i].Pinned
		}
		if copied[i].OrderHint != copied[j].OrderHint {
			return copied[i].OrderHint < copied[j].OrderHint
		}
		if copied[i].Y != copied[j].Y {
			return copied[i].Y < copied[j].Y
		}
		if copied[i].X != copied[j].X {
			return copied[i].X < copied[j].X
		}
		return copied[i].InstanceID < copied[j].InstanceID
	})
	return copied
}

func indexWidgets(widgets []dashboarddomain.WidgetDefinition) map[string]dashboarddomain.WidgetDefinition {
	indexed := make(map[string]dashboarddomain.WidgetDefinition, len(widgets))
	for _, widget := range widgets {
		indexed[widget.WidgetKey] = widget
	}
	return indexed
}

func widgetAllowed(widget dashboarddomain.WidgetDefinition, viewer dashboarddomain.Viewer, contextKey string) bool {
	if !strings.EqualFold(widget.Status, "active") && widget.Status != "" {
		return false
	}
	if len(widget.SupportedContexts) > 0 && !containsFold(widget.SupportedContexts, contextKey) {
		return false
	}
	if len(widget.AllowedRoles) > 0 && !containsFold(widget.AllowedRoles, viewer.Role) {
		return false
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
	return strings.EqualFold(role, "owner") || strings.EqualFold(role, "admin") || strings.EqualFold(role, "service")
}

func clampInt(value, minValue, maxValue, fallback int) int {
	if value == 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
