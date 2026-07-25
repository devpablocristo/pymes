package iamoutbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformiam "github.com/devpablocristo/platform/iam/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresFinalizer struct {
	uow *postgres.UnitOfWork[pgx.Tx]
}

func NewPostgresFinalizer(pool *pgxpool.Pool) (*PostgresFinalizer, error) {
	if pool == nil {
		return nil, errors.New("iam outbox: PostgreSQL pool is required")
	}
	uow, err := postgres.NewPgxUnitOfWork(pool)
	if err != nil {
		return nil, fmt.Errorf("iam outbox: create finalizer unit of work: %w", err)
	}
	return &PostgresFinalizer{uow: uow}, nil
}

func (finalizer *PostgresFinalizer) Finalize(ctx context.Context, result Finalization) error {
	if finalizer == nil || finalizer.uow == nil {
		return errors.New("iam outbox: PostgreSQL finalizer is not configured")
	}
	if err := result.Event.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(result.MessageID) == "" {
		return fmt.Errorf("%w: message ID is required", ErrInvalidEvent)
	}
	organization, err := validateProviderOrganization(result.ProviderOrganization, result.Event)
	if err != nil {
		return err
	}
	invitation, err := validateProviderInvitation(
		result.ProviderInvitation,
		organization.ID,
		result.Event,
	)
	if err != nil {
		return err
	}

	return finalizer.uow.WithinTx(ctx, func(txContext context.Context) error {
		tx, txErr := postgres.Tx[pgx.Tx](txContext)
		if txErr != nil {
			return fmt.Errorf("iam outbox: resolve finalization transaction: %w", txErr)
		}

		request, loadErr := loadProvisioningRequest(
			txContext,
			tx,
			result.Event.RequestID,
		)
		if loadErr != nil {
			return loadErr
		}
		if err := request.matches(result); err != nil {
			return err
		}
		if request.Status != "queued" && request.Status != "provisioned" {
			return fmt.Errorf(
				"iam outbox: provisioning request %s has terminal status %q",
				request.ID,
				request.Status,
			)
		}

		// The organization ID comes from the durable, locked provisioning
		// request. It is never accepted from an identity token or HTTP input.
		if _, err := tx.Exec(txContext, `
			SELECT set_config('app.org_id', $1, true)
		`, request.OrganizationID); err != nil {
			return fmt.Errorf("iam outbox: apply organization transaction context: %w", err)
		}

		store, err := platformiam.NewPostgresStore(tx)
		if err != nil {
			return fmt.Errorf("iam outbox: create IAM transaction store: %w", err)
		}
		localOrganization, err := store.GetOrganization(txContext, request.OrganizationID)
		if err != nil {
			return fmt.Errorf("iam outbox: load local organization: %w", err)
		}
		if localOrganization.Provider != ProviderClerk ||
			localOrganization.Slug != result.Event.Slug {
			return fmt.Errorf("iam outbox: local organization does not match provisioning request")
		}
		if localOrganization.ExternalID != "" &&
			localOrganization.ExternalID != organization.ID {
			return fmt.Errorf(
				"iam outbox: local organization is already bound to Clerk organization %q",
				localOrganization.ExternalID,
			)
		}
		if localOrganization.Status != platformiam.OrganizationProvisioning &&
			localOrganization.Status != platformiam.OrganizationActive {
			return fmt.Errorf(
				"iam outbox: local organization has incompatible status %q",
				localOrganization.Status,
			)
		}
		localOrganization.ExternalID = organization.ID
		localOrganization.Name = result.Event.Name
		localOrganization.Slug = result.Event.Slug
		if _, err := store.UpdateOrganization(txContext, localOrganization); err != nil {
			return fmt.Errorf("iam outbox: attach Clerk organization: %w", err)
		}

		if err := installDefaultAccountingChart(
			txContext,
			tx,
			request.OrganizationID,
		); err != nil {
			return err
		}

		if err := persistOwnerInvitation(
			txContext,
			store,
			request.OrganizationID,
			result.Event,
			invitation,
		); err != nil {
			return err
		}

		command, err := tx.Exec(txContext, `
			UPDATE app.organization_provisioning_requests
			SET status = 'provisioned',
			    updated_at = now()
			WHERE id = $1::uuid
			  AND status IN ('queued', 'provisioned')
		`, request.ID)
		if err != nil {
			return fmt.Errorf("iam outbox: mark provisioning request complete: %w", err)
		}
		if command.RowsAffected() != 1 {
			return fmt.Errorf("iam outbox: provisioning request %s changed concurrently", request.ID)
		}
		return nil
	})
}

func installDefaultAccountingChart(
	ctx context.Context,
	tx pgx.Tx,
	organizationID string,
) error {
	var installed int
	if err := tx.QueryRow(ctx, `
		SELECT accounting.install_chart_template($1::uuid, 'ar-pyme', 1)
	`, organizationID).Scan(&installed); err != nil {
		return fmt.Errorf("iam outbox: install default accounting chart: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		WITH configured AS (
			SELECT
				fiscal_year_start_month,
				(now() AT TIME ZONE timezone)::date AS local_today
			  FROM accounting.organization_settings
			 WHERE org_id = $1::uuid
		),
		fiscal_year AS (
			SELECT make_date(
				extract(year FROM local_today)::integer
					- CASE
						WHEN extract(month FROM local_today)::integer
							< fiscal_year_start_month
						THEN 1
						ELSE 0
					  END,
				fiscal_year_start_month,
				1
			) AS start_date
			  FROM configured
		)
		SELECT accounting.ensure_fiscal_year(
			$1::uuid,
			fiscal_year.start_date,
			'system:provisioning'
		)
		  FROM fiscal_year
	`, organizationID); err != nil {
		return fmt.Errorf(
			"iam outbox: create initial accounting fiscal year: %w",
			err,
		)
	}
	return nil
}

type durableProvisioningRequest struct {
	ID              string
	OrganizationID  string
	Provider        string
	Slug            string
	Name            string
	OwnerEmail      string
	OutboxMessageID string
	Status          string
}

func loadProvisioningRequest(
	ctx context.Context,
	tx pgx.Tx,
	requestID string,
) (durableProvisioningRequest, error) {
	request := durableProvisioningRequest{}
	err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			organization_id::text,
			provider,
			slug,
			organization_name,
			owner_email_normalized,
			outbox_message_id,
			status
		FROM app.organization_provisioning_requests
		WHERE id = $1::uuid
		FOR UPDATE
	`, requestID).Scan(
		&request.ID,
		&request.OrganizationID,
		&request.Provider,
		&request.Slug,
		&request.Name,
		&request.OwnerEmail,
		&request.OutboxMessageID,
		&request.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return durableProvisioningRequest{}, fmt.Errorf(
			"iam outbox: provisioning request %s does not exist",
			requestID,
		)
	}
	if err != nil {
		return durableProvisioningRequest{}, fmt.Errorf(
			"iam outbox: load provisioning request: %w",
			err,
		)
	}
	return request, nil
}

func (request durableProvisioningRequest) matches(result Finalization) error {
	event := result.Event
	if request.ID != event.RequestID ||
		request.OrganizationID != event.OrganizationID ||
		request.Provider != event.Provider ||
		request.Slug != event.Slug ||
		request.Name != event.Name ||
		request.OwnerEmail != event.OwnerEmail ||
		request.OutboxMessageID != result.MessageID {
		return fmt.Errorf(
			"iam outbox: event does not match durable provisioning request %s",
			request.ID,
		)
	}
	return nil
}

func persistOwnerInvitation(
	ctx context.Context,
	store *platformiam.PostgresStore,
	organizationID string,
	event ProvisionOrganizationEvent,
	providerInvitation clerk.Invitation,
) error {
	invitations, err := store.ListInvitationsByOrganization(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("iam outbox: list local invitations: %w", err)
	}

	var existing *platformiam.Invitation
	for index := range invitations {
		invitation := &invitations[index]
		if invitation.Provider == ProviderClerk &&
			invitation.ExternalID == providerInvitation.ID {
			existing = invitation
			break
		}
		if invitation.Status == platformiam.InvitationPending &&
			invitation.Email == event.OwnerEmail {
			if existing != nil && existing.ID != invitation.ID {
				return fmt.Errorf(
					"iam outbox: multiple local pending invitations exist for %q",
					event.OwnerEmail,
				)
			}
			existing = invitation
		}
	}

	expiry := providerInvitation.ExpiresAt.UTC()
	if existing == nil {
		_, err := store.CreateInvitation(ctx, platformiam.Invitation{
			ID:             uuid.NewString(),
			OrganizationID: organizationID,
			Provider:       ProviderClerk,
			ExternalID:     providerInvitation.ID,
			Email:          event.OwnerEmail,
			Role:           LocalOwnerRole,
			Status:         platformiam.InvitationPending,
			ExpiresAt:      expiry,
		})
		if err != nil {
			return fmt.Errorf("iam outbox: create local owner invitation: %w", err)
		}
		return nil
	}

	if existing.OrganizationID != organizationID ||
		existing.Provider != ProviderClerk ||
		existing.Email != event.OwnerEmail ||
		existing.Role != LocalOwnerRole {
		return fmt.Errorf("iam outbox: local owner invitation conflicts with provider state")
	}
	if existing.Status != platformiam.InvitationPending {
		// An accepted invitation may be observed if the provider webhook won
		// the race with an outbox acknowledgement. Never regress its state.
		if existing.ExternalID == providerInvitation.ID &&
			existing.Status == platformiam.InvitationAccepted {
			return nil
		}
		return fmt.Errorf(
			"iam outbox: local owner invitation has incompatible status %q",
			existing.Status,
		)
	}
	if existing.ExternalID != "" && existing.ExternalID != providerInvitation.ID {
		return fmt.Errorf(
			"iam outbox: local pending invitation is bound to Clerk invitation %q",
			existing.ExternalID,
		)
	}
	existing.ExternalID = providerInvitation.ID
	existing.ExpiresAt = expiry
	if _, err := store.UpdateInvitation(ctx, *existing); err != nil {
		return fmt.Errorf("iam outbox: update local owner invitation: %w", err)
	}
	return nil
}
