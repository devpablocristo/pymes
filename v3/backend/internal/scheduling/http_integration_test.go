package scheduling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v3/backend/internal/scheduling/handler/dto"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type staticOrganizationDirectory struct {
	organizationID string
}

func (d staticOrganizationDirectory) ResolvePublicOrganization(
	context.Context,
	string,
) (string, error) {
	return d.organizationID, nil
}

func TestSchedulingHTTPEndToEnd(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	suffix := uuid.NewString()
	organizationID := "org_http_sched_" + suffix
	partyID := "party_http_sched_" + suffix
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.organizations (id,name,slug,status)
		VALUES ($1,$2,$3,'ready')`,
		organizationID, "HTTP Scheduling "+suffix, "http-scheduling-"+suffix,
	); err != nil {
		t.Fatal(err)
	}
	tenantTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tenantTx.Exec(
		ctx,
		"SELECT set_config('app.org_id',$1,true)",
		organizationID,
	); err != nil {
		_ = tenantTx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := tenantTx.Exec(ctx, `
		INSERT INTO app.parties (org_id,id,kind,display_name)
		VALUES ($1,$2,'customer','Ada HTTP')`,
		organizationID, partyID,
	); err != nil {
		_ = tenantTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tenantTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	repository := NewPostgresRepository(pool)
	codec, err := NewHMACActionTokenCodec(
		[]byte("01234567890123456789012345678901"),
	)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(
		repository,
		NewPlatformScheduling(),
		codec,
		WithOrganizationDirectory(staticOrganizationDirectory{
			organizationID: organizationID,
		}),
	)
	branchID, serviceID, professionalID := uuid.New(), uuid.New(), uuid.New()
	if _, err := service.CreateBranch(ctx, domain.Branch{
		OrganizationID: organizationID,
		ID:             branchID,
		Code:           "centro",
		Slug:           "centro",
		Name:           "Centro",
		Timezone:       "America/Argentina/Buenos_Aires",
		Address:        "Calle 1",
		Active:         true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateResource(ctx, domain.Resource{
		OrganizationID: organizationID,
		ID:             professionalID,
		BranchID:       branchID,
		Code:           "profesional",
		Name:           "Profesional",
		Kind:           domain.ResourceProfessional,
		Capacity:       1,
		Timezone:       "America/Argentina/Buenos_Aires",
		Active:         true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateService(ctx, domain.Service{
		OrganizationID:      organizationID,
		ID:                  serviceID,
		Code:                "consulta",
		Name:                "Consulta",
		DurationMinutes:     60,
		BufferBeforeMinutes: 15,
		BufferAfterMinutes:  15,
		SlotMinutes:         30,
		Price:               "100.00",
		Currency:            "ARS",
		Mode:                domain.FulfillmentInPerson,
		MaxParticipants:     1,
		AllowWaitlist:       true,
		Active:              true,
	}, []domain.ResourceRequirement{{
		ResourceID: &professionalID,
		Kind:       domain.ResourceProfessional,
		Mode:       domain.AllocationExclusive,
		Units:      1,
	}}); err != nil {
		t.Fatal(err)
	}
	for _, weekday := range []time.Weekday{time.Monday, time.Tuesday} {
		for _, rule := range []domain.AvailabilityRule{
			{
				OrganizationID: organizationID,
				ID:             uuid.New(),
				BranchID:       branchID,
				Kind:           domain.AvailabilityBranch,
				Weekday:        weekday,
				StartMinute:    9 * 60,
				EndMinute:      18 * 60,
				Timezone:       "America/Argentina/Buenos_Aires",
				Active:         true,
			},
			{
				OrganizationID: organizationID,
				ID:             uuid.New(),
				BranchID:       branchID,
				ResourceID:     &professionalID,
				Kind:           domain.AvailabilityResource,
				Weekday:        weekday,
				StartMinute:    9 * 60,
				EndMinute:      18 * 60,
				Timezone:       "America/Argentina/Buenos_Aires",
				Active:         true,
			},
		} {
			if _, err := service.CreateAvailabilityRule(ctx, rule); err != nil {
				t.Fatal(err)
			}
		}
	}

	principal := Principal{
		OrganizationID:     organizationID,
		ActorID:            "user-http",
		Role:               "admin",
		Permissions:        []string{domain.PermissionRead, domain.PermissionOperate, domain.PermissionManage},
		OrganizationStatus: "ready",
		MembershipStatus:   "active",
	}
	handler := NewHTTPHandler(
		service,
		authenticatorFake{principal: principal},
	).Handler()

	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, time.August, 3, 0, 0, 0, 0, location)
	availabilityBody := map[string]any{
		"branch_id":    branchID,
		"service_id":   serviceID,
		"from":         day,
		"until":        day.Add(24 * time.Hour),
		"participants": 1,
		"allocations": []map[string]any{{
			"resource_id": professionalID,
			"mode":        domain.AllocationExclusive,
			"units":       1,
		}},
	}
	availability := performSchedulingJSON(
		t, handler, http.MethodPost,
		fmt.Sprintf(
			"/api/v1/organizations/%s/scheduling/availability",
			organizationID,
		),
		availabilityBody, "", http.StatusOK,
	)
	var slots []dto.Slot
	if err := json.Unmarshal(availability.Body.Bytes(), &slots); err != nil {
		t.Fatal(err)
	}
	if len(slots) < 4 {
		t.Fatalf("available slots=%d body=%s", len(slots), availability.Body.String())
	}

	bookingBody := map[string]any{
		"branch_id":  branchID,
		"service_id": serviceID,
		"customer": map[string]any{
			"party_id": partyID,
			"name":     "Ada HTTP",
		},
		"start_at":     slots[0].StartAt,
		"participants": 1,
		"allocations": []map[string]any{{
			"resource_id": professionalID,
			"mode":        domain.AllocationExclusive,
			"units":       1,
		}},
	}
	type concurrentResult struct {
		response *httptest.ResponseRecorder
		key      string
	}
	results := make(chan concurrentResult, 2)
	var wait sync.WaitGroup
	for index := range 2 {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			key := fmt.Sprintf("http-concurrent-%d-%s", index, suffix)
			results <- concurrentResult{
				response: performSchedulingJSONNoFail(
					handler, http.MethodPost,
					fmt.Sprintf(
						"/api/v1/organizations/%s/scheduling/bookings",
						organizationID,
					),
					bookingBody, key,
				),
				key: key,
			}
		}(index)
	}
	wait.Wait()
	close(results)
	var created []dto.Booking
	createdCount, conflictCount := 0, 0
	for result := range results {
		switch result.response.Code {
		case http.StatusCreated:
			createdCount++
			if err := json.Unmarshal(result.response.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
		case http.StatusConflict:
			conflictCount++
		default:
			t.Fatalf(
				"concurrent booking %s status=%d body=%s",
				result.key, result.response.Code, result.response.Body.String(),
			)
		}
	}
	if createdCount != 1 || conflictCount != 1 || len(created) != 1 ||
		created[0].CustomerName != "Ada HTTP" {
		t.Fatalf(
			"created=%d conflicts=%d booking=%+v",
			createdCount, conflictCount, created,
		)
	}

	rescheduleBody := map[string]any{
		"expected_version": 1,
		"start_at":         slots[3].StartAt,
		"duration_minutes": 90,
		"allocations": []map[string]any{{
			"resource_id": professionalID,
			"mode":        domain.AllocationExclusive,
			"units":       1,
		}},
	}
	rescheduledResponse := performSchedulingJSON(
		t, handler, http.MethodPost,
		fmt.Sprintf(
			"/api/v1/organizations/%s/scheduling/bookings/%s/reschedule",
			organizationID, created[0].ID,
		),
		rescheduleBody, "http-reschedule-"+suffix, http.StatusOK,
	)
	var rescheduled dto.Booking
	if err := json.Unmarshal(rescheduledResponse.Body.Bytes(), &rescheduled); err != nil {
		t.Fatal(err)
	}
	if rescheduled.DurationMinutes != 90 ||
		!rescheduled.EndAt.Equal(rescheduled.StartAt.Add(90*time.Minute)) {
		t.Fatalf("resize response=%+v", rescheduled)
	}
	cancelResponse := performSchedulingJSON(
		t, handler, http.MethodPost,
		fmt.Sprintf(
			"/api/v1/organizations/%s/scheduling/bookings/%s/cancel",
			organizationID, rescheduled.ID,
		),
		map[string]any{
			"expected_version": 1,
			"reason":           "Cliente solicitó la cancelación",
		},
		"http-cancel-"+suffix, http.StatusOK,
	)
	var cancelled dto.Booking
	if err := json.Unmarshal(cancelResponse.Body.Bytes(), &cancelled); err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != domain.BookingCancelled ||
		cancelled.CancellationReason != "Cliente solicitó la cancelación" {
		t.Fatalf("cancel response=%+v", cancelled)
	}

	crossTenant := performSchedulingJSONNoFail(
		handler, http.MethodGet,
		"/api/v1/organizations/org-other/scheduling/bookings/"+cancelled.ID.String(),
		nil, "",
	)
	if crossTenant.Code != http.StatusForbidden {
		t.Fatalf(
			"cross-tenant status=%d body=%s",
			crossTenant.Code, crossTenant.Body.String(),
		)
	}
	publicCatalog := performSchedulingJSON(
		t, handler, http.MethodGet,
		"/api/v1/public/scheduling/http/catalog",
		nil, "", http.StatusOK,
	)
	if bytes.Contains(publicCatalog.Body.Bytes(), []byte(organizationID)) ||
		bytes.Contains(publicCatalog.Body.Bytes(), []byte(partyID)) {
		t.Fatalf("public catalog leaked tenant/customer identity: %s", publicCatalog.Body.String())
	}
}

func performSchedulingJSON(
	t *testing.T,
	handler http.Handler,
	method, path string,
	body any,
	idempotencyKey string,
	expectedStatus int,
) *httptest.ResponseRecorder {
	t.Helper()
	response := performSchedulingJSONNoFail(
		handler, method, path, body, idempotencyKey,
	)
	if response.Code != expectedStatus {
		t.Fatalf(
			"%s %s status=%d want=%d body=%s",
			method, path, response.Code, expectedStatus, response.Body.String(),
		)
	}
	return response
}

func performSchedulingJSONNoFail(
	handler http.Handler,
	method, path string,
	body any,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	var encoded []byte
	if body != nil {
		encoded, _ = json.Marshal(body)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
		request.Header.Set("X-Source-Version", "1")
		request.Header.Set("X-Request-ID", "request-"+idempotencyKey)
		request.Header.Set("X-Correlation-ID", "correlation-"+idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
