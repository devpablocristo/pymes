package wire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devpablocristo/platform/errors/go/domainerr"
)

const clerkBackendAPIBaseURL = "https://api.clerk.com/v1"

// clerkJSVersion es el query param `_clerk_js_version` que el FAPI exige
// para procesar `sign_ins?strategy=ticket`. Lo emulamos tal como lo manda
// el SDK frontend; si Clerk deprecara la versión hay que bumpearla acá.
const clerkJSVersion = "6.8.0"

// clerkAPIError es el error tipado que devuelven los métodos del cliente
// Clerk cuando la API responde con status >= 400. Permite a los callers
// ramificar con errors.As en vez de string matching sobre el mensaje.
//
// Wraps domainerr.UpstreamError vía Unwrap, así errors.Is/IsKind contra
// domainerr siguen funcionando para callers que solo necesitan el kind.
type clerkAPIError struct {
	StatusCode int
	// Code es Errors[0].Code de la API de Clerk si la respuesta fue parseable.
	// Vacío si Clerk no devolvió un código (ej. error de red, body no-JSON).
	Code  string
	inner domainerr.Error
}

func (e *clerkAPIError) Error() string { return e.inner.Error() }
func (e *clerkAPIError) Unwrap() error { return e.inner }

// asClerkAPIError extrae un *clerkAPIError de la cadena de wrapping, si existe.
func asClerkAPIError(err error) (*clerkAPIError, bool) {
	var apiErr *clerkAPIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

type clerkTenantClient interface {
	CreateOrganization(ctx context.Context, input clerkCreateOrganizationInput) (clerkOrganization, error)
	CreateOrganizationInvitation(ctx context.Context, input clerkCreateOrganizationInvitationInput) (clerkOrganizationInvitation, error)
	CreateOrganizationMembership(ctx context.Context, organizationID, userID, role string) error
	GetUser(ctx context.Context, userID string) (clerkUserProfile, error)
	GetUserIDByEmail(ctx context.Context, email string) (string, error)
	DeleteOrganization(ctx context.Context, organizationID string) error
	DeleteOrganizationMembership(ctx context.Context, organizationID, userID string) error
	RevokeOrganizationInvitation(ctx context.Context, input clerkRevokeOrganizationInvitationInput) error
	UserHasOrganizationMembership(ctx context.Context, organizationID, userID string) (bool, error)
	AcceptOrganizationInvitationTicket(ctx context.Context, ticket string) error
	SetUserPassword(ctx context.Context, userID, password string) error
}

type clerkCreateOrganizationInput struct {
	Name           string
	CreatedBy      string
	PublicMetadata map[string]any
}

type clerkOrganization struct {
	ID   string
	Name string
}

type clerkUserProfile struct {
	ID              string
	Email           string
	FirstName       string
	LastName        string
	Name            string
	ImageURL        string
	PasswordEnabled bool
}

func (p clerkUserProfile) DisplayName() string {
	if name := strings.TrimSpace(p.Name); name != "" {
		return name
	}
	first := strings.TrimSpace(p.FirstName)
	last := strings.TrimSpace(p.LastName)
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

type clerkCreateOrganizationInvitationInput struct {
	OrganizationID string
	InviterUserID  string
	Email          string
	Role           string
	RedirectURL    string
	PublicMetadata map[string]any
}

type clerkOrganizationInvitation struct {
	ID        string
	URL       string
	ExpiresAt *time.Time
}

type clerkRevokeOrganizationInvitationInput struct {
	OrganizationID   string
	InvitationID     string
	RequestingUserID string
}

type clerkBackendClient struct {
	secretKey       string
	baseURL         string
	frontendBaseURL string
	httpClient      *http.Client
}

func newClerkBackendClient(secretKey, jwksURL string) clerkTenantClient {
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return nil
	}
	return &clerkBackendClient{
		secretKey:       secretKey,
		baseURL:         clerkBackendAPIBaseURL,
		frontendBaseURL: deriveClerkFrontendBaseURL(jwksURL),
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

// deriveClerkFrontendBaseURL extracts the FAPI base URL (e.g.
// "https://selected-tick-48.clerk.accounts.dev") from the configured JWKS_URL
// (e.g. "https://selected-tick-48.clerk.accounts.dev/.well-known/jwks.json").
// Returns "" if the JWKS URL is empty or malformed; callers must handle that.
func deriveClerkFrontendBaseURL(jwksURL string) string {
	jwksURL = strings.TrimSpace(jwksURL)
	if jwksURL == "" {
		return ""
	}
	parsed, err := url.Parse(jwksURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (c *clerkBackendClient) CreateOrganization(ctx context.Context, input clerkCreateOrganizationInput) (clerkOrganization, error) {
	payload := map[string]any{
		"name":       strings.TrimSpace(input.Name),
		"created_by": strings.TrimSpace(input.CreatedBy),
	}
	if len(input.PublicMetadata) > 0 {
		payload["public_metadata"] = input.PublicMetadata
	}
	var out struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/organizations", payload, &out); err != nil {
		return clerkOrganization{}, err
	}
	if strings.TrimSpace(out.ID) == "" {
		return clerkOrganization{}, domainerr.UpstreamError("clerk organization response missing id")
	}
	return clerkOrganization{
		ID:   strings.TrimSpace(out.ID),
		Name: strings.TrimSpace(out.Name),
	}, nil
}

func (c *clerkBackendClient) CreateOrganizationInvitation(ctx context.Context, input clerkCreateOrganizationInvitationInput) (clerkOrganizationInvitation, error) {
	tenantID := strings.TrimSpace(input.OrganizationID)
	payload := map[string]any{
		"inviter_user_id": strings.TrimSpace(input.InviterUserID),
		"email_address":   strings.TrimSpace(input.Email),
		"role":            strings.TrimSpace(input.Role),
	}
	if redirectURL := strings.TrimSpace(input.RedirectURL); redirectURL != "" {
		payload["redirect_url"] = redirectURL
	}
	if len(input.PublicMetadata) > 0 {
		payload["public_metadata"] = input.PublicMetadata
	}
	var out struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		ExpiresAt any    `json:"expires_at"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/organizations/"+url.PathEscape(tenantID)+"/invitations", payload, &out); err != nil {
		return clerkOrganizationInvitation{}, err
	}
	if strings.TrimSpace(out.ID) == "" {
		return clerkOrganizationInvitation{}, domainerr.UpstreamError("clerk invitation response missing id")
	}
	// Clerk Backend API responde siempre snake_case (`expires_at`). El TTL
	// local del row (tenantInviteTTL en saas_store_invitations.go) es la
	// fuente de verdad, así que si por alguna razón Clerk no devuelve el
	// campo, el row mantiene su propio expires_at sin romper nada.
	return clerkOrganizationInvitation{
		ID:        strings.TrimSpace(out.ID),
		URL:       strings.TrimSpace(out.URL),
		ExpiresAt: parseClerkTime(out.ExpiresAt),
	}, nil
}

func (c *clerkBackendClient) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	email = normalizeEmail(email)
	if email == "" {
		return "", nil
	}
	var users []struct {
		ID string `json:"id"`
	}
	q := "/users?email_address=" + url.QueryEscape(email) + "&limit=1"
	if err := c.doJSON(ctx, http.MethodGet, q, nil, &users); err != nil {
		return "", err
	}
	if len(users) == 0 {
		return "", nil
	}
	return strings.TrimSpace(users[0].ID), nil
}

func (c *clerkBackendClient) GetUser(ctx context.Context, userID string) (clerkUserProfile, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return clerkUserProfile{}, domainerr.Validation("clerk user_id is required")
	}
	var out struct {
		ID                    string `json:"id"`
		FirstName             string `json:"first_name"`
		LastName              string `json:"last_name"`
		Username              string `json:"username"`
		ImageURL              string `json:"image_url"`
		ProfileImageURL       string `json:"profile_image_url"`
		PasswordEnabled       bool   `json:"password_enabled"`
		PrimaryEmailAddressID string `json:"primary_email_address_id"`
		EmailAddresses        []struct {
			ID           string `json:"id"`
			EmailAddress string `json:"email_address"`
		} `json:"email_addresses"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/users/"+url.PathEscape(userID), nil, &out); err != nil {
		return clerkUserProfile{}, err
	}
	email := ""
	for _, item := range out.EmailAddresses {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(out.PrimaryEmailAddressID) {
			email = strings.TrimSpace(item.EmailAddress)
			break
		}
	}
	if email == "" && len(out.EmailAddresses) > 0 {
		email = strings.TrimSpace(out.EmailAddresses[0].EmailAddress)
	}
	imageURL := strings.TrimSpace(out.ImageURL)
	if imageURL == "" {
		imageURL = strings.TrimSpace(out.ProfileImageURL)
	}
	return clerkUserProfile{
		ID:              strings.TrimSpace(out.ID),
		Email:           normalizeEmail(email),
		FirstName:       strings.TrimSpace(out.FirstName),
		LastName:        strings.TrimSpace(out.LastName),
		Name:            strings.TrimSpace(out.Username),
		ImageURL:        imageURL,
		PasswordEnabled: out.PasswordEnabled,
	}, nil
}

// minInitialPasswordLength es la longitud mínima del primer password setup
// para invitados. Coincide con la default de Clerk Dashboard; si lo cambias
// acá hay que alinearlo en el dashboard también.
const minInitialPasswordLength = 8

// validateInitialPassword chequea la política mínima del primer password
// setup. Devuelve un domainerr.Validation listo para devolver al caller.
// Centraliza la regla para que cliente Clerk y handler HTTP no la dupliquen.
func validateInitialPassword(password string) error {
	if len(password) < minInitialPasswordLength {
		return domainerr.Validation(fmt.Sprintf("password must be at least %d characters", minInitialPasswordLength))
	}
	return nil
}

// SetUserPassword fija una password al user vía Clerk Backend API. Pensado
// para el flow de "primer setup" del invitado que entró por ticket — el SDK
// frontend rechaza el cambio sin elevated auth, así que delegamos al backend
// con secret key. Si el user YA tiene password, Clerk respondería 422
// (validación contra cambios sin current_password); el flow lo previene
// gateando contra `password_enabled` en el caller.
func (c *clerkBackendClient) SetUserPassword(ctx context.Context, userID, password string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domainerr.Validation("clerk user_id is required")
	}
	if err := validateInitialPassword(password); err != nil {
		return err
	}
	payload := map[string]any{"password": password}
	return c.doJSON(ctx, http.MethodPatch, "/users/"+url.PathEscape(userID), payload, nil)
}

func (c *clerkBackendClient) DeleteOrganization(ctx context.Context, organizationID string) error {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil
	}
	return c.doJSON(ctx, http.MethodDelete, "/organizations/"+url.PathEscape(organizationID), nil, nil)
}

func (c *clerkBackendClient) DeleteOrganizationMembership(ctx context.Context, organizationID, userID string) error {
	organizationID = strings.TrimSpace(organizationID)
	userID = strings.TrimSpace(userID)
	if organizationID == "" || userID == "" {
		return nil
	}
	err := c.doJSON(ctx, http.MethodDelete, "/organizations/"+url.PathEscape(organizationID)+"/memberships/"+url.PathEscape(userID), nil, nil)
	if apiErr, ok := asClerkAPIError(err); ok && apiErr.StatusCode == http.StatusNotFound {
		// Idempotente: si la membership no existe en Clerk (drift Clerk↔Pymes),
		// igual completamos la baja en Pymes. Sin esto, el botón Eliminar tiraba
		// 502 cuando la membership ya había sido removida fuera de banda.
		return nil
	}
	return err
}

func (c *clerkBackendClient) RevokeOrganizationInvitation(ctx context.Context, input clerkRevokeOrganizationInvitationInput) error {
	tenantID := strings.TrimSpace(input.OrganizationID)
	invID := strings.TrimSpace(input.InvitationID)
	payload := map[string]any{"requesting_user_id": strings.TrimSpace(input.RequestingUserID)}
	return c.doJSON(ctx, http.MethodPost, "/organizations/"+url.PathEscape(tenantID)+"/invitations/"+url.PathEscape(invID)+"/revoke", payload, nil)
}

func (c *clerkBackendClient) CreateOrganizationMembership(ctx context.Context, organizationID, userID, role string) error {
	organizationID = strings.TrimSpace(organizationID)
	userID = strings.TrimSpace(userID)
	role = strings.TrimSpace(role)
	if organizationID == "" || userID == "" || role == "" {
		return nil
	}
	payload := map[string]any{"user_id": userID, "role": role}
	return c.doJSON(ctx, http.MethodPost, "/organizations/"+url.PathEscape(organizationID)+"/memberships", payload, nil)
}

// AcceptOrganizationInvitationTicket processes an organization invitation ticket
// against the Frontend API of Clerk. This is what the Clerk JS SDK does
// internally when it sees `__clerk_ticket=...` in the URL: it POSTs to
// `/v1/client/sign_ins?strategy=ticket&ticket=...` on the FAPI host. The call
// requires no authentication (FAPI is public). On success the invitation moves
// to status `accepted` and the user becomes a member of the organization.
//
// We need this server-side because when the invited user already has an active
// Clerk session in the browser, the SDK shows "You're already signed in" and
// never processes the ticket. By calling this endpoint from the backend with
// only the ticket (extracted from the email link), we bypass the SDK entirely.
//
// Returns nil if the ticket was already accepted (idempotent), so callers can
// invoke this safely even when the membership might already exist.
func (c *clerkBackendClient) AcceptOrganizationInvitationTicket(ctx context.Context, ticket string) error {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return nil
	}
	if strings.TrimSpace(c.frontendBaseURL) == "" {
		return domainerr.Unavailable("clerk frontend api base url is not configured")
	}
	endpoint := strings.TrimRight(c.frontendBaseURL, "/") + "/v1/client/sign_ins?strategy=ticket&ticket=" + url.QueryEscape(ticket) + "&_clerk_js_version=" + clerkJSVersion
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domainerr.UpstreamError("clerk frontend api request failed")
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	// 400 with code `organization_invitation_already_accepted` is expected on
	// retries — el ticket sólo se consume una vez. Tratamos como éxito.
	code := clerkErrorCode(data)
	if resp.StatusCode == http.StatusBadRequest && code == "organization_invitation_already_accepted" {
		return nil
	}
	message := clerkErrorMessage(data)
	if message == "" {
		message = fmt.Sprintf("clerk frontend api returned %d", resp.StatusCode)
	}
	return &clerkAPIError{
		StatusCode: resp.StatusCode,
		Code:       code,
		inner:      domainerr.UpstreamError(message),
	}
}

func (c *clerkBackendClient) UserHasOrganizationMembership(ctx context.Context, organizationID, userID string) (bool, error) {
	organizationID = strings.TrimSpace(organizationID)
	userID = strings.TrimSpace(userID)
	if organizationID == "" || userID == "" {
		return false, nil
	}
	// Consultamos desde la perspectiva del usuario: el filter `user_id[]` en
	// `/organizations/{id}/memberships` no aplica filtro y devuelve todos los
	// miembros, lo que daría falsos positivos.
	var out struct {
		Data []struct {
			Organization struct {
				ID string `json:"id"`
			} `json:"organization"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/users/"+url.PathEscape(userID)+"/organization_memberships?limit=200", nil, &out); err != nil {
		return false, err
	}
	for _, m := range out.Data {
		if strings.TrimSpace(m.Organization.ID) == organizationID {
			return true, nil
		}
	}
	return false, nil
}

func (c *clerkBackendClient) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domainerr.UpstreamError("clerk request failed")
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := clerkErrorMessage(data)
		if message == "" {
			message = fmt.Sprintf("clerk returned %d", resp.StatusCode)
		} else {
			message = fmt.Sprintf("clerk returned %d: %s", resp.StatusCode, message)
		}
		return &clerkAPIError{
			StatusCode: resp.StatusCode,
			Code:       clerkErrorCode(data),
			inner:      domainerr.UpstreamError(message),
		}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return domainerr.UpstreamError("invalid clerk response")
	}
	return nil
}

// clerkErrorCode extrae el primer Errors[].Code de un response de error Clerk.
// Vacío si el body no parsea como JSON o no incluye códigos.
func clerkErrorCode(data []byte) string {
	var out struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if len(data) == 0 || json.Unmarshal(data, &out) != nil {
		return ""
	}
	for _, item := range out.Errors {
		if code := strings.TrimSpace(item.Code); code != "" {
			return code
		}
	}
	return ""
}

func clerkErrorMessage(data []byte) string {
	var out struct {
		Message string `json:"message"`
		Errors  []struct {
			Message     string `json:"message"`
			LongMessage string `json:"long_message"`
			Code        string `json:"code"`
		} `json:"errors"`
	}
	if len(data) == 0 || json.Unmarshal(data, &out) != nil {
		return ""
	}
	if msg := strings.TrimSpace(out.Message); msg != "" {
		return msg
	}
	for _, item := range out.Errors {
		if msg := strings.TrimSpace(item.LongMessage); msg != "" {
			return msg
		}
		if msg := strings.TrimSpace(item.Message); msg != "" {
			return msg
		}
		if code := strings.TrimSpace(item.Code); code != "" {
			return code
		}
	}
	return ""
}

func parseClerkTime(raw any) *time.Time {
	switch v := raw.(type) {
	case float64:
		t := time.UnixMilli(int64(v)).UTC()
		return &t
	case string:
		if v == "" {
			return nil
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			utc := t.UTC()
			return &utc
		}
	}
	return nil
}
