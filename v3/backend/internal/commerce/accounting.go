// Package commerce contains the private accounting HTTP adapter.
// architecture:adapter external
package commerce

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	accountinghelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/helpers"
	accountingapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	"github.com/google/uuid"
)

type HTTPAccountingClient struct {
	BaseURL        string
	Client         HTTPDoer
	Tokens         InternalTokenSource
	PlatformTokens PlatformTokenSource
}

// HTTPAccountingProvisioningClient is the privileged control-plane adapter.
// It deliberately remains separate from HTTPAccountingClient so the runtime
// accounting workload never needs access to organization-schema provisioning.
type HTTPAccountingProvisioningClient struct {
	BaseURL        string
	Client         HTTPDoer
	Tokens         InternalTokenSource
	PlatformTokens PlatformTokenSource
}

func (c HTTPAccountingProvisioningClient) ProvisionOrganization(ctx context.Context, organization organizationdomain.Organization) error {
	if strings.TrimSpace(organization.ID) == "" || strings.TrimSpace(organization.Name) == "" {
		return fmt.Errorf("accounting provision: organization ID and display name are required")
	}
	baseURL := strings.TrimSuffix(strings.TrimSpace(c.BaseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("accounting provision: base URL is required")
	}

	body, payloadDigest, err := accountinghelpers.EncodePayload(
		accountingapi.ProvisioningPayload{
			OrganizationID: organization.ID,
			DisplayName:    organization.Name,
		},
	)
	if err != nil {
		return fmt.Errorf("encode accounting provisioning command: %w", err)
	}
	idempotencyKey := accountinghelpers.ProvisioningIdempotencyKey(
		organization.ID,
	)
	ctx, requestID, correlationID := accountingProvisioningMetadata(ctx)

	client, err := accountingapi.NewClientWithResponses(
		baseURL,
		accountingapi.WithHTTPClient(c.client()),
		accountingapi.WithRequestEditorFn(internalRequestEditor(
			baseURL,
			organization.ID,
			"accounting-provisioning",
			idempotencyKey,
			correlationID,
			c.Tokens,
			c.PlatformTokens,
		)),
		accountingapi.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set("X-Request-ID", requestID)
			request.Header.Set("X-Payload-Digest", payloadDigest)
			return nil
		}),
	)
	if err != nil {
		return err
	}
	response, err := client.ProvisionOrganizationWithBodyWithResponse(
		ctx,
		organization.ID,
		&accountingapi.ProvisionOrganizationParams{
			IdempotencyKey: idempotencyKey,
			XCorrelationID: correlationID,
		},
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	if response.StatusCode() != http.StatusCreated && response.StatusCode() != http.StatusOK {
		return accountinghelpers.DecodeServiceError(
			"accounting provision",
			response.Status(),
			response.Body,
		)
	}
	return nil
}

func accountingProvisioningMetadata(ctx context.Context) (context.Context, string, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if metadata, ok := identityusecases.RequestMetadataFromContext(ctx); ok &&
		len(metadata.CorrelationID) <= 255 {
		return ctx, metadata.RequestID, metadata.CorrelationID
	}
	requestID := uuid.NewString()
	metadata := identityusecases.RequestMetadata{
		RequestID:     requestID,
		CorrelationID: requestID,
	}
	return identityusecases.WithRequestMetadata(ctx, metadata), metadata.RequestID, metadata.CorrelationID
}

func (c HTTPAccountingProvisioningClient) client() HTTPDoer {
	if c.Client != nil {
		return c.Client
	}
	return NewServiceHTTPClient()
}

func (c HTTPAccountingClient) Post(ctx context.Context, command domain.PostingCommand) (domain.AccountingEvent, error) {
	lines := make([]map[string]any, 0, len(command.Lines))
	for _, line := range command.Lines {
		currency := line.Debit.Currency
		if currency == "" {
			currency = line.Credit.Currency
		}
		payloadLine := map[string]any{
			"account_code": line.AccountCode, "debit": line.Debit.Amount, "credit": line.Credit.Amount,
			"currency": currency, "memo": line.Memo, "open_item": line.OpenItem, "party_ref": line.PartyRef,
		}
		if line.FunctionalAmount != "" {
			payloadLine["functional_amount"] = line.FunctionalAmount
		}
		lines = append(lines, payloadLine)
	}
	key := accountinghelpers.Fallback(command.IdempotencyKey, command.CommandID)
	correlationID := accountinghelpers.Fallback(command.CorrelationID, key)
	sourceVersion := accountinghelpers.PositiveVersion(command.SourceVersion)
	payload := map[string]any{
		"command_id":      command.CommandID,
		"organization_id": command.OrganizationID,
		"idempotency_key": key,
		"correlation_id":  correlationID,
		"source_version":  sourceVersion,
		"snapshot_digest": command.SnapshotDigest,
		"source":          map[string]any{"type": command.SourceType, "id": command.SourceID, "version": sourceVersion, "digest": command.SnapshotDigest},
		"effective_at":    command.EffectiveAt.UTC(),
		"description":     command.Description,
		"lines":           lines,
	}
	if command.ExchangeRate != "" {
		payload["exchange_rate"] = command.ExchangeRate
	}
	if command.RelatedSource != nil {
		payload["related_source"] = command.RelatedSource
	}
	if command.OriginalJournalEntryID != "" {
		payload["original_journal_entry_id"] = command.OriginalJournalEntryID
	}
	var body accountingapi.SubmitPostingCommandJSONRequestBody
	if err := accountinghelpers.TranscodeJSON(payload, &body); err != nil {
		return domain.AccountingEvent{}, fmt.Errorf("encode accounting posting command: %w", err)
	}
	client, err := c.generatedClient(command.OrganizationID, key, correlationID)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	response, err := client.SubmitPostingCommandWithResponse(
		ctx,
		command.OrganizationID,
		&accountingapi.SubmitPostingCommandParams{IdempotencyKey: key, XCorrelationID: correlationID},
		body,
	)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	return accountinghelpers.DecodeAccountingEvent(
		"accounting post",
		response.Status(),
		response.StatusCode(),
		response.Body,
		response.JSON200,
		response.JSON201,
	)
}

func (c HTTPAccountingClient) Reverse(ctx context.Context, command domain.ReversalCommand) (domain.AccountingEvent, error) {
	payload := map[string]any{
		"command_id": command.CommandID, "original_journal_entry_id": command.OriginalJournalEntryID,
		"effective_at": command.EffectiveAt.UTC(), "reason": command.Reason,
	}
	key := accountinghelpers.Fallback(command.IdempotencyKey, command.CommandID)
	correlationID := accountinghelpers.Fallback(command.CorrelationID, key)
	digest := command.SnapshotDigest
	if digest == "" {
		var err error
		digest, err = accountinghelpers.SnapshotDigest(payload)
		if err != nil {
			return domain.AccountingEvent{}, fmt.Errorf("hash accounting reversal: %w", err)
		}
	}
	payload["organization_id"] = command.OrganizationID
	payload["idempotency_key"] = key
	payload["correlation_id"] = correlationID
	payload["source_version"] = accountinghelpers.PositiveVersion(command.SourceVersion)
	payload["snapshot_digest"] = digest
	var body accountingapi.ReverseJournalEntryJSONRequestBody
	if err := accountinghelpers.TranscodeJSON(payload, &body); err != nil {
		return domain.AccountingEvent{}, fmt.Errorf("encode accounting reversal: %w", err)
	}
	client, err := c.generatedClient(command.OrganizationID, key, correlationID)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	response, err := client.ReverseJournalEntryWithResponse(
		ctx,
		command.OrganizationID,
		&accountingapi.ReverseJournalEntryParams{IdempotencyKey: key, XCorrelationID: correlationID},
		body,
	)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	return accountinghelpers.DecodeAccountingEvent(
		"accounting reversal",
		response.Status(),
		response.StatusCode(),
		response.Body,
		response.JSON200,
		response.JSON201,
	)
}

func (c HTTPAccountingClient) ApplyOpenItem(ctx context.Context, command domain.AccountingApplicationCommand) (domain.AccountingEvent, error) {
	payload := map[string]any{
		"command_id": command.CommandID, "debit_open_item_id": command.DebitOpenItemID,
		"credit_open_item_id": command.CreditOpenItemID, "amount": command.Amount,
		"applied_at": command.AppliedAt.UTC(),
	}
	key := accountinghelpers.Fallback(command.IdempotencyKey, command.CommandID)
	correlationID := accountinghelpers.Fallback(command.CorrelationID, key)
	digest := command.SnapshotDigest
	if digest == "" {
		var err error
		digest, err = accountinghelpers.SnapshotDigest(payload)
		if err != nil {
			return domain.AccountingEvent{}, fmt.Errorf("hash accounting application: %w", err)
		}
	}
	payload["organization_id"] = command.OrganizationID
	payload["idempotency_key"] = key
	payload["correlation_id"] = correlationID
	payload["source_version"] = accountinghelpers.PositiveVersion(command.SourceVersion)
	payload["snapshot_digest"] = digest
	var body accountingapi.ApplyOpenItemJSONRequestBody
	if err := accountinghelpers.TranscodeJSON(payload, &body); err != nil {
		return domain.AccountingEvent{}, fmt.Errorf("encode accounting application: %w", err)
	}
	client, err := c.generatedClient(command.OrganizationID, key, correlationID)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	response, err := client.ApplyOpenItemWithResponse(
		ctx,
		command.OrganizationID,
		&accountingapi.ApplyOpenItemParams{IdempotencyKey: key, XCorrelationID: correlationID},
		body,
	)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	return accountinghelpers.DecodeAccountingEvent(
		"accounting application",
		response.Status(),
		response.StatusCode(),
		response.Body,
		response.JSON200,
		response.JSON201,
	)
}

func (c HTTPAccountingClient) ReverseOpenItemApplication(ctx context.Context, command domain.AccountingApplicationReversalCommand) (domain.AccountingEvent, error) {
	payload := map[string]any{
		"command_id": command.CommandID, "application_id": command.ApplicationID,
		"reversed_at": command.ReversedAt.UTC(), "reason": command.Reason,
	}
	key := accountinghelpers.Fallback(command.IdempotencyKey, command.CommandID)
	correlationID := accountinghelpers.Fallback(command.CorrelationID, key)
	digest := command.SnapshotDigest
	if digest == "" {
		var err error
		digest, err = accountinghelpers.SnapshotDigest(payload)
		if err != nil {
			return domain.AccountingEvent{}, fmt.Errorf("hash accounting application reversal: %w", err)
		}
	}
	payload["organization_id"] = command.OrganizationID
	payload["idempotency_key"] = key
	payload["correlation_id"] = correlationID
	payload["source_version"] = accountinghelpers.PositiveVersion(command.SourceVersion)
	payload["snapshot_digest"] = digest
	var body accountingapi.ReverseOpenItemApplicationJSONRequestBody
	if err := accountinghelpers.TranscodeJSON(payload, &body); err != nil {
		return domain.AccountingEvent{}, fmt.Errorf("encode accounting application reversal: %w", err)
	}
	client, err := c.generatedClient(command.OrganizationID, key, correlationID)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	response, err := client.ReverseOpenItemApplicationWithResponse(
		ctx,
		command.OrganizationID,
		&accountingapi.ReverseOpenItemApplicationParams{IdempotencyKey: key, XCorrelationID: correlationID},
		body,
	)
	if err != nil {
		return domain.AccountingEvent{}, err
	}
	return accountinghelpers.DecodeAccountingEvent(
		"accounting application reversal",
		response.Status(),
		response.StatusCode(),
		response.Body,
		response.JSON200,
		response.JSON201,
	)
}

func (c HTTPAccountingClient) generatedClient(organizationID, idempotencyKey, correlationID string) (*accountingapi.ClientWithResponses, error) {
	return accountingapi.NewClientWithResponses(
		strings.TrimSuffix(c.BaseURL, "/"),
		accountingapi.WithHTTPClient(c.client()),
		accountingapi.WithRequestEditorFn(internalRequestEditor(
			strings.TrimSuffix(c.BaseURL, "/"),
			organizationID,
			"accounting",
			idempotencyKey,
			correlationID,
			c.Tokens,
			c.PlatformTokens,
		)),
	)
}
func (c HTTPAccountingClient) client() HTTPDoer {
	if c.Client != nil {
		return c.Client
	}
	return NewServiceHTTPClient()
}

func internalRequestEditor(
	baseURL string,
	organizationID string,
	audience string,
	idempotencyKey string,
	correlationID string,
	tokens InternalTokenSource,
	platformTokens PlatformTokenSource,
) func(context.Context, *http.Request) error {
	return func(ctx context.Context, request *http.Request) error {
		request.Header.Set("Idempotency-Key", idempotencyKey)
		request.Header.Set("X-Correlation-ID", correlationID)
		if tokens != nil {
			requestID := correlationID
			if metadata, ok := identityusecases.RequestMetadataFromContext(ctx); ok {
				if metadata.RequestID != "" {
					requestID = metadata.RequestID
				}
			}
			tokenContext := identityusecases.WithRequestMetadata(
				ctx,
				identityusecases.RequestMetadata{
					RequestID:     requestID,
					CorrelationID: correlationID,
				},
			)
			token, err := tokens.Token(tokenContext, audience, organizationID)
			if err != nil {
				return err
			}
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if platformTokens != nil {
			token, err := platformTokens.PlatformToken(ctx, baseURL)
			if err != nil {
				return err
			}
			request.Header.Set("X-Serverless-Authorization", "Bearer "+token)
		}
		return nil
	}
}

type serviceError = accountinghelpers.ServiceError
