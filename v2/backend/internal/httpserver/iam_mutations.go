package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	platformiam "github.com/devpablocristo/platform/iam/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	organizationUpdateOperation = "organization.update"
	memberRoleChangeOperation   = "membership.role-change"
	memberRemoveOperation       = "membership.remove"
	ownershipTransferOperation  = "ownership.transfer"
	invitationCreateOperation   = "invitation.create"
	invitationResendOperation   = "invitation.resend"
	invitationRevokeOperation   = "invitation.revoke"

	organizationUpdateTopic = "iam.organization.update.requested.v1"
	memberRoleChangeTopic   = "iam.membership.role-change.requested.v1"
	memberRemoveTopic       = "iam.membership.remove.requested.v1"
	ownershipTransferTopic  = "iam.ownership.transfer.requested.v1"
	invitationCreateTopic   = "iam.invitation.create.requested.v1"
	invitationResendTopic   = "iam.invitation.resend.requested.v1"
	invitationRevokeTopic   = "iam.invitation.revoke.requested.v1"

	maxIAMCommandBody = 4 << 10
	invitationTTL     = 7 * 24 * time.Hour
)

var (
	errIAMRoleConflict      = errors.New("IAM role conflict")
	errIAMInvitationPending = errors.New("IAM invitation pending")
)

type iamCommandEvent struct {
	SchemaVersion          int        `json:"schema_version"`
	Operation              string     `json:"operation"`
	OrganizationID         string     `json:"organization_id"`
	ExternalOrganizationID string     `json:"external_organization_id,omitempty"`
	ActorUserID            string     `json:"actor_user_id"`
	ActorMembershipID      string     `json:"actor_membership_id"`
	ResourceID             string     `json:"resource_id"`
	ExternalResourceID     string     `json:"external_resource_id,omitempty"`
	Name                   string     `json:"name,omitempty"`
	Email                  string     `json:"email,omitempty"`
	Role                   string     `json:"role,omitempty"`
	PreviousRole           string     `json:"previous_role,omitempty"`
	ExpiresAt              *time.Time `json:"expires_at,omitempty"`
	AppliedLocally         bool       `json:"applied_locally"`
}

type storedIAMCommand struct {
	Topic string
	Event iamCommandEvent
}

type organizationRecord struct {
	ID         string
	ExternalID string
	Name       string
	Slug       string
	Status     string
}

type memberRecord struct {
	MembershipID string
	Role         string
	Status       string
	ExternalID   string
	UserID       string
	Email        string
	DisplayName  string
	AvatarURL    string
}

type invitationRecord struct {
	ID         string
	Email      string
	Role       string
	Status     string
	ExternalID string
	ExpiresAt  time.Time
}

func (h *IAMAPI) UpdateCurrentOrganization(
	w http.ResponseWriter,
	r *http.Request,
	params api.UpdateCurrentOrganizationParams,
) {
	key, ok := validateIdempotencyKey(w, params.IdempotencyKey)
	if !ok {
		return
	}
	var input api.UpdateOrganizationInput
	if !decodeIAMCommandBody(w, r, &input) {
		return
	}
	name := strings.TrimSpace(input.Name)
	if !validOrganizationName(name) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "name must contain 1 to 120 characters")
		return
	}

	var response api.Organization
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			if err := requirePermission(active, claims, productiam.PermissionOrganizationUpdate); err != nil {
				return err
			}
			commandKey := iamCommandKey(
				active.OrganizationID,
				active.UserID,
				organizationUpdateOperation,
				key,
			)
			stored, replay, err := loadIAMCommand(ctx, tx, commandKey)
			if err != nil {
				return err
			}
			if replay {
				if !stored.matches(
					organizationUpdateTopic,
					organizationUpdateOperation,
					active.OrganizationID,
					active.UserID,
				) ||
					stored.Event.Name != name {
					return errIAMRoleConflict
				}
				organization, loadErr := loadCurrentOrganizationForUpdate(ctx, tx)
				if loadErr != nil {
					return loadErr
				}
				response, loadErr = mapOrganization(organization, active.Role, api.SyncStatusQueued)
				return loadErr
			}

			organization, err := loadCurrentOrganizationForUpdate(ctx, tx)
			if err != nil {
				return err
			}
			updated, err := tx.Exec(ctx, `
				UPDATE iam.organizations
				SET name = $1, updated_at = now()
			`, name)
			if err != nil {
				return fmt.Errorf("update organization: %w", err)
			}
			if updated.RowsAffected() != 1 {
				return errIAMRoleConflict
			}
			organization.Name = name
			event := newIAMCommandEvent(
				organizationUpdateOperation,
				active,
				claims,
				organization.ID,
			)
			event.ExternalResourceID = organization.ExternalID
			event.Name = name
			event.AppliedLocally = true
			if err := h.appendIAMCommand(ctx, tx, commandKey, organizationUpdateTopic, event); err != nil {
				return err
			}
			response, err = mapOrganization(organization, active.Role, api.SyncStatusQueued)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) UpdateTeamMember(
	w http.ResponseWriter,
	r *http.Request,
	memberID api.MemberID,
	params api.UpdateTeamMemberParams,
) {
	key, ok := validateIdempotencyKey(w, params.IdempotencyKey)
	if !ok {
		return
	}
	var input api.UpdateMemberInput
	if !decodeIAMCommandBody(w, r, &input) {
		return
	}
	if !input.Role.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "role must be admin or member")
		return
	}
	desiredRole := productiam.Role(input.Role)

	var response api.Member
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			actorRole, err := effectiveActorRole(active, claims, productiam.PermissionMemberUpdate)
			if err != nil {
				return err
			}
			commandKey := iamCommandKey(
				active.OrganizationID,
				active.UserID,
				memberRoleChangeOperation,
				key,
			)
			stored, replay, err := loadIAMCommand(ctx, tx, commandKey)
			if err != nil {
				return err
			}
			target, err := loadMemberForUpdate(ctx, tx, memberID.String())
			if err != nil {
				return err
			}
			if replay {
				if !stored.matches(
					memberRoleChangeTopic,
					memberRoleChangeOperation,
					active.OrganizationID,
					active.UserID,
				) ||
					stored.Event.ResourceID != target.MembershipID ||
					stored.Event.Role != string(desiredRole) {
					return errIAMRoleConflict
				}
				response, err = mapMember(target, api.SyncStatusQueued)
				return err
			}
			if target.Status != string(platformiam.MembershipActive) {
				return errIAMRoleConflict
			}
			currentRole, err := productiam.ParseRole(target.Role)
			if err != nil || !productiam.CanChangeRole(actorRole, currentRole, desiredRole) {
				return errIAMForbidden
			}

			appliedLocally := roleRank(desiredRole) <= roleRank(currentRole)
			if appliedLocally && desiredRole != currentRole {
				updated, err := tx.Exec(ctx, `
					UPDATE iam.memberships
					SET role = $1, updated_at = now()
					WHERE id = $2::uuid
				`, desiredRole, target.MembershipID)
				if err != nil {
					return fmt.Errorf("update member role: %w", err)
				}
				if updated.RowsAffected() != 1 {
					return errIAMRoleConflict
				}
				target.Role = string(desiredRole)
			}
			event := newIAMCommandEvent(
				memberRoleChangeOperation,
				active,
				claims,
				target.MembershipID,
			)
			event.ExternalResourceID = target.ExternalID
			event.Role = string(desiredRole)
			event.PreviousRole = string(currentRole)
			event.AppliedLocally = appliedLocally
			if err := h.appendIAMCommand(ctx, tx, commandKey, memberRoleChangeTopic, event); err != nil {
				return err
			}
			response, err = mapMember(target, api.SyncStatusQueued)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) RemoveTeamMember(
	w http.ResponseWriter,
	r *http.Request,
	memberID api.MemberID,
	params api.RemoveTeamMemberParams,
) {
	key, ok := validateIdempotencyKey(w, params.IdempotencyKey)
	if !ok {
		return
	}

	var response api.Member
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			actorRole, err := effectiveActorRole(active, claims, productiam.PermissionMemberRemove)
			if err != nil {
				return err
			}
			commandKey := iamCommandKey(
				active.OrganizationID,
				active.UserID,
				memberRemoveOperation,
				key,
			)
			stored, replay, err := loadIAMCommand(ctx, tx, commandKey)
			if err != nil {
				return err
			}
			target, err := loadMemberForUpdate(ctx, tx, memberID.String())
			if err != nil {
				return err
			}
			if replay {
				if !stored.matches(
					memberRemoveTopic,
					memberRemoveOperation,
					active.OrganizationID,
					active.UserID,
				) ||
					stored.Event.ResourceID != target.MembershipID {
					return errIAMRoleConflict
				}
				response, err = mapMember(target, api.SyncStatusQueued)
				return err
			}
			targetRole, err := productiam.ParseRole(target.Role)
			if err != nil || target.Status != string(platformiam.MembershipActive) {
				return errIAMRoleConflict
			}
			if !productiam.CanRemove(actorRole, targetRole) {
				return errIAMForbidden
			}
			removed, err := tx.Exec(ctx, `
				UPDATE iam.memberships
				SET status = 'removed', removed_at = now(), updated_at = now()
				WHERE id = $1::uuid
			`, target.MembershipID)
			if err != nil {
				return fmt.Errorf("remove team member: %w", err)
			}
			if removed.RowsAffected() != 1 {
				return errIAMRoleConflict
			}
			target.Status = string(platformiam.MembershipRemoved)
			event := newIAMCommandEvent(memberRemoveOperation, active, claims, target.MembershipID)
			event.ExternalResourceID = target.ExternalID
			event.PreviousRole = string(targetRole)
			event.AppliedLocally = true
			if err := h.appendIAMCommand(ctx, tx, commandKey, memberRemoveTopic, event); err != nil {
				return err
			}
			response, err = mapMember(target, api.SyncStatusQueued)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) TransferOwnership(
	w http.ResponseWriter,
	r *http.Request,
	params api.TransferOwnershipParams,
) {
	key, ok := validateIdempotencyKey(w, params.IdempotencyKey)
	if !ok {
		return
	}
	var input api.TransferOwnershipInput
	if !decodeIAMCommandBody(w, r, &input) {
		return
	}
	if input.MemberId == uuid.Nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "member_id must be a non-zero UUID")
		return
	}

	var response api.Member
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			actorRole, err := effectiveActorRole(active, claims, productiam.PermissionOwnershipTransfer)
			if err != nil {
				return err
			}
			if actorRole != productiam.RoleOwner || input.MemberId.String() == active.MembershipID {
				return errIAMForbidden
			}
			actor, err := loadMemberForUpdate(ctx, tx, active.MembershipID)
			if err != nil {
				return err
			}
			if actor.Status != string(platformiam.MembershipActive) ||
				actor.Role != string(productiam.RoleOwner) {
				return errIAMRoleConflict
			}
			commandKey := iamCommandKey(
				active.OrganizationID,
				active.UserID,
				ownershipTransferOperation,
				key,
			)
			stored, replay, err := loadIAMCommand(ctx, tx, commandKey)
			if err != nil {
				return err
			}
			target, err := loadMemberForUpdate(ctx, tx, input.MemberId.String())
			if err != nil {
				return err
			}
			if replay {
				if !stored.matches(
					ownershipTransferTopic,
					ownershipTransferOperation,
					active.OrganizationID,
					active.UserID,
				) ||
					stored.Event.ResourceID != target.MembershipID {
					return errIAMRoleConflict
				}
				response, err = mapMember(target, api.SyncStatusQueued)
				return err
			}
			targetRole, err := productiam.ParseRole(target.Role)
			if err != nil ||
				target.Status != string(platformiam.MembershipActive) ||
				targetRole == productiam.RoleOwner {
				return errIAMRoleConflict
			}

			// external_id proves only that a provider membership once existed;
			// it does not prove the current Clerk role. The worker first
			// reconciles org:admin and only then swaps both local roles in one
			// transaction, leaving the current owner authoritative until then.
			appliedLocally := false
			event := newIAMCommandEvent(
				ownershipTransferOperation,
				active,
				claims,
				target.MembershipID,
			)
			event.ExternalResourceID = target.ExternalID
			event.Role = string(productiam.RoleOwner)
			event.PreviousRole = string(targetRole)
			event.AppliedLocally = appliedLocally
			if err := h.appendIAMCommand(ctx, tx, commandKey, ownershipTransferTopic, event); err != nil {
				return err
			}
			response, err = mapMember(target, api.SyncStatusQueued)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) CreateTeamInvitation(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTeamInvitationParams,
) {
	key, ok := validateIdempotencyKey(w, params.IdempotencyKey)
	if !ok {
		return
	}
	var input api.CreateInvitationInput
	if !decodeIAMCommandBody(w, r, &input) {
		return
	}
	email, err := normalizeInvitationEmail(string(input.Email))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	if !input.Role.Valid() {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "role must be admin or member")
		return
	}
	invitedRole := productiam.Role(input.Role)

	var response api.Invitation
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			actorRole, err := effectiveActorRole(active, claims, productiam.PermissionInvitationCreate)
			if err != nil {
				return err
			}
			if !productiam.CanInvite(actorRole, invitedRole) {
				return errIAMForbidden
			}
			commandKey := iamCommandKey(
				active.OrganizationID,
				active.UserID,
				invitationCreateOperation,
				key,
			)
			stored, replay, err := loadIAMCommand(ctx, tx, commandKey)
			if err != nil {
				return err
			}
			if replay {
				if !stored.matches(
					invitationCreateTopic,
					invitationCreateOperation,
					active.OrganizationID,
					active.UserID,
				) ||
					stored.Event.Email != email ||
					stored.Event.Role != string(invitedRole) {
					return errIAMRoleConflict
				}
				invitation, loadErr := loadInvitationForUpdate(ctx, tx, stored.Event.ResourceID)
				if loadErr != nil {
					return loadErr
				}
				response, loadErr = mapInvitation(invitation, api.SyncStatusQueued)
				return loadErr
			}

			now, err := h.commandNow()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE iam.invitations
				SET status = 'expired', updated_at = $1
				WHERE status = 'pending' AND expires_at <= $1
			`, now); err != nil {
				return fmt.Errorf("expire stale invitations: %w", err)
			}
			var pendingID string
			err = tx.QueryRow(ctx, `
				SELECT id::text
				FROM iam.invitations
				WHERE email_normalized = $1 AND status = 'pending'
				LIMIT 1
				FOR UPDATE
			`, email).Scan(&pendingID)
			if err == nil {
				return errIAMInvitationPending
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("check pending invitation: %w", err)
			}
			var memberExists bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM iam.memberships AS membership
					JOIN iam.users AS iam_user ON iam_user.id = membership.user_id
					WHERE membership.status = 'active'
					  AND lower(iam_user.primary_email) = $1
				)
			`, email).Scan(&memberExists); err != nil {
				return fmt.Errorf("check existing member email: %w", err)
			}
			if memberExists {
				return errIAMRoleConflict
			}

			expiresAt := now.Add(invitationTTL)
			invitation := invitationRecord{
				ID:        uuid.NewString(),
				Email:     email,
				Role:      string(invitedRole),
				Status:    string(platformiam.InvitationPending),
				ExpiresAt: expiresAt,
			}
			inserted, err := tx.Exec(ctx, `
				INSERT INTO iam.invitations (
					id, org_id, provider, email_normalized, role, status, expires_at
				) VALUES (
					$1::uuid,
					nullif(current_setting('app.org_id', true), '')::uuid,
					'clerk',
					$2,
					$3,
					'pending',
					$4
				)
			`, invitation.ID, invitation.Email, invitation.Role, invitation.ExpiresAt)
			if err != nil {
				if isPendingInvitationConflict(err) {
					return errIAMInvitationPending
				}
				return fmt.Errorf("create invitation: %w", err)
			}
			if inserted.RowsAffected() != 1 {
				return errIAMRoleConflict
			}
			event := newIAMCommandEvent(
				invitationCreateOperation,
				active,
				claims,
				invitation.ID,
			)
			event.Email = invitation.Email
			event.Role = invitation.Role
			event.ExpiresAt = &invitation.ExpiresAt
			event.AppliedLocally = true
			if err := h.appendIAMCommand(ctx, tx, commandKey, invitationCreateTopic, event); err != nil {
				return err
			}
			response, err = mapInvitation(invitation, api.SyncStatusQueued)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) ResendTeamInvitation(
	w http.ResponseWriter,
	r *http.Request,
	invitationID api.InvitationID,
	params api.ResendTeamInvitationParams,
) {
	h.mutateInvitation(
		w,
		r,
		invitationID,
		params.IdempotencyKey,
		invitationResendOperation,
		invitationResendTopic,
		false,
	)
}

func (h *IAMAPI) RevokeTeamInvitation(
	w http.ResponseWriter,
	r *http.Request,
	invitationID api.InvitationID,
	params api.RevokeTeamInvitationParams,
) {
	h.mutateInvitation(
		w,
		r,
		invitationID,
		params.IdempotencyKey,
		invitationRevokeOperation,
		invitationRevokeTopic,
		true,
	)
}

func (h *IAMAPI) mutateInvitation(
	w http.ResponseWriter,
	r *http.Request,
	invitationID api.InvitationID,
	rawKey api.IdempotencyKey,
	operation string,
	topic string,
	revoke bool,
) {
	key, ok := validateIdempotencyKey(w, rawKey)
	if !ok {
		return
	}
	var response api.Invitation
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			actorRole, err := effectiveActorRole(
				active,
				claims,
				productiam.PermissionInvitationManage,
			)
			if err != nil {
				return err
			}
			commandKey := iamCommandKey(
				active.OrganizationID,
				active.UserID,
				operation,
				key,
			)
			stored, replay, err := loadIAMCommand(ctx, tx, commandKey)
			if err != nil {
				return err
			}
			invitation, err := loadInvitationForUpdate(ctx, tx, invitationID.String())
			if err != nil {
				return err
			}
			invitedRole, err := productiam.ParseRole(invitation.Role)
			if err != nil ||
				(actorRole == productiam.RoleAdmin && invitedRole != productiam.RoleMember) {
				return errIAMForbidden
			}
			if replay {
				if !stored.matches(
					topic,
					operation,
					active.OrganizationID,
					active.UserID,
				) ||
					stored.Event.ResourceID != invitation.ID {
					return errIAMRoleConflict
				}
				response, err = mapInvitation(invitation, api.SyncStatusQueued)
				return err
			}
			if revoke && invitation.Status == string(platformiam.InvitationRevoked) {
				response, err = mapInvitation(invitation, api.SyncStatusSynced)
				return err
			}
			if invitation.Status != string(platformiam.InvitationPending) {
				return errIAMRoleConflict
			}

			now, err := h.commandNow()
			if err != nil {
				return err
			}
			if revoke {
				updated, err := tx.Exec(ctx, `
					UPDATE iam.invitations
					SET status = 'revoked', revoked_at = $1, updated_at = $1
					WHERE id = $2::uuid
				`, now, invitation.ID)
				if err != nil {
					return fmt.Errorf("revoke invitation: %w", err)
				}
				if updated.RowsAffected() != 1 {
					return errIAMRoleConflict
				}
				invitation.Status = string(platformiam.InvitationRevoked)
			} else {
				invitation.ExpiresAt = now.Add(invitationTTL)
				updated, err := tx.Exec(ctx, `
					UPDATE iam.invitations
					SET expires_at = $1, updated_at = $2
					WHERE id = $3::uuid
				`, invitation.ExpiresAt, now, invitation.ID)
				if err != nil {
					return fmt.Errorf("refresh invitation expiry: %w", err)
				}
				if updated.RowsAffected() != 1 {
					return errIAMRoleConflict
				}
			}
			event := newIAMCommandEvent(operation, active, claims, invitation.ID)
			event.ExternalResourceID = invitation.ExternalID
			event.Email = invitation.Email
			event.Role = invitation.Role
			event.ExpiresAt = &invitation.ExpiresAt
			event.AppliedLocally = revoke
			if err := h.appendIAMCommand(ctx, tx, commandKey, topic, event); err != nil {
				return err
			}
			response, err = mapInvitation(invitation, api.SyncStatusQueued)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func validateIdempotencyKey(w http.ResponseWriter, raw api.IdempotencyKey) (string, bool) {
	key := string(raw)
	if key != strings.TrimSpace(key) || len(key) < 8 || len(key) > 255 {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"REQUEST_INVALID",
			"Idempotency-Key must contain 8 to 255 visible ASCII characters",
		)
		return "", false
	}
	for _, character := range key {
		if character < 0x21 || character > 0x7e {
			writeAPIError(
				w,
				http.StatusBadRequest,
				"REQUEST_INVALID",
				"Idempotency-Key must contain 8 to 255 visible ASCII characters",
			)
			return "", false
		}
	}
	return key, true
}

func decodeIAMCommandBody(w http.ResponseWriter, r *http.Request, destination any) bool {
	if r == nil || r.Body == nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "JSON request body is required")
		return false
	}
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Content-Type must be application/json")
			return false
		}
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxIAMCommandBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid JSON request body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "JSON request body must contain one object")
		return false
	}
	return true
}

func normalizeInvitationEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || len(normalized) > 320 {
		return "", errors.New("email must be a valid address up to 320 characters")
	}
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized || address.Name != "" {
		return "", errors.New("email must be a valid address up to 320 characters")
	}
	return normalized, nil
}

func isPendingInvitationConflict(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) &&
		postgresError.Code == "23505" &&
		postgresError.ConstraintName == "iam_invitations_pending_email_uidx"
}

func validOrganizationName(name string) bool {
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 120 {
		return false
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func (h *IAMAPI) commandNow() (time.Time, error) {
	if h == nil || h.now == nil {
		return time.Time{}, errors.New("IAM clock is not configured")
	}
	now := h.now().UTC()
	if now.IsZero() {
		return time.Time{}, errors.New("IAM clock returned zero time")
	}
	return now, nil
}

func effectiveActorRole(
	active platformiam.ActiveMembership,
	claims clerkadapter.SessionClaims,
	permission productiam.Permission,
) (productiam.Role, error) {
	localRole, err := productiam.ParseRole(active.Role)
	if err != nil {
		return "", errIAMForbidden
	}
	effectiveRole, err := productiam.EffectiveRole(localRole, claims.OrganizationRole)
	if err != nil || !productiam.HasPermission(effectiveRole, permission) {
		return "", errIAMForbidden
	}
	return effectiveRole, nil
}

func roleRank(role productiam.Role) int {
	switch role {
	case productiam.RoleMember:
		return 1
	case productiam.RoleAdmin:
		return 2
	case productiam.RoleOwner:
		return 3
	default:
		return 0
	}
}

func iamCommandKey(organizationID, actorUserID, operation, key string) string {
	return "iam:" + organizationID + ":" + actorUserID + ":" + operation + ":" + key
}

func loadIAMCommand(
	ctx context.Context,
	tx pgx.Tx,
	key string,
) (storedIAMCommand, bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
		return storedIAMCommand{}, false, fmt.Errorf("lock IAM command: %w", err)
	}
	var (
		topic   string
		payload []byte
	)
	err := tx.QueryRow(ctx, `
		SELECT topic, payload
		FROM public.platform_outbox_messages
		WHERE idempotency_key = $1
	`, key).Scan(&topic, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return storedIAMCommand{}, false, nil
	}
	if err != nil {
		return storedIAMCommand{}, false, fmt.Errorf("load IAM command: %w", err)
	}
	event := iamCommandEvent{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return storedIAMCommand{}, false, fmt.Errorf("decode IAM command replay: %w", err)
	}
	event.Operation = strings.TrimSpace(event.Operation)
	event.OrganizationID = strings.TrimSpace(event.OrganizationID)
	event.ExternalOrganizationID = strings.TrimSpace(event.ExternalOrganizationID)
	event.ActorUserID = strings.TrimSpace(event.ActorUserID)
	event.ActorMembershipID = strings.TrimSpace(event.ActorMembershipID)
	event.ResourceID = strings.TrimSpace(event.ResourceID)
	event.ExternalResourceID = strings.TrimSpace(event.ExternalResourceID)
	event.Name = strings.TrimSpace(event.Name)
	event.Email = strings.ToLower(strings.TrimSpace(event.Email))
	event.Role = strings.TrimSpace(event.Role)
	event.PreviousRole = strings.TrimSpace(event.PreviousRole)
	if event.SchemaVersion != 1 || event.Operation == "" ||
		event.OrganizationID == "" || event.ActorUserID == "" ||
		event.ActorMembershipID == "" || event.ResourceID == "" {
		return storedIAMCommand{}, false, errors.New("invalid stored IAM command")
	}
	return storedIAMCommand{Topic: topic, Event: event}, true, nil
}

func (command storedIAMCommand) matches(
	topic string,
	operation string,
	organizationID string,
	actorUserID string,
) bool {
	return command.Topic == topic &&
		command.Event.SchemaVersion == 1 &&
		command.Event.Operation == operation &&
		command.Event.OrganizationID == organizationID &&
		command.Event.ActorUserID == actorUserID
}

func newIAMCommandEvent(
	operation string,
	active platformiam.ActiveMembership,
	claims clerkadapter.SessionClaims,
	resourceID string,
) iamCommandEvent {
	return iamCommandEvent{
		SchemaVersion:          1,
		Operation:              operation,
		OrganizationID:         active.OrganizationID,
		ExternalOrganizationID: claims.OrganizationID,
		ActorUserID:            active.UserID,
		ActorMembershipID:      active.MembershipID,
		ResourceID:             resourceID,
	}
}

func (h *IAMAPI) appendIAMCommand(
	ctx context.Context,
	tx pgx.Tx,
	key string,
	topic string,
	event iamCommandEvent,
) error {
	if h == nil || h.outboxAppender == nil {
		return errors.New("IAM outbox is not configured")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode IAM command: %w", err)
	}
	if _, err := h.outboxAppender.Append(ctx, tx, platformoutbox.MessageInput{
		IdempotencyKey: key,
		Topic:          topic,
		Payload:        payload,
		Headers: map[string]string{
			"content-type":   "application/json",
			"schema-version": "1",
		},
	}); err != nil {
		return fmt.Errorf("append IAM command: %w", err)
	}
	return nil
}

func loadCurrentOrganizationForUpdate(
	ctx context.Context,
	tx pgx.Tx,
) (organizationRecord, error) {
	organization := organizationRecord{}
	err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			coalesce(external_id, ''),
			name,
			coalesce(slug, ''),
			status
		FROM iam.organizations
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`).Scan(
		&organization.ID,
		&organization.ExternalID,
		&organization.Name,
		&organization.Slug,
		&organization.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return organizationRecord{}, errIAMRoleConflict
	}
	if err != nil {
		return organizationRecord{}, fmt.Errorf("load active organization: %w", err)
	}
	return organization, nil
}

func loadMemberForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	membershipID string,
) (memberRecord, error) {
	member := memberRecord{}
	err := tx.QueryRow(ctx, `
		SELECT
			membership.id::text,
			membership.role,
			membership.status,
			coalesce(membership.external_id, ''),
			iam_user.id::text,
			iam_user.primary_email,
			iam_user.name,
			coalesce(iam_user.avatar_url, '')
		FROM iam.memberships AS membership
		JOIN iam.users AS iam_user ON iam_user.id = membership.user_id
		WHERE membership.id = $1::uuid
		LIMIT 1
		FOR UPDATE OF membership
	`, membershipID).Scan(
		&member.MembershipID,
		&member.Role,
		&member.Status,
		&member.ExternalID,
		&member.UserID,
		&member.Email,
		&member.DisplayName,
		&member.AvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return memberRecord{}, errIAMRoleConflict
	}
	if err != nil {
		return memberRecord{}, fmt.Errorf("load team member: %w", err)
	}
	return member, nil
}

func loadInvitationForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	invitationID string,
) (invitationRecord, error) {
	invitation := invitationRecord{}
	err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			email_normalized,
			role,
			status,
			coalesce(external_id, ''),
			expires_at
		FROM iam.invitations
		WHERE id = $1::uuid
		LIMIT 1
		FOR UPDATE
	`, invitationID).Scan(
		&invitation.ID,
		&invitation.Email,
		&invitation.Role,
		&invitation.Status,
		&invitation.ExternalID,
		&invitation.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return invitationRecord{}, errIAMRoleConflict
	}
	if err != nil {
		return invitationRecord{}, fmt.Errorf("load invitation: %w", err)
	}
	return invitation, nil
}

func mapOrganization(
	organization organizationRecord,
	role string,
	syncStatus api.SyncStatus,
) (api.Organization, error) {
	organizationID, err := uuid.Parse(organization.ID)
	if err != nil {
		return api.Organization{}, err
	}
	parsedRole, err := productiam.ParseRole(role)
	if err != nil {
		return api.Organization{}, err
	}
	status := api.OrganizationStatus(organization.Status)
	if !status.Valid() {
		return api.Organization{}, fmt.Errorf("unsupported organization status %q", organization.Status)
	}
	var switchKey *string
	if organization.ExternalID != "" {
		externalID := organization.ExternalID
		switchKey = &externalID
	}
	return api.Organization{
		Id:         organizationID,
		Name:       organization.Name,
		Role:       api.Role(parsedRole),
		Slug:       organization.Slug,
		Status:     status,
		SwitchKey:  switchKey,
		SyncStatus: syncStatus,
	}, nil
}

func mapMember(member memberRecord, syncStatus api.SyncStatus) (api.Member, error) {
	membershipID, err := uuid.Parse(member.MembershipID)
	if err != nil {
		return api.Member{}, err
	}
	userID, err := uuid.Parse(member.UserID)
	if err != nil {
		return api.Member{}, err
	}
	role, err := productiam.ParseRole(member.Role)
	if err != nil {
		return api.Member{}, err
	}
	status := api.MembershipStatus(member.Status)
	if !status.Valid() {
		return api.Member{}, fmt.Errorf("unsupported membership status %q", member.Status)
	}
	result := api.Member{
		Id:         membershipID,
		Role:       api.Role(role),
		Status:     status,
		SyncStatus: syncStatus,
		User: api.User{
			DisplayName: member.DisplayName,
			Email:       openapi_types.Email(member.Email),
			Id:          userID,
		},
	}
	if member.AvatarURL != "" {
		result.User.AvatarUrl = &member.AvatarURL
	}
	return result, nil
}

func mapInvitation(
	invitation invitationRecord,
	syncStatus api.SyncStatus,
) (api.Invitation, error) {
	invitationID, err := uuid.Parse(invitation.ID)
	if err != nil {
		return api.Invitation{}, err
	}
	role, err := productiam.ParseRole(invitation.Role)
	if err != nil {
		return api.Invitation{}, err
	}
	status := api.InvitationStatus(invitation.Status)
	if !status.Valid() {
		return api.Invitation{}, fmt.Errorf("unsupported invitation status %q", invitation.Status)
	}
	return api.Invitation{
		Email:      openapi_types.Email(invitation.Email),
		ExpiresAt:  invitation.ExpiresAt,
		Id:         invitationID,
		Role:       api.Role(role),
		Status:     status,
		SyncStatus: syncStatus,
	}, nil
}
