package workerhost

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	arcauthority "github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/authority"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
	fiscalpostgres "github.com/devpablocristo/pymes/v2/backend/internal/fiscal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Host struct {
	pool          *pgxpool.Pool
	kms           fiscal.KMS
	objects       fiscal.ObjectStore
	renderer      fiscal.ArtifactRenderer
	transport     ar.SOAPTransport
	tickets       *arcauthority.MemoryTickets
	workerID      string
	leaseDuration time.Duration
	orgBatchSize  int
}

type Result struct {
	OrganizationFound bool
	Processed         bool
}

func New(
	pool *pgxpool.Pool,
	kms fiscal.KMS,
	objects fiscal.ObjectStore,
	renderer fiscal.ArtifactRenderer,
	transport ar.SOAPTransport,
	workerID string,
	leaseDuration time.Duration,
) (*Host, error) {
	workerID = strings.TrimSpace(workerID)
	if pool == nil || kms == nil || objects == nil || renderer == nil ||
		transport == nil || workerID == "" {
		return nil, errors.New("fiscal worker host dependencies are required")
	}
	if leaseDuration <= 0 || leaseDuration > 15*time.Minute {
		return nil, errors.New("fiscal worker lease must be between zero and fifteen minutes")
	}
	return &Host{
		pool: pool, kms: kms, objects: objects, renderer: renderer,
		transport: transport, tickets: arcauthority.NewMemoryTickets(),
		workerID: workerID, leaseDuration: leaseDuration, orgBatchSize: 32,
	}, nil
}

// RunOnce looks up only organization identifiers through the audited
// SECURITY DEFINER queue function. Every business read/write then happens in a
// short transaction with app.org_id set, so FORCE RLS remains active without
// retaining a PostgreSQL transaction during WSAA/WSFE network calls.
func (host *Host) RunOnce(ctx context.Context) (Result, error) {
	rows, err := host.pool.Query(ctx, `
		SELECT org_id
		  FROM fiscal.pending_organizations($1)`,
		host.orgBatchSize,
	)
	if err != nil {
		return Result{}, fmt.Errorf("list organizations with fiscal work: %w", err)
	}
	var organizations []string
	for rows.Next() {
		var organizationID string
		if err := rows.Scan(&organizationID); err != nil {
			rows.Close()
			return Result{}, fmt.Errorf("scan fiscal work organization: %w", err)
		}
		organizations = append(organizations, organizationID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Result{}, fmt.Errorf("iterate fiscal work organizations: %w", err)
	}
	rows.Close()
	if len(organizations) == 0 {
		return Result{}, nil
	}

	result := Result{OrganizationFound: true}
	for _, organizationID := range organizations {
		processed, processErr := host.runOrganization(ctx, organizationID)
		if processed {
			result.Processed = true
			return result, processErr
		}
		if processErr != nil {
			return result, processErr
		}
	}
	return result, nil
}

func (host *Host) runOrganization(ctx context.Context, organizationID string) (bool, error) {
	parsedOrganizationID, err := uuid.Parse(organizationID)
	if err != nil {
		return false, fmt.Errorf("parse fiscal worker organization: %w", err)
	}
	repository, err := fiscalpostgres.NewTenant(host.pool, parsedOrganizationID)
	if err != nil {
		return false, err
	}
	credentials, err := fiscalpostgres.NewCredentialProvider(host.pool, host.objects)
	if err != nil {
		return false, err
	}
	authenticator := &wsaa.Authenticator{
		Client: wsaa.NewClient(host.transport), Tickets: host.tickets, KMS: host.kms,
	}
	authority, err := arcauthority.New(credentials, authenticator, host.transport)
	if err != nil {
		return false, err
	}
	processor := fiscal.NewProcessor(
		repository, authority, host.renderer, host.objects,
	)
	worker := fiscal.NewWorker(
		repository, processor, host.workerID, host.leaseDuration,
	)
	return worker.RunOnce(ctx)
}

func (host *Host) Run(
	ctx context.Context,
	pollInterval time.Duration,
	observe func(Result, error),
) error {
	if pollInterval <= 0 {
		return errors.New("fiscal worker poll interval must be positive")
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			result, err := host.RunOnce(ctx)
			if observe != nil {
				observe(result, err)
			}
			timer.Reset(pollInterval)
		}
	}
}
