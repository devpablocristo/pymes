// Package governanceproxy proxies policy/approval requests from Pymes to Nexus Governance API.
package governanceproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/platform/http/go/httpclient"
	"github.com/devpablocristo/platform/kernels/governance/go/governanceclient"
)

// Client wraps core/governance/go. Product code only passes tenant IDs; this
// adapter owns Nexus' current tenant-scope wire header.
type Client struct {
	core   *governanceclient.Client
	caller *httpclient.Caller
}

type submitRequestWireBody struct {
	RequesterType  string         `json:"requester_type"`
	RequesterID    string         `json:"requester_id"`
	RequesterName  string         `json:"requester_name,omitempty"`
	ActionType     string         `json:"action_type"`
	TargetSystem   string         `json:"target_system,omitempty"`
	TargetResource string         `json:"target_resource,omitempty"`
	ActionBinding  map[string]any `json:"action_binding,omitempty"`
	Params         map[string]any `json:"params,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	Context        string         `json:"context,omitempty"`
}

// NewClient crea un nuevo cliente HTTP hacia Nexus Governance.
func NewClient(baseURL, apiKey string) *Client {
	header := make(http.Header)
	if strings.TrimSpace(apiKey) != "" {
		header.Set("X-API-Key", strings.TrimSpace(apiKey))
	}
	return &Client{
		core: governanceclient.NewClient(baseURL, apiKey),
		caller: &httpclient.Caller{
			BaseURL:     baseURL,
			Header:      header,
			HTTP:        &http.Client{Timeout: 30 * time.Second},
			MaxBodySize: 1 << 20,
		},
	}
}

func (c *Client) SubmitRequestForTenant(ctx context.Context, tenantID, idempotencyKey string, body governanceclient.SubmitRequestBody) (governanceclient.SubmitResponse, error) {
	opts := nexusTenantScopeOpts(tenantID)
	if strings.TrimSpace(idempotencyKey) != "" {
		opts = append(opts, httpclient.WithIdempotencyKey(strings.TrimSpace(idempotencyKey)))
	}

	var out governanceclient.SubmitResponse
	st, raw, err := c.caller.DoJSON(ctx, http.MethodPost, "/v1/requests", submitWireBody(body), opts...)
	if err != nil {
		return out, fmt.Errorf("governance submit: %w", err)
	}
	if st != http.StatusCreated {
		return out, fmt.Errorf("governance submit: status %d body %s", st, governanceclient.ParseErrorBody(raw))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode submit response: %w", err)
	}
	return out, nil
}

func submitWireBody(body governanceclient.SubmitRequestBody) submitRequestWireBody {
	return submitRequestWireBody{
		RequesterType:  body.RequesterType,
		RequesterID:    body.RequesterID,
		RequesterName:  body.RequesterName,
		ActionType:     body.ActionType,
		TargetSystem:   body.TargetSystem,
		TargetResource: body.TargetResource,
		ActionBinding:  actionBindingFromParams(body.Params),
		Params:         body.Params,
		Reason:         body.Reason,
		Context:        body.Context,
	}
}

func actionBindingFromParams(params map[string]any) map[string]any {
	if len(params) == 0 {
		return nil
	}
	raw, ok := params["action_binding"]
	if !ok {
		return nil
	}
	binding, ok := raw.(map[string]any)
	if !ok || len(binding) == 0 {
		return nil
	}
	out := make(map[string]any, len(binding))
	for key, value := range binding {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func (c *Client) SimulateRequestForTenant(ctx context.Context, tenantID string, body governanceclient.SimulateRequestBody) (governanceclient.SimulateResponse, error) {
	return c.core.SimulateRequest(ctx, body, governanceclient.WithOrgID(strings.TrimSpace(tenantID)))
}

func (c *Client) GetRequestForTenant(ctx context.Context, tenantID, id string) (governanceclient.RequestSummary, int, error) {
	var out governanceclient.RequestSummary
	st, raw, err := c.caller.DoJSON(ctx, http.MethodGet, "/v1/requests/"+strings.TrimSpace(id), nil, nexusTenantScopeOpts(tenantID)...)
	if err != nil {
		return out, 0, fmt.Errorf("governance get request: %w", err)
	}
	if st == http.StatusNotFound {
		return out, st, nil
	}
	if st != http.StatusOK {
		return out, st, fmt.Errorf("governance get request: status %d body %s", st, governanceclient.ParseErrorBody(raw))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, st, fmt.Errorf("decode get response: %w", err)
	}
	return out, st, nil
}

func (c *Client) ListPoliciesForTenant(ctx context.Context, tenantID string) (int, []byte, error) {
	return c.core.ListPolicies(ctx, governanceclient.WithOrgID(strings.TrimSpace(tenantID)))
}

func (c *Client) GetPolicyForTenant(ctx context.Context, tenantID, id string) (int, []byte, error) {
	return c.core.GetPolicy(ctx, strings.TrimSpace(id), governanceclient.WithOrgID(strings.TrimSpace(tenantID)))
}

func (c *Client) CreatePolicyForTenant(ctx context.Context, tenantID string, body any) (int, []byte, error) {
	return c.core.CreatePolicy(ctx, body, governanceclient.WithOrgID(strings.TrimSpace(tenantID)))
}

func (c *Client) UpdatePolicyForTenant(ctx context.Context, tenantID, id string, body any) (int, []byte, error) {
	return c.core.UpdatePolicy(ctx, strings.TrimSpace(id), body, governanceclient.WithOrgID(strings.TrimSpace(tenantID)))
}

func (c *Client) DeletePolicyForTenant(ctx context.Context, tenantID, id string) (int, error) {
	return c.core.DeletePolicy(ctx, strings.TrimSpace(id), governanceclient.WithOrgID(strings.TrimSpace(tenantID)))
}

func (c *Client) ListPendingApprovalsForTenant(ctx context.Context, tenantID string) (int, []byte, error) {
	return c.caller.DoJSON(ctx, http.MethodGet, "/v1/approvals/pending", nil, nexusTenantScopeOpts(tenantID)...)
}

func (c *Client) ApproveForTenant(ctx context.Context, tenantID, id string, body any) (int, []byte, error) {
	return c.caller.DoJSON(ctx, http.MethodPost, "/v1/approvals/"+strings.TrimSpace(id)+"/approve", body, nexusTenantScopeOpts(tenantID)...)
}

func (c *Client) RejectForTenant(ctx context.Context, tenantID, id string, body any) (int, []byte, error) {
	return c.caller.DoJSON(ctx, http.MethodPost, "/v1/approvals/"+strings.TrimSpace(id)+"/reject", body, nexusTenantScopeOpts(tenantID)...)
}

func (c *Client) ListActionTypes(ctx context.Context) (int, []byte, error) {
	return c.core.ListActionTypes(ctx)
}

func (c *Client) ListPendingApprovals(ctx context.Context) (int, []byte, error) {
	return c.core.ListPendingApprovals(ctx)
}

func (c *Client) GetRequest(ctx context.Context, id string) (governanceclient.RequestSummary, int, error) {
	return c.core.GetRequest(ctx, id)
}

func nexusTenantScopeOpts(tenantID string) []httpclient.RequestOption {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil
	}
	return []httpclient.RequestOption{httpclient.WithHeader("X-Org-ID", tenantID)}
}
