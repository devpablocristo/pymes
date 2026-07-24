// Package administration implements the product-wide owner use cases. It is
// intentionally independent from Clerk: provider reconciliation is emitted
// through the existing IAM outbox port.
package administration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	platformlifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/provisioning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrForbidden      = errors.New("administration: global owner required")
	ErrNotFound       = errors.New("administration: resource not found")
	ErrConflict       = errors.New("administration: state conflict")
	ErrLastOwner      = errors.New("administration: last active owner")
	ErrInvalidInput   = errors.New("administration: invalid input")
	ErrProviderBacked = errors.New("administration: identity provider is unavailable")
)

const (
	organizationUpdateTopic = "iam.organization.update.requested.v1"
	invitationCreateTopic   = "iam.invitation.create.requested.v1"
)

type Tenant struct {
	ID             string
	ExternalID     string
	Name           string
	Slug           string
	Status         string
	LifecycleState string
	ArchivedAt     *time.Time
	TrashedAt      *time.Time
	PurgeAfter     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Membership struct {
	ID         string
	TenantID   string
	TenantName string
	Role       string
	Status     string
}

type User struct {
	ID             string
	ExternalID     string
	Email          string
	EmailVerified  bool
	DisplayName    string
	AvatarURL      string
	Status         string
	ProductRole    string
	LifecycleState string
	ArchivedAt     *time.Time
	TrashedAt      *time.Time
	PurgeAfter     *time.Time
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Memberships    []Membership
}

type Invitation struct {
	ID        string
	Email     string
	Role      string
	Status    string
	ExpiresAt time.Time
}

type TenantFilter struct {
	Status         string
	LifecycleState string
	Query          string
}

type UserFilter struct {
	Status         string
	LifecycleState string
	Query          string
}

type CreateTenantInput struct {
	Name       string
	Slug       string
	AdminEmail string
}

type UpdateTenantInput struct {
	Name *string
	Slug *string
}

type CreateUserInput struct {
	Email    string
	TenantID string
	Role     string
}

type UpdateUserInput struct {
	DisplayName *string
	Email       *string
	ProductRole *string
	Version     int64
}

type commandEvent struct {
	SchemaVersion          int        `json:"schema_version"`
	Operation              string     `json:"operation"`
	OrganizationID         string     `json:"organization_id"`
	ExternalOrganizationID string     `json:"external_organization_id,omitempty"`
	ActorUserID            string     `json:"actor_user_id"`
	ActorMembershipID      string     `json:"actor_membership_id"`
	ResourceID             string     `json:"resource_id"`
	ExternalResourceID     string     `json:"external_resource_id,omitempty"`
	Name                   string     `json:"name,omitempty"`
	Slug                   string     `json:"slug,omitempty"`
	Email                  string     `json:"email,omitempty"`
	Role                   string     `json:"role,omitempty"`
	ExpiresAt              *time.Time `json:"expires_at,omitempty"`
	AppliedLocally         bool       `json:"applied_locally"`
}

type Service struct {
	pool        *pgxpool.Pool
	provisioner *provisioning.Service
	outbox      *platformoutbox.Store
	provider    Provider
	lifecycle   *platformlifecycle.Service
}

type Provider interface {
	DeleteOrganization(context.Context, string) error
	DeleteUser(context.Context, string) error
	UpdateUserEmail(context.Context, string, string) error
}

func NewService(pool *pgxpool.Pool, providers ...Provider) (*Service, error) {
	if pool == nil {
		return nil, errors.New("administration: PostgreSQL pool is required")
	}
	provisioner, err := provisioning.NewService(pool)
	if err != nil {
		return nil, err
	}
	outbox, err := platformoutbox.NewStore(pool, platformoutbox.StoreConfig{
		Table:              pgx.Identifier{platformoutbox.DefaultTableName},
		DefaultMaxAttempts: 12,
	})
	if err != nil {
		return nil, err
	}
	service := &Service{pool: pool, provisioner: provisioner, outbox: outbox}
	if len(providers) > 0 {
		service.provider = providers[0]
	}
	service.lifecycle, err = newLifecycleService(pool, service.provider)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func (service *Service) IsOwner(ctx context.Context, subject string) (bool, error) {
	if service == nil || service.pool == nil || strings.TrimSpace(subject) == "" {
		return false, ErrForbidden
	}
	var owner bool
	err := service.pool.QueryRow(
		ctx,
		`SELECT app.is_global_owner('clerk', $1)`,
		strings.TrimSpace(subject),
	).Scan(&owner)
	return owner, err
}

func (service *Service) ListTenants(
	ctx context.Context,
	subject string,
	filter TenantFilter,
) ([]Tenant, error) {
	var tenants []Tenant
	err := service.withOwnerTx(ctx, subject, func(ctx context.Context, tx pgx.Tx, _ string) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, coalesce(external_id, ''), name, coalesce(slug, ''),
			       status,
			       CASE
			           WHEN trashed_at IS NOT NULL THEN 'trashed'
			           WHEN archived_at IS NOT NULL THEN 'archived'
			           ELSE 'active'
			       END,
			       archived_at, trashed_at, purge_after, created_at, updated_at
			FROM iam.organizations
			WHERE ($1 = '' OR status = $1)
			  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR slug ILIKE '%' || $2 || '%')
			  AND ($3 = '' OR CASE
			           WHEN trashed_at IS NOT NULL THEN 'trashed'
			           WHEN archived_at IS NOT NULL THEN 'archived'
			           ELSE 'active'
			      END = $3)
			ORDER BY lower(name), id
		`, strings.TrimSpace(filter.Status), strings.TrimSpace(filter.Query),
			strings.TrimSpace(filter.LifecycleState))
		if err != nil {
			return fmt.Errorf("list tenants: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var tenant Tenant
			if err := rows.Scan(
				&tenant.ID,
				&tenant.ExternalID,
				&tenant.Name,
				&tenant.Slug,
				&tenant.Status,
				&tenant.LifecycleState,
				&tenant.ArchivedAt,
				&tenant.TrashedAt,
				&tenant.PurgeAfter,
				&tenant.CreatedAt,
				&tenant.UpdatedAt,
			); err != nil {
				return fmt.Errorf("scan tenant: %w", err)
			}
			tenants = append(tenants, tenant)
		}
		return rows.Err()
	})
	return tenants, err
}

func (service *Service) GetTenant(ctx context.Context, subject, tenantID string) (Tenant, error) {
	var tenant Tenant
	err := service.withOwnerTx(ctx, subject, func(ctx context.Context, tx pgx.Tx, _ string) error {
		return scanTenant(tx.QueryRow(ctx, `
			SELECT id::text, coalesce(external_id, ''), name, coalesce(slug, ''),
			       status,
			       CASE
			           WHEN trashed_at IS NOT NULL THEN 'trashed'
			           WHEN archived_at IS NOT NULL THEN 'archived'
			           ELSE 'active'
			       END,
			       archived_at, trashed_at, purge_after, created_at, updated_at
			FROM iam.organizations WHERE id = $1::uuid
		`, tenantID), &tenant)
	})
	return tenant, err
}

func (service *Service) CreateTenant(
	ctx context.Context,
	subject string,
	input CreateTenantInput,
) (Tenant, error) {
	if ok, err := service.IsOwner(ctx, subject); err != nil || !ok {
		if err != nil {
			return Tenant{}, err
		}
		return Tenant{}, ErrForbidden
	}
	result, err := service.provisioner.Provision(ctx, provisioning.Input{
		Name:       input.Name,
		Slug:       input.Slug,
		OwnerEmail: input.AdminEmail,
	})
	if err != nil {
		if errors.Is(err, provisioning.ErrInvalidInput) {
			return Tenant{}, ErrInvalidInput
		}
		if errors.Is(err, provisioning.ErrPayloadConflict) || errors.Is(err, provisioning.ErrSlugConflict) {
			return Tenant{}, ErrConflict
		}
		return Tenant{}, err
	}
	return service.GetTenant(ctx, subject, result.OrganizationID)
}

func (service *Service) UpdateTenant(
	ctx context.Context,
	subject, tenantID, idempotencyKey string,
	input UpdateTenantInput,
) (Tenant, error) {
	var response Tenant
	err := service.withOwnerTx(ctx, subject, func(ctx context.Context, tx pgx.Tx, actorUserID string) error {
		if err := scanTenant(tx.QueryRow(ctx, `
			SELECT id::text, coalesce(external_id, ''), name, coalesce(slug, ''),
			       status,
			       CASE
			           WHEN trashed_at IS NOT NULL THEN 'trashed'
			           WHEN archived_at IS NOT NULL THEN 'archived'
			           ELSE 'active'
			       END,
			       archived_at, trashed_at, purge_after, created_at, updated_at
			FROM iam.organizations WHERE id = $1::uuid FOR UPDATE
		`, tenantID), &response); err != nil {
			return err
		}
		if response.LifecycleState != string(platformlifecycle.StateActive) {
			return ErrConflict
		}
		if input.Name != nil {
			response.Name = strings.TrimSpace(*input.Name)
		}
		if input.Slug != nil {
			response.Slug = strings.TrimSpace(*input.Slug)
		}
		if response.Name == "" || response.Slug == "" {
			return ErrInvalidInput
		}
		tag, err := tx.Exec(ctx, `
			UPDATE iam.organizations
			SET name = $1, slug = $2, updated_at = now()
			WHERE id = $3::uuid
		`, response.Name, response.Slug, tenantID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return ErrConflict
			}
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrNotFound
		}
		event := commandEvent{
			SchemaVersion:          1,
			Operation:              "organization.update",
			OrganizationID:         response.ID,
			ExternalOrganizationID: response.ExternalID,
			ActorUserID:            actorUserID,
			ActorMembershipID:      actorUserID,
			ResourceID:             response.ID,
			Name:                   response.Name,
			Slug:                   response.Slug,
			AppliedLocally:         true,
		}
		if response.ExternalID != "" {
			if err := service.appendCommand(ctx, tx, idempotencyKey, organizationUpdateTopic, event); err != nil {
				return err
			}
		}
		return scanTenant(tx.QueryRow(ctx, `
			SELECT id::text, coalesce(external_id, ''), name, coalesce(slug, ''),
			       status,
			       CASE
			           WHEN trashed_at IS NOT NULL THEN 'trashed'
			           WHEN archived_at IS NOT NULL THEN 'archived'
			           ELSE 'active'
			       END,
			       archived_at, trashed_at, purge_after, created_at, updated_at
			FROM iam.organizations WHERE id = $1::uuid
		`, tenantID), &response)
	})
	return response, err
}

func (service *Service) ArchiveTenant(ctx context.Context, subject, tenantID, reason string) error {
	return service.executeLifecycle(ctx, subject, TenantResourceType, tenantID, "archive", reason)
}

func (service *Service) UnarchiveTenant(ctx context.Context, subject, tenantID, reason string) error {
	return service.executeLifecycle(ctx, subject, TenantResourceType, tenantID, "unarchive", reason)
}

func (service *Service) TrashTenant(ctx context.Context, subject, tenantID, reason string) error {
	return service.executeLifecycle(ctx, subject, TenantResourceType, tenantID, "trash", reason)
}

func (service *Service) RestoreTenant(ctx context.Context, subject, tenantID, reason string) error {
	return service.executeLifecycle(ctx, subject, TenantResourceType, tenantID, "restore", reason)
}

func (service *Service) PurgeTenant(ctx context.Context, subject, tenantID, reason string) error {
	return service.executeLifecycle(ctx, subject, TenantResourceType, tenantID, "purge", reason)
}

func (service *Service) ListUsers(
	ctx context.Context,
	subject string,
	filter UserFilter,
) ([]User, error) {
	var users []User
	err := service.withOwnerTx(ctx, subject, func(ctx context.Context, tx pgx.Tx, _ string) error {
		rows, err := tx.Query(ctx, `
			SELECT iam_user.id::text, iam_user.external_id, iam_user.primary_email,
			       iam_user.email_verified, iam_user.name, coalesce(iam_user.avatar_url, ''),
			       iam_user.status,
			       CASE WHEN global_role.status = 'active' THEN 'owner' ELSE 'user' END,
			       CASE
			           WHEN iam_user.trashed_at IS NOT NULL THEN 'trashed'
			           WHEN iam_user.archived_at IS NOT NULL THEN 'archived'
			           ELSE 'active'
			       END,
			       iam_user.archived_at, iam_user.trashed_at, iam_user.purge_after,
			       iam_user.xmin::text::bigint, iam_user.created_at, iam_user.updated_at
			FROM iam.users AS iam_user
			LEFT JOIN app.global_user_roles AS global_role ON global_role.user_id = iam_user.id
			WHERE ($1 = '' OR iam_user.status = $1)
			  AND ($2 = '' OR iam_user.name ILIKE '%' || $2 || '%'
			                 OR iam_user.primary_email ILIKE '%' || $2 || '%')
			  AND ($3 = '' OR CASE
			           WHEN iam_user.trashed_at IS NOT NULL THEN 'trashed'
			           WHEN iam_user.archived_at IS NOT NULL THEN 'archived'
			           ELSE 'active'
			      END = $3)
			ORDER BY lower(iam_user.primary_email), iam_user.id
		`, strings.TrimSpace(filter.Status), strings.TrimSpace(filter.Query),
			strings.TrimSpace(filter.LifecycleState))
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			user, err := scanUserRows(rows)
			if err != nil {
				return err
			}
			users = append(users, user)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for index := range users {
			memberships, err := loadMemberships(ctx, tx, users[index].ID)
			if err != nil {
				return err
			}
			users[index].Memberships = memberships
		}
		return nil
	})
	return users, err
}

func (service *Service) GetUser(ctx context.Context, subject, userID string) (User, error) {
	var response User
	err := service.withOwnerTx(ctx, subject, func(ctx context.Context, tx pgx.Tx, _ string) error {
		var err error
		response, err = loadUser(ctx, tx, userID)
		return err
	})
	return response, err
}

func (service *Service) CreateUser(
	ctx context.Context,
	subject, idempotencyKey string,
	input CreateUserInput,
) (Invitation, error) {
	var response Invitation
	err := service.withOwnerTx(ctx, subject, func(ctx context.Context, tx pgx.Tx, actorUserID string) error {
		email := strings.ToLower(strings.TrimSpace(input.Email))
		if email == "" || (input.Role != "admin" && input.Role != "member") {
			return ErrInvalidInput
		}
		var externalOrganizationID string
		err := tx.QueryRow(ctx, `
			SELECT coalesce(external_id, '')
			FROM iam.organizations
			WHERE id = $1::uuid
			  AND status = 'active'
			  AND archived_at IS NULL
			  AND trashed_at IS NULL
		`, input.TenantID).Scan(&externalOrganizationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if externalOrganizationID == "" {
			return ErrConflict
		}
		now := time.Now().UTC()
		expiresAt := now.Add(7 * 24 * time.Hour)
		response = Invitation{
			ID:        uuid.NewString(),
			Email:     email,
			Role:      input.Role,
			Status:    "pending",
			ExpiresAt: expiresAt,
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO iam.invitations (
				id, org_id, provider, email_normalized, role, status, expires_at
			) VALUES ($1::uuid, $2::uuid, 'clerk', $3, $4, 'pending', $5)
		`, response.ID, input.TenantID, email, input.Role, expiresAt)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return ErrConflict
			}
			return err
		}
		event := commandEvent{
			SchemaVersion:          1,
			Operation:              "invitation.create",
			OrganizationID:         input.TenantID,
			ExternalOrganizationID: externalOrganizationID,
			ActorUserID:            actorUserID,
			ActorMembershipID:      actorUserID,
			ResourceID:             response.ID,
			Email:                  email,
			Role:                   input.Role,
			ExpiresAt:              &expiresAt,
			AppliedLocally:         true,
		}
		return service.appendCommand(ctx, tx, idempotencyKey, invitationCreateTopic, event)
	})
	return response, err
}

func (service *Service) UpdateUser(
	ctx context.Context,
	subject, userID string,
	input UpdateUserInput,
) (User, error) {
	var response User
	err := service.withOwnerTx(ctx, subject, func(ctx context.Context, tx pgx.Tx, actorUserID string) error {
		current, err := loadUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		if current.LifecycleState != string(platformlifecycle.StateActive) {
			return ErrConflict
		}
		if current.Version != input.Version {
			return ErrConflict
		}
		name := current.DisplayName
		email := current.Email
		if input.DisplayName != nil {
			name = strings.TrimSpace(*input.DisplayName)
		}
		if input.Email != nil {
			email = strings.ToLower(strings.TrimSpace(*input.Email))
		}
		if name == "" || email == "" {
			return ErrInvalidInput
		}
		if email != current.Email && current.ExternalID != "" {
			if service.provider == nil {
				return ErrProviderBacked
			}
			if err := service.provider.UpdateUserEmail(ctx, current.ExternalID, email); err != nil {
				return fmt.Errorf("update provider user email: %w", err)
			}
		}
		if input.ProductRole != nil {
			switch *input.ProductRole {
			case "owner":
				_, err = tx.Exec(ctx, `
					INSERT INTO app.global_user_roles (user_id, role, status)
					VALUES ($1::uuid, 'owner', 'active')
					ON CONFLICT (user_id) DO UPDATE
					SET status = 'active', disabled_at = NULL,
					    version = app.global_user_roles.version + 1, updated_at = now()
				`, userID)
			case "user":
				if current.ProductRole == "owner" {
					if err = ensureAnotherActiveOwner(ctx, tx); err != nil {
						return err
					}
					_, err = tx.Exec(ctx, `
						UPDATE app.global_user_roles
						SET status = 'disabled', disabled_at = now(),
						    version = version + 1, updated_at = now()
						WHERE user_id = $1::uuid
					`, userID)
				}
			default:
				return ErrInvalidInput
			}
			if err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx, `
			UPDATE iam.users
			SET name = $1, primary_email = $2, updated_at = now()
			WHERE id = $3::uuid AND xmin::text::bigint = $4
		`, name, email, userID, input.Version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		if actorUserID == userID && input.ProductRole != nil && *input.ProductRole == "user" {
			// Self-demotion is valid only when another owner remains, checked above.
		}
		response, err = loadUser(ctx, tx, userID)
		return err
	})
	return response, err
}

func (service *Service) ArchiveUser(ctx context.Context, subject, userID, reason string) error {
	return service.executeLifecycle(ctx, subject, UserResourceType, userID, "archive", reason)
}

func (service *Service) UnarchiveUser(ctx context.Context, subject, userID, reason string) error {
	return service.executeLifecycle(ctx, subject, UserResourceType, userID, "unarchive", reason)
}

func (service *Service) TrashUser(ctx context.Context, subject, userID, reason string) error {
	return service.executeLifecycle(ctx, subject, UserResourceType, userID, "trash", reason)
}

func (service *Service) RestoreUser(ctx context.Context, subject, userID, reason string) error {
	return service.executeLifecycle(ctx, subject, UserResourceType, userID, "restore", reason)
}

func (service *Service) PurgeUser(ctx context.Context, subject, userID, reason string) error {
	return service.executeLifecycle(ctx, subject, UserResourceType, userID, "purge", reason)
}

func (service *Service) executeLifecycle(
	ctx context.Context,
	subject, resourceType, resourceID, action, reason string,
) error {
	if service == nil || service.lifecycle == nil {
		return ErrConflict
	}
	id, err := uuid.Parse(strings.TrimSpace(resourceID))
	if err != nil {
		return ErrInvalidInput
	}
	lifecycleCtx := contextWithLifecycleActor(ctx, subject)
	switch action {
	case "archive":
		err = service.lifecycle.Archive(lifecycleCtx, &platformlifecycle.ArchiveRequest{
			ResourceType: resourceType, ResourceID: id, TenantID: lifecycleScope,
			Actor: strings.TrimSpace(subject), Reason: strings.TrimSpace(reason),
		})
	case "unarchive":
		err = service.lifecycle.Unarchive(lifecycleCtx, &platformlifecycle.UnarchiveRequest{
			ResourceType: resourceType, ResourceID: id, TenantID: lifecycleScope,
			Actor: strings.TrimSpace(subject), Reason: strings.TrimSpace(reason),
		})
	case "trash":
		err = service.lifecycle.Trash(lifecycleCtx, &platformlifecycle.TrashRequest{
			ResourceType: resourceType, ResourceID: id, TenantID: lifecycleScope,
			Actor: strings.TrimSpace(subject), Reason: strings.TrimSpace(reason),
		})
	case "restore":
		err = service.lifecycle.Restore(lifecycleCtx, &platformlifecycle.RestoreRequest{
			ResourceType: resourceType, ResourceID: id, TenantID: lifecycleScope,
			Actor: strings.TrimSpace(subject), Reason: strings.TrimSpace(reason),
		})
	case "purge":
		err = service.lifecycle.Purge(lifecycleCtx, &platformlifecycle.PurgeRequest{
			ResourceType: resourceType, ResourceID: id, TenantID: lifecycleScope,
			Actor: strings.TrimSpace(subject), Reason: strings.TrimSpace(reason),
			MustBeTrashed: true,
		})
	default:
		return ErrInvalidInput
	}
	if errors.Is(err, platformlifecycle.ErrMustBeTrashed) ||
		errors.Is(err, platformlifecycle.ErrArchiveNotAllowed) ||
		errors.Is(err, platformlifecycle.ErrTrashNotAllowed) ||
		errors.Is(err, platformlifecycle.ErrPurgeNotAllowed) {
		return ErrConflict
	}
	if errors.Is(err, platformlifecycle.ErrReasonRequired) {
		return ErrInvalidInput
	}
	return err
}

func (service *Service) withOwnerTx(
	ctx context.Context,
	subject string,
	fn func(context.Context, pgx.Tx, string) error,
) error {
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.actor_provider', 'clerk', true)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.actor_subject', $1, true)`, strings.TrimSpace(subject)); err != nil {
		return err
	}
	var actorUserID string
	err = tx.QueryRow(ctx, `
		SELECT iam_user.id::text
		FROM iam.users AS iam_user
		JOIN app.global_user_roles AS global_role ON global_role.user_id = iam_user.id
		WHERE iam_user.provider = 'clerk'
		  AND iam_user.external_id = $1
		  AND iam_user.status = 'active'
		  AND iam_user.archived_at IS NULL
		  AND iam_user.trashed_at IS NULL
		  AND iam_user.email_verified
		  AND global_role.role = 'owner'
		  AND global_role.status = 'active'
	`, strings.TrimSpace(subject)).Scan(&actorUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if err := fn(ctx, tx, actorUserID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (service *Service) appendCommand(
	ctx context.Context,
	tx pgx.Tx,
	idempotencyKey, topic string,
	event commandEvent,
) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = service.outbox.Append(ctx, tx, platformoutbox.MessageInput{
		IdempotencyKey: strings.Join([]string{
			"iam.admin",
			event.Operation,
			event.ResourceID,
			strings.TrimSpace(idempotencyKey),
		}, ":"),
		Topic:   topic,
		Payload: payload,
		Headers: map[string]string{
			"content-type":   "application/json",
			"schema-version": "1",
		},
	})
	return err
}

func ensureAnotherActiveOwner(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtext('app.global_user_roles:active-owner'))`,
	); err != nil {
		return err
	}
	var activeOwners int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.global_user_roles AS global_role
		JOIN iam.users AS iam_user ON iam_user.id = global_role.user_id
		WHERE global_role.status = 'active'
		  AND iam_user.status = 'active'
		  AND iam_user.archived_at IS NULL
		  AND iam_user.trashed_at IS NULL
	`).Scan(&activeOwners); err != nil {
		return err
	}
	if activeOwners <= 1 {
		return ErrLastOwner
	}
	return nil
}

func scanTenant(row pgx.Row, tenant *Tenant) error {
	err := row.Scan(
		&tenant.ID,
		&tenant.ExternalID,
		&tenant.Name,
		&tenant.Slug,
		&tenant.Status,
		&tenant.LifecycleState,
		&tenant.ArchivedAt,
		&tenant.TrashedAt,
		&tenant.PurgeAfter,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

type rowScanner interface {
	Scan(...any) error
}

func scanUserRows(row rowScanner) (User, error) {
	var user User
	err := row.Scan(
		&user.ID,
		&user.ExternalID,
		&user.Email,
		&user.EmailVerified,
		&user.DisplayName,
		&user.AvatarURL,
		&user.Status,
		&user.ProductRole,
		&user.LifecycleState,
		&user.ArchivedAt,
		&user.TrashedAt,
		&user.PurgeAfter,
		&user.Version,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func loadUser(ctx context.Context, tx pgx.Tx, userID string) (User, error) {
	user, err := scanUserRows(tx.QueryRow(ctx, `
		SELECT iam_user.id::text, iam_user.external_id, iam_user.primary_email,
		       iam_user.email_verified, iam_user.name, coalesce(iam_user.avatar_url, ''),
		       iam_user.status,
		       CASE WHEN global_role.status = 'active' THEN 'owner' ELSE 'user' END,
		       CASE
		           WHEN iam_user.trashed_at IS NOT NULL THEN 'trashed'
		           WHEN iam_user.archived_at IS NOT NULL THEN 'archived'
		           ELSE 'active'
		       END,
		       iam_user.archived_at, iam_user.trashed_at, iam_user.purge_after,
		       iam_user.xmin::text::bigint, iam_user.created_at, iam_user.updated_at
		FROM iam.users AS iam_user
		LEFT JOIN app.global_user_roles AS global_role ON global_role.user_id = iam_user.id
		WHERE iam_user.id = $1::uuid
	`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	user.Memberships, err = loadMemberships(ctx, tx, userID)
	return user, err
}

func loadMemberships(ctx context.Context, tx pgx.Tx, userID string) ([]Membership, error) {
	rows, err := tx.Query(ctx, `
		SELECT membership.id::text, organization.id::text, organization.name,
		       membership.role, membership.status
		FROM iam.memberships AS membership
		JOIN iam.organizations AS organization ON organization.id = membership.org_id
		WHERE membership.user_id = $1::uuid
		ORDER BY lower(organization.name), membership.id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	memberships := make([]Membership, 0)
	for rows.Next() {
		var membership Membership
		if err := rows.Scan(
			&membership.ID,
			&membership.TenantID,
			&membership.TenantName,
			&membership.Role,
			&membership.Status,
		); err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}
	return memberships, rows.Err()
}
