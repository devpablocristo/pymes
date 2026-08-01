// Package commerce contains the private fiscal HTTP adapter.
// architecture:adapter external
package commerce

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	fiscalhelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/helpers"
	fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

type HTTPFiscalClient struct {
	BaseURL        string
	Client         HTTPDoer
	Tokens         InternalTokenSource
	PlatformTokens PlatformTokenSource
}

func (c HTTPFiscalClient) Authorize(
	ctx context.Context,
	fiscal domain.FiscalRequest,
) (domain.FiscalResult, error) {
	return c.call(
		ctx,
		fiscal,
		fallback(fiscal.IdempotencyKey, fiscal.RequestID),
		false,
	)
}

func (c HTTPFiscalClient) Consult(
	ctx context.Context,
	fiscal domain.FiscalRequest,
) (domain.FiscalResult, error) {
	return c.call(
		ctx,
		fiscal,
		fallback(fiscal.IdempotencyKey, fiscal.RequestID),
		true,
	)
}

func (c HTTPFiscalClient) call(
	ctx context.Context,
	fiscal domain.FiscalRequest,
	key string,
	consult bool,
) (domain.FiscalResult, error) {
	payload, err := fiscalhelpers.DecodeSnapshot(fiscal.FiscalSnapshot)
	if err != nil {
		return domain.FiscalResult{}, err
	}
	correlationID := fallback(fiscal.CorrelationID, key)
	payload["request_id"] = fiscal.RequestID
	payload["organization_id"] = fiscal.OrganizationID
	payload["idempotency_key"] = key
	payload["correlation_id"] = correlationID
	payload["source_version"] = positiveVersion(fiscal.SourceVersion)
	payload["credential_ref"] = fiscal.CredentialRef
	payload["point_of_sale"] = fiscal.Voucher.PointOfSale
	payload["document_type"] = fiscal.Voucher.DocumentType
	payload["voucher_number"] = fiscal.Voucher.VoucherNumber
	payload["snapshot_digest"] = fiscal.SnapshotDigest
	var body fiscalapi.FiscalRequest
	if err := transcodeJSON(payload, &body); err != nil {
		return domain.FiscalResult{}, fmt.Errorf("encode fiscal request: %w", err)
	}
	client, err := c.generatedClient(
		fiscal.OrganizationID,
		key,
		correlationID,
	)
	if err != nil {
		return domain.FiscalResult{}, err
	}
	params := &fiscalapi.RequestAuthorizationParams{
		IdempotencyKey: key,
		XCorrelationID: correlationID,
	}
	if consult {
		response, err := client.ConsultAuthorizationWithResponse(
			ctx,
			fiscal.OrganizationID,
			fiscal.RequestID,
			&fiscalapi.ConsultAuthorizationParams{
				IdempotencyKey: key,
				XCorrelationID: correlationID,
			},
			body,
		)
		if err != nil {
			return domain.FiscalResult{}, err
		}
		return decodeFiscalResult(
			"fiscal consultation",
			response.Status(),
			response.StatusCode(),
			response.Body,
			response.JSON200,
		)
	}
	response, err := client.RequestAuthorizationWithResponse(
		ctx,
		fiscal.OrganizationID,
		params,
		body,
	)
	if err != nil {
		return domain.FiscalResult{}, err
	}
	return decodeFiscalResult(
		"fiscal authorization",
		response.Status(),
		response.StatusCode(),
		response.Body,
		response.JSON201,
		response.JSON202,
	)
}

func (c HTTPFiscalClient) generatedClient(
	organizationID string,
	idempotencyKey string,
	correlationID string,
) (*fiscalapi.ClientWithResponses, error) {
	return fiscalapi.NewClientWithResponses(
		strings.TrimSuffix(c.BaseURL, "/"),
		fiscalapi.WithHTTPClient(c.client()),
		fiscalapi.WithRequestEditorFn(internalRequestEditor(
			strings.TrimSuffix(c.BaseURL, "/"),
			organizationID,
			"fiscal",
			idempotencyKey,
			correlationID,
			c.Tokens,
			c.PlatformTokens,
		)),
	)
}

func (c HTTPFiscalClient) client() HTTPDoer {
	if c.Client != nil {
		return c.Client
	}
	return NewServiceHTTPClient()
}

func decodeFiscalResult(
	service string,
	status string,
	statusCode int,
	body []byte,
	candidates ...*fiscalapi.FiscalResult,
) (domain.FiscalResult, error) {
	if statusCode != http.StatusOK &&
		statusCode != http.StatusCreated &&
		statusCode != http.StatusAccepted {
		return domain.FiscalResult{}, generatedServiceError(
			service,
			status,
			body,
		)
	}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		var result domain.FiscalResult
		if err := transcodeJSON(candidate, &result); err != nil {
			return domain.FiscalResult{}, fmt.Errorf(
				"decode %s response: %w",
				service,
				err,
			)
		}
		return result, nil
	}
	return domain.FiscalResult{}, generatedServiceError(service, status, body)
}
