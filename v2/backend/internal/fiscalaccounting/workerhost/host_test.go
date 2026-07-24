package workerhost

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscalaccounting"
)

func TestRunOnceProcessesEveryDiscoveredTenant(t *testing.T) {
	t.Parallel()

	processedID := uuid.New()
	noWorkID := uuid.New()
	failedID := uuid.New()
	wantFailure := errors.New("posting failed")

	var (
		lock    sync.Mutex
		visited []uuid.UUID
	)
	host, err := newHost(
		func(_ context.Context, limit int) ([]uuid.UUID, error) {
			if limit != 3 {
				t.Fatalf("organization batch = %d", limit)
			}
			return []uuid.UUID{processedID, noWorkID, failedID}, nil
		},
		func(
			_ context.Context,
			organizationID uuid.UUID,
		) (fiscalaccounting.Result, error) {
			lock.Lock()
			visited = append(visited, organizationID)
			lock.Unlock()
			switch organizationID {
			case processedID:
				return fiscalaccounting.Result{
					OrganizationID: organizationID,
					IntentID:       uuid.New(),
				}, nil
			case noWorkID:
				return fiscalaccounting.Result{},
					fiscalaccounting.ErrNoWork
			default:
				return fiscalaccounting.Result{}, wantFailure
			}
		},
		3,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := host.RunOnce(context.Background())
	if !errors.Is(err, wantFailure) {
		t.Fatalf("run error = %v", err)
	}
	if result.OrganizationsFound != 3 ||
		result.Processed != 1 ||
		result.NoWork != 1 ||
		result.Failed != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(visited) != 3 {
		t.Fatalf("visited tenants = %v", visited)
	}
}

func TestRunOnceTreatsNoWorkAsNormal(t *testing.T) {
	t.Parallel()

	host, err := newHost(
		func(context.Context, int) ([]uuid.UUID, error) {
			return []uuid.UUID{uuid.New()}, nil
		},
		func(
			context.Context,
			uuid.UUID,
		) (fiscalaccounting.Result, error) {
			return fiscalaccounting.Result{}, fiscalaccounting.ErrNoWork
		},
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := host.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run error = %v", err)
	}
	if result.NoWork != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunStopsCleanlyWhenContextIsCancelled(t *testing.T) {
	t.Parallel()

	called := make(chan struct{}, 1)
	host, err := newHost(
		func(context.Context, int) ([]uuid.UUID, error) {
			called <- struct{}{}
			return nil, nil
		},
		func(
			context.Context,
			uuid.UUID,
		) (fiscalaccounting.Result, error) {
			t.Fatal("tenant runner must not be called without organizations")
			return fiscalaccounting.Result{}, nil
		},
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- host.Run(ctx, time.Hour, nil)
	}()
	select {
	case <-called:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("host did not run its initial cycle")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("host did not stop after cancellation")
	}
}

func TestNewHostRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	validLister := func(context.Context, int) ([]uuid.UUID, error) {
		return nil, nil
	}
	validRunner := func(
		context.Context,
		uuid.UUID,
	) (fiscalaccounting.Result, error) {
		return fiscalaccounting.Result{}, nil
	}
	tests := []struct {
		name   string
		lister organizationLister
		runner tenantRunner
		batch  int
	}{
		{name: "missing lister", runner: validRunner, batch: 1},
		{name: "missing runner", lister: validLister, batch: 1},
		{
			name:   "zero batch",
			lister: validLister,
			runner: validRunner,
		},
		{
			name:   "oversized batch",
			lister: validLister,
			runner: validRunner,
			batch:  maxOrganizationBatchSize + 1,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := newHost(
				test.lister,
				test.runner,
				test.batch,
			); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
