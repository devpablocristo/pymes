package wire

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authn "github.com/devpablocristo/core/authn/go"
	"github.com/devpablocristo/core/config/go/envconfig"
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
	registerProtected(mux, authMW, "GET /tenants/{tenant_id}/members", func(w http.ResponseWriter, r *http.Request) {
		handleListMembers(w, r, store)
	})
	registerProtected(mux, authMW, "PATCH /tenants/{tenant_id}/members/{user_id}", func(w http.ResponseWriter, r *http.Request) {
		handleUpdateTenantMember(w, r, store)
	})
	registerProtected(mux, authMW, "DELETE /tenants/{tenant_id}/members/{user_id}", func(w http.ResponseWriter, r *http.Request) {
		handleRemoveTenantMember(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenants/{tenant_id}/ownership/transfer", func(w http.ResponseWriter, r *http.Request) {
		handleTransferTenantOwnership(w, r, store)
	})
	registerProtected(mux, authMW, "GET /tenants/{tenant_id}/invites", func(w http.ResponseWriter, r *http.Request) {
		handleListTenantInvites(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenants/{tenant_id}/invites", func(w http.ResponseWriter, r *http.Request) {
		handleCreateTenantInvite(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenant-invites/{invite_id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		handleRevokeTenantInvite(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenant-invites/{invite_id}/resend", func(w http.ResponseWriter, r *http.Request) {
		handleResendTenantInvite(w, r, store)
	})
	registerProtected(mux, authMW, "GET /tenants/{tenant_id}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		handleListAPIKeys(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenants/{tenant_id}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		handleCreateAPIKey(w, r, store)
	})
	registerProtected(mux, authMW, "DELETE /tenants/{tenant_id}/api-keys/{key_id}", func(w http.ResponseWriter, r *http.Request) {
		handleDeleteAPIKey(w, r, store)
	})
	registerProtected(mux, authMW, "POST /tenants/{tenant_id}/api-keys/{key_id}/rotate", func(w http.ResponseWriter, r *http.Request) {
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
	Name string `json:"name"`
	Slug string `json:"slug"`
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
	Token string `json:"token"`
}

func handleCreateTenant(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) {
	user, err := authenticatedClerkUser(r.Context(), r.Header.Get("Authorization"), httpAuth)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
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
		created, err := store.CreateAPIKey(r.Context(), existing.ID.String(), "onboarding setup", nil)
		if err != nil {
			httperr.WriteFrom(w, err)
			return
		}
		writeCreateTenantResponse(w, http.StatusOK, existing.ID.String(), optionalString(existing.ClerkOrgID), slug, created.Secret, "", created.APIKey)
		return
	}
	if store.clerk == nil {
		httperr.WriteFrom(w, domainerr.Unavailable("clerk backend client is not configured"))
		return
	}
	clerkOrgID := ""
	clerkOrg, err := store.clerk.CreateOrganization(r.Context(), clerkCreateOrganizationInput{
		Name:      name,
		Slug:      slug,
		CreatedBy: user.ExternalID,
	})
	if err != nil {
		if !envconfig.IsLocal(store.environment) {
			httperr.WriteFrom(w, err)
			return
		}
		slog.Warn("clerk organization create failed in local environment; creating tenant with local membership only", "err", err, "slug", slug)
	}
	clerkOrgID = strings.TrimSpace(clerkOrg.ID)
	tenantID, rawKey, key, scopes, err := store.CreateTenantWithOwner(r.Context(), name, slug, clerkOrgID, user.ExternalID, user.Email, user.Name, user.AvatarURL)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	writeCreateTenantResponse(w, http.StatusCreated, tenantID, clerkOrgID, slug, rawKey, key.KeyPrefix, tenantAPIKeyDTO{
		ID:        key.ID.String(),
		TenantID:  tenantID,
		Name:      key.Name,
		Scopes:    scopes,
		CreatedAt: key.CreatedAt,
	})
}

func writeCreateTenantResponse(w http.ResponseWriter, status int, tenantID, clerkOrgID, slug, rawKey, keyPrefix string, key tenantAPIKeyDTO) {
	httperr.WriteJSON(w, status, map[string]any{
		"tenant_id":    tenantID,
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

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
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
		"tenant_id":    principal.TenantID,
		"role":         principal.Role,
		"product_role": authz.ProductRole(principal.Role, principal.Scopes),
		"scopes":       principal.Scopes,
		"actor":        principal.Actor,
		"auth_method":  principal.AuthMethod,
	}
	tenant := map[string]any{
		"id": principal.TenantID,
	}
	membership := map[string]any{
		"role": principal.Role,
	}
	user := map[string]any{
		"external_id": principal.Actor,
	}
	if store != nil {
		name, slug, okName, err := store.GetTenantNameSlugByID(r.Context(), principal.TenantID)
		if err != nil {
			slog.Warn("session tenant name lookup", "err", err, "tenant_id", principal.TenantID)
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
		if settings, okSettings, err := store.loadTenantSettings(r.Context(), principal.TenantID); err != nil {
			slog.Warn("session tenant settings lookup", "err", err, "tenant_id", principal.TenantID)
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
	if principal.AuthMethod == "jwt" && !ok && strings.TrimSpace(httpAuth.JWKSURL) != "" {
		raw, bearerOK := authn.BearerToken(r.Header.Get("Authorization"))
		if bearerOK && strings.TrimSpace(raw) != "" {
			if claims, vErr := verifyJWTClaimsMap(ctx, raw, httpAuth.JWKSURL, httpAuth.JWTIssuer); vErr == nil {
				if sub := stringClaim(claims, "sub"); sub != "" && sub == strings.TrimSpace(principal.Actor) {
					email := clerkEmailFromClaims(claims)
					name := clerkDisplayNameFromClaims(claims)
					if email == "" {
						email = placeholderClerkEmail(principal.Actor)
					}
					if name == "" {
						name = "User"
					}
					user, err = store.UpsertUser(ctx, principal.Actor, email, name, nil)
					ok = err == nil
				}
			}
		}
	}
	var userPayload any
	if ok {
		userPayload = user
	}
	return map[string]any{
		"user": userPayload,
		"membership": map[string]any{
			"tenant_id": principal.TenantID,
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
	tenantID, ok := authorizedTenantID(w, r)
	if !ok {
		return
	}
	items, err := store.ListTenantMembers(r.Context(), tenantID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleListTenantInvites(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	tenantID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	items, err := store.ListTenantInvitations(r.Context(), tenantID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleCreateTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	tenantID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	principal, _ := tenantPrincipalFromContext(r.Context())
	var req tenantInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	item, err := store.CreateTenantInvitation(r.Context(), tenantID, principal.Actor, req.Email, req.Role)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusCreated, map[string]any{"invite": item})
}

func handleAcceptTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore, httpAuth pymesSaaSHTTPAuth) {
	user, err := authenticatedClerkUser(r.Context(), r.Header.Get("Authorization"), httpAuth)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	var req tenantInviteAcceptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	item, clerkTenantID, err := store.AcceptTenantInvitation(r.Context(), req.Token, user)
	if err != nil {
		var de domainerr.Error
		if errors.As(err, &de) && de.Message() == "invite_expired" {
			httperr.Write(w, http.StatusGone, "invite_expired", "invite_expired")
			return
		}
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"invite": item, "clerk_org_id": clerkTenantID})
}

func handleRevokeTenantInvite(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	principal, okPrincipal := tenantPrincipalFromContext(r.Context())
	if !okPrincipal {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	tenantID := principal.TenantID
	if _, err := store.requireTenantOwner(r.Context(), tenantID, principal.Actor); err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	item, err := store.RevokeTenantInvitation(r.Context(), tenantID, strings.TrimSpace(r.PathValue("invite_id")), principal.Actor)
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
	tenantID := principal.TenantID
	if _, err := store.requireTenantOwner(r.Context(), tenantID, principal.Actor); err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	item, err := store.ResendTenantInvitation(r.Context(), tenantID, strings.TrimSpace(r.PathValue("invite_id")), principal.Actor)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"invite": item})
}

func handleUpdateTenantMember(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	tenantID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	var req tenantMemberUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	item, err := store.UpdateTenantMemberRole(r.Context(), tenantID, strings.TrimSpace(r.PathValue("user_id")), req.Role)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"member": item})
}

func handleRemoveTenantMember(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	tenantID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	if err := store.RemoveTenantMember(r.Context(), tenantID, strings.TrimSpace(r.PathValue("user_id"))); err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleTransferTenantOwnership(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	tenantID, ok := authorizedTenantOwner(w, r, store)
	if !ok {
		return
	}
	principal, _ := tenantPrincipalFromContext(r.Context())
	var req tenantOwnershipTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	if err := store.TransferTenantOwnership(r.Context(), tenantID, principal.Actor, req.UserID); err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleListAPIKeys(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	tenantID, ok := authorizedTenantIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	rows, err := store.listAPIKeyRows(r.Context(), tenantID)
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
	tenantID, ok := authorizedTenantIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	created, err := store.CreateAPIKey(r.Context(), tenantID, req.Name, req.Scopes)
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
	tenantID, ok := authorizedTenantIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	rotated, err := store.RotateAPIKey(r.Context(), tenantID, strings.TrimSpace(r.PathValue("key_id")))
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
	tenantID, ok := authorizedTenantIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	if err := store.DeleteAPIKey(r.Context(), tenantID, strings.TrimSpace(r.PathValue("key_id"))); err != nil {
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
	status, err := runtime.GetBillingStatus(r.Context(), principal.TenantID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{
		"tenant_id":          principal.TenantID,
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
		TenantID:   principal.TenantID,
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
		TenantID:  principal.TenantID,
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
	tenantID := strings.TrimSpace(r.PathValue("tenant_id"))
	if tenantID == "" {
		httperr.BadRequest(w, "tenant_id is required")
		return "", false
	}
	if principal.TenantID != tenantID {
		httperr.Write(w, http.StatusForbidden, "FORBIDDEN", "cross-tenant access denied")
		return "", false
	}
	return tenantID, true
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
	tenantID := strings.TrimSpace(r.PathValue("tenant_id"))
	if tenantID == "" {
		httperr.BadRequest(w, "tenant_id is required")
		return "", false
	}
	if principal.TenantID != tenantID {
		httperr.Write(w, http.StatusForbidden, "FORBIDDEN", "cross-tenant access denied")
		return "", false
	}
	return tenantID, true
}

func authorizedTenantOwner(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) (string, bool) {
	tenantID, ok := authorizedTenantID(w, r)
	if !ok {
		return "", false
	}
	principal, _ := tenantPrincipalFromContext(r.Context())
	if _, err := store.requireTenantOwner(r.Context(), tenantID, principal.Actor); err != nil {
		httperr.WriteFrom(w, err)
		return "", false
	}
	return tenantID, true
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
