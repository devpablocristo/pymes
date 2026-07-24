package wsfev1

import (
	"context"
	"errors"
	"fmt"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
)

const (
	actionAuthorize      = Namespace + "FECAESolicitar"
	actionLastAuthorized = Namespace + "FECompUltimoAutorizado"
	actionConsult        = Namespace + "FECompConsultar"
)

type Client struct {
	Transport   ar.SOAPTransport
	Environment ar.Environment
}

func NewClient(transport ar.SOAPTransport, environment ar.Environment) (*Client, error) {
	if transport == nil {
		return nil, errors.New("WSFE SOAP transport is required")
	}
	if !environment.Valid() {
		return nil, errors.New("valid WSFE environment is required")
	}
	return &Client{Transport: transport, Environment: environment}, nil
}

func (client *Client) LastAuthorized(
	ctx context.Context,
	auth Auth,
	pointOfSale int,
	voucherType ar.VoucherType,
) (int64, error) {
	if client == nil || client.Transport == nil {
		return 0, errors.New("WSFE client is not configured")
	}
	envelope, err := BuildLastAuthorizedEnvelope(auth, pointOfSale, voucherType)
	if err != nil {
		return 0, err
	}
	response, err := client.call(ctx, actionLastAuthorized, envelope)
	if err != nil {
		return 0, err
	}
	return ParseLastAuthorizedResponse(response)
}

func (client *Client) Authorize(
	ctx context.Context,
	auth Auth,
	request Request,
) (AuthorizationResult, error) {
	if client == nil || client.Transport == nil {
		return AuthorizationResult{}, errors.New("WSFE client is not configured")
	}
	envelope, err := BuildAuthorizeEnvelope(auth, request)
	if err != nil {
		return AuthorizationResult{}, err
	}
	response, err := client.call(ctx, actionAuthorize, envelope)
	if err != nil {
		return AuthorizationResult{}, fmt.Errorf("%w: %v", fiscal.ErrUncertainResponse, err)
	}
	result, err := ParseAuthorizeResponse(response)
	if err != nil {
		// A malformed/fault response arrived after FECAESolicitar. Conservatively
		// reconcile the exact number before any retry.
		return AuthorizationResult{}, fmt.Errorf("%w: %v", fiscal.ErrUncertainResponse, err)
	}
	return result, nil
}

func (client *Client) Consult(
	ctx context.Context,
	auth Auth,
	pointOfSale int,
	voucherType ar.VoucherType,
	number int64,
) (ConsultResult, error) {
	if client == nil || client.Transport == nil {
		return ConsultResult{}, errors.New("WSFE client is not configured")
	}
	envelope, err := BuildConsultEnvelope(auth, pointOfSale, voucherType, number)
	if err != nil {
		return ConsultResult{}, err
	}
	response, err := client.call(ctx, actionConsult, envelope)
	if err != nil {
		return ConsultResult{}, err
	}
	return ParseConsultResponse(response)
}

func (client *Client) call(ctx context.Context, action string, envelope []byte) ([]byte, error) {
	endpoints, err := ar.EndpointsFor(client.Environment)
	if err != nil {
		return nil, err
	}
	return client.Transport.Call(ctx, ar.SOAPRequest{
		Endpoint: endpoints.WSFE,
		Action:   action,
		Envelope: envelope,
	})
}
