package ar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Environment string

const (
	Homologation Environment = "homologation"
	Production   Environment = "production"
)

func (environment Environment) Valid() bool {
	return environment == Homologation || environment == Production
}

type ServiceEndpoints struct {
	WSAA string
	WSFE string
}

func EndpointsFor(environment Environment) (ServiceEndpoints, error) {
	switch environment {
	case Homologation:
		return ServiceEndpoints{
			WSAA: "https://wsaahomo.afip.gov.ar/ws/services/LoginCms",
			WSFE: "https://wswhomo.afip.gov.ar/wsfev1/service.asmx",
		}, nil
	case Production:
		return ServiceEndpoints{
			WSAA: "https://wsaa.afip.gov.ar/ws/services/LoginCms",
			WSFE: "https://servicios1.afip.gov.ar/wsfev1/service.asmx",
		}, nil
	default:
		return ServiceEndpoints{}, fmt.Errorf("invalid ARCA environment %q", environment)
	}
}

type SOAPRequest struct {
	Endpoint string
	Action   string
	Envelope []byte
}

type SOAPTransport interface {
	Call(ctx context.Context, request SOAPRequest) ([]byte, error)
}

type HTTPTransport struct {
	Client      *http.Client
	MaxResponse int64
	UserAgent   string
}

func NewHTTPTransport() *HTTPTransport {
	return &HTTPTransport{
		Client:      &http.Client{Timeout: 30 * time.Second},
		MaxResponse: 4 << 20,
		UserAgent:   "pymes-v2-fiscal/1",
	}
}

func (transport *HTTPTransport) Call(ctx context.Context, request SOAPRequest) ([]byte, error) {
	if transport == nil {
		return nil, errors.New("nil ARCA HTTP transport")
	}
	if strings.TrimSpace(request.Endpoint) == "" || len(request.Envelope) == 0 {
		return nil, errors.New("ARCA SOAP endpoint and envelope are required")
	}
	client := transport.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	maxResponse := transport.MaxResponse
	if maxResponse <= 0 {
		maxResponse = 4 << 20
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx, http.MethodPost, request.Endpoint, bytes.NewReader(request.Envelope),
	)
	if err != nil {
		return nil, fmt.Errorf("build ARCA SOAP request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "text/xml; charset=utf-8")
	httpRequest.Header.Set("SOAPAction", request.Action)
	if transport.UserAgent != "" {
		httpRequest.Header.Set("User-Agent", transport.UserAgent)
	}

	response, err := client.Do(httpRequest)
	if err != nil {
		// The caller decides whether a transport error is ambiguous for the
		// operation. Request/response bodies and credentials are never logged.
		return nil, fmt.Errorf("ARCA SOAP transport: %w", err)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if readErr != nil {
		return nil, fmt.Errorf("read ARCA SOAP response: %w", readErr)
	}
	if int64(len(body)) > maxResponse {
		return nil, errors.New("ARCA SOAP response exceeds configured limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("ARCA SOAP HTTP status %d", response.StatusCode)
	}
	return body, nil
}
