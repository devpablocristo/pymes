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
	GetUser(ctx context.Context, userID string) (clerkUserProfile, error)
	DeleteOrganization(ctx context.Context, organizationID string) error
	DeleteOrganizationMembership(ctx context.Context, organizationID, userID string) error
	RevokeOrganizationInvitation(ctx context.Context, input clerkRevokeOrganizationInvitationInput) error
	UserHasOrganizationMembership(ctx context.Context, organizationID, userID string) (bool, error)
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
	ID        string
	Email     string
	FirstName string
	LastName  string
	Name      string
	ImageURL  string
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
		ID             string `json:"id"`
		URL            string `json:"url"`
		ExpiresAt      any    `json:"expires_at"`
		ExpiresAtCamel any    `json:"expiresAt"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/organizations/"+url.PathEscape(tenantID)+"/invitations", payload, &out); err != nil {
		return clerkOrganizationInvitation{}, err
	}
	if strings.TrimSpace(out.ID) == "" {
		return clerkOrganizationInvitation{}, domainerr.UpstreamError("clerk invitation response missing id")
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
		ID:        strings.TrimSpace(out.ID),
		Email:     normalizeEmail(email),
		FirstName: strings.TrimSpace(out.FirstName),
		LastName:  strings.TrimSpace(out.LastName),
		Name:      strings.TrimSpace(out.Username),
		ImageURL:  imageURL,
	}, nil
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
	return c.doJSON(ctx, http.MethodDelete, "/organizations/"+url.PathEscape(organizationID)+"/memberships/"+url.PathEscape(userID), nil, nil)
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
		message := clerkErrorMessage(data)
		if message == "" {
			message = fmt.Sprintf("clerk returned %d", resp.StatusCode)
		} else {
			message = fmt.Sprintf("clerk returned %d: %s", resp.StatusCode, message)
		}
		return domainerr.UpstreamError(message)
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return domainerr.UpstreamError("invalid clerk response")
	}
	return nil
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
