package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *IAMAPI) GetTeamMemberBusinessPermissions(
	w http.ResponseWriter,
	r *http.Request,
	memberID api.MemberID,
) {
	var response api.MemberBusinessPermissions
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionTeamView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			_ platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			var err error
			response, err = loadMemberBusinessPermissions(ctx, tx, memberID)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) UpdateTeamMemberBusinessPermissions(
	w http.ResponseWriter,
	r *http.Request,
	memberID api.MemberID,
	params api.UpdateTeamMemberBusinessPermissionsParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	var input api.MemberBusinessPermissionsInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	desired, err := normalizeDelegatedPermissions(input.Permissions)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}

	var response api.MemberBusinessPermissions
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionMemberUpdate,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			var status string
			if err := tx.QueryRow(ctx, `
				SELECT membership.status
				FROM iam.memberships AS membership
				WHERE membership.id = $1`,
				memberID,
			).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
				return errBusinessNotFound
			} else if err != nil {
				return fmt.Errorf("load delegated-permission member: %w", err)
			}
			if status != string(platformiam.MembershipActive) {
				return fmt.Errorf("%w: member is not active", errBusinessInvalidTransition)
			}

			if _, err := tx.Exec(ctx, `
				DELETE FROM iam.membership_permissions
				WHERE membership_id = $1
				  AND permission IN ('accounting:manage', 'fiscal:manage')`,
				memberID,
			); err != nil {
				return fmt.Errorf("replace delegated business permissions: %w", err)
			}
			for _, permission := range desired {
				if _, err := tx.Exec(ctx, `
					INSERT INTO iam.membership_permissions (
						org_id,
						membership_id,
						permission,
						granted_by
					)
					SELECT
						membership.org_id,
						membership.id,
						$2,
						$3
					FROM iam.memberships AS membership
					WHERE membership.id = $1`,
					memberID,
					permission,
					strings.TrimSpace(active.UserID),
				); err != nil {
					return fmt.Errorf("grant delegated business permission: %w", err)
				}
			}
			response = api.MemberBusinessPermissions{
				MemberId:    memberID,
				Permissions: desired,
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func loadMemberBusinessPermissions(
	ctx context.Context,
	tx pgx.Tx,
	memberID uuid.UUID,
) (api.MemberBusinessPermissions, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM iam.memberships AS membership
			WHERE membership.id = $1
			  AND membership.status = 'active'
		)`,
		memberID,
	).Scan(&exists); err != nil {
		return api.MemberBusinessPermissions{}, fmt.Errorf(
			"check delegated-permission member: %w",
			err,
		)
	}
	if !exists {
		return api.MemberBusinessPermissions{}, errBusinessNotFound
	}

	rows, err := tx.Query(ctx, `
		SELECT permission
		FROM iam.membership_permissions
		WHERE membership_id = $1
		  AND permission IN ('accounting:manage', 'fiscal:manage')
		ORDER BY permission`,
		memberID,
	)
	if err != nil {
		return api.MemberBusinessPermissions{}, fmt.Errorf(
			"list delegated business permissions: %w",
			err,
		)
	}
	defer rows.Close()
	permissions := make([]api.DelegatedBusinessPermission, 0, 2)
	for rows.Next() {
		var permission api.DelegatedBusinessPermission
		if err := rows.Scan(&permission); err != nil {
			return api.MemberBusinessPermissions{}, fmt.Errorf(
				"scan delegated business permission: %w",
				err,
			)
		}
		if !permission.Valid() {
			return api.MemberBusinessPermissions{}, fmt.Errorf(
				"unsupported delegated business permission %q",
				permission,
			)
		}
		permissions = append(permissions, permission)
	}
	if err := rows.Err(); err != nil {
		return api.MemberBusinessPermissions{}, fmt.Errorf(
			"iterate delegated business permissions: %w",
			err,
		)
	}
	return api.MemberBusinessPermissions{
		MemberId:    memberID,
		Permissions: permissions,
	}, nil
}

func normalizeDelegatedPermissions(
	values []api.DelegatedBusinessPermission,
) ([]api.DelegatedBusinessPermission, error) {
	if len(values) > 2 {
		return nil, errors.New("at most two delegated business permissions are allowed")
	}
	seen := make(map[api.DelegatedBusinessPermission]struct{}, len(values))
	normalized := make([]api.DelegatedBusinessPermission, 0, len(values))
	for _, value := range values {
		if !value.Valid() {
			return nil, fmt.Errorf("unsupported delegated business permission %q", value)
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, fmt.Errorf("delegated business permission %q is duplicated", value)
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	slices.Sort(normalized)
	return normalized, nil
}
