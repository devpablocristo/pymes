package wire

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/core/http/go/httperr"
	saasbilling "github.com/devpablocristo/core/saas/go/billing"
	billingdomain "github.com/devpablocristo/core/saas/go/billing/usecases/domain"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/authz"
)

// pymesSaaSHTTPAuth configura lectura segura del JWT en handlers (sync perezoso de perfil Clerk).
type pymesSaaSHTTPAuth struct {
	JWKSURL   string
	JWTIssuer string
}

func registerPymesSaaSRoutes(
	mux *http.ServeMux,
	store *pymesSaaSStore,
	authMW func(http.Handler) http.Handler,
	billingRuntime *saasbilling.Runtime,
	httpAuth pymesSaaSHTTPAuth,
) {
	registerPublic(mux, "GET /tenants", func(w http.ResponseWriter, r *http.Request) {
		handleListMyTenants(w, r, store, httpAuth)
	})
	registerPublic(mux, "POST /tenants", func(w http.ResponseWriter, r *http.Request) {
		handleCreateTenant(w, r, store, httpAuth)
	})
	registerPublic(mux, "POST /tenant-invites/accept", func(w http.ResponseWriter, r *http.Request) {
		handleAcceptTenantInvite(w, r, store, httpAuth)
	})
	registerPublic(mux, "GET /tenant-invites/preview", func(w http.ResponseWriter, r *http.Request) {
		handlePreviewTenantInvite(w, r, store)
	})
	// El email de invitación apunta a este endpoint (a través del `redirect_url`
	// que pasamos a Clerk al crear la invitation). Aceptamos el ticket
	// server-side via FAPI — esto cubre el caso "user invitado ya tiene sesión
	// Clerk activa", donde el SDK frontend no procesa el ticket — y luego
	// redirigimos al dashboard del tenant.
	registerPublic(mux, "GET /tenant-invites/exchange", func(w http.ResponseWriter, r *http.Request) {
		handleExchangeTenantInvite(w, r, store)
	})
	// Webhook receiver de Clerk. Verifica firma SVIX, persiste el evento en
	// `webhook_events_clerk` (idempotente por svix_id) y deja el dispatch a
	// fases siguientes (Phase 6.5+). Sin auth porque la firma SVIX es la
	// autorización (validada server-side con CLERK_WEBHOOK_SECRET).
	registerPublic(mux, "POST /webhooks/clerk", func(w http.ResponseWriter, r *http.Request) {
		handleClerkWebhook(w, r, store)
	})

	// Sesión de producto: envuelve el Principal del kernel con tenant_id + product_role.
	registerProtected(mux, authMW, "GET /session", func(w http.ResponseWriter, r *http.Request) {
		handleSessionEnriched(w, r, store)
	})

	registerProtected(mux, authMW, "GET /users/me", func(w http.ResponseWriter, r *http.Request) {
		handleGetMe(w, r, store, httpAuth)
	})
	registerProtected(mux, authMW, "PATCH /users/me/profile", func(w http.ResponseWriter, r *http.Request) {
		handlePatchMeProfile(w, r, store, httpAuth)
	})
	// Set inicial de password vía backend (Clerk SDK frontend rechaza el cambio
	// sin elevated auth para users que llegaron por ticket). El handler usa el
	// secret key de Clerk para PATCH /users/{id}, gateado a users que NO tienen
	// password configurado todavía. Es PÚBLICO (sin slug binding) porque la
	// pantalla aparece antes de que haya un tenant activo confirmado en el
	// frontend; valida el JWT manualmente contra JWKS para autenticar.
	registerPublic(mux, "POST /users/me/set-initial-password", func(w http.ResponseWriter, r *http.Request) {
		handleSetInitialPassword(w, r, store, httpAuth)
	})
	registerProtected(mux, authMW, "GET /tenants/{org_id}/members", func(w http.ResponseWriter, r *http.Request) {
		handleListMembers(w, r, store)
	})
	registerProtected(mux, authMW, "PATCH /tenants/{org_id}/members/{user_id}", func(w http.ResponseWriter, r *http.Request) {
		handleUpdateTenantMember(w, r, store)
	})
	registerProtected(mux, authMW, "DELETE /tenants/{org_id}/members/{user_id}", func(w http.ResponseWriter, r *http.Request) {
		handleRemoveTenantMember(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenants/{org_id}/ownership/transfer", func(w http.ResponseWriter, r *http.Request) {
		handleTransferTenantOwnership(w, r, store)
	})
	registerProtected(mux, authMW, "GET /tenants/{org_id}/invites", func(w http.ResponseWriter, r *http.Request) {
		handleListTenantInvites(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenants/{org_id}/invites", func(w http.ResponseWriter, r *http.Request) {
		handleCreateTenantInvite(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenant-invites/{invite_id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		handleRevokeTenantInvite(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenant-invites/{invite_id}/resend", func(w http.ResponseWriter, r *http.Request) {
		handleResendTenantInvite(w, r, store)
	})
	registerProtected(mux, authMW, "GET /tenants/{org_id}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		handleListAPIKeys(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenants/{org_id}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		handleCreateAPIKey(w, r, store)
	})
	registerProtected(mux, authMW, "DELETE /tenants/{org_id}/api-keys/{key_id}", func(w http.ResponseWriter, r *http.Request) {
		handleDeleteAPIKey(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenants/{org_id}/api-keys/{key_id}/rotate", func(w http.ResponseWriter, r *http.Request) {
		handleRotateAPIKey(w, r, store)
	})
	registerProtected(mux, authMW, "GET /billing/status", func(w http.ResponseWriter, r *http.Request) {
		handleBillingStatus(w, r, billingRuntime)
	})
	registerProtected(mux, authMW, "POST /billing/checkout", func(w http.ResponseWriter, r *http.Request) {
		handleBillingCheckout(w, r, billingRuntime)
	})
	registerProtected(mux, authMW, "POST /billing/portal", func(w http.ResponseWriter, r *http.Request) {
		handleBillingPortal(w, r, billingRuntime)
	})
}

func registerProtected(mux *http.ServeMux, authMW func(http.Handler) http.Handler, pattern string, next http.HandlerFunc) {
	if authMW == nil {
		mux.HandleFunc(pattern, next)
		return
	}
	mux.Handle(pattern, authMW(http.HandlerFunc(next)))
}

func registerPublic(mux *http.ServeMux, pattern string, next http.HandlerFunc) {
	mux.HandleFunc(pattern, next)
}

type createTenantRequest struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	ClerkOrgID string `json:"clerk_org_id"`
}

type createAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type billingCheckoutRequest struct {
	PlanCode   string `json:"plan_code"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

type billingPortalRequest struct {
	ReturnURL string `json:"return_url"`
}

type tenantInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type tenantMemberUpdateRequest struct {
	Role string `json:"role"`
}

type tenantOwnershipTransferRequest struct {
	UserID string `json:"user_id"`
}

type tenantInviteAcceptRequest struct {
	Token       string `json:"token"`
	ClerkTicket string `json:"clerk_ticket"`
}

func handleCreateTenant(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) {
	user, err := authenticatedClerkUser(r.Context(), r.Header.Get("Authorization"), httpAuth)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	user = store.enrichAuthenticatedClerkUser(r.Context(), user)
	var req createTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httperr.BadRequest(w, "name is required")
		return
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = slugifyTenantName(name)
	}
	if existing, role, ok, err := store.FindTenantBySlugForExternalUser(r.Context(), slug, user.ExternalID); err != nil {
		httperr.WriteFrom(w, err)
		return
	} else if ok {
		if role != "owner" && role != "admin" {
			httperr.WriteFrom(w, domainerr.Conflict("tenant slug already exists"))
			return
		}
		clerkOrgID := clerkTenantIDFromTenant(existing)
		if clerkOrgID == "" {
			httperr.WriteFrom(w, domainerr.Unavailable("tenant provisioning is missing its Clerk organization"))
			return
		}
		if store.clerk != nil {
			member, err := store.clerk.UserHasOrganizationMembership(r.Context(), clerkOrgID, user.ExternalID)
			if err != nil {
				httperr.WriteFrom(w, err)
				return
			}
			if !member {
				httperr.WriteFrom(w, domainerr.Forbidden("clerk tenant organization membership is required"))
				return
			}
		}
		created, err := store.CreateAPIKey(r.Context(), existing.ID.String(), "onboarding setup", nil)
		if err != nil {
			httperr.WriteFrom(w, err)
			return
		}
		writeCreateTenantResponse(w, http.StatusOK, existing.ID.String(), clerkOrgID, slug, created.Secret, "", created.APIKey)
		return
	}
	orgID, clerkOrgID, rawKey, key, scopes, err := store.CreateTenantWithClerkOrganization(r.Context(), name, slug, strings.TrimSpace(req.ClerkOrgID), user.ExternalID, user.Email, user.Name, user.AvatarURL)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	writeCreateTenantResponse(w, http.StatusCreated, orgID, clerkOrgID, slug, rawKey, key.KeyPrefix, tenantAPIKeyDTO{
		ID:        key.ID.String(),
		OrgID:  orgID,
		Name:      key.Name,
		Scopes:    scopes,
		CreatedAt: key.CreatedAt,
	})
}

func writeCreateTenantResponse(w http.ResponseWriter, status int, orgID, clerkOrgID, slug, rawKey, keyPrefix string, key tenantAPIKeyDTO) {
	httperr.WriteJSON(w, status, map[string]any{
		"org_id":    orgID,
		"clerk_org_id": strings.TrimSpace(clerkOrgID),
		"slug":         strings.TrimSpace(slug),
		"raw_key":      rawKey,
		"key": map[string]any{
			"id":         key.ID,
			"name":       key.Name,
			"key_prefix": strings.TrimSpace(keyPrefix),
			"scopes":     key.Scopes,
			"created_at": key.CreatedAt,
		},
	})
}

func handleListMyTenants(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) {
	user, err := authenticatedClerkUser(r.Context(), r.Header.Get("Authorization"), httpAuth)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	items, err := store.ListTenantsForUser(r.Context(), user.ExternalID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleSessionEnriched(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	principal, ok := tenantPrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	auth := map[string]any{
		"org_id":    principal.OrgID,
		"role":         principal.Role,
		"product_role": authz.ProductRole(principal.Role, principal.Scopes),
		"scopes":       principal.Scopes,
		"actor":        principal.Actor,
		"auth_method":  principal.AuthMethod,
	}
	tenant := map[string]any{
		"id": principal.OrgID,
	}
	membership := map[string]any{
		"role": principal.Role,
	}
	user := map[string]any{
		"external_id": principal.Actor,
	}
	if store != nil {
		name, slug, okName, err := store.GetTenantNameSlugByID(r.Context(), principal.OrgID)
		if err != nil {
			slog.Warn("session tenant name lookup", "err", err, "org_id", principal.OrgID)
		} else if okName {
			if strings.TrimSpace(name) != "" {
				auth["tenant_name"] = strings.TrimSpace(name)
				tenant["name"] = strings.TrimSpace(name)
			}
			if strings.TrimSpace(slug) != "" {
				auth["tenant_slug"] = strings.TrimSpace(slug)
				tenant["slug"] = strings.TrimSpace(slug)
			}
		}
		if settings, okSettings, err := store.loadTenantSettings(r.Context(), principal.OrgID); err != nil {
			slog.Warn("session tenant settings lookup", "err", err, "org_id", principal.OrgID)
		} else if okSettings {
			if vertical := strings.TrimSpace(settings.Vertical); vertical != "" {
				auth["vertical"] = vertical
			}
			if settings.OnboardingCompletedAt != nil {
				auth["onboarding_completed_at"] = settings.OnboardingCompletedAt
			}
		}
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{
		"auth":       auth,
		"tenant":     tenant,
		"membership": membership,
		"user":       user,
	})
}

func enrichMeProfileWithUserExtras(ctx context.Context, store *pymesSaaSStore, out map[string]any) (map[string]any, error) {
	if store == nil {
		return out, nil
	}
	uObj, ok := out["user"].(map[string]any)
	if !ok || uObj == nil {
		return out, nil
	}
	extID, _ := uObj["external_id"].(string)
	if strings.TrimSpace(extID) == "" {
		return out, nil
	}
	phone, givenName, familyName, _, err := store.GetUserProfileExtrasByExternalID(ctx, extID)
	if err != nil {
		return nil, err
	}
	uObj["phone"] = phone
	uObj["given_name"] = givenName
	uObj["family_name"] = familyName
	return out, nil
}

func writeEnrichedMeProfile(w http.ResponseWriter, ctx context.Context, store *pymesSaaSStore, profile map[string]any) {
	out, err := enrichMeProfileWithUserExtras(ctx, store, profile)
	if err != nil {
		slog.Warn("enrich me profile", "err", err)
		httperr.WriteJSON(w, http.StatusOK, profile)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, out)
}

func loadMeProfile(ctx context.Context, r *http.Request, principal tenantPrincipal, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) (map[string]any, error) {
	user, ok, err := store.FindUserByExternalID(ctx, principal.Actor)
	if err != nil {
		return nil, err
	}
	needsProfileHydration := !ok || isPlaceholderClerkEmail(user.Email) || isSyntheticClerkName(user.Name, principal.Actor)
	if principal.AuthMethod == "jwt" && needsProfileHydration && strings.TrimSpace(httpAuth.JWKSURL) != "" {
		authUser, authErr := authenticatedClerkUser(ctx, r.Header.Get("Authorization"), httpAuth)
		if authErr == nil && strings.TrimSpace(authUser.ExternalID) == strings.TrimSpace(principal.Actor) {
			authUser = store.enrichAuthenticatedClerkUser(ctx, authUser)
			user, err = store.UpsertUser(ctx, authUser.ExternalID, authUser.Email, authUser.Name, authUser.AvatarURL)
			ok = err == nil
		}
	}
	var userPayload any
	if ok {
		userPayload = user
	}
	return map[string]any{
		"user": userPayload,
		"membership": map[string]any{
			"org_id": principal.OrgID,
			"role":      principal.Role,
			"scopes":    principal.Scopes,
		},
	}, nil
}

func handleGetMe(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) {
	principal, ok := tenantPrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	profile, err := loadMeProfile(r.Context(), r, principal, store, httpAuth)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	writeEnrichedMeProfile(w, r.Context(), store, profile)
}

type setInitialPasswordRequest struct {
	Password string `json:"password"`
}

// handleSetInitialPassword fija una password al user actual vía Clerk Backend
// API. Solo permitido cuando el user todavía NO tiene password configurado —
// pensado para invitados que llegaron por ticket. Si ya tiene password debe
// usar el flow normal de Clerk (que requiere elevated auth).
//
// Endpoint público sin slug binding: la pantalla aparece antes de que haya
// tenant activo confirmado en frontend, así que validamos JWT manualmente
// contra JWKS (mismo patrón que `accept invite`).
func handleSetInitialPassword(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) {
	user, err := authenticatedClerkUser(r.Context(), r.Header.Get("Authorization"), httpAuth)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	clerkUserID := strings.TrimSpace(user.ExternalID)
	if clerkUserID == "" {
		httperr.Unauthorized(w, "missing clerk user id")
		return
	}
	if store.clerk == nil {
		httperr.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "clerk_unavailable", "message": "clerk client not configured"})
		return
	}
	var req setInitialPasswordRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	if validErr := validateInitialPassword(req.Password); validErr != nil {
		httperr.WriteFrom(w, validErr)
		return
	}
	profile, getErr := store.clerk.GetUser(r.Context(), clerkUserID)
	if getErr != nil {
		store.logger.Error("set initial password: get user failed", "clerk_user_id", clerkUserID, "err", getErr.Error())
		httperr.WriteFrom(w, getErr)
		return
	}
	if profile.PasswordEnabled {
		httperr.WriteJSON(w, http.StatusConflict, map[string]string{
			"code":    "password_already_set",
			"message": "user already has a password configured",
		})
		return
	}
	if setErr := store.clerk.SetUserPassword(r.Context(), clerkUserID, req.Password); setErr != nil {
		store.logger.Error("set initial password: clerk patch failed", "clerk_user_id", clerkUserID, "err", setErr.Error())
		httperr.WriteFrom(w, setErr)
		return
	}
	// Audit-log: el endpoint es público (autentica vía JWT manual) y permite
	// cambiar credenciales. Dejamos rastro de quién lo invocó para que un
	// posible abuso (JWT robado de invitado pre-password) sea detectable.
	store.logger.Info("user.password.set_initial",
		"clerk_user_id", clerkUserID,
		"ip", clientIPFromRequest(r),
		"user_agent", r.Header.Get("User-Agent"),
	)
	w.WriteHeader(http.StatusNoContent)
}

// clientIPFromRequest extrae la IP del cliente respetando X-Forwarded-For
// (típicamente seteado por el reverse proxy) y cae a r.RemoteAddr si no
// está. Devuelve la primera IP del XFF (la más cercana al cliente).
func clientIPFromRequest(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return xff
	}
	return r.RemoteAddr
}

func handlePatchMeProfile(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) {
	principal, ok := tenantPrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	if principal.AuthMethod != "jwt" {
		httperr.Forbidden(w, "profile update requires a user session (JWT)")
		return
	}
	var req PatchMeProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	if req.Name == nil && req.GivenName == nil && req.FamilyName == nil && req.Phone == nil {
		httperr.BadRequest(w, "no fields to update")
		return
	}
	err := store.PatchUserPersonalFromRequest(r.Context(), principal.Actor, &req)
	if errors.Is(err, ErrUserProfileNotFound) {
		httperr.NotFound(w, "user not found")
		return
	}
	if err != nil {
		msg := err.Error()
		switch msg {
		case "name cannot be empty", "name too long", "phone too long",
			"given name too long", "family name too long":
			httperr.BadRequest(w, msg)
		default:
			slog.Error("patch user profile failed", "error", err)
			httperr.WriteFrom(w, err)
		}
		return
	}
	profile, err := loadMeProfile(r.Context(), r, principal, store, httpAuth)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	writeEnrichedMeProfile(w, r.Context(), store, profile)
}

func handleListMembers(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantID(w, r)
	if !ok {
		return
	}
	items, err := store.ListTenantMembers(r.Context(), orgID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleListTenantInvites(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	items, err := store.ListTenantInvitations(r.Context(), orgID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleCreateTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	principal, _ := tenantPrincipalFromContext(r.Context())
	var req tenantInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	item, err := store.CreateTenantInvitation(r.Context(), orgID, principal.Actor, req.Email, req.Role)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusCreated, map[string]any{"invite": item})
}

func handlePreviewTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	item, err := store.PreviewTenantInvitation(r.Context(), token)
	if err != nil {
		var de domainerr.Error
		if errors.As(err, &de) && de.Message() == "invite_expired" {
			httperr.Write(w, http.StatusGone, "invite_expired", "invite_expired")
			return
		}
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"invite": item})
}

func handleAcceptTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) {
	user, err := authenticatedClerkUser(r.Context(), r.Header.Get("Authorization"), httpAuth)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	user = store.enrichAuthenticatedClerkUser(r.Context(), user)
	var req tenantInviteAcceptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	item, clerkTenantID, err := store.AcceptTenantInvitation(r.Context(), req.Token, strings.TrimSpace(req.ClerkTicket), user)
	if err != nil {
		var de domainerr.Error
		if errors.As(err, &de) && de.Message() == "invite_expired" {
			httperr.Write(w, http.StatusGone, "invite_expired", "invite_expired")
			return
		}
		httperr.WriteFrom(w, err)
		return
	}
	_, tenantSlug, _, err := store.GetTenantNameSlugByID(r.Context(), item.OrgID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"invite": item, "clerk_org_id": clerkTenantID, "tenant_slug": tenantSlug})
}

// handleExchangeTenantInvite es el endpoint público al que Clerk redirige el
// link del email (vía el `redirect_url` que pasamos al crear la invitation).
// Procesa el `__clerk_ticket` server-side contra la Frontend API de Clerk —
// esto resuelve el caso "user invitado ya tiene sesión Clerk activa", donde
// el SDK frontend no procesa el ticket — y luego redirige al dashboard del
// tenant. Si no hay ticket o algo falla, hace fallback al flow viejo del
// frontend (`/invite/accept?token=...`) para mantener compatibilidad.
func handleExchangeTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	ctx := r.Context()
	q := r.URL.Query()
	ticket := strings.TrimSpace(q.Get("__clerk_ticket"))
	token := strings.TrimSpace(q.Get("token"))
	if token == "" {
		redirectInviteFallback(w, r, store, "", "missing_token")
		return
	}
	if ticket == "" {
		// No hay ticket: signup nuevo lo maneja el frontend. Redirigimos al
		// flow viejo (Clerk SDK procesa el flujo de signup).
		redirectInviteFallback(w, r, store, token, "")
		return
	}

	preview, err := store.PreviewTenantInvitation(ctx, token)
	if err != nil {
		redirectInviteFallback(w, r, store, token, "preview_failed")
		return
	}
	if store.clerk == nil {
		redirectInviteFallback(w, r, store, token, "clerk_unavailable")
		return
	}

	// Buscar el user en Clerk por email. Si no existe, dejamos el flow legacy
	// (signup vía `<SignIn>` del SDK) para crearlo.
	clerkUserID, err := store.clerk.GetUserIDByEmail(ctx, preview.Email)
	if err != nil {
		redirectInviteFallback(w, r, store, token, "clerk_user_lookup_failed")
		return
	}
	if clerkUserID == "" {
		redirectInviteFallback(w, r, store, token, "")
		return
	}

	// Procesar el ticket server-side: marca la invitation Clerk como
	// `accepted` y agrega al user a la org.
	if err := store.clerk.AcceptOrganizationInvitationTicket(ctx, ticket); err != nil {
		redirectInviteFallback(w, r, store, token, "ticket_failed")
		return
	}

	// Hidratar perfil del user para el upsert local.
	profile, err := store.clerk.GetUser(ctx, clerkUserID)
	if err != nil {
		redirectInviteFallback(w, r, store, token, "profile_failed")
		return
	}
	var avatar *string
	if v := strings.TrimSpace(profile.ImageURL); v != "" {
		avatar = &v
	}
	user := clerkAuthenticatedUser{
		ExternalID: clerkUserID,
		Email:      profile.Email,
		Name:       profile.DisplayName(),
		AvatarURL:  avatar,
	}
	if strings.TrimSpace(user.Email) == "" {
		user.Email = preview.Email
	}

	// Aceptar en Pymes (crea row en `org_members`, marca invitation
	// `accepted`). Pasamos también el ticket por si la membership de Clerk
	// no se reflejó aún y el `UserHasOrganizationMembership` devuelve false:
	// el método interno re-procesa el ticket idempotentemente.
	item, clerkOrgID, err := store.AcceptTenantInvitation(ctx, token, ticket, user)
	if err != nil {
		redirectInviteFallback(w, r, store, token, "accept_failed")
		return
	}
	_, slug, _, err := store.GetTenantNameSlugByID(ctx, item.OrgID)
	if err != nil || strings.TrimSpace(slug) == "" {
		redirectInviteFallback(w, r, store, token, "tenant_lookup_failed")
		return
	}

	// Redirigir al dashboard del tenant. `activate_org` deja que App.tsx
	// haga `setActive` con la org recién agregada y resuelva la task
	// `choose-organization` de Clerk. Si el invitado no tiene password
	// configurado en Clerk (típico de users que entraron solo via ticket),
	// el frontend lo detecta directamente con `user.passwordEnabled === false`
	// del SDK Clerk y muestra `RequirePasswordView` antes del dashboard —
	// no hace falta propagar un query flag, que de hecho se perdía cuando
	// el flow pasaba por OnboardingPage.
	dest := store.resolvedFrontendURL()
	params := url.Values{}
	if strings.TrimSpace(clerkOrgID) != "" {
		params.Set("activate_org", clerkOrgID)
	}
	target := dest + "/" + slug + "/dashboard"
	if encoded := params.Encode(); encoded != "" {
		target += "?" + encoded
	}
	store.logger.Debug("invite exchange redirect",
		"tenant_slug", slug,
		"password_enabled", profile.PasswordEnabled,
	)
	http.Redirect(w, r, target, http.StatusFound)
}

// redirectInviteFallback envía el browser al flow viejo del frontend (que
// maneja signup nuevo via Clerk SDK). Mantiene la compatibilidad cuando no
// podemos procesar el ticket server-side.
func redirectInviteFallback(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, token, reason string) {
	base := store.resolvedFrontendURL()
	dest, err := url.Parse(base + "/invite/accept")
	if err != nil {
		http.Redirect(w, r, base+"/invite/accept", http.StatusFound)
		return
	}
	q := dest.Query()
	if token != "" {
		q.Set("token", token)
	}
	// Reenvía los params de Clerk para que el SDK frontend procese el flujo
	// (signup nuevo o intento alternativo).
	if v := strings.TrimSpace(r.URL.Query().Get("__clerk_ticket")); v != "" {
		q.Set("__clerk_ticket", v)
	}
	if v := strings.TrimSpace(r.URL.Query().Get("__clerk_status")); v != "" {
		q.Set("__clerk_status", v)
	}
	if reason != "" {
		q.Set("exchange_error", reason)
	}
	dest.RawQuery = q.Encode()
	http.Redirect(w, r, dest.String(), http.StatusFound)
}

func handleRevokeTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	principal, okPrincipal := tenantPrincipalFromContext(r.Context())
	if !okPrincipal {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	orgID := principal.OrgID
	if _, err := store.requireTenantOwner(r.Context(), orgID, principal.Actor); err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	item, err := store.RevokeTenantInvitation(r.Context(), orgID, strings.TrimSpace(r.PathValue("invite_id")), principal.Actor)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"invite": item})
}

func handleResendTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	principal, okPrincipal := tenantPrincipalFromContext(r.Context())
	if !okPrincipal {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	orgID := principal.OrgID
	if _, err := store.requireTenantOwner(r.Context(), orgID, principal.Actor); err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	item, err := store.ResendTenantInvitation(r.Context(), orgID, strings.TrimSpace(r.PathValue("invite_id")), principal.Actor)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"invite": item})
}

func handleUpdateTenantMember(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	var req tenantMemberUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	item, err := store.UpdateTenantMemberRole(r.Context(), orgID, strings.TrimSpace(r.PathValue("user_id")), req.Role)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"member": item})
}

func handleRemoveTenantMember(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	userID := strings.TrimSpace(r.PathValue("user_id"))
	if err := store.RemoveTenantMember(r.Context(), orgID, userID); err != nil {
		store.logger.Error("remove tenant member failed",
			"org_id", orgID,
			"user_id", userID,
			"err", err.Error(),
		)
		httperr.WriteFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleTransferTenantOwnership(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	principal, _ := tenantPrincipalFromContext(r.Context())
	var req tenantOwnershipTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	if err := store.TransferTenantOwnership(r.Context(), orgID, principal.Actor, req.UserID); err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleListAPIKeys(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	rows, err := store.listAPIKeyRows(r.Context(), orgID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		scopes, err := store.loadKeyScopes(r.Context(), row.ID)
		if err != nil {
			httperr.WriteFrom(w, err)
			return
		}
		items = append(items, map[string]any{
			"id":         row.ID.String(),
			"name":       row.Name,
			"key_prefix": row.KeyPrefix,
			"scopes":     scopes,
			"created_at": row.CreatedAt,
		})
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleCreateAPIKey(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	created, err := store.CreateAPIKey(r.Context(), orgID, req.Name, req.Scopes)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusCreated, map[string]any{
		"key": map[string]any{
			"id":         created.APIKey.ID,
			"name":       created.APIKey.Name,
			"key_prefix": prefixFromSecret(created.Secret),
			"scopes":     created.APIKey.Scopes,
			"created_at": created.APIKey.CreatedAt,
		},
		"raw_key": created.Secret,
	})
}

func handleRotateAPIKey(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	rotated, err := store.RotateAPIKey(r.Context(), orgID, strings.TrimSpace(r.PathValue("key_id")))
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{
		"key": map[string]any{
			"id":         rotated.APIKey.ID,
			"name":       rotated.APIKey.Name,
			"key_prefix": prefixFromSecret(rotated.Secret),
			"scopes":     rotated.APIKey.Scopes,
			"created_at": rotated.APIKey.CreatedAt,
		},
		"raw_key": rotated.Secret,
	})
}

func handleDeleteAPIKey(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedTenantIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	if err := store.DeleteAPIKey(r.Context(), orgID, strings.TrimSpace(r.PathValue("key_id"))); err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleBillingStatus(w http.ResponseWriter, r *http.Request, runtime *saasbilling.Runtime) {
	principal, ok := tenantPrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	status, err := runtime.GetBillingStatus(r.Context(), principal.OrgID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{
		"org_id":          principal.OrgID,
		"plan_code":          status.PlanCode,
		"status":             status.BillingStatus,
		"hard_limits":        status.HardLimits,
		"usage":              status.Usage,
		"current_period_end": status.CurrentPeriodEnd,
	})
}

func handleBillingCheckout(w http.ResponseWriter, r *http.Request, runtime *saasbilling.Runtime) {
	principal, ok := tenantPrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	var req billingCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	url, err := runtime.CreateCheckoutSession(r.Context(), billingdomain.CheckoutInput{
		TenantID:   principal.OrgID,
		PlanCode:   billingdomain.PlanCode(strings.TrimSpace(req.PlanCode)),
		SuccessURL: strings.TrimSpace(req.SuccessURL),
		CancelURL:  strings.TrimSpace(req.CancelURL),
		Actor:      nullableString(principal.Actor),
	})
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"checkout_url": url})
}

func handleBillingPortal(w http.ResponseWriter, r *http.Request, runtime *saasbilling.Runtime) {
	principal, ok := tenantPrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	var req billingPortalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	url, err := runtime.CreatePortalSession(r.Context(), billingdomain.PortalInput{
		TenantID:  principal.OrgID,
		ReturnURL: strings.TrimSpace(req.ReturnURL),
		Actor:     nullableString(principal.Actor),
	})
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"portal_url": url})
}

func authorizedTenantID(w http.ResponseWriter, r *http.Request) (string, bool) {
	principal, ok := tenantPrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return "", false
	}
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	if orgID == "" {
		httperr.BadRequest(w, "org_id is required")
		return "", false
	}
	if principal.OrgID != orgID {
		httperr.Write(w, http.StatusForbidden, "FORBIDDEN", "cross-tenant access denied")
		return "", false
	}
	return orgID, true
}

// apiKeyManagementAllowed alinea con la consola: solo privilegiados o scopes admin de consola.
func apiKeyManagementAllowed(principal tenantPrincipal) bool {
	return authz.CanManageAPIKeys(principal.Role, principal.Scopes, principal.AuthMethod)
}

// authorizedTenantIDForAPIKeyManagement exige además de coincidencia de tenant, permisos de admin de producto.
func authorizedTenantIDForAPIKeyManagement(w http.ResponseWriter, r *http.Request) (string, bool) {
	principal, ok := tenantPrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return "", false
	}
	if !apiKeyManagementAllowed(principal) {
		httperr.Write(w, http.StatusForbidden, "FORBIDDEN", "api key management requires admin privileges")
		return "", false
	}
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	if orgID == "" {
		httperr.BadRequest(w, "org_id is required")
		return "", false
	}
	if principal.OrgID != orgID {
		httperr.Write(w, http.StatusForbidden, "FORBIDDEN", "cross-tenant access denied")
		return "", false
	}
	return orgID, true
}

func authorizedTenantOwner(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) (string, bool) {
	orgID, ok := authorizedTenantID(w, r)
	if !ok {
		return "", false
	}
	principal, _ := tenantPrincipalFromContext(r.Context())
	if _, err := store.requireTenantOwner(r.Context(), orgID, principal.Actor); err != nil {
		httperr.WriteFrom(w, err)
		return "", false
	}
	return orgID, true
}

func prefixFromSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if len(secret) <= 12 {
		return secret
	}
	return secret[:12]
}

func nullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
