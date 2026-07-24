package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/devpablocristo/pymes/v2/backend/internal/administration"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *IAMAPI) ListAdminTenants(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListAdminTenantsParams,
) {
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	filter := administration.TenantFilter{}
	if params.Status != nil {
		filter.Status = string(*params.Status)
	}
	if params.LifecycleState != nil {
		filter.LifecycleState = string(*params.LifecycleState)
	}
	if params.Query != nil {
		filter.Query = strings.TrimSpace(*params.Query)
	}
	tenants, err := h.administration.ListTenants(r.Context(), claims.Subject, filter)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	start, end, next, err := pageBounds(params.Cursor, params.Limit, len(tenants))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	items := make([]api.AdminTenant, 0, end-start)
	for _, tenant := range tenants[start:end] {
		item, err := mapAdminTenant(tenant)
		if err != nil {
			h.writeAdministrationError(w, err)
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, api.AdminTenantList{
		Items: items,
		Page:  api.PageInfo{NextCursor: next, Total: len(tenants)},
	})
}

func (h *IAMAPI) CreateAdminTenant(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateAdminTenantParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	var input api.CreateAdminTenantInput
	if !decodeIAMCommandBody(w, r, &input) {
		return
	}
	tenant, err := h.administration.CreateTenant(r.Context(), claims.Subject, administration.CreateTenantInput{
		Name:       strings.TrimSpace(input.Name),
		Slug:       strings.TrimSpace(input.Slug),
		AdminEmail: string(input.AdminEmail),
	})
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	response, err := mapAdminTenant(tenant)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	response.SyncStatus = api.SyncStatusQueued
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) GetAdminTenant(w http.ResponseWriter, r *http.Request, tenantID api.TenantID) {
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	tenant, err := h.administration.GetTenant(r.Context(), claims.Subject, tenantID.String())
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	response, err := mapAdminTenant(tenant)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) UpdateAdminTenant(
	w http.ResponseWriter,
	r *http.Request,
	tenantID api.TenantID,
	params api.UpdateAdminTenantParams,
) {
	key, ok := validateIdempotencyKey(w, params.IdempotencyKey)
	if !ok {
		return
	}
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	var input api.UpdateAdminTenantInput
	if !decodeIAMCommandBody(w, r, &input) {
		return
	}
	if input.Name == nil && input.Slug == nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "name or slug is required")
		return
	}
	tenant, err := h.administration.UpdateTenant(
		r.Context(),
		claims.Subject,
		tenantID.String(),
		key,
		administration.UpdateTenantInput{Name: input.Name, Slug: input.Slug},
	)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	response, err := mapAdminTenant(tenant)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	response.SyncStatus = api.SyncStatusQueued
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) ArchiveAdminTenant(
	w http.ResponseWriter,
	r *http.Request,
	tenantID api.TenantID,
	params api.ArchiveAdminTenantParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminTenantLifecycle(w, r, tenantID, "archive")
}

func (h *IAMAPI) UnarchiveAdminTenant(
	w http.ResponseWriter,
	r *http.Request,
	tenantID api.TenantID,
	params api.UnarchiveAdminTenantParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminTenantLifecycle(w, r, tenantID, "unarchive")
}

func (h *IAMAPI) TrashAdminTenant(
	w http.ResponseWriter,
	r *http.Request,
	tenantID api.TenantID,
	params api.TrashAdminTenantParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminTenantLifecycle(w, r, tenantID, "trash")
}

func (h *IAMAPI) RestoreAdminTenant(
	w http.ResponseWriter,
	r *http.Request,
	tenantID api.TenantID,
	params api.RestoreAdminTenantParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminTenantLifecycle(w, r, tenantID, "restore")
}

func (h *IAMAPI) executeAdminTenantLifecycle(
	w http.ResponseWriter,
	r *http.Request,
	tenantID api.TenantID,
	action string,
) {
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	reason, ok := decodeLifecycleReason(w, r)
	if !ok {
		return
	}
	var err error
	switch action {
	case "archive":
		err = h.administration.ArchiveTenant(r.Context(), claims.Subject, tenantID.String(), reason)
	case "unarchive":
		err = h.administration.UnarchiveTenant(r.Context(), claims.Subject, tenantID.String(), reason)
	case "trash":
		err = h.administration.TrashTenant(r.Context(), claims.Subject, tenantID.String(), reason)
	case "restore":
		err = h.administration.RestoreTenant(r.Context(), claims.Subject, tenantID.String(), reason)
	case "purge":
		err = h.administration.PurgeTenant(r.Context(), claims.Subject, tenantID.String(), reason)
	default:
		err = administration.ErrInvalidInput
	}
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IAMAPI) PurgeAdminTenant(
	w http.ResponseWriter,
	r *http.Request,
	tenantID api.TenantID,
	params api.PurgeAdminTenantParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminTenantLifecycle(w, r, tenantID, "purge")
}

func (h *IAMAPI) ListAdminUsers(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListAdminUsersParams,
) {
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	filter := administration.UserFilter{}
	if params.Status != nil {
		filter.Status = string(*params.Status)
	}
	if params.LifecycleState != nil {
		filter.LifecycleState = string(*params.LifecycleState)
	}
	if params.Query != nil {
		filter.Query = strings.TrimSpace(*params.Query)
	}
	users, err := h.administration.ListUsers(r.Context(), claims.Subject, filter)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	start, end, next, err := pageBounds(params.Cursor, params.Limit, len(users))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	items := make([]api.AdminUser, 0, end-start)
	for _, user := range users[start:end] {
		item, err := mapAdminUser(user)
		if err != nil {
			h.writeAdministrationError(w, err)
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, api.AdminUserList{
		Items: items,
		Page:  api.PageInfo{NextCursor: next, Total: len(users)},
	})
}

func (h *IAMAPI) CreateAdminUser(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateAdminUserParams,
) {
	key, ok := validateIdempotencyKey(w, params.IdempotencyKey)
	if !ok {
		return
	}
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	var input api.CreateAdminUserInput
	if !decodeIAMCommandBody(w, r, &input) {
		return
	}
	if input.Role != api.RoleAdmin && input.Role != api.RoleMember {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "role must be admin or member")
		return
	}
	invitation, err := h.administration.CreateUser(
		r.Context(),
		claims.Subject,
		key,
		administration.CreateUserInput{
			Email:    string(input.Email),
			TenantID: input.TenantId.String(),
			Role:     string(input.Role),
		},
	)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, api.Invitation{
		Email:      openapi_types.Email(invitation.Email),
		ExpiresAt:  invitation.ExpiresAt,
		Id:         uuid.MustParse(invitation.ID),
		Role:       api.Role(invitation.Role),
		Status:     api.InvitationStatus(invitation.Status),
		SyncStatus: api.SyncStatusQueued,
	})
}

func (h *IAMAPI) GetAdminUser(w http.ResponseWriter, r *http.Request, userID api.UserID) {
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	user, err := h.administration.GetUser(r.Context(), claims.Subject, userID.String())
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	response, err := mapAdminUser(user)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) UpdateAdminUser(
	w http.ResponseWriter,
	r *http.Request,
	userID api.UserID,
	params api.UpdateAdminUserParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	var input api.UpdateAdminUserInput
	if !decodeIAMCommandBody(w, r, &input) {
		return
	}
	if input.DisplayName == nil && input.Email == nil && input.ProductRole == nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "an editable field is required")
		return
	}
	var productRole *string
	if input.ProductRole != nil {
		value := string(*input.ProductRole)
		productRole = &value
	}
	user, err := h.administration.UpdateUser(
		r.Context(),
		claims.Subject,
		userID.String(),
		administration.UpdateUserInput{
			DisplayName: input.DisplayName,
			Email:       emailPointer(input.Email),
			ProductRole: productRole,
			Version:     input.Version,
		},
	)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	response, err := mapAdminUser(user)
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (h *IAMAPI) ArchiveAdminUser(
	w http.ResponseWriter,
	r *http.Request,
	userID api.UserID,
	params api.ArchiveAdminUserParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminUserLifecycle(w, r, userID, "archive")
}

func (h *IAMAPI) UnarchiveAdminUser(
	w http.ResponseWriter,
	r *http.Request,
	userID api.UserID,
	params api.UnarchiveAdminUserParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminUserLifecycle(w, r, userID, "unarchive")
}

func (h *IAMAPI) TrashAdminUser(
	w http.ResponseWriter,
	r *http.Request,
	userID api.UserID,
	params api.TrashAdminUserParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminUserLifecycle(w, r, userID, "trash")
}

func (h *IAMAPI) RestoreAdminUser(
	w http.ResponseWriter,
	r *http.Request,
	userID api.UserID,
	params api.RestoreAdminUserParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminUserLifecycle(w, r, userID, "restore")
}

func (h *IAMAPI) executeAdminUserLifecycle(
	w http.ResponseWriter,
	r *http.Request,
	userID api.UserID,
	action string,
) {
	claims, ok := h.verifyAdminIdentity(w, r)
	if !ok {
		return
	}
	reason, ok := decodeLifecycleReason(w, r)
	if !ok {
		return
	}
	var err error
	switch action {
	case "archive":
		err = h.administration.ArchiveUser(r.Context(), claims.Subject, userID.String(), reason)
	case "unarchive":
		err = h.administration.UnarchiveUser(r.Context(), claims.Subject, userID.String(), reason)
	case "trash":
		err = h.administration.TrashUser(r.Context(), claims.Subject, userID.String(), reason)
	case "restore":
		err = h.administration.RestoreUser(r.Context(), claims.Subject, userID.String(), reason)
	case "purge":
		err = h.administration.PurgeUser(r.Context(), claims.Subject, userID.String(), reason)
	default:
		err = administration.ErrInvalidInput
	}
	if err != nil {
		h.writeAdministrationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IAMAPI) PurgeAdminUser(
	w http.ResponseWriter,
	r *http.Request,
	userID api.UserID,
	params api.PurgeAdminUserParams,
) {
	if _, ok := validateIdempotencyKey(w, params.IdempotencyKey); !ok {
		return
	}
	h.executeAdminUserLifecycle(w, r, userID, "purge")
}

func decodeLifecycleReason(w http.ResponseWriter, r *http.Request) (string, bool) {
	var input api.LifecycleCommandInput
	if !decodeIAMCommandBody(w, r, &input) {
		return "", false
	}
	if input.Reason == nil {
		return "", true
	}
	return strings.TrimSpace(*input.Reason), true
}

func (h *IAMAPI) verifyAdminIdentity(
	w http.ResponseWriter,
	r *http.Request,
) (claims identityClaims, ok bool) {
	verified, ok := h.verifyIdentity(w, r)
	if !ok {
		return identityClaims{}, false
	}
	if h.administration == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "Administration is unavailable")
		return identityClaims{}, false
	}
	owner, err := h.administration.IsOwner(r.Context(), verified.Subject)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "Administration is unavailable")
		return identityClaims{}, false
	}
	if !owner {
		writeAPIError(w, http.StatusForbidden, "IAM_FORBIDDEN", "Global owner role is required")
		return identityClaims{}, false
	}
	return identityClaims{Subject: verified.Subject}, true
}

type identityClaims struct {
	Subject string
}

func (h *IAMAPI) writeAdministrationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, administration.ErrForbidden):
		writeAPIError(w, http.StatusForbidden, "IAM_FORBIDDEN", "Global owner role is required")
	case errors.Is(err, administration.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Resource not found")
	case errors.Is(err, administration.ErrInvalidInput):
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid administration request")
	case errors.Is(err, administration.ErrLastOwner):
		writeAPIError(w, http.StatusConflict, "IAM_LAST_OWNER", "At least one active global owner is required")
	case errors.Is(err, administration.ErrConflict):
		writeAPIError(w, http.StatusConflict, "IAM_STATE_CONFLICT", "Administration state conflicts with the request")
	case errors.Is(err, administration.ErrProviderBacked):
		writeAPIError(w, http.StatusServiceUnavailable, "AUTH_PROVIDER_UNAVAILABLE", "Identity provider is unavailable")
	default:
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "Administration is unavailable")
	}
}

func mapAdminTenant(tenant administration.Tenant) (api.AdminTenant, error) {
	id, err := uuid.Parse(tenant.ID)
	if err != nil {
		return api.AdminTenant{}, err
	}
	status := api.OrganizationStatus(tenant.Status)
	if !status.Valid() {
		return api.AdminTenant{}, errors.New("invalid tenant status")
	}
	syncStatus := api.SyncStatusSynced
	if tenant.Status == string(api.OrganizationStatusProvisioning) {
		syncStatus = api.SyncStatusQueued
	}
	lifecycleState := api.LifecycleState(tenant.LifecycleState)
	if !lifecycleState.Valid() {
		return api.AdminTenant{}, errors.New("invalid tenant lifecycle state")
	}
	return api.AdminTenant{
		ArchivedAt:     tenant.ArchivedAt,
		CreatedAt:      tenant.CreatedAt,
		Id:             id,
		LifecycleState: lifecycleState,
		Name:           tenant.Name,
		PurgeAfter:     tenant.PurgeAfter,
		Slug:           tenant.Slug,
		Status:         status,
		SyncStatus:     syncStatus,
		TrashedAt:      tenant.TrashedAt,
		UpdatedAt:      tenant.UpdatedAt,
	}, nil
}

func mapAdminUser(user administration.User) (api.AdminUser, error) {
	id, err := uuid.Parse(user.ID)
	if err != nil {
		return api.AdminUser{}, err
	}
	status := api.UserStatus(user.Status)
	if !status.Valid() {
		return api.AdminUser{}, errors.New("invalid user status")
	}
	productRole := api.ProductRole(user.ProductRole)
	if !productRole.Valid() {
		return api.AdminUser{}, errors.New("invalid product role")
	}
	lifecycleState := api.LifecycleState(user.LifecycleState)
	if !lifecycleState.Valid() {
		return api.AdminUser{}, errors.New("invalid user lifecycle state")
	}
	memberships := make([]api.AdminMembership, 0, len(user.Memberships))
	for _, membership := range user.Memberships {
		membershipID, err := uuid.Parse(membership.ID)
		if err != nil {
			return api.AdminUser{}, err
		}
		tenantID, err := uuid.Parse(membership.TenantID)
		if err != nil {
			return api.AdminUser{}, err
		}
		memberships = append(memberships, api.AdminMembership{
			Id:         membershipID,
			Role:       api.Role(membership.Role),
			Status:     api.MembershipStatus(membership.Status),
			TenantId:   tenantID,
			TenantName: membership.TenantName,
		})
	}
	var avatarURL *string
	if strings.TrimSpace(user.AvatarURL) != "" {
		value := user.AvatarURL
		avatarURL = &value
	}
	return api.AdminUser{
		ArchivedAt:     user.ArchivedAt,
		AvatarUrl:      avatarURL,
		CreatedAt:      user.CreatedAt,
		DisplayName:    user.DisplayName,
		Email:          openapi_types.Email(user.Email),
		EmailVerified:  user.EmailVerified,
		Id:             id,
		LifecycleState: lifecycleState,
		Memberships:    memberships,
		ProductRole:    productRole,
		PurgeAfter:     user.PurgeAfter,
		Status:         status,
		TrashedAt:      user.TrashedAt,
		UpdatedAt:      user.UpdatedAt,
		Version:        user.Version,
	}, nil
}

func emailPointer(value *openapi_types.Email) *string {
	if value == nil {
		return nil
	}
	email := string(*value)
	return &email
}
