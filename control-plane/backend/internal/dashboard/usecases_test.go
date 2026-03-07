package dashboard

import (
	"context"
	"testing"

	"github.com/google/uuid"

	dashboarddomain "github.com/devpablocristo/pymes/control-plane/backend/internal/dashboard/usecases/domain"
)

type fakeDashboardRepo struct {
	widgets      []dashboarddomain.WidgetDefinition
	defaults     map[string]dashboarddomain.DefaultLayout
	userLayouts  map[string]dashboarddomain.UserLayout
	resolvedUser *uuid.UUID
}

func (f *fakeDashboardRepo) ListWidgets(ctx context.Context) ([]dashboarddomain.WidgetDefinition, error) {
	_ = ctx
	return append([]dashboarddomain.WidgetDefinition(nil), f.widgets...), nil
}

func (f *fakeDashboardRepo) GetDefaultLayout(ctx context.Context, context string) (dashboarddomain.DefaultLayout, error) {
	_ = ctx
	return f.defaults[context], nil
}

func (f *fakeDashboardRepo) GetUserLayout(ctx context.Context, actor, context string) (dashboarddomain.UserLayout, bool, error) {
	_ = ctx
	layout, ok := f.userLayouts[actor+":"+context]
	return layout, ok, nil
}

func (f *fakeDashboardRepo) SaveUserLayout(ctx context.Context, in dashboarddomain.UserLayout) error {
	_ = ctx
	if f.userLayouts == nil {
		f.userLayouts = map[string]dashboarddomain.UserLayout{}
	}
	f.userLayouts[in.UserActor+":"+in.Context] = in
	return nil
}

func (f *fakeDashboardRepo) ResetUserLayout(ctx context.Context, actor, context string) error {
	_ = ctx
	delete(f.userLayouts, actor+":"+context)
	return nil
}

func (f *fakeDashboardRepo) ResolveUserID(ctx context.Context, actor string) (*uuid.UUID, error) {
	_ = ctx
	_ = actor
	return f.resolvedUser, nil
}

func (f *fakeDashboardRepo) LoadSalesSummary(ctx context.Context, orgID uuid.UUID) (dashboarddomain.SalesSummaryData, error) {
	_ = ctx
	_ = orgID
	return dashboarddomain.SalesSummaryData{}, nil
}
func (f *fakeDashboardRepo) LoadCashflowSummary(ctx context.Context, orgID uuid.UUID) (dashboarddomain.CashflowSummaryData, error) {
	_ = ctx
	_ = orgID
	return dashboarddomain.CashflowSummaryData{}, nil
}
func (f *fakeDashboardRepo) LoadQuotesPipeline(ctx context.Context, orgID uuid.UUID) (dashboarddomain.QuotesPipelineData, error) {
	_ = ctx
	_ = orgID
	return dashboarddomain.QuotesPipelineData{}, nil
}
func (f *fakeDashboardRepo) LoadLowStock(ctx context.Context, orgID uuid.UUID) (dashboarddomain.LowStockData, error) {
	_ = ctx
	_ = orgID
	return dashboarddomain.LowStockData{}, nil
}
func (f *fakeDashboardRepo) LoadRecentSales(ctx context.Context, orgID uuid.UUID) (dashboarddomain.RecentSalesData, error) {
	_ = ctx
	_ = orgID
	return dashboarddomain.RecentSalesData{}, nil
}
func (f *fakeDashboardRepo) LoadTopProducts(ctx context.Context, orgID uuid.UUID) (dashboarddomain.TopProductsData, error) {
	_ = ctx
	_ = orgID
	return dashboarddomain.TopProductsData{}, nil
}
func (f *fakeDashboardRepo) LoadBillingStatus(ctx context.Context, orgID uuid.UUID) (dashboarddomain.BillingStatusData, error) {
	_ = ctx
	_ = orgID
	return dashboarddomain.BillingStatusData{}, nil
}
func (f *fakeDashboardRepo) LoadAuditActivity(ctx context.Context, orgID uuid.UUID) (dashboarddomain.AuditActivityData, error) {
	_ = ctx
	_ = orgID
	return dashboarddomain.AuditActivityData{}, nil
}

func TestGetUsesDefaultLayoutWhenUserLayoutMissing(t *testing.T) {
	repo := newFakeDashboardRepo()
	uc := NewUsecases(repo)
	viewer := dashboarddomain.Viewer{OrgID: uuid.New(), Actor: "user-1", Role: "admin"}

	out, err := uc.Get(context.Background(), viewer, "home")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if out.Layout.Source != "default" {
		t.Fatalf("Layout.Source = %q; want default", out.Layout.Source)
	}
	if len(out.Layout.Items) != 3 {
		t.Fatalf("len(Layout.Items) = %d; want 3", len(out.Layout.Items))
	}
}

func TestGetUsesPersonalizedLayoutWhenPresent(t *testing.T) {
	repo := newFakeDashboardRepo()
	repo.userLayouts["user-1:home"] = dashboarddomain.UserLayout{
		UserActor:     "user-1",
		Context:       "home",
		LayoutVersion: 3,
		Items: []dashboarddomain.LayoutItem{
			{InstanceID: "quotes-1", WidgetKey: "quotes.pipeline", W: 4, H: 2, Visible: true, OrderHint: 0},
			{InstanceID: "sales-1", WidgetKey: "sales.summary", W: 4, H: 2, Visible: true, OrderHint: 1},
		},
		LastAppliedDefaultLayoutKey: "home.base.v1",
	}
	uc := NewUsecases(repo)
	viewer := dashboarddomain.Viewer{OrgID: uuid.New(), Actor: "user-1", Role: "admin"}

	out, err := uc.Get(context.Background(), viewer, "home")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if out.Layout.Source != "user" {
		t.Fatalf("Layout.Source = %q; want user", out.Layout.Source)
	}
	if out.Layout.Version != 3 {
		t.Fatalf("Layout.Version = %d; want 3", out.Layout.Version)
	}
	if got := out.Layout.Items[0].WidgetKey; got != "quotes.pipeline" {
		t.Fatalf("first widget = %q; want quotes.pipeline", got)
	}
}

func TestListWidgetsFiltersByRole(t *testing.T) {
	repo := newFakeDashboardRepo()
	uc := NewUsecases(repo)
	viewer := dashboarddomain.Viewer{OrgID: uuid.New(), Actor: "user-1", Role: "member"}

	out, err := uc.ListWidgets(context.Background(), viewer, "home")
	if err != nil {
		t.Fatalf("ListWidgets() error = %v", err)
	}
	for _, item := range out.Items {
		if item.WidgetKey == "billing.subscription" {
			t.Fatal("billing.subscription should not be visible to member role")
		}
	}
}

func TestSaveRejectsWidgetsOutsideAllowedCatalog(t *testing.T) {
	repo := newFakeDashboardRepo()
	uc := NewUsecases(repo)
	viewer := dashboarddomain.Viewer{OrgID: uuid.New(), Actor: "user-1", Role: "member"}

	_, err := uc.Save(context.Background(), dashboarddomain.SaveDashboardInput{
		Viewer:  viewer,
		Context: "home",
		Items: []dashboarddomain.LayoutItem{{
			InstanceID: "billing-1",
			WidgetKey:  "billing.subscription",
			W:          4,
			H:          2,
			Visible:    true,
		}},
	})
	if err == nil {
		t.Fatal("Save() error = nil; want rejection for disallowed widget")
	}
}

func TestResetReturnsDefaultLayoutAfterPersonalization(t *testing.T) {
	repo := newFakeDashboardRepo()
	uc := NewUsecases(repo)
	viewer := dashboarddomain.Viewer{OrgID: uuid.New(), Actor: "user-1", Role: "admin"}

	_, err := uc.Save(context.Background(), dashboarddomain.SaveDashboardInput{
		Viewer:  viewer,
		Context: "home",
		Items: []dashboarddomain.LayoutItem{
			{InstanceID: "sales-1", WidgetKey: "sales.summary", W: 4, H: 2, Visible: true},
			{InstanceID: "quotes-1", WidgetKey: "quotes.pipeline", W: 4, H: 2, Visible: false},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	out, err := uc.Reset(context.Background(), viewer, "home")
	if err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if out.Layout.Source != "default" {
		t.Fatalf("Layout.Source = %q; want default", out.Layout.Source)
	}
	if len(out.Layout.Items) != 3 {
		t.Fatalf("len(Layout.Items) = %d; want 3", len(out.Layout.Items))
	}
}

func newFakeDashboardRepo() *fakeDashboardRepo {
	userID := uuid.New()
	return &fakeDashboardRepo{
		widgets: []dashboarddomain.WidgetDefinition{
			{WidgetKey: "sales.summary", Title: "Ventas", Status: "active", AllowedRoles: []string{"owner", "admin", "member"}, SupportedContexts: []string{"home"}, DefaultSize: dashboarddomain.WidgetSize{W: 4, H: 2}, MinW: 3, MinH: 2, MaxW: 6, MaxH: 3, DataEndpoint: "/v1/dashboard-data/sales-summary"},
			{WidgetKey: "quotes.pipeline", Title: "Presupuestos", Status: "active", AllowedRoles: []string{"owner", "admin", "member"}, SupportedContexts: []string{"home"}, DefaultSize: dashboarddomain.WidgetSize{W: 4, H: 2}, MinW: 3, MinH: 2, MaxW: 6, MaxH: 3, DataEndpoint: "/v1/dashboard-data/quotes-pipeline"},
			{WidgetKey: "audit.activity", Title: "Actividad", Status: "active", AllowedRoles: []string{"owner", "admin", "member"}, SupportedContexts: []string{"home"}, DefaultSize: dashboarddomain.WidgetSize{W: 6, H: 3}, MinW: 4, MinH: 3, MaxW: 8, MaxH: 6, DataEndpoint: "/v1/dashboard-data/audit-activity"},
			{WidgetKey: "billing.subscription", Title: "Billing", Status: "active", AllowedRoles: []string{"owner", "admin"}, SupportedContexts: []string{"home"}, DefaultSize: dashboarddomain.WidgetSize{W: 4, H: 2}, MinW: 3, MinH: 2, MaxW: 6, MaxH: 4, DataEndpoint: "/v1/dashboard-data/billing-status"},
		},
		defaults: map[string]dashboarddomain.DefaultLayout{
			"home": {
				LayoutKey: "home.base.v1",
				Context:   "home",
				Items: []dashboarddomain.LayoutItem{
					{InstanceID: "sales-1", WidgetKey: "sales.summary", W: 4, H: 2, Visible: true, OrderHint: 0},
					{InstanceID: "quotes-1", WidgetKey: "quotes.pipeline", W: 4, H: 2, Visible: true, OrderHint: 1},
					{InstanceID: "audit-1", WidgetKey: "audit.activity", W: 6, H: 3, Visible: true, OrderHint: 2},
				},
			},
		},
		userLayouts:  map[string]dashboarddomain.UserLayout{},
		resolvedUser: &userID,
	}
}
