// Package provisioning implements privileged, provider-queued organization
// provisioning for Pymes v2.
package provisioning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformiam "github.com/devpablocristo/platform/iam/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ProviderClerk = "clerk"

	ProvisionOrganizationTopic = "iam.organization.provision.requested.v1"
)

var (
	ErrInvalidInput    = errors.New("IAM_PROVISION_INVALID")
	ErrPayloadConflict = errors.New("IAM_PROVISION_PAYLOAD_CONFLICT")
	ErrSlugConflict    = errors.New("IAM_PROVISION_SLUG_CONFLICT")

	slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

type Input struct {
	Name       string
	Slug       string
	OwnerEmail string
}

type Result struct {
	RequestID       string `json:"request_id"`
	OrganizationID  string `json:"organization_id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	OwnerEmail      string `json:"owner_email"`
	Status          string `json:"status"`
	OutboxMessageID string `json:"outbox_message_id"`
}

type messageAppender interface {
	Append(context.Context, pgx.Tx, platformoutbox.MessageInput) (platformoutbox.Message, error)
}

type idGenerator interface {
	NewID() string
}

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.NewString() }

type Service struct {
	uow      *postgres.UnitOfWork[pgx.Tx]
	appender messageAppender
	ids      idGenerator
}

func NewService(pool *pgxpool.Pool) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", ErrInvalidInput)
	}

	uow, err := postgres.NewPgxUnitOfWork(pool)
	if err != nil {
		return nil, fmt.Errorf("create provisioning unit of work: %w", err)
	}
	appender, err := platformoutbox.NewStore(pool, platformoutbox.StoreConfig{
		Table:              pgx.Identifier{platformoutbox.DefaultTableName},
		DefaultMaxAttempts: 12,
	})
	if err != nil {
		return nil, fmt.Errorf("create provisioning outbox store: %w", err)
	}
	return newService(uow, appender, uuidGenerator{})
}

func newService(
	uow *postgres.UnitOfWork[pgx.Tx],
	appender messageAppender,
	ids idGenerator,
) (*Service, error) {
	if uow == nil || appender == nil || ids == nil {
		return nil, fmt.Errorf("%w: provisioning dependencies are required", ErrInvalidInput)
	}
	return &Service{uow: uow, appender: appender, ids: ids}, nil
}

func (service *Service) Provision(ctx context.Context, input Input) (Result, error) {
	prepared, err := prepare(input)
	if err != nil {
		return Result{}, err
	}
	if service == nil || service.uow == nil || service.appender == nil || service.ids == nil {
		return Result{}, fmt.Errorf("%w: provisioning service is not configured", ErrInvalidInput)
	}

	result := Result{}
	err = service.uow.WithinTx(ctx, func(txContext context.Context) error {
		tx, txErr := postgres.Tx[pgx.Tx](txContext)
		if txErr != nil {
			return fmt.Errorf("resolve provisioning transaction: %w", txErr)
		}

		request := provisioningRequest{
			RequestID:       service.ids.NewID(),
			OrganizationID:  service.ids.NewID(),
			Provider:        ProviderClerk,
			Name:            prepared.Name,
			Slug:            prepared.Slug,
			OwnerEmail:      prepared.OwnerEmail,
			PayloadHash:     prepared.PayloadHash,
			OutboxMessageID: service.ids.NewID(),
			Status:          "queued",
		}

		inserted, insertErr := insertRequest(ctx, tx, request)
		if insertErr != nil {
			return insertErr
		}
		if !inserted {
			existing, loadErr := loadRequestBySlug(ctx, tx, prepared.Slug)
			if loadErr != nil {
				return loadErr
			}
			if existing.PayloadHash != prepared.PayloadHash {
				return fmt.Errorf("%w: slug %q was requested with a different payload", ErrPayloadConflict, prepared.Slug)
			}
			result = existing.result()
			return nil
		}

		iamStore, storeErr := platformiam.NewPostgresStore(tx)
		if storeErr != nil {
			return fmt.Errorf("create IAM transaction store: %w", storeErr)
		}
		_, createErr := iamStore.CreateOrganization(ctx, platformiam.Organization{
			ID:       request.OrganizationID,
			Provider: ProviderClerk,
			Name:     request.Name,
			Slug:     request.Slug,
			Status:   platformiam.OrganizationProvisioning,
		})
		if createErr != nil {
			if errors.Is(createErr, platformiam.ErrConflict) || isUniqueViolation(createErr) {
				return fmt.Errorf("%w: slug %q already belongs to another organization", ErrSlugConflict, request.Slug)
			}
			return fmt.Errorf("create provisioning organization: %w", createErr)
		}

		payload, payloadErr := request.eventPayload()
		if payloadErr != nil {
			return payloadErr
		}
		if _, appendErr := service.appender.Append(ctx, tx, platformoutbox.MessageInput{
			ID:             request.OutboxMessageID,
			IdempotencyKey: "iam.provision-org:" + request.Slug,
			Topic:          ProvisionOrganizationTopic,
			Payload:        payload,
			Headers: map[string]string{
				"content-type": "application/json",
			},
		}); appendErr != nil {
			return fmt.Errorf("append organization provisioning event: %w", appendErr)
		}

		result = request.result()
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

type preparedInput struct {
	Name        string
	Slug        string
	OwnerEmail  string
	PayloadHash string
}

func prepare(input Input) (preparedInput, error) {
	name := strings.TrimSpace(input.Name)
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	ownerEmail := strings.ToLower(strings.TrimSpace(input.OwnerEmail))

	if name == "" || len(name) > 200 {
		return preparedInput{}, fmt.Errorf("%w: organization name must contain 1 to 200 characters", ErrInvalidInput)
	}
	if len(slug) > 63 || !slugPattern.MatchString(slug) {
		return preparedInput{}, fmt.Errorf("%w: slug must be a lowercase DNS label up to 63 characters", ErrInvalidInput)
	}
	address, err := mail.ParseAddress(ownerEmail)
	if err != nil || !strings.EqualFold(address.Address, ownerEmail) {
		return preparedInput{}, fmt.Errorf("%w: owner email is invalid", ErrInvalidInput)
	}

	canonical, err := json.Marshal(struct {
		Provider   string `json:"provider"`
		Name       string `json:"name"`
		Slug       string `json:"slug"`
		OwnerEmail string `json:"owner_email"`
	}{
		Provider:   ProviderClerk,
		Name:       name,
		Slug:       slug,
		OwnerEmail: ownerEmail,
	})
	if err != nil {
		return preparedInput{}, fmt.Errorf("encode provisioning payload: %w", err)
	}
	digest := sha256.Sum256(canonical)

	return preparedInput{
		Name:        name,
		Slug:        slug,
		OwnerEmail:  ownerEmail,
		PayloadHash: hex.EncodeToString(digest[:]),
	}, nil
}

type provisioningRequest struct {
	RequestID       string
	OrganizationID  string
	Provider        string
	Name            string
	Slug            string
	OwnerEmail      string
	PayloadHash     string
	OutboxMessageID string
	Status          string
}

func insertRequest(ctx context.Context, tx pgx.Tx, request provisioningRequest) (bool, error) {
	var requestID string
	err := tx.QueryRow(ctx, `
		INSERT INTO app.organization_provisioning_requests (
			id,
			organization_id,
			provider,
			slug,
			organization_name,
			owner_email_normalized,
			payload_sha256,
			outbox_message_id,
			status
		) VALUES (
			$1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9
		)
		ON CONFLICT (slug) DO NOTHING
		RETURNING id::text
	`,
		request.RequestID,
		request.OrganizationID,
		request.Provider,
		request.Slug,
		request.Name,
		request.OwnerEmail,
		request.PayloadHash,
		request.OutboxMessageID,
		request.Status,
	).Scan(&requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("insert organization provisioning request: %w", err)
	}
	return true, nil
}

func loadRequestBySlug(ctx context.Context, tx pgx.Tx, slug string) (provisioningRequest, error) {
	request := provisioningRequest{}
	err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			organization_id::text,
			provider,
			organization_name,
			slug,
			owner_email_normalized,
			payload_sha256,
			outbox_message_id,
			status
		FROM app.organization_provisioning_requests
		WHERE slug = $1
	`, slug).Scan(
		&request.RequestID,
		&request.OrganizationID,
		&request.Provider,
		&request.Name,
		&request.Slug,
		&request.OwnerEmail,
		&request.PayloadHash,
		&request.OutboxMessageID,
		&request.Status,
	)
	if err != nil {
		return provisioningRequest{}, fmt.Errorf("load organization provisioning request: %w", err)
	}
	return request, nil
}

func (request provisioningRequest) eventPayload() ([]byte, error) {
	payload, err := json.Marshal(struct {
		SchemaVersion  int    `json:"schema_version"`
		RequestID      string `json:"request_id"`
		Provider       string `json:"provider"`
		OrganizationID string `json:"organization_id"`
		Name           string `json:"name"`
		Slug           string `json:"slug"`
		OwnerEmail     string `json:"owner_email"`
		OwnerRole      string `json:"owner_role"`
		ProviderRole   string `json:"provider_role"`
	}{
		SchemaVersion:  1,
		RequestID:      request.RequestID,
		Provider:       request.Provider,
		OrganizationID: request.OrganizationID,
		Name:           request.Name,
		Slug:           request.Slug,
		OwnerEmail:     request.OwnerEmail,
		OwnerRole:      "owner",
		ProviderRole:   "org:admin",
	})
	if err != nil {
		return nil, fmt.Errorf("encode organization provisioning event: %w", err)
	}
	return payload, nil
}

func (request provisioningRequest) result() Result {
	return Result{
		RequestID:       request.RequestID,
		OrganizationID:  request.OrganizationID,
		Name:            request.Name,
		Slug:            request.Slug,
		OwnerEmail:      request.OwnerEmail,
		Status:          request.Status,
		OutboxMessageID: request.OutboxMessageID,
	}
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
