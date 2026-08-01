// Package commerce contains the private fiscal HTTP adapter.
// architecture:adapter external
package commerce

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

func (c HTTPFiscalClient) RequestCredentialCSR(
	ctx context.Context,
	organizationID,
	idempotencyKey,
	correlationID string,
	input domain.FiscalCredentialCSRInput,
) (domain.FiscalCredentialCSRResult, error) {
	client, err := c.generatedClient(organizationID, idempotencyKey, correlationID)
	if err != nil {
		return domain.FiscalCredentialCSRResult{}, err
	}
	response, err := client.RequestFiscalCredentialCSRWithResponse(
		ctx,
		organizationID,
		&fiscalapi.RequestFiscalCredentialCSRParams{
			IdempotencyKey: idempotencyKey,
			XCorrelationID: correlationID,
		},
		fiscalhelpers.CredentialCSRRequest(input),
	)
	if err != nil {
		return domain.FiscalCredentialCSRResult{}, err
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return domain.FiscalCredentialCSRResult{}, generatedServiceError(
			"fiscal credential CSR",
			response.Status(),
			response.Body,
		)
	}
	return fiscalhelpers.CredentialCSRResult(*response.JSON201), nil
}

func (c HTTPFiscalClient) GetCredential(
	ctx context.Context,
	organizationID,
	credentialID,
	correlationID string,
) (domain.FiscalCredential, error) {
	client, err := c.generatedClient(
		organizationID,
		"credential-read:"+credentialID,
		correlationID,
	)
	if err != nil {
		return domain.FiscalCredential{}, err
	}
	response, err := client.GetFiscalCredentialWithResponse(
		ctx,
		organizationID,
		credentialID,
		&fiscalapi.GetFiscalCredentialParams{XCorrelationID: correlationID},
	)
	if err != nil {
		return domain.FiscalCredential{}, err
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return domain.FiscalCredential{}, generatedServiceError(
			"fiscal credential",
			response.Status(),
			response.Body,
		)
	}
	return fiscalhelpers.Credential(*response.JSON200), nil
}

func (c HTTPFiscalClient) UploadCertificate(
	ctx context.Context,
	organizationID,
	credentialID,
	correlationID string,
	input domain.FiscalCertificateUpload,
) (domain.FiscalCredential, error) {
	client, err := c.generatedClient(
		organizationID,
		"certificate:"+credentialID+":"+strconv.Itoa(input.ExpectedVersion),
		correlationID,
	)
	if err != nil {
		return domain.FiscalCredential{}, err
	}
	response, err := client.UploadFiscalCertificateWithResponse(
		ctx,
		organizationID,
		credentialID,
		&fiscalapi.UploadFiscalCertificateParams{XCorrelationID: correlationID},
		fiscalhelpers.CertificateUpload(input),
	)
	if err != nil {
		return domain.FiscalCredential{}, err
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return domain.FiscalCredential{}, generatedServiceError(
			"fiscal certificate",
			response.Status(),
			response.Body,
		)
	}
	return fiscalhelpers.Credential(*response.JSON200), nil
}

func (c HTTPFiscalClient) ConfigurePointOfSale(
	ctx context.Context,
	organizationID,
	credentialID,
	correlationID string,
	pointOfSale int,
	enabled bool,
) (domain.FiscalPointOfSale, error) {
	client, err := c.generatedClient(
		organizationID,
		pointOfSaleOperationKey("configure", credentialID, pointOfSale, enabled),
		correlationID,
	)
	if err != nil {
		return domain.FiscalPointOfSale{}, err
	}
	response, err := client.ConfigureFiscalPointOfSaleWithResponse(
		ctx,
		organizationID,
		credentialID,
		pointOfSale,
		&fiscalapi.ConfigureFiscalPointOfSaleParams{XCorrelationID: correlationID},
		fiscalapi.ConfigureFiscalPointOfSaleJSONRequestBody{Enabled: enabled},
	)
	if err != nil {
		return domain.FiscalPointOfSale{}, err
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return domain.FiscalPointOfSale{}, generatedServiceError(
			"fiscal point of sale",
			response.Status(),
			response.Body,
		)
	}
	return fiscalhelpers.PointOfSale(*response.JSON200), nil
}

func (c HTTPFiscalClient) ValidatePointOfSale(
	ctx context.Context,
	organizationID,
	credentialID,
	correlationID string,
	pointOfSale int,
	enabled bool,
) (domain.FiscalPointOfSale, error) {
	client, err := c.generatedClient(
		organizationID,
		pointOfSaleOperationKey("validate", credentialID, pointOfSale, enabled),
		correlationID,
	)
	if err != nil {
		return domain.FiscalPointOfSale{}, err
	}
	response, err := client.ValidateFiscalPointOfSaleWithResponse(
		ctx,
		organizationID,
		credentialID,
		pointOfSale,
		&fiscalapi.ValidateFiscalPointOfSaleParams{XCorrelationID: correlationID},
		fiscalapi.ValidateFiscalPointOfSaleJSONRequestBody{Enabled: enabled},
	)
	if err != nil {
		return domain.FiscalPointOfSale{}, err
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return domain.FiscalPointOfSale{}, generatedServiceError(
			"fiscal point of sale validation",
			response.Status(),
			response.Body,
		)
	}
	return fiscalhelpers.PointOfSale(*response.JSON200), nil
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

func pointOfSaleOperationKey(
	operation,
	credentialID string,
	pointOfSale int,
	enabled bool,
) string {
	return operation + ":" + credentialID + ":" + strconv.Itoa(pointOfSale) + ":" + strconv.FormatBool(enabled)
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
