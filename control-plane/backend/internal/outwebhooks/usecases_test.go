package outwebhooks

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	webhookmodels "github.com/devpablocristo/pymes/control-plane/backend/internal/outwebhooks/repository/models"
	webhookdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/outwebhooks/usecases/domain"
)

type testResolver struct {
	addrs map[string][]net.IPAddr
	err   error
}

func (r testResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	_ = ctx
	if r.err != nil {
		return nil, r.err
	}
	return r.addrs[host], nil
}

type testRepo struct {
	created     webhookdomain.Endpoint
	updated     webhookdomain.Endpoint
	current     webhookdomain.Endpoint
	createCalls int
	updateCalls int
}

func (r *testRepo) ListEndpoints(ctx context.Context, orgID uuid.UUID) ([]webhookdomain.Endpoint, error) {
	_ = ctx
	_ = orgID
	return nil, nil
}

func (r *testRepo) CreateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error) {
	_ = ctx
	r.createCalls++
	r.created = in
	return in, nil
}

func (r *testRepo) GetEndpoint(ctx context.Context, orgID, id uuid.UUID) (webhookdomain.Endpoint, error) {
	_ = ctx
	_ = orgID
	_ = id
	return r.current, nil
}

func (r *testRepo) UpdateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error) {
	_ = ctx
	r.updateCalls++
	r.updated = in
	return in, nil
}

func (r *testRepo) DeleteEndpoint(ctx context.Context, orgID, id uuid.UUID) error {
	_ = ctx
	_ = orgID
	_ = id
	return nil
}

func (r *testRepo) ListDeliveries(ctx context.Context, orgID, endpointID uuid.UUID, limit int) ([]webhookdomain.Delivery, error) {
	_ = ctx
	_ = orgID
	_ = endpointID
	_ = limit
	return nil, nil
}

func (r *testRepo) CreateOutbox(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error {
	_ = ctx
	_ = orgID
	_ = eventType
	_ = payload
	return nil
}

func (r *testRepo) ListPendingOutbox(ctx context.Context, limit int) ([]webhookmodels.OutboxModel, error) {
	_ = ctx
	_ = limit
	return nil, nil
}

func (r *testRepo) MarkOutbox(ctx context.Context, id uuid.UUID, status, lastError string) error {
	_ = ctx
	_ = id
	_ = status
	_ = lastError
	return nil
}

func (r *testRepo) ListEndpointsForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]webhookdomain.Endpoint, error) {
	_ = ctx
	_ = orgID
	_ = eventType
	return nil, nil
}

func (r *testRepo) CreateDelivery(ctx context.Context, endpointID uuid.UUID, eventType string, payload map[string]any, statusCode *int, responseBody string, attempts int, nextRetry, deliveredAt *time.Time) (webhookdomain.Delivery, error) {
	_ = ctx
	_ = endpointID
	_ = eventType
	_ = payload
	_ = statusCode
	_ = responseBody
	_ = attempts
	_ = nextRetry
	_ = deliveredAt
	return webhookdomain.Delivery{}, nil
}

func (r *testRepo) ListRetryableDeliveries(ctx context.Context, limit int) ([]webhookmodels.DeliveryModel, error) {
	_ = ctx
	_ = limit
	return nil, nil
}

func (r *testRepo) GetDelivery(ctx context.Context, id uuid.UUID) (webhookdomain.Delivery, error) {
	_ = ctx
	_ = id
	return webhookdomain.Delivery{}, nil
}

func (r *testRepo) UpdateDeliveryResult(ctx context.Context, id uuid.UUID, statusCode *int, responseBody string, attempts int, nextRetry, deliveredAt *time.Time) error {
	_ = ctx
	_ = id
	_ = statusCode
	_ = responseBody
	_ = attempts
	_ = nextRetry
	_ = deliveredAt
	return nil
}

func (r *testRepo) GetEndpointByID(ctx context.Context, id uuid.UUID) (webhookdomain.Endpoint, error) {
	_ = ctx
	_ = id
	return webhookdomain.Endpoint{}, nil
}

func (r *testRepo) DeleteOldDeliveries(ctx context.Context, olderThan time.Time) (int64, error) {
	_ = ctx
	_ = olderThan
	return 0, nil
}

func TestCreateEndpointRejectsResolvedPrivateHost(t *testing.T) {
	repo := &testRepo{}
	uc := NewUsecases(repo)
	uc.resolver = testResolver{
		addrs: map[string][]net.IPAddr{
			"internal.example": {{IP: net.ParseIP("10.1.2.3")}},
		},
	}

	_, err := uc.CreateEndpoint(context.Background(), webhookdomain.Endpoint{
		OrgID: uuid.New(),
		URL:   "https://internal.example/hooks",
	})
	if err == nil {
		t.Fatal("CreateEndpoint() error = nil, want private network validation error")
	}
	if !strings.Contains(err.Error(), "private") {
		t.Fatalf("CreateEndpoint() error = %q, want private network validation", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("CreateEndpoint() createCalls = %d, want 0", repo.createCalls)
	}
}

func TestCreateEndpointRejectsUnresolvedHost(t *testing.T) {
	repo := &testRepo{}
	uc := NewUsecases(repo)
	uc.resolver = testResolver{err: errors.New("lookup failed")}

	_, err := uc.CreateEndpoint(context.Background(), webhookdomain.Endpoint{
		OrgID: uuid.New(),
		URL:   "https://missing.example/hooks",
	})
	if err == nil {
		t.Fatal("CreateEndpoint() error = nil, want resolution error")
	}
	if !strings.Contains(err.Error(), "resolved") {
		t.Fatalf("CreateEndpoint() error = %q, want host resolution validation", err)
	}
}

func TestCreateEndpointAllowsResolvedPublicHost(t *testing.T) {
	repo := &testRepo{}
	uc := NewUsecases(repo)
	uc.resolver = testResolver{
		addrs: map[string][]net.IPAddr{
			"public.example": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	got, err := uc.CreateEndpoint(context.Background(), webhookdomain.Endpoint{
		OrgID:    uuid.New(),
		URL:      "https://public.example/hooks",
		IsActive: true,
		Events:   []string{"sales.created"},
	})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("CreateEndpoint() createCalls = %d, want 1", repo.createCalls)
	}
	if got.Secret == "" {
		t.Fatal("CreateEndpoint() secret = empty, want generated secret")
	}
}
