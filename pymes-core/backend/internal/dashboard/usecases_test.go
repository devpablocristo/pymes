package dashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	dashboarddomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/dashboard/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type fakeDashboardRepo struct {
	widgets        []dashboarddomain.WidgetDefinition
	topServices    dashboarddomain.TopServicesData
	topServicesErr error
}

func (f *fakeDashboardRepo) ListWidgets(ctx context.Context) ([]dashboarddomain.WidgetDefinition, error) {
	_ = ctx
	return append([]dashboarddomain.WidgetDefinition(nil), f.widgets...), nil
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

func (f *fakeDashboardRepo) LoadTopServices(ctx context.Context, orgID uuid.UUID) (dashboarddomain.TopServicesData, error) {
	_ = ctx
	_ = orgID
	return f.topServices, f.topServicesErr
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

func TestGetWidgetDataReturnsTopServices(t *testing.T) {
	repo := &fakeDashboardRepo{
		widgets: []dashboarddomain.WidgetDefinition{
			{
				WidgetKey:         "services.top",
				Title:             "Servicios top",
				DataEndpoint:      "/v1/dashboard-data/top-services",
				Status:            "active",
				AllowedRoles:      []string{"admin"},
				SupportedContexts: []string{"home"},
			},
		},
		topServices: dashboarddomain.TopServicesData{
			Period: "2026-04",
			Items: []dashboarddomain.TopService{
				{ServiceID: "svc-1", Name: "Mantenimiento", Quantity: 2, Total: 37000},
			},
		},
	}
	uc := NewUsecases(repo)

	out, err := uc.GetWidgetData(context.Background(), dashboarddomain.Viewer{
		OrgID: uuid.New(),
		Role:  "admin",
	}, "home", "top-services")
	if err != nil {
		t.Fatalf("GetWidgetData() error = %v", err)
	}

	data, ok := out.(dashboarddomain.TopServicesData)
	if !ok {
		t.Fatalf("GetWidgetData() type = %T; want dashboarddomain.TopServicesData", out)
	}
	if len(data.Items) != 1 {
		t.Fatalf("len(Items) = %d; want 1", len(data.Items))
	}
	if data.Items[0].ServiceID != "svc-1" {
		t.Fatalf("ServiceID = %q; want svc-1", data.Items[0].ServiceID)
	}
}

func TestGetWidgetDataRejectsWidgetHiddenByRole(t *testing.T) {
	repo := &fakeDashboardRepo{
		widgets: []dashboarddomain.WidgetDefinition{
			{
				WidgetKey:         "billing.subscription",
				Title:             "Billing",
				DataEndpoint:      "/v1/dashboard-data/billing-status",
				Status:            "active",
				AllowedRoles:      []string{"owner", "admin"},
				SupportedContexts: []string{"home"},
			},
		},
	}
	uc := NewUsecases(repo)

	_, err := uc.GetWidgetData(context.Background(), dashboarddomain.Viewer{
		OrgID: uuid.New(),
		Role:  "member",
	}, "home", "billing-status")
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("GetWidgetData() error = %v; want ErrNotFound", err)
	}
}

func TestGetWidgetDataRequiresOrgID(t *testing.T) {
	repo := &fakeDashboardRepo{
		widgets: []dashboarddomain.WidgetDefinition{
			{
				WidgetKey:         "services.top",
				Title:             "Servicios top",
				DataEndpoint:      "/v1/dashboard-data/top-services",
				Status:            "active",
				AllowedRoles:      []string{"admin"},
				SupportedContexts: []string{"home"},
			},
		},
	}
	uc := NewUsecases(repo)

	_, err := uc.GetWidgetData(context.Background(), dashboarddomain.Viewer{
		Role: "admin",
	}, "home", "top-services")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("GetWidgetData() error = %v; want ErrBadInput", err)
	}
}

func TestGetWidgetDataAllowsScopedViewerWithoutRole(t *testing.T) {
	repo := &fakeDashboardRepo{
		widgets: []dashboarddomain.WidgetDefinition{
			{
				WidgetKey:         "services.top",
				Title:             "Servicios top",
				DataEndpoint:      "/v1/dashboard-data/top-services",
				Status:            "active",
				AllowedRoles:      []string{"admin"},
				SupportedContexts: []string{"home"},
			},
		},
		topServices: dashboarddomain.TopServicesData{
			Period: "2026-04",
			Items: []dashboarddomain.TopService{
				{ServiceID: "svc-2", Name: "Instalacion", Quantity: 1, Total: 25000},
			},
		},
	}
	uc := NewUsecases(repo)

	out, err := uc.GetWidgetData(context.Background(), dashboarddomain.Viewer{
		OrgID:  uuid.New(),
		Scopes: []string{"admin:console:read"},
	}, "home", "top-services")
	if err != nil {
		t.Fatalf("GetWidgetData() error = %v", err)
	}

	data, ok := out.(dashboarddomain.TopServicesData)
	if !ok {
		t.Fatalf("GetWidgetData() type = %T; want dashboarddomain.TopServicesData", out)
	}
	if len(data.Items) != 1 || data.Items[0].ServiceID != "svc-2" {
		t.Fatalf("unexpected top services payload: %+v", data.Items)
	}
}
