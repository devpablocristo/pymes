package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var errIAMForbidden = errors.New("IAM permission denied")

func (h *IAMAPI) ListTeamMembers(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListTeamMembersParams,
) {
	var members []api.Member
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			if err := requirePermission(
				active,
				claims,
				productiam.PermissionTeamView,
			); err != nil {
				return err
			}
			var err error
			members, err = loadTeamMembers(ctx, tx)
			return err
		},
	) {
		return
	}
	start, end, nextCursor, err := pageBounds(params.Cursor, params.Limit, len(members))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, api.MemberList{
		Items: members[start:end],
		Page: api.PageInfo{
			NextCursor: nextCursor,
			Total:      len(members),
		},
	})
}

func (h *IAMAPI) ListTeamInvitations(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListTeamInvitationsParams,
) {
	status := ""
	if params.Status != nil {
		if !params.Status.Valid() {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid invitation status")
			return
		}
		status = string(*params.Status)
	}
	var invitations []api.Invitation
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			if err := requirePermission(
				active,
				claims,
				productiam.PermissionInvitationManage,
			); err != nil {
				return err
			}
			var err error
			invitations, err = loadTeamInvitations(ctx, tx, status)
			return err
		},
	) {
		return
	}
	start, end, nextCursor, err := pageBounds(params.Cursor, params.Limit, len(invitations))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, api.InvitationList{
		Items: invitations[start:end],
		Page: api.PageInfo{
			NextCursor: nextCursor,
			Total:      len(invitations),
		},
	})
}

func requirePermission(
	active platformiam.ActiveMembership,
	claims clerkadapter.SessionClaims,
	permission productiam.Permission,
) error {
	localRole, err := productiam.ParseRole(active.Role)
	if err != nil {
		return fmt.Errorf("%w: %v", errIAMForbidden, err)
	}
	effectiveRole, err := productiam.EffectiveRole(localRole, claims.OrganizationRole)
	if err != nil {
		return fmt.Errorf("%w: %v", errIAMForbidden, err)
	}
	if !productiam.HasPermission(effectiveRole, permission) {
		return errIAMForbidden
	}
	return nil
}

func loadTeamMembers(ctx context.Context, tx pgx.Tx) ([]api.Member, error) {
	rows, err := tx.Query(ctx, `
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
		JOIN iam.users AS iam_user
		  ON iam_user.id = membership.user_id
		WHERE membership.status = 'active'
		ORDER BY lower(iam_user.name), lower(iam_user.primary_email), membership.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer rows.Close()

	members := make([]api.Member, 0)
	for rows.Next() {
		var (
			membershipID string
			role         string
			status       string
			externalID   string
			userID       string
			email        string
			displayName  string
			avatarURL    string
		)
		if err := rows.Scan(
			&membershipID,
			&role,
			&status,
			&externalID,
			&userID,
			&email,
			&displayName,
			&avatarURL,
		); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		parsedRole, err := productiam.ParseRole(role)
		if err != nil {
			return nil, err
		}
		apiStatus := api.MembershipStatus(status)
		if !apiStatus.Valid() {
			return nil, fmt.Errorf("unsupported membership status %q", status)
		}
		membershipUUID, err := uuid.Parse(membershipID)
		if err != nil {
			return nil, err
		}
		userUUID, err := uuid.Parse(userID)
		if err != nil {
			return nil, err
		}
		syncStatus := api.SyncStatusPending
		if externalID != "" {
			syncStatus = api.SyncStatusSynced
		}
		member := api.Member{
			Id:         membershipUUID,
			Role:       api.Role(parsedRole),
			Status:     apiStatus,
			SyncStatus: syncStatus,
			User: api.User{
				DisplayName: displayName,
				Email:       openapi_types.Email(email),
				Id:          userUUID,
			},
		}
		if avatarURL != "" {
			member.User.AvatarUrl = &avatarURL
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members: %w", err)
	}
	return members, nil
}

func loadTeamInvitations(
	ctx context.Context,
	tx pgx.Tx,
	statusFilter string,
) ([]api.Invitation, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			invitation.id::text,
			invitation.email_normalized,
			invitation.role,
			invitation.status,
			coalesce(invitation.external_id, ''),
			invitation.expires_at
		FROM iam.invitations AS invitation
		WHERE ($1 = '' OR invitation.status = $1)
		ORDER BY invitation.created_at DESC, invitation.id
	`, strings.TrimSpace(statusFilter))
	if err != nil {
		return nil, fmt.Errorf("list team invitations: %w", err)
	}
	defer rows.Close()

	invitations := make([]api.Invitation, 0)
	for rows.Next() {
		var (
			invitationID string
			email        string
			role         string
			status       string
			externalID   string
			expiresAt    time.Time
		)
		if err := rows.Scan(
			&invitationID,
			&email,
			&role,
			&status,
			&externalID,
			&expiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan team invitation: %w", err)
		}
		parsedRole, err := productiam.ParseRole(role)
		if err != nil {
			return nil, err
		}
		apiStatus := api.InvitationStatus(status)
		if !apiStatus.Valid() {
			return nil, fmt.Errorf("unsupported invitation status %q", status)
		}
		invitationUUID, err := uuid.Parse(invitationID)
		if err != nil {
			return nil, err
		}
		syncStatus := api.SyncStatusQueued
		if externalID != "" {
			syncStatus = api.SyncStatusSynced
		}
		invitations = append(invitations, api.Invitation{
			Email:      openapi_types.Email(email),
			ExpiresAt:  expiresAt,
			Id:         invitationUUID,
			Role:       api.Role(parsedRole),
			Status:     apiStatus,
			SyncStatus: syncStatus,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team invitations: %w", err)
	}
	return invitations, nil
}
