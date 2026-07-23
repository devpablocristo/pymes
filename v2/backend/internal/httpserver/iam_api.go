package httpserver

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type ClerkSessionVerifier interface {
	VerifySession(context.Context, string) (clerkadapter.SessionClaims, error)
}

type ClerkIdentityVerifier interface {
	VerifyIdentity(context.Context, string) (clerkadapter.SessionClaims, error)
}

type ClerkSessionManager interface {
	ListSessions(context.Context, clerkadapter.SessionListInput) ([]clerkadapter.Session, error)
	GetSession(context.Context, string) (clerkadapter.Session, error)
	RevokeSession(context.Context, string) error
}

type ClerkWebhookVerifier interface {
	VerifyAndDecode([]byte, http.Header) (clerkadapter.WebhookEvent, error)
}

type WebhookInbox interface {
	ReceiveWebhookEvent(
		context.Context,
		platformiam.WebhookEvent,
	) (platformiam.WebhookEvent, bool, error)
}

type OutboxAppender interface {
	Append(
		context.Context,
		pgx.Tx,
		platformoutbox.MessageInput,
	) (platformoutbox.Message, error)
}

type SessionTransactor interface {
	WithinSessionTx(
		context.Context,
		platformiam.VerifiedSession,
		platformiam.SessionTxFunc,
	) error
}

type IAMDependencies struct {
	Verifier              ClerkSessionVerifier
	IdentityVerifier      ClerkIdentityVerifier
	Transactor            SessionTransactor
	OrganizationDirectory productiam.OrganizationDirectory
	SessionManager        ClerkSessionManager
	WebhookVerifier       ClerkWebhookVerifier
	WebhookInbox          WebhookInbox
	OutboxAppender        OutboxAppender
	Now                   func() time.Time
}

// IAMAPI owns the product HTTP boundary. Provider and persistence adapters are
// injected as the IAM slices are enabled; an absent Clerk configuration always
// fails closed.
type IAMAPI struct {
	clerk                 config.ClerkConfig
	verifier              ClerkSessionVerifier
	identityVerifier      ClerkIdentityVerifier
	transactor            SessionTransactor
	organizationDirectory productiam.OrganizationDirectory
	sessionManager        ClerkSessionManager
	webhookVerifier       ClerkWebhookVerifier
	webhookInbox          WebhookInbox
	outboxAppender        OutboxAppender
	now                   func() time.Time
}

func NewIAMAPI(clerk config.ClerkConfig, dependencies ...IAMDependencies) *IAMAPI {
	handler := &IAMAPI{clerk: clerk, now: time.Now}
	if len(dependencies) > 0 {
		handler.verifier = dependencies[0].Verifier
		handler.identityVerifier = dependencies[0].IdentityVerifier
		handler.transactor = dependencies[0].Transactor
		handler.organizationDirectory = dependencies[0].OrganizationDirectory
		handler.sessionManager = dependencies[0].SessionManager
		handler.webhookVerifier = dependencies[0].WebhookVerifier
		handler.webhookInbox = dependencies[0].WebhookInbox
		handler.outboxAppender = dependencies[0].OutboxAppender
		if dependencies[0].Now != nil {
			handler.now = dependencies[0].Now
		}
	}
	return handler
}

func (h *IAMAPI) GetRuntimeConfig(w http.ResponseWriter, _ *http.Request) {
	response := api.RuntimeConfig{}
	response.Auth.Provider = api.Clerk
	response.Auth.Configured = h.clerk.Configured()
	if response.Auth.Configured {
		response.Auth.PublishableKey = &h.clerk.PublishableKey
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) GetCurrentSession(w http.ResponseWriter, r *http.Request) {
	var response api.CurrentSession
	if !h.withinOrganizationTx(
		w,
		r,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			claims clerkadapter.SessionClaims,
		) error {
			var loadErr error
			response, loadErr = loadCurrentSession(ctx, tx, active, claims)
			return loadErr
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) withinOrganizationTx(
	w http.ResponseWriter,
	r *http.Request,
	fn func(
		context.Context,
		pgx.Tx,
		platformiam.ActiveMembership,
		clerkadapter.SessionClaims,
	) error,
) bool {
	claims, ok := h.verifyOrganizationSession(w, r)
	if !ok {
		return false
	}
	verified := platformiam.VerifiedSession{
		Provider:               "clerk",
		Subject:                claims.Subject,
		SessionID:              claims.SessionID,
		ExternalOrganizationID: claims.OrganizationID,
		ProviderRole:           claims.OrganizationRole,
		ProviderPermissions:    claims.OrganizationPermissions,
		IssuedAt:               claims.IssuedAt,
		ExpiresAt:              claims.ExpiresAt,
	}
	err := h.transactor.WithinSessionTx(
		r.Context(),
		verified,
		func(ctx context.Context, tx pgx.Tx, active platformiam.ActiveMembership) error {
			return fn(ctx, tx, active, claims)
		},
	)
	if errors.Is(err, platformiam.ErrActiveMembershipRequired) {
		writeAPIError(
			w,
			http.StatusForbidden,
			"IAM_MEMBERSHIP_REQUIRED",
			"An active local membership is required",
		)
		return false
	}
	if errors.Is(err, platformiam.ErrInvalidVerifiedSession) {
		writeAPIError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid session token")
		return false
	}
	if errors.Is(err, errIAMForbidden) {
		writeAPIError(w, http.StatusForbidden, "IAM_FORBIDDEN", "Insufficient permission")
		return false
	}
	if errors.Is(err, errIAMRoleConflict) {
		writeAPIError(w, http.StatusConflict, "IAM_ROLE_CONFLICT", "IAM state conflicts with the requested operation")
		return false
	}
	if errors.Is(err, errIAMInvitationPending) {
		writeAPIError(w, http.StatusConflict, "IAM_INVITATION_PENDING", "A pending invitation already exists")
		return false
	}
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "IAM is unavailable")
		return false
	}
	return true
}

func (h *IAMAPI) ListMyOrganizations(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListMyOrganizationsParams,
) {
	claims, ok := h.verifyIdentity(w, r)
	if !ok {
		return
	}
	if h.organizationDirectory == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "IAM is unavailable")
		return
	}
	organizations, err := h.organizationDirectory.ListActiveOrganizations(
		r.Context(),
		"clerk",
		claims.Subject,
	)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "IAM is unavailable")
		return
	}
	start, end, nextCursor, err := pageBounds(params.Cursor, params.Limit, len(organizations))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	items := make([]api.Organization, 0, end-start)
	for _, organization := range organizations[start:end] {
		id, parseErr := uuid.Parse(organization.OrganizationID)
		if parseErr != nil {
			writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "IAM is unavailable")
			return
		}
		switchKey := organization.ExternalOrganizationID
		items = append(items, api.Organization{
			Id:         id,
			Name:       organization.Name,
			Role:       api.Role(organization.Role),
			Slug:       organization.Slug,
			Status:     api.OrganizationStatusActive,
			SwitchKey:  &switchKey,
			SyncStatus: api.SyncStatusSynced,
		})
	}
	writeJSON(w, http.StatusOK, api.OrganizationList{
		Items: items,
		Page: api.PageInfo{
			NextCursor: nextCursor,
			Total:      len(organizations),
		},
	})
}

func (h *IAMAPI) ListMySessions(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListMySessionsParams,
) {
	claims, ok := h.verifyIdentity(w, r)
	if !ok {
		return
	}
	if h.sessionManager == nil {
		h.writeUnavailable(w)
		return
	}
	sessions, err := h.sessionManager.ListSessions(r.Context(), clerkadapter.SessionListInput{
		ListInput:      clerkadapter.ListInput{Limit: 100},
		ProviderUserID: claims.Subject,
	})
	if err != nil {
		h.writeClerkError(w, err)
		return
	}
	ownedSessions := make([]clerkadapter.Session, 0, len(sessions))
	for _, session := range sessions {
		if session.UserID == claims.Subject {
			ownedSessions = append(ownedSessions, session)
		}
	}
	start, end, nextCursor, err := pageBounds(params.Cursor, params.Limit, len(ownedSessions))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	items := make([]api.DeviceSession, 0, end-start)
	for _, session := range ownedSessions[start:end] {
		status := api.SessionStatus(strings.ToLower(strings.TrimSpace(session.Status)))
		if !status.Valid() {
			writeAPIError(
				w,
				http.StatusServiceUnavailable,
				"AUTH_PROVIDER_UNAVAILABLE",
				"Authentication provider returned an unsupported session status",
			)
			return
		}
		items = append(items, api.DeviceSession{
			CreatedAt:    session.CreatedAt,
			Current:      session.ID == claims.SessionID,
			ExpiresAt:    session.ExpiresAt,
			Id:           session.ID,
			LastActiveAt: session.LastActiveAt,
			Status:       status,
		})
	}
	writeJSON(w, http.StatusOK, api.SessionList{
		Items: items,
		Page: api.PageInfo{
			NextCursor: nextCursor,
			Total:      len(ownedSessions),
		},
	})
}

func (h *IAMAPI) RevokeMySession(
	w http.ResponseWriter,
	r *http.Request,
	sessionID api.SessionID,
	params api.RevokeMySessionParams,
) {
	if strings.TrimSpace(string(params.IdempotencyKey)) == "" {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"REQUEST_INVALID",
			"Idempotency-Key is required",
		)
		return
	}
	claims, ok := h.verifyIdentity(w, r)
	if !ok {
		return
	}
	if h.sessionManager == nil {
		h.writeUnavailable(w)
		return
	}
	session, err := h.sessionManager.GetSession(r.Context(), string(sessionID))
	if clerkadapter.IsNotFound(err) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		h.writeClerkError(w, err)
		return
	}
	if session.UserID != claims.Subject {
		writeAPIError(w, http.StatusForbidden, "IAM_FORBIDDEN", "Session does not belong to user")
		return
	}
	if err := h.sessionManager.RevokeSession(r.Context(), string(sessionID)); err != nil {
		h.writeClerkError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h *IAMAPI) ReceiveClerkWebhook(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(h.clerk.WebhookSecret) == "" ||
		h.webhookVerifier == nil ||
		h.webhookInbox == nil {
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"AUTH_NOT_CONFIGURED",
			"Clerk webhook is not configured",
		)
		return
	}

	const maxWebhookBody = 1 << 20
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"WEBHOOK_INVALID_PAYLOAD",
			"Invalid webhook payload",
		)
		return
	}
	event, err := h.webhookVerifier.VerifyAndDecode(payload, r.Header)
	switch {
	case errors.Is(err, clerkadapter.ErrInvalidWebhookSignature):
		writeAPIError(
			w,
			http.StatusUnauthorized,
			"WEBHOOK_INVALID_SIGNATURE",
			"Invalid webhook signature",
		)
		return
	case errors.Is(err, clerkadapter.ErrInvalidWebhookPayload):
		writeAPIError(
			w,
			http.StatusBadRequest,
			"WEBHOOK_INVALID_PAYLOAD",
			"Invalid webhook payload",
		)
		return
	case err != nil:
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"AUTH_PROVIDER_UNAVAILABLE",
			"Authentication provider is temporarily unavailable",
		)
		return
	}
	if _, _, err := h.webhookInbox.ReceiveWebhookEvent(r.Context(), platformiam.WebhookEvent{
		Provider:   "clerk",
		ExternalID: event.ID,
		EventType:  string(event.Type),
		Payload:    append([]byte(nil), payload...),
		OccurredAt: event.Timestamp,
	}); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "IAM is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h *IAMAPI) writeUnavailable(w http.ResponseWriter) {
	code := "AUTH_PROVIDER_UNAVAILABLE"
	message := "Authentication provider is temporarily unavailable"
	if !h.clerk.Configured() {
		code = "AUTH_NOT_CONFIGURED"
		message = "Authentication is not configured"
	}
	writeAPIError(w, http.StatusServiceUnavailable, code, message)
}

func (h *IAMAPI) verifyOrganizationSession(
	w http.ResponseWriter,
	r *http.Request,
) (clerkadapter.SessionClaims, bool) {
	if !h.clerk.Configured() || h.verifier == nil || h.transactor == nil {
		h.writeUnavailable(w)
		return clerkadapter.SessionClaims{}, false
	}
	token, err := bearerToken(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid session token")
		return clerkadapter.SessionClaims{}, false
	}
	claims, err := h.verifier.VerifySession(r.Context(), token)
	switch {
	case errors.Is(err, clerkadapter.ErrOrganizationRequired):
		writeAPIError(
			w,
			http.StatusForbidden,
			"AUTH_ORGANIZATION_REQUIRED",
			"An active organization is required",
		)
		return clerkadapter.SessionClaims{}, false
	case errors.Is(err, clerkadapter.ErrInvalidSessionToken),
		errors.Is(err, clerkadapter.ErrPendingSession):
		writeAPIError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid session token")
		return clerkadapter.SessionClaims{}, false
	case err != nil:
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"AUTH_PROVIDER_UNAVAILABLE",
			"Authentication provider is temporarily unavailable",
		)
		return clerkadapter.SessionClaims{}, false
	default:
		return claims, true
	}
}

func (h *IAMAPI) verifyIdentity(
	w http.ResponseWriter,
	r *http.Request,
) (clerkadapter.SessionClaims, bool) {
	if !h.clerk.Configured() || h.identityVerifier == nil {
		h.writeUnavailable(w)
		return clerkadapter.SessionClaims{}, false
	}
	token, err := bearerToken(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid session token")
		return clerkadapter.SessionClaims{}, false
	}
	claims, err := h.identityVerifier.VerifyIdentity(r.Context(), token)
	switch {
	case errors.Is(err, clerkadapter.ErrInvalidSessionToken),
		errors.Is(err, clerkadapter.ErrPendingSession),
		errors.Is(err, clerkadapter.ErrOrganizationRequired):
		writeAPIError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid session token")
		return clerkadapter.SessionClaims{}, false
	case err != nil:
		h.writeClerkError(w, err)
		return clerkadapter.SessionClaims{}, false
	default:
		return claims, true
	}
}

func (h *IAMAPI) writeClerkError(w http.ResponseWriter, err error) {
	if clerkadapter.IsRateLimited(err) {
		var providerError *clerkadapter.APIError
		if errors.As(err, &providerError) {
			if delay, ok := providerError.RetryAfter(); ok {
				seconds := int64((delay + time.Second - 1) / time.Second)
				w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
			}
		}
		writeAPIError(
			w,
			http.StatusTooManyRequests,
			"AUTH_PROVIDER_RATE_LIMITED",
			"Authentication provider rate limit reached",
		)
		return
	}
	writeAPIError(
		w,
		http.StatusServiceUnavailable,
		"AUTH_PROVIDER_UNAVAILABLE",
		"Authentication provider is temporarily unavailable",
	)
}

func bearerToken(r *http.Request) (string, error) {
	if r == nil {
		return "", errors.New("request is nil")
	}
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return "", errors.New("exactly one Authorization header is required")
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") ||
		strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("invalid bearer authorization")
	}
	return parts[1], nil
}

func pageBounds(cursor *api.Cursor, limit *api.Limit, total int) (int, int, *string, error) {
	pageSize := 25
	if limit != nil {
		pageSize = int(*limit)
	}
	if pageSize < 1 || pageSize > 100 {
		return 0, 0, nil, fmt.Errorf("limit must be between 1 and 100")
	}
	start := 0
	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(*cursor))
		if err != nil {
			return 0, 0, nil, fmt.Errorf("invalid cursor")
		}
		start, err = strconv.Atoi(string(decoded))
		if err != nil || start < 0 {
			return 0, 0, nil, fmt.Errorf("invalid cursor")
		}
	}
	if start > total {
		return 0, 0, nil, fmt.Errorf("cursor is outside the result set")
	}
	end := min(start+pageSize, total)
	var nextCursor *string
	if end < total {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
		nextCursor = &encoded
	}
	return start, end, nextCursor, nil
}

func loadCurrentSession(
	ctx context.Context,
	tx pgx.Tx,
	active platformiam.ActiveMembership,
	claims clerkadapter.SessionClaims,
) (api.CurrentSession, error) {
	var (
		userID         string
		email          string
		displayName    string
		avatarURL      string
		organizationID string
		name           string
		slug           string
		externalID     string
		membershipID   string
		localRole      string
		status         string
	)
	err := tx.QueryRow(ctx, `
		SELECT
			iam_user.id::text,
			iam_user.primary_email,
			iam_user.name,
			coalesce(iam_user.avatar_url, ''),
			organization.id::text,
			organization.name,
			coalesce(organization.slug, ''),
			coalesce(organization.external_id, ''),
			membership.id::text,
			membership.role,
			membership.status
		FROM iam.memberships AS membership
		JOIN iam.organizations AS organization
		  ON organization.id = membership.org_id
		JOIN iam.users AS iam_user
		  ON iam_user.id = membership.user_id
		WHERE membership.id = $1
		  AND membership.org_id = $2
		  AND membership.user_id = $3
	`, active.MembershipID, active.OrganizationID, active.UserID).Scan(
		&userID,
		&email,
		&displayName,
		&avatarURL,
		&organizationID,
		&name,
		&slug,
		&externalID,
		&membershipID,
		&localRole,
		&status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.CurrentSession{}, platformiam.ErrActiveMembershipRequired
	}
	if err != nil {
		return api.CurrentSession{}, err
	}
	if status != "active" {
		return api.CurrentSession{}, platformiam.ErrActiveMembershipRequired
	}

	parsedLocalRole, err := productiam.ParseRole(localRole)
	if err != nil {
		return api.CurrentSession{}, err
	}
	effectiveRole, err := productiam.EffectiveRole(parsedLocalRole, claims.OrganizationRole)
	if err != nil {
		return api.CurrentSession{}, err
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return api.CurrentSession{}, err
	}
	organizationUUID, err := uuid.Parse(organizationID)
	if err != nil {
		return api.CurrentSession{}, err
	}
	membershipUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return api.CurrentSession{}, err
	}

	permissions := productiam.Permissions(effectiveRole)
	permissionNames := make([]api.Permission, len(permissions))
	for index, permission := range permissions {
		permissionNames[index] = api.Permission(permission)
	}
	localAPIRole := api.Role(parsedLocalRole)
	effectiveAPIRole := api.Role(effectiveRole)
	response := api.CurrentSession{
		Membership: api.Membership{
			Id:     membershipUUID,
			Role:   localAPIRole,
			Status: api.MembershipStatusActive,
		},
		Organization: api.Organization{
			Id:         organizationUUID,
			Name:       name,
			Role:       localAPIRole,
			Slug:       slug,
			Status:     api.OrganizationStatusActive,
			SyncStatus: api.SyncStatusSynced,
		},
		Permissions: permissionNames,
		Role:        effectiveAPIRole,
		SessionId:   claims.SessionID,
		User: api.User{
			DisplayName: displayName,
			Email:       openapi_types.Email(email),
			Id:          userUUID,
		},
	}
	if avatarURL != "" {
		response.User.AvatarUrl = &avatarURL
	}
	if externalID != "" {
		response.Organization.SwitchKey = &externalID
	}
	return response, nil
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	response := api.ErrorResponse{}
	response.Error.Code = code
	response.Error.Message = message
	if requestID := strings.TrimSpace(w.Header().Get("X-Request-Id")); requestID != "" {
		response.Error.RequestId = &requestID
	}
	writeJSON(w, status, response)
}
