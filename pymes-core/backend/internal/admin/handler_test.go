package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ctxkeys "github.com/devpablocristo/core/security/go/contextkeys"
	admindomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/usecases/domain"
)

type fakeUsecases struct {
	settings    admindomain.TenantSettings
	updatePatch admindomain.TenantSettingsPatch
	updateActor *string
}

func (f *fakeUsecases) GetBootstrap(_ context.Context, _ string, _ string, _ []string, _ string, _ string) (map[string]any, error) {
	return nil, nil
}

func (f *fakeUsecases) GetTenantSettings(_ context.Context, _ string) (admindomain.TenantSettings, error) {
	return f.settings, nil
}

func (f *fakeUsecases) UpdateTenantSettings(_ context.Context, _ string, patch admindomain.TenantSettingsPatch, actor *string) (admindomain.TenantSettings, error) {
	f.updatePatch = patch
	f.updateActor = actor
	f.settings.SchedulingEnabled = patch.SchedulingEnabled != nil && *patch.SchedulingEnabled
	return f.settings, nil
}

func (f *fakeUsecases) ListActivity(_ context.Context, _ string, _ int) ([]admindomain.ActivityEvent, error) {
	return nil, nil
}

func TestHandlerUpdateTenantSettingsAcceptsSchedulingEnabled(t *testing.T) {
	t.Parallel()

	repo := &fakeUsecases{settings: baseTenantSettings()}
	rec := performTenantSettingsUpdate(t, repo, `{"scheduling_enabled":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.updatePatch.SchedulingEnabled == nil || !*repo.updatePatch.SchedulingEnabled {
		t.Fatalf("expected scheduling_enabled to be forwarded")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["scheduling_enabled"] != true {
		t.Fatalf("expected scheduling_enabled=true in response, got %#v", body["scheduling_enabled"])
	}
}

func performTenantSettingsUpdate(t *testing.T, uc usecasesPort, body string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, "00000000-0000-0000-0000-000000000001")
		c.Set(ctxkeys.CtxKeyActor, "owner@example.com")
		c.Set(ctxkeys.CtxKeyRole, "owner")
		c.Set(ctxkeys.CtxKeyScopes, []string{"admin:console:write"})
		c.Set(ctxkeys.CtxKeyAuthMethod, "jwt")
		c.Next()
	})

	auth := router.Group("/v1")
	NewHandler(uc).RegisterRoutes(auth)

	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/tenant-settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func baseTenantSettings() admindomain.TenantSettings {
	return admindomain.TenantSettings{
		OrgID:                    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		PlanCode:                 "starter",
		HardLimits:               map[string]any{},
		BillingStatus:            "trialing",
		Currency:                 "ARS",
		SupportedCurrencies:      []string{"ARS"},
		TaxRate:                  21,
		QuotePrefix:              "PRE",
		SalePrefix:               "VTA",
		NextQuoteNumber:          1,
		NextSaleNumber:           1,
		AllowNegativeStock:       true,
		PurchasePrefix:           "COM",
		NextPurchaseNumber:       1,
		ReturnPrefix:             "DEV",
		CreditNotePrefix:         "NC",
		NextReturnNumber:         1,
		NextCreditNoteNumber:     1,
		BusinessName:             "Taller Norte",
		ClientLabel:              "clientes",
		UsesBilling:              true,
		PaymentMethod:            "mixed",
		Vertical:                 "workshops",
		SchedulingEnabled:        false,
		SchedulingLabel:         "Turno",
		SchedulingReminderHours: 24,
		DefaultRateType:          "blue",
		UpdatedAt:                time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	}
}
