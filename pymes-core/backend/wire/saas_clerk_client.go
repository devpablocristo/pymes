package wire

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
)

const clerkBackendAPIBaseURL = "https://api.clerk.com/v1"

type clerkTenantClient interface {
	CreateOrganization(ctx context.Context, input clerkCreateOrganizationInput) (clerkOrganization, error)
	CreateOrganizationInvitation(ctx context.Context, input clerkCreateOrganizationInvitationInput) (clerkOrganizationInvitation, error)
	RevokeOrganizationInvitation(ctx context.Context, input clerkRevokeOrganizationInvitationInput) error
	UserHasOrganizationMembership(ctx context.Context, organizationID, userID string) (bool, error)
}

type clerkCreateOrganizationInput struct {
	Name      string
	Slug      string
	CreatedBy string
}

type clerkOrganization struct {
	ID string
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
	secretKey  string
	baseURL    string
	httpClient *http.Client
}

func newClerkBackendClient(secretKey string) clerkTenantClient {
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return nil
	}
	return &clerkBackendClient{
		secretKey:  secretKey,
		baseURL:    clerkBackendAPIBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *clerkBackendClient) CreateOrganization(ctx context.Context, input clerkCreateOrganizationInput) (clerkOrganization, error) {
	payload := map[string]any{
		"name":       strings.TrimSpace(input.Name),
		"created_by": strings.TrimSpace(input.CreatedBy),
	}
	if slug := strings.TrimSpace(input.Slug); slug != "" {
		payload["slug"] = slug
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/organizations", payload, &out); err != nil {
		return clerkOrganization{}, err
	}
	if strings.TrimSpace(out.ID) == "" {
		return clerkOrganization{}, domainerr.UpstreamError("clerk organization response missing id")
	}
	return clerkOrganization{ID: strings.TrimSpace(out.ID)}, nil
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
		ID             string `json:"id"`
		URL            string `json:"url"`
		ExpiresAt      any    `json:"expires_at"`
		ExpiresAtCamel any    `json:"expiresAt"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/organizations/"+url.PathEscape(tenantID)+"/invitations", payload, &out); err != nil {
		return clerkOrganizationInvitation{}, err
	}
	expiresAt := parseClerkTime(out.ExpiresAt)
	if expiresAt == nil {
		expiresAt = parseClerkTime(out.ExpiresAtCamel)
	}
	return clerkOrganizationInvitation{
		ID:        strings.TrimSpace(out.ID),
		URL:       strings.TrimSpace(out.URL),
		ExpiresAt: expiresAt,
	}, nil
}

func (c *clerkBackendClient) RevokeOrganizationInvitation(ctx context.Context, input clerkRevokeOrganizationInvitationInput) error {
	tenantID := strings.TrimSpace(input.OrganizationID)
	invID := strings.TrimSpace(input.InvitationID)
	payload := map[string]any{"requesting_user_id": strings.TrimSpace(input.RequestingUserID)}
	return c.doJSON(ctx, http.MethodPost, "/organizations/"+url.PathEscape(tenantID)+"/invitations/"+url.PathEscape(invID)+"/revoke", payload, nil)
}

func (c *clerkBackendClient) UserHasOrganizationMembership(ctx context.Context, organizationID, userID string) (bool, error) {
	u := "/organizations/" + url.PathEscape(strings.TrimSpace(organizationID)) + "/memberships"
	q := url.Values{}
	q.Set("limit", "1")
	q.Add("user_id[]", strings.TrimSpace(userID))
	var out struct {
		Data            []json.RawMessage `json:"data"`
		TotalCount      int               `json:"total_count"`
		TotalCountCamel int               `json:"totalCount"`
	}
	if err := c.doJSON(ctx, http.MethodGet, u+"?"+q.Encode(), nil, &out); err != nil {
		return false, err
	}
	return out.TotalCount > 0 || out.TotalCountCamel > 0 || len(out.Data) > 0, nil
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
		return domainerr.UpstreamError(fmt.Sprintf("clerk returned %d", resp.StatusCode))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return domainerr.UpstreamError("invalid clerk response")
	}
	return nil
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
