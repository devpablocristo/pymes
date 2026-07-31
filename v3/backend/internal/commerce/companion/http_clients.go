// Package companion contains HTTP adapters for private commerce dependencies.
package companion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
)

type HTTPAccountingClient struct {
	BaseURL string
	Client  HTTPDoer
	Tokens  identityaccess.TokenSource
}

func (c HTTPAccountingClient) ProvisionOrganization(ctx context.Context, organization organizationdomain.Organization) error {
	key := "provision-organization:" + organization.ID + ":1"
	request, err := c.request(ctx, http.MethodPut, "/internal/v1/organizations/"+url.PathEscape(organization.ID), organization.ID, key, key, map[string]string{"organization_id": organization.ID, "display_name": organization.Name})
	if err != nil {
		return err
	}
	response, err := c.client().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return fmt.Errorf("accounting provision returned %s", response.Status)
	}
	return nil
}
func (c HTTPAccountingClient) Post(ctx context.Context, command domain.PostingCommand) (domain.AccountingEvent, error) {
	lines := make([]map[string]any, 0, len(command.Lines))
	for _, line := range command.Lines {
		currency := line.Debit.Currency
		if currency == "" {
			currency = line.Credit.Currency
		}
		lines = append(lines, map[string]any{
			"account_code": line.AccountCode, "debit": line.Debit.Amount, "credit": line.Credit.Amount,
			"currency": currency, "memo": line.Memo, "open_item": line.OpenItem, "party_ref": line.PartyRef,
		})
	}
	payload := map[string]any{"command_id": command.CommandID, "organization_id": command.OrganizationID, "source": map[string]any{"type": command.SourceType, "id": command.SourceID, "version": command.SourceVersion, "digest": command.SnapshotDigest}, "effective_at": command.EffectiveAt.UTC(), "description": command.Description, "lines": lines}
	request, err := c.request(ctx, http.MethodPost, "/internal/v1/organizations/"+url.PathEscape(command.OrganizationID)+"/posting-commands", command.OrganizationID, command.CommandID, fallback(command.CorrelationID, command.CommandID), payload)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	response, err := c.client().Do(request)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return domain.AccountingEvent{}, decodeServiceError("accounting post", response)
	}
	var result domain.AccountingEvent
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return domain.AccountingEvent{}, err
	}
	return result, nil
}

func (c HTTPAccountingClient) Reverse(ctx context.Context, command domain.ReversalCommand) (domain.AccountingEvent, error) {
	payload := map[string]any{
		"command_id": command.CommandID, "original_journal_entry_id": command.OriginalJournalEntryID,
		"effective_at": command.EffectiveAt.UTC(), "reason": command.Reason,
	}
	request, err := c.request(ctx, http.MethodPost, "/internal/v1/organizations/"+url.PathEscape(command.OrganizationID)+"/reversals", command.OrganizationID, command.CommandID, fallback(command.CorrelationID, command.CommandID), payload)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	response, err := c.client().Do(request)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return domain.AccountingEvent{}, decodeServiceError("accounting reversal", response)
	}
	var result domain.AccountingEvent
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return domain.AccountingEvent{}, err
	}
	return result, nil
}

func (c HTTPAccountingClient) ApplyOpenItem(ctx context.Context, command domain.AccountingApplicationCommand) (domain.AccountingEvent, error) {
	payload := map[string]any{
		"command_id": command.CommandID, "debit_open_item_id": command.DebitOpenItemID,
		"credit_open_item_id": command.CreditOpenItemID, "amount": command.Amount,
		"applied_at": command.AppliedAt.UTC(),
	}
	request, err := c.request(ctx, http.MethodPost, "/internal/v1/organizations/"+url.PathEscape(command.OrganizationID)+"/open-item-applications", command.OrganizationID, command.CommandID, fallback(command.CorrelationID, command.CommandID), payload)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	response, err := c.client().Do(request)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return domain.AccountingEvent{}, decodeServiceError("accounting application", response)
	}
	var result domain.AccountingEvent
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return domain.AccountingEvent{}, err
	}
	return result, nil
}

func (c HTTPAccountingClient) ReverseOpenItemApplication(ctx context.Context, command domain.AccountingApplicationReversalCommand) (domain.AccountingEvent, error) {
	payload := map[string]any{
		"command_id": command.CommandID, "application_id": command.ApplicationID,
		"reversed_at": command.ReversedAt.UTC(), "reason": command.Reason,
	}
	request, err := c.request(ctx, http.MethodPost, "/internal/v1/organizations/"+url.PathEscape(command.OrganizationID)+"/open-item-application-reversals", command.OrganizationID, command.CommandID, fallback(command.CorrelationID, command.CommandID), payload)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	response, err := c.client().Do(request)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return domain.AccountingEvent{}, decodeServiceError("accounting application reversal", response)
	}
	var result domain.AccountingEvent
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return domain.AccountingEvent{}, err
	}
	return result, nil
}
func (c HTTPAccountingClient) request(ctx context.Context, method, path, organizationID, idempotencyKey, correlationID string, value any) (*http.Request, error) {
	return internalRequest(ctx, c.BaseURL, method, path, organizationID, "accounting", idempotencyKey, correlationID, value, c.Tokens)
}
func (c HTTPAccountingClient) client() HTTPDoer {
	if c.Client != nil {
		return c.Client
	}
	return NewServiceHTTPClient()
}

type HTTPFiscalClient struct {
	BaseURL string
	Client  HTTPDoer
	Tokens  identityaccess.TokenSource
}

func (c HTTPFiscalClient) Authorize(ctx context.Context, fiscal domain.FiscalRequest) (domain.FiscalResult, error) {
	return c.call(ctx, "/internal/v1/organizations/"+url.PathEscape(fiscal.OrganizationID)+"/authorizations", fiscal, fiscal.RequestID)
}
func (c HTTPFiscalClient) Consult(ctx context.Context, fiscal domain.FiscalRequest) (domain.FiscalResult, error) {
	return c.call(ctx, "/internal/v1/organizations/"+url.PathEscape(fiscal.OrganizationID)+"/authorizations/"+url.PathEscape(fiscal.RequestID)+"/consult", fiscal, "consult:"+fiscal.RequestID)
}
func (c HTTPFiscalClient) call(ctx context.Context, path string, fiscal domain.FiscalRequest, key string) (domain.FiscalResult, error) {
	var payload map[string]any
	if err := json.Unmarshal(fiscal.FiscalSnapshot, &payload); err != nil || len(payload) == 0 {
		return domain.FiscalResult{}, fmt.Errorf("fiscal snapshot is required")
	}
	payload["request_id"], payload["organization_id"], payload["credential_ref"], payload["point_of_sale"], payload["document_type"], payload["voucher_number"], payload["snapshot_digest"] = fiscal.RequestID, fiscal.OrganizationID, fiscal.CredentialRef, fiscal.Voucher.PointOfSale, fiscal.Voucher.DocumentType, fiscal.Voucher.VoucherNumber, fiscal.SnapshotDigest
	request, err := internalRequest(ctx, c.BaseURL, http.MethodPost, path, fiscal.OrganizationID, "fiscal", key, fallback(fiscal.CorrelationID, key), payload, c.Tokens)
	if err != nil {
		return domain.FiscalResult{}, err
	}
	response, err := c.client().Do(request)
	if err != nil {
		return domain.FiscalResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK && response.StatusCode != http.StatusAccepted {
		return domain.FiscalResult{}, decodeServiceError("fiscal request", response)
	}
	var result domain.FiscalResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return domain.FiscalResult{}, err
	}
	return result, nil
}
func (c HTTPFiscalClient) client() HTTPDoer {
	if c.Client != nil {
		return c.Client
	}
	return NewServiceHTTPClient()
}

func internalRequest(ctx context.Context, baseURL, method, path, organizationID, audience, idempotencyKey, correlationID string, value any, tokens identityaccess.TokenSource) (*http.Request, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	request.Header.Set("X-Correlation-ID", correlationID)
	if tokens != nil {
		token, err := tokens.Token(ctx, audience, organizationID)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request, nil
}

func fallback(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

type serviceError struct {
	Service string `json:"-"`
	Code    string `json:"code"`
	Title   string `json:"title"`
	Status  string `json:"-"`
}

func (e serviceError) Error() string {
	if e.Code != "" {
		return e.Service + " returned " + e.Code
	}
	return e.Service + " returned " + e.Status
}

func decodeServiceError(service string, response *http.Response) error {
	result := serviceError{Service: service, Status: response.Status}
	_ = json.NewDecoder(response.Body).Decode(&result)
	return result
}
