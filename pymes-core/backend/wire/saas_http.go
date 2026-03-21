package wire

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/backend/go/httperr"
	saasbilling "github.com/devpablocristo/core/saas/go/billing"
	billingdomain "github.com/devpablocristo/core/saas/go/billing/usecases/domain"
	kerneldomain "github.com/devpablocristo/core/saas/go/kernel/usecases/domain"
	saasmiddleware "github.com/devpablocristo/core/saas/go/middleware"
	saasusers "github.com/devpablocristo/core/saas/go/users"
	"github.com/devpablocristo/core/saas/go/users/handler/dto"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/authz"
)

func registerPymesSaaSRoutes(mux *http.ServeMux, store *pymesSaaSStore, authMW func(http.Handler) http.Handler, billingRuntime *saasbilling.Runtime) {
	registerPublic(mux, "POST /orgs", func(w http.ResponseWriter, r *http.Request) {
		handleCreateOrg(w, r, store)
	})

	// Sesión de producto: envuelve el Principal del kernel (core/saas/go/session) con org_id + product_role.
	registerProtected(mux, authMW, "GET /session", handleSessionEnriched)

	registerProtected(mux, authMW, "GET /users/me", func(w http.ResponseWriter, r *http.Request) {
		handleGetMe(w, r, store)
	})
	registerProtected(mux, authMW, "GET /orgs/{org_id}/members", func(w http.ResponseWriter, r *http.Request) {
		handleListMembers(w, r, store)
	})
	registerProtected(mux, authMW, "GET /orgs/{org_id}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		handleListAPIKeys(w, r, store)
	})
	registerProtected(mux, authMW, "POST /orgs/{org_id}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		handleCreateAPIKey(w, r, store)
	})
	registerProtected(mux, authMW, "DELETE /orgs/{org_id}/api-keys/{key_id}", func(w http.ResponseWriter, r *http.Request) {
		handleDeleteAPIKey(w, r, store)
	})
	registerProtected(mux, authMW, "POST /orgs/{org_id}/api-keys/{key_id}/rotate", func(w http.ResponseWriter, r *http.Request) {
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

func handleCreateOrg(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	var req struct {
		Name  string `json:"name"`
		Slug  string `json:"slug"`
		Actor string `json:"actor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, "invalid request body")
		return
	}
	orgID, rawKey, key, scopes, err := store.CreateOrgWithDefaultKey(r.Context(), req.Name, req.Slug, req.Actor)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusCreated, map[string]any{
		"org_id":  orgID,
		"raw_key": rawKey,
		"key": map[string]any{
			"id":         key.ID.String(),
			"name":       key.Name,
			"key_prefix": key.KeyPrefix,
			"scopes":     scopes,
			"created_at": key.CreatedAt,
		},
	})
}

func handleSessionEnriched(w http.ResponseWriter, r *http.Request) {
	principal, ok := saasmiddleware.PrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{
		"auth": map[string]any{
			"org_id":       principal.TenantID,
			"tenant_id":    principal.TenantID,
			"role":         principal.Role,
			"product_role": authz.ProductRole(principal.Role),
			"scopes":       principal.Scopes,
			"actor":        principal.Actor,
			"auth_method":  principal.AuthMethod,
		},
	})
}

func handleGetMe(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	principal, ok := saasmiddleware.PrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	usersUC := saasusers.NewUseCases(store)
	resp, err := saasusers.NewHandler(usersUC).GetMe(r.Context(), dto.GetMeRequest{
		OrgID:      principal.TenantID,
		ExternalID: principal.Actor,
		Role:       principal.Role,
		Scopes:     principal.Scopes,
	})
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, resp.Profile)
}

func handleListMembers(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedOrgID(w, r)
	if !ok {
		return
	}
	items, err := store.ListOrgMembers(r.Context(), orgID)
	if err != nil {
		httperr.WriteFrom(w, err)
		return
	}
	httperr.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleListAPIKeys(w http.ResponseWriter, r *http.Request, store *pymesSaaSStore) {
	orgID, ok := authorizedOrgIDForAPIKeyManagement(w, r)
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
	orgID, ok := authorizedOrgIDForAPIKeyManagement(w, r)
	if !ok {
		return
	}
	var req struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
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
	orgID, ok := authorizedOrgIDForAPIKeyManagement(w, r)
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
	orgID, ok := authorizedOrgIDForAPIKeyManagement(w, r)
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
	principal, ok := saasmiddleware.PrincipalFromContext(r.Context())
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
		"org_id":             principal.TenantID,
		"plan_code":          status.PlanCode,
		"status":             status.BillingStatus,
		"hard_limits":        status.HardLimits,
		"usage":              status.Usage,
		"current_period_end": status.CurrentPeriodEnd,
	})
}

func handleBillingCheckout(w http.ResponseWriter, r *http.Request, runtime *saasbilling.Runtime) {
	principal, ok := saasmiddleware.PrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	var req struct {
		PlanCode   string `json:"plan_code"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
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
	principal, ok := saasmiddleware.PrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return
	}
	var req struct {
		ReturnURL string `json:"return_url"`
	}
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

func authorizedOrgID(w http.ResponseWriter, r *http.Request) (string, bool) {
	principal, ok := saasmiddleware.PrincipalFromContext(r.Context())
	if !ok {
		httperr.Unauthorized(w, "principal not found")
		return "", false
	}
	orgID := strings.TrimSpace(r.PathValue("org_id"))
	if orgID == "" {
		httperr.BadRequest(w, "org_id is required")
		return "", false
	}
	if principal.TenantID != orgID {
		httperr.Write(w, http.StatusForbidden, httperr.CodeUnauthorized, "cross-org access denied")
		return "", false
	}
	return orgID, true
}

// apiKeyManagementAllowed alinea con la consola: solo privilegiados o scopes admin de consola.
func apiKeyManagementAllowed(principal kerneldomain.Principal) bool {
	return authz.IsAdmin(principal.Role, principal.Scopes)
}

// authorizedOrgIDForAPIKeyManagement exige además de coincidencia de tenant, permisos de admin de producto.
func authorizedOrgIDForAPIKeyManagement(w http.ResponseWriter, r *http.Request) (string, bool) {
	principal, ok := saasmiddleware.PrincipalFromContext(r.Context())
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
	if principal.TenantID != orgID {
		httperr.Write(w, http.StatusForbidden, httperr.CodeUnauthorized, "cross-org access denied")
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
