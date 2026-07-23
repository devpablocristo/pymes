package iamsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformiam "github.com/devpablocristo/platform/iam/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type action uint8

const (
	actionOrganizationUpdate action = iota + 1
	actionMembershipEnsure
	actionMembershipRemove
	actionOwnershipTransfer
	actionInvitationEnsure
	actionInvitationResend
	actionInvitationRevoke
	actionNoop
)

type snapshot struct {
	MessageID      string
	CommandCreated time.Time
	Event          Event
	Action         action
	Organization   platformiam.Organization
	Actor          platformiam.Membership
	ActorUser      platformiam.User
	Membership     platformiam.Membership
	MemberUser     platformiam.User
	Invitation     platformiam.Invitation
	ProviderRole   string
	ApplyRole      bool
	ApplyOwnership bool
}

type providerResult struct {
	Organization    *clerk.Organization
	Membership      *clerk.OrganizationMembership
	MembershipGone  bool
	Invitation      *clerk.Invitation
	InvitationGone  bool
	ProviderSkipped bool
}

type commandRepository interface {
	Prepare(context.Context, platformoutbox.Publication, Event) (snapshot, error)
	Finalize(context.Context, platformoutbox.Publication, snapshot, providerResult) error
}

type PostgresRepository struct {
	uow *postgres.UnitOfWork[pgx.Tx]
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("iam sync: PostgreSQL pool is required")
	}
	uow, err := postgres.NewPgxUnitOfWork(pool)
	if err != nil {
		return nil, fmt.Errorf("iam sync: create unit of work: %w", err)
	}
	return &PostgresRepository{uow: uow}, nil
}

func (repository *PostgresRepository) Prepare(
	ctx context.Context,
	publication platformoutbox.Publication,
	event Event,
) (snapshot, error) {
	if repository == nil || repository.uow == nil {
		return snapshot{}, errors.New("iam sync: repository is not configured")
	}
	var prepared snapshot
	err := repository.uow.WithinTx(ctx, func(txContext context.Context) error {
		tx, err := postgres.Tx[pgx.Tx](txContext)
		if err != nil {
			return fmt.Errorf("iam sync: resolve prepare transaction: %w", err)
		}
		if err := applyTenantContext(txContext, tx, event); err != nil {
			return err
		}
		createdAt, err := validateDurableMessage(txContext, tx, publication, event, false)
		if err != nil {
			return err
		}
		store, err := platformiam.NewPostgresStore(tx)
		if err != nil {
			return fmt.Errorf("iam sync: create IAM store: %w", err)
		}
		prepared = snapshot{
			MessageID:      publication.MessageID,
			CommandCreated: createdAt,
			Event:          event,
		}
		prepared.Organization, err = store.GetOrganization(txContext, event.OrganizationID)
		if err != nil {
			return fmt.Errorf("iam sync: load organization: %w", err)
		}
		if err := validateOrganization(prepared.Organization, event); err != nil {
			return err
		}
		prepared.Actor, err = store.GetMembership(txContext, event.ActorMembershipID)
		if err != nil {
			return fmt.Errorf("iam sync: load actor membership: %w", err)
		}
		if prepared.Actor.OrganizationID != event.OrganizationID ||
			prepared.Actor.UserID != event.ActorUserID {
			return fmt.Errorf("%w: actor does not match command", ErrDurableConflict)
		}
		prepared.ActorUser, err = store.GetUser(txContext, event.ActorUserID)
		if err != nil {
			return fmt.Errorf("iam sync: load actor user: %w", err)
		}
		if prepared.ActorUser.Provider != providerClerk ||
			strings.TrimSpace(prepared.ActorUser.ExternalID) == "" {
			return fmt.Errorf("%w: actor has no Clerk identity", ErrDurableConflict)
		}

		switch publication.Topic {
		case OrganizationUpdateTopic:
			prepared.Action = actionOrganizationUpdate
		case MemberRoleChangeTopic, MemberRemoveTopic, OwnershipTransferTopic:
			if err := prepareMembership(txContext, store, publication.Topic, &prepared); err != nil {
				return err
			}
		case InvitationCreateTopic, InvitationResendTopic, InvitationRevokeTopic:
			if err := prepareInvitation(txContext, store, publication.Topic, &prepared); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %q", ErrUnsupportedTopic, publication.Topic)
		}
		return nil
	})
	if err != nil {
		return snapshot{}, err
	}
	return prepared, nil
}

func prepareMembership(
	ctx context.Context,
	store *platformiam.PostgresStore,
	topic string,
	prepared *snapshot,
) error {
	event := prepared.Event
	target, err := store.GetMembership(ctx, event.ResourceID)
	if err != nil {
		return fmt.Errorf("iam sync: load target membership: %w", err)
	}
	if target.OrganizationID != event.OrganizationID || target.Provider != providerClerk {
		return fmt.Errorf("%w: target membership does not match command", ErrDurableConflict)
	}
	user, err := store.GetUser(ctx, target.UserID)
	if err != nil {
		return fmt.Errorf("iam sync: load target user: %w", err)
	}
	if user.Provider != providerClerk || strings.TrimSpace(user.ExternalID) == "" {
		return fmt.Errorf("%w: target member has no Clerk identity", ErrDurableConflict)
	}
	prepared.Membership = target
	prepared.MemberUser = user

	switch topic {
	case MemberRemoveTopic:
		if target.Status != platformiam.MembershipRemoved {
			return fmt.Errorf("%w: membership removal is not blocked locally", ErrDurableConflict)
		}
		prepared.Action = actionMembershipRemove
	case MemberRoleChangeTopic:
		if target.Status == platformiam.MembershipRemoved {
			prepared.Action = actionMembershipRemove
			return nil
		}
		if target.Status != platformiam.MembershipActive {
			return fmt.Errorf("%w: target membership is not active", ErrDurableConflict)
		}
		desiredRole := target.Role
		if !event.AppliedLocally && target.Role == event.PreviousRole {
			desiredRole = event.Role
			prepared.ApplyRole = true
		}
		prepared.ProviderRole, err = providerRole(desiredRole)
		if err != nil {
			return err
		}
		prepared.Action = actionMembershipEnsure
	case OwnershipTransferTopic:
		if prepared.Actor.Status == platformiam.MembershipActive &&
			prepared.Actor.Role == "admin" &&
			target.Status == platformiam.MembershipActive &&
			target.Role == "owner" {
			prepared.ProviderRole = clerkAdministratorRole
			prepared.Action = actionOwnershipTransfer
			return nil
		}
		if prepared.Actor.Status != platformiam.MembershipActive ||
			prepared.Actor.Role != "owner" ||
			target.Status != platformiam.MembershipActive ||
			target.Role != event.PreviousRole {
			return fmt.Errorf("%w: ownership transfer no longer matches local authority", ErrDurableConflict)
		}
		prepared.ProviderRole = clerkAdministratorRole
		prepared.ApplyOwnership = true
		prepared.Action = actionOwnershipTransfer
	}
	return nil
}

func prepareInvitation(
	ctx context.Context,
	store *platformiam.PostgresStore,
	topic string,
	prepared *snapshot,
) error {
	invitation, err := store.GetInvitation(ctx, prepared.Event.ResourceID)
	if err != nil {
		return fmt.Errorf("iam sync: load invitation: %w", err)
	}
	if invitation.OrganizationID != prepared.Event.OrganizationID ||
		invitation.Provider != providerClerk ||
		invitation.Email != prepared.Event.Email ||
		invitation.Role != prepared.Event.Role {
		return fmt.Errorf("%w: invitation does not match command", ErrDurableConflict)
	}
	prepared.Invitation = invitation

	switch invitation.Status {
	case platformiam.InvitationAccepted:
		prepared.Action = actionNoop
		return nil
	case platformiam.InvitationRevoked, platformiam.InvitationExpired:
		prepared.Action = actionInvitationRevoke
		return nil
	case platformiam.InvitationPending:
	default:
		return fmt.Errorf("%w: unsupported invitation status %q", ErrDurableConflict, invitation.Status)
	}

	switch topic {
	case InvitationCreateTopic:
		prepared.Action = actionInvitationEnsure
	case InvitationResendTopic:
		prepared.Action = actionInvitationResend
	case InvitationRevokeTopic:
		return fmt.Errorf("%w: revoked invitation is unexpectedly pending", ErrStateChanged)
	}
	return nil
}

func (repository *PostgresRepository) Finalize(
	ctx context.Context,
	publication platformoutbox.Publication,
	prepared snapshot,
	result providerResult,
) error {
	if repository == nil || repository.uow == nil {
		return errors.New("iam sync: repository is not configured")
	}
	return repository.uow.WithinTx(ctx, func(txContext context.Context) error {
		tx, err := postgres.Tx[pgx.Tx](txContext)
		if err != nil {
			return fmt.Errorf("iam sync: resolve finalization transaction: %w", err)
		}
		if err := applyTenantContext(txContext, tx, prepared.Event); err != nil {
			return err
		}
		if _, err := validateDurableMessage(txContext, tx, publication, prepared.Event, true); err != nil {
			return err
		}
		store, err := platformiam.NewPostgresStore(tx)
		if err != nil {
			return fmt.Errorf("iam sync: create finalization store: %w", err)
		}

		switch prepared.Action {
		case actionOrganizationUpdate:
			return finalizeOrganization(txContext, store, prepared, result)
		case actionMembershipEnsure:
			return finalizeMembership(txContext, tx, store, prepared, result)
		case actionMembershipRemove:
			return finalizeRemoval(txContext, store, prepared, result)
		case actionOwnershipTransfer:
			return finalizeOwnership(txContext, tx, store, prepared, result)
		case actionInvitationEnsure, actionInvitationResend:
			return finalizeInvitation(txContext, store, prepared, result)
		case actionInvitationRevoke:
			return finalizeInvitationRevocation(txContext, store, prepared, result)
		case actionNoop:
			if !result.ProviderSkipped {
				return fmt.Errorf("%w: no-op command performed a provider effect", ErrDurableConflict)
			}
			return nil
		default:
			return fmt.Errorf("%w: unsupported action %d", ErrDurableConflict, prepared.Action)
		}
	})
}

func finalizeOrganization(
	ctx context.Context,
	store *platformiam.PostgresStore,
	prepared snapshot,
	result providerResult,
) error {
	current, err := store.GetOrganization(ctx, prepared.Organization.ID)
	if err != nil {
		return fmt.Errorf("iam sync: reload organization: %w", err)
	}
	if !sameOrganizationIntent(current, prepared.Organization) {
		return ErrStateChanged
	}
	if result.Organization == nil ||
		result.Organization.ID != current.ExternalID ||
		strings.TrimSpace(result.Organization.Name) != current.Name {
		return fmt.Errorf("%w: Clerk organization does not match local state", ErrDurableConflict)
	}
	return nil
}

func finalizeMembership(
	ctx context.Context,
	tx pgx.Tx,
	store *platformiam.PostgresStore,
	prepared snapshot,
	result providerResult,
) error {
	current, err := store.GetMembership(ctx, prepared.Membership.ID)
	if err != nil {
		return fmt.Errorf("iam sync: reload membership: %w", err)
	}
	if result.Membership == nil || result.MembershipGone ||
		result.Membership.OrganizationID != prepared.Organization.ExternalID ||
		result.Membership.User.ID != prepared.MemberUser.ExternalID ||
		result.Membership.Role != prepared.ProviderRole {
		return fmt.Errorf("%w: Clerk membership does not match requested role", ErrDurableConflict)
	}
	if prepared.ApplyRole {
		if current.Status == platformiam.MembershipActive && current.Role == prepared.Event.Role {
			return attachMembershipExternalID(ctx, tx, current, result.Membership.ID)
		}
		if current.Status != platformiam.MembershipActive ||
			current.Role != prepared.Event.PreviousRole {
			return ErrStateChanged
		}
		command, err := tx.Exec(ctx, `
			UPDATE iam.memberships
			SET role = $1,
			    external_id = COALESCE(NULLIF(external_id, ''), NULLIF($2, '')),
			    updated_at = now()
			WHERE id = $3::uuid
			  AND status = 'active'
			  AND role = $4
		`, prepared.Event.Role, result.Membership.ID, current.ID, prepared.Event.PreviousRole)
		if err != nil {
			return fmt.Errorf("iam sync: apply elevated local role: %w", err)
		}
		if command.RowsAffected() != 1 {
			return ErrStateChanged
		}
		return nil
	}
	if !sameMembershipIntent(current, prepared.Membership) {
		return ErrStateChanged
	}
	return attachMembershipExternalID(ctx, tx, current, result.Membership.ID)
}

func attachMembershipExternalID(
	ctx context.Context,
	tx pgx.Tx,
	membership platformiam.Membership,
	externalID string,
) error {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return fmt.Errorf("%w: Clerk membership ID is empty", ErrDurableConflict)
	}
	if membership.ExternalID != "" && membership.ExternalID != externalID {
		return fmt.Errorf("%w: local membership is bound to another Clerk membership", ErrDurableConflict)
	}
	if membership.ExternalID == externalID {
		return nil
	}
	command, err := tx.Exec(ctx, `
		UPDATE iam.memberships
		SET external_id = $1, updated_at = now()
		WHERE id = $2::uuid
		  AND (external_id IS NULL OR external_id = '' OR external_id = $1)
	`, externalID, membership.ID)
	if err != nil {
		return fmt.Errorf("iam sync: attach Clerk membership ID: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrStateChanged
	}
	return nil
}

func finalizeRemoval(
	ctx context.Context,
	store *platformiam.PostgresStore,
	prepared snapshot,
	result providerResult,
) error {
	current, err := store.GetMembership(ctx, prepared.Membership.ID)
	if err != nil {
		return fmt.Errorf("iam sync: reload removed membership: %w", err)
	}
	if current.Status != platformiam.MembershipRemoved {
		return fmt.Errorf("%w: local membership removal was reverted", ErrDurableConflict)
	}
	if !result.MembershipGone {
		return fmt.Errorf("%w: Clerk membership still exists", ErrDurableConflict)
	}
	return nil
}

func finalizeOwnership(
	ctx context.Context,
	tx pgx.Tx,
	store *platformiam.PostgresStore,
	prepared snapshot,
	result providerResult,
) error {
	actor, err := store.GetMembership(ctx, prepared.Actor.ID)
	if err != nil {
		return fmt.Errorf("iam sync: reload ownership actor: %w", err)
	}
	target, err := store.GetMembership(ctx, prepared.Membership.ID)
	if err != nil {
		return fmt.Errorf("iam sync: reload ownership target: %w", err)
	}
	if result.Membership == nil ||
		result.Membership.Role != clerkAdministratorRole ||
		result.Membership.User.ID != prepared.MemberUser.ExternalID {
		return fmt.Errorf("%w: Clerk ownership target is not an administrator", ErrDurableConflict)
	}

	if actor.Status == platformiam.MembershipActive &&
		actor.Role == "admin" &&
		target.Status == platformiam.MembershipActive &&
		target.Role == "owner" {
		return attachMembershipExternalID(ctx, tx, target, result.Membership.ID)
	}
	if !prepared.ApplyOwnership ||
		actor.Status != platformiam.MembershipActive ||
		actor.Role != "owner" ||
		target.Status != platformiam.MembershipActive ||
		target.Role != prepared.Event.PreviousRole {
		return ErrStateChanged
	}
	if err := attachMembershipExternalID(ctx, tx, target, result.Membership.ID); err != nil {
		return err
	}
	demoted, err := tx.Exec(ctx, `
		UPDATE iam.memberships
		SET role = 'admin', updated_at = now()
		WHERE id = $1::uuid AND role = 'owner' AND status = 'active'
	`, actor.ID)
	if err != nil {
		return fmt.Errorf("iam sync: demote previous owner: %w", err)
	}
	if demoted.RowsAffected() != 1 {
		return ErrStateChanged
	}
	promoted, err := tx.Exec(ctx, `
		UPDATE iam.memberships
		SET role = 'owner', updated_at = now()
		WHERE id = $1::uuid AND role = $2 AND status = 'active'
	`, target.ID, prepared.Event.PreviousRole)
	if err != nil {
		return fmt.Errorf("iam sync: promote new owner: %w", err)
	}
	if promoted.RowsAffected() != 1 {
		return ErrStateChanged
	}
	return nil
}

func finalizeInvitation(
	ctx context.Context,
	store *platformiam.PostgresStore,
	prepared snapshot,
	result providerResult,
) error {
	current, err := store.GetInvitation(ctx, prepared.Invitation.ID)
	if err != nil {
		return fmt.Errorf("iam sync: reload invitation: %w", err)
	}
	if !sameInvitationIntent(current, prepared.Invitation) {
		return ErrStateChanged
	}
	if result.Invitation == nil ||
		result.InvitationGone ||
		result.Invitation.OrganizationID != prepared.Organization.ExternalID ||
		result.Invitation.Email != current.Email ||
		result.Invitation.Role != mustProviderRole(current.Role) ||
		result.Invitation.Status != "pending" ||
		strings.TrimSpace(result.Invitation.ID) == "" {
		return fmt.Errorf("%w: Clerk invitation does not match local invitation", ErrDurableConflict)
	}
	if current.ExternalID != "" &&
		prepared.Action == actionInvitationEnsure &&
		current.ExternalID != result.Invitation.ID {
		return fmt.Errorf("%w: local invitation is bound to another Clerk invitation", ErrDurableConflict)
	}
	current.ExternalID = result.Invitation.ID
	if result.Invitation.ExpiresAt != nil && !result.Invitation.ExpiresAt.IsZero() {
		current.ExpiresAt = result.Invitation.ExpiresAt.UTC()
	}
	if _, err := store.UpdateInvitation(ctx, current); err != nil {
		return fmt.Errorf("iam sync: attach Clerk invitation: %w", err)
	}
	return nil
}

func finalizeInvitationRevocation(
	ctx context.Context,
	store *platformiam.PostgresStore,
	prepared snapshot,
	result providerResult,
) error {
	current, err := store.GetInvitation(ctx, prepared.Invitation.ID)
	if err != nil {
		return fmt.Errorf("iam sync: reload revoked invitation: %w", err)
	}
	switch current.Status {
	case platformiam.InvitationRevoked, platformiam.InvitationExpired, platformiam.InvitationAccepted:
	default:
		return fmt.Errorf("%w: local invitation revocation was reverted", ErrDurableConflict)
	}
	if !result.InvitationGone {
		return fmt.Errorf("%w: Clerk invitation is still pending", ErrDurableConflict)
	}
	return nil
}

func applyTenantContext(ctx context.Context, tx pgx.Tx, event Event) error {
	if _, err := tx.Exec(ctx, `
		SELECT
			set_config('app.org_id', $1, true),
			set_config('app.user_id', $2, true)
	`, event.OrganizationID, event.ActorUserID); err != nil {
		return fmt.Errorf("iam sync: apply transaction context: %w", err)
	}
	return nil
}

func validateDurableMessage(
	ctx context.Context,
	tx pgx.Tx,
	publication platformoutbox.Publication,
	event Event,
	lock bool,
) (time.Time, error) {
	lockClause := "FOR SHARE"
	if lock {
		lockClause = "FOR UPDATE"
	}
	var (
		idempotencyKey string
		topic          string
		payload        []byte
		createdAt      time.Time
		publishedAt    *time.Time
		failedAt       *time.Time
	)
	err := tx.QueryRow(ctx, `
		SELECT idempotency_key, topic, payload, created_at, published_at, failed_at
		FROM public.platform_outbox_messages
		WHERE id = $1
	`+lockClause, publication.MessageID).Scan(
		&idempotencyKey,
		&topic,
		&payload,
		&createdAt,
		&publishedAt,
		&failedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, fmt.Errorf("%w: outbox message does not exist", ErrDurableConflict)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("iam sync: load durable outbox message: %w", err)
	}
	if idempotencyKey != publication.IdempotencyKey ||
		topic != publication.Topic ||
		!bytes.Equal(payload, publication.Payload) ||
		publishedAt != nil ||
		failedAt != nil ||
		!validIdempotencyKey(idempotencyKey, event) {
		return time.Time{}, fmt.Errorf("%w: publication does not match durable outbox message", ErrDurableConflict)
	}
	return createdAt.UTC(), nil
}

func validateOrganization(organization platformiam.Organization, event Event) error {
	if organization.ID != event.OrganizationID ||
		organization.Provider != providerClerk ||
		organization.ExternalID != event.ExternalOrganizationID ||
		organization.Status != platformiam.OrganizationActive {
		return fmt.Errorf("%w: organization does not match verified command context", ErrDurableConflict)
	}
	return nil
}

func sameOrganizationIntent(left, right platformiam.Organization) bool {
	return left.ID == right.ID &&
		left.Provider == right.Provider &&
		left.ExternalID == right.ExternalID &&
		left.Name == right.Name &&
		left.Slug == right.Slug &&
		left.Status == right.Status
}

func sameMembershipIntent(left, right platformiam.Membership) bool {
	return left.ID == right.ID &&
		left.OrganizationID == right.OrganizationID &&
		left.UserID == right.UserID &&
		left.Provider == right.Provider &&
		left.ExternalID == right.ExternalID &&
		left.Role == right.Role &&
		left.Status == right.Status
}

func sameInvitationIntent(left, right platformiam.Invitation) bool {
	return left.ID == right.ID &&
		left.OrganizationID == right.OrganizationID &&
		left.Provider == right.Provider &&
		left.ExternalID == right.ExternalID &&
		left.Email == right.Email &&
		left.Role == right.Role &&
		left.Status == right.Status &&
		left.ExpiresAt.Equal(right.ExpiresAt)
}

func mustProviderRole(local string) string {
	role, _ := providerRole(local)
	return role
}
