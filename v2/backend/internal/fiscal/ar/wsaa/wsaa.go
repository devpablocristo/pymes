package wsaa

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
)

const (
	ServiceWSFE = "wsfe"
	wsaaNS      = "http://wsaa.view.wsfe.dvadac.desein.afip.gov"
)

var servicePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type AccessTicket struct {
	Token     string    `json:"-"`
	Sign      string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (ticket AccessTicket) ValidAt(now time.Time) bool {
	return strings.TrimSpace(ticket.Token) != "" &&
		strings.TrimSpace(ticket.Sign) != "" &&
		ticket.ExpiresAt.After(now.Add(5*time.Minute))
}

type loginTicketRequest struct {
	XMLName xml.Name          `xml:"loginTicketRequest"`
	Version string            `xml:"version,attr"`
	Header  loginTicketHeader `xml:"header"`
	Service string            `xml:"service"`
}

type loginTicketHeader struct {
	UniqueID       int64  `xml:"uniqueId"`
	GenerationTime string `xml:"generationTime"`
	ExpirationTime string `xml:"expirationTime"`
}

// BuildTRA creates the short-lived LoginTicketRequest without interpolating
// untrusted XML. The unique ID is time based as required by WSAA.
func BuildTRA(service string, now time.Time) ([]byte, error) {
	service = strings.TrimSpace(service)
	if !servicePattern.MatchString(service) {
		return nil, errors.New("invalid WSAA service name")
	}
	now = now.Truncate(time.Second)
	request := loginTicketRequest{
		Version: "1.0",
		Header: loginTicketHeader{
			UniqueID:       now.Unix(),
			GenerationTime: now.Add(-10 * time.Minute).Format(time.RFC3339),
			ExpirationTime: now.Add(10 * time.Minute).Format(time.RFC3339),
		},
		Service: service,
	}
	body, err := xml.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal WSAA TRA: %w", err)
	}
	return append([]byte(xml.Header), body...), nil
}

func LoginEnvelope(cmsBase64 string) ([]byte, error) {
	if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cmsBase64)); err != nil {
		return nil, errors.New("WSAA CMS must be valid base64")
	}
	type loginCMS struct {
		XMLName xml.Name `xml:"wsaa:loginCms"`
		Input   string   `xml:"wsaa:in0"`
	}
	type body struct {
		XMLName xml.Name `xml:"soapenv:Body"`
		Login   loginCMS
	}
	type envelope struct {
		XMLName   xml.Name `xml:"soapenv:Envelope"`
		SoapEnvNS string   `xml:"xmlns:soapenv,attr"`
		WSAANS    string   `xml:"xmlns:wsaa,attr"`
		Body      body
	}
	return xml.Marshal(envelope{
		SoapEnvNS: "http://schemas.xmlsoap.org/soap/envelope/",
		WSAANS:    wsaaNS,
		Body:      body{Login: loginCMS{Input: strings.TrimSpace(cmsBase64)}},
	})
}

func ParseLoginResponse(raw []byte) (AccessTicket, error) {
	var outer struct {
		Return string `xml:"Body>loginCmsResponse>loginCmsReturn"`
		Fault  struct {
			Code   string `xml:"faultcode"`
			Reason string `xml:"faultstring"`
		} `xml:"Body>Fault"`
	}
	if err := xml.Unmarshal(raw, &outer); err != nil {
		return AccessTicket{}, fmt.Errorf("parse WSAA SOAP response: %w", err)
	}
	if strings.TrimSpace(outer.Fault.Reason) != "" {
		return AccessTicket{}, fmt.Errorf(
			"WSAA fault %s: %s",
			strings.TrimSpace(outer.Fault.Code),
			strings.TrimSpace(outer.Fault.Reason),
		)
	}
	inner := strings.TrimSpace(html.UnescapeString(outer.Return))
	if inner == "" {
		return AccessTicket{}, errors.New("WSAA response has empty loginCmsReturn")
	}
	var response struct {
		Token      string `xml:"credentials>token"`
		Sign       string `xml:"credentials>sign"`
		Expiration string `xml:"header>expirationTime"`
	}
	if err := xml.Unmarshal([]byte(inner), &response); err != nil {
		return AccessTicket{}, fmt.Errorf("parse WSAA access ticket: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(response.Expiration))
	if err != nil {
		return AccessTicket{}, fmt.Errorf("parse WSAA expiration: %w", err)
	}
	ticket := AccessTicket{
		Token:     strings.TrimSpace(response.Token),
		Sign:      strings.TrimSpace(response.Sign),
		ExpiresAt: expiresAt.UTC(),
	}
	if ticket.Token == "" || ticket.Sign == "" {
		return AccessTicket{}, errors.New("WSAA access ticket has empty credentials")
	}
	return ticket, nil
}

type Client struct {
	Transport ar.SOAPTransport
	Now       func() time.Time
}

func NewClient(transport ar.SOAPTransport) *Client {
	return &Client{Transport: transport, Now: time.Now}
}

func (client *Client) Login(
	ctx context.Context,
	environment ar.Environment,
	service string,
	certificatePEM []byte,
	keyReference string,
	kms fiscal.KMS,
) (AccessTicket, error) {
	if client == nil || client.Transport == nil || kms == nil {
		return AccessTicket{}, errors.New("WSAA client dependencies are incomplete")
	}
	endpoints, err := ar.EndpointsFor(environment)
	if err != nil {
		return AccessTicket{}, err
	}
	now := time.Now()
	if client.Now != nil {
		now = client.Now()
	}
	tra, err := BuildTRA(service, now)
	if err != nil {
		return AccessTicket{}, err
	}
	cms, err := SignTRAWithKMS(ctx, tra, certificatePEM, keyReference, kms)
	if err != nil {
		return AccessTicket{}, err
	}
	envelope, err := LoginEnvelope(cms)
	if err != nil {
		return AccessTicket{}, err
	}
	response, err := client.Transport.Call(ctx, ar.SOAPRequest{
		Endpoint: endpoints.WSAA,
		Envelope: envelope,
	})
	if err != nil {
		return AccessTicket{}, fmt.Errorf("WSAA LoginCms transport: %w", err)
	}
	ticket, err := ParseLoginResponse(response)
	if err != nil {
		return AccessTicket{}, err
	}
	if !ticket.ValidAt(now) {
		return AccessTicket{}, errors.New("WSAA returned an already expired or near-expiry ticket")
	}
	return ticket, nil
}

type TicketKey struct {
	OrganizationID         uuid.UUID
	Environment            ar.Environment
	Service                string
	CertificateFingerprint string
}

type TicketRepository interface {
	GetTicket(ctx context.Context, key TicketKey) (AccessTicket, error)
	SaveTicket(ctx context.Context, key TicketKey, ticket AccessTicket) error
}

type Credentials struct {
	OrganizationID uuid.UUID
	Environment    ar.Environment
	Service        string
	CUIT           ar.CUIT
	CertificatePEM []byte
	KeyReference   string
}

// Authenticator caches TA credentials by organization, environment, service,
// and certificate fingerprint. Tickets and key material have no JSON fields and
// must not be included in logs by adapters.
type Authenticator struct {
	Client  *Client
	Tickets TicketRepository
	KMS     fiscal.KMS
	Now     func() time.Time
}

func (authenticator *Authenticator) AccessTicket(
	ctx context.Context,
	credentials Credentials,
) (AccessTicket, error) {
	if authenticator == nil || authenticator.Client == nil ||
		authenticator.Tickets == nil || authenticator.KMS == nil {
		return AccessTicket{}, errors.New("WSAA authenticator dependencies are incomplete")
	}
	if credentials.OrganizationID == uuid.Nil || !credentials.Environment.Valid() {
		return AccessTicket{}, errors.New("WSAA organization and environment are required")
	}
	publicKey, err := authenticator.KMS.PublicKey(ctx, credentials.KeyReference)
	if err != nil {
		return AccessTicket{}, fmt.Errorf("load KMS public key: %w", err)
	}
	now := time.Now()
	if authenticator.Now != nil {
		now = authenticator.Now()
	}
	certificate, err := ar.ValidateCertificate(
		credentials.CertificatePEM, publicKey, credentials.CUIT, now,
	)
	if err != nil {
		return AccessTicket{}, err
	}
	service := strings.TrimSpace(credentials.Service)
	if service == "" {
		service = ServiceWSFE
	}
	key := TicketKey{
		OrganizationID:         credentials.OrganizationID,
		Environment:            credentials.Environment,
		Service:                service,
		CertificateFingerprint: certificate.Fingerprint,
	}
	if ticket, err := authenticator.Tickets.GetTicket(ctx, key); err == nil {
		if ticket.ValidAt(now) {
			return ticket, nil
		}
	} else if !errors.Is(err, fiscal.ErrNotFound) {
		return AccessTicket{}, err
	}
	ticket, err := authenticator.Client.Login(
		ctx, credentials.Environment, service, credentials.CertificatePEM,
		credentials.KeyReference, authenticator.KMS,
	)
	if err != nil {
		return AccessTicket{}, err
	}
	if err := authenticator.Tickets.SaveTicket(ctx, key, ticket); err != nil {
		return AccessTicket{}, err
	}
	return ticket, nil
}
