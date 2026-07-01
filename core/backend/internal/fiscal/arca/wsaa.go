package arca

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"go.mozilla.org/pkcs7"
)

// traTTL es la ventana de validez que se declara en el LoginTicketRequest.
const traTTL = 10 * time.Minute

// wsaaNS es el namespace de la operación LoginCms del WSAA (dominio real de ARCA).
const wsaaNS = "http://wsaa.view.wsfe.dvadac.desein.afip.gov"

// BuildTRA arma el LoginTicketRequest (XML) para un servicio (ej. "wsfe").
func BuildTRA(service string, now time.Time) []byte {
	gen := now.Add(-traTTL)
	exp := now.Add(traTTL)
	tra := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<loginTicketRequest version="1.0">`+
		`<header><uniqueId>%d</uniqueId><generationTime>%s</generationTime><expirationTime>%s</expirationTime></header>`+
		`<service>%s</service>`+
		`</loginTicketRequest>`,
		now.Unix(), gen.Format(time.RFC3339), exp.Format(time.RFC3339), service)
	return []byte(tra)
}

// SignTRA firma el TRA como CMS/PKCS#7 (SHA-256, no-detached) con el certificado
// X.509 + clave privada, y devuelve el DER en base64 (lo que espera LoginCms).
func SignTRA(tra []byte, certPEM, keyPEM string) (string, error) {
	cert, err := parseCertificate(certPEM)
	if err != nil {
		return "", fmt.Errorf("parse certificate: %w", err)
	}
	key, err := parsePrivateKey(keyPEM)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	sd, err := pkcs7.NewSignedData(tra)
	if err != nil {
		return "", err
	}
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	if err := sd.AddSigner(cert, key, pkcs7.SignerInfoConfig{}); err != nil {
		return "", err
	}
	der, err := sd.Finish()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// Login ejecuta el flujo WSAA: arma el TRA, lo firma y llama a LoginCms.
// Devuelve el TA (token + sign + vencimiento) para usar en WSFEv1.
func (c *Client) Login(ctx context.Context, creds Credentials, service string) (TA, error) {
	cms, err := SignTRA(BuildTRA(service, time.Now()), creds.CertPEM, creds.KeyPEM)
	if err != nil {
		return TA{}, err
	}
	env := loginEnvelope(cms)
	body, err := c.doSOAP(ctx, EndpointsFor(creds.Production).WSAA, "", env)
	if err != nil {
		return TA{}, err
	}
	return parseLoginResponse(body)
}

func loginEnvelope(cms string) []byte {
	return []byte(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:wsaa="` + wsaaNS + `">` +
		`<soapenv:Header/><soapenv:Body><wsaa:loginCms><wsaa:in0>` + cms + `</wsaa:in0></wsaa:loginCms></soapenv:Body>` +
		`</soapenv:Envelope>`)
}

// parseLoginResponse extrae el TA del SOAP response. loginCmsReturn contiene el
// XML del TA escapado como texto; se lo desescapa y se parsea.
func parseLoginResponse(body []byte) (TA, error) {
	var outer struct {
		Return string `xml:"Body>loginCmsResponse>loginCmsReturn"`
		Fault  string `xml:"Body>Fault>faultstring"`
	}
	if err := xml.Unmarshal(body, &outer); err != nil {
		return TA{}, fmt.Errorf("parse wsaa response: %w", err)
	}
	if strings.TrimSpace(outer.Fault) != "" {
		return TA{}, fmt.Errorf("wsaa fault: %s", outer.Fault)
	}
	if strings.TrimSpace(outer.Return) == "" {
		return TA{}, errors.New("wsaa: empty loginCmsReturn")
	}
	inner := html.UnescapeString(outer.Return)
	var ta struct {
		Token string `xml:"credentials>token"`
		Sign  string `xml:"credentials>sign"`
		Exp   string `xml:"header>expirationTime"`
	}
	if err := xml.Unmarshal([]byte(inner), &ta); err != nil {
		return TA{}, fmt.Errorf("parse login ticket: %w", err)
	}
	expires, err := time.Parse(time.RFC3339, strings.TrimSpace(ta.Exp))
	if err != nil {
		// Fallback: vencimiento conservador de 6h si ARCA cambia el formato.
		expires = time.Now().Add(6 * time.Hour)
	}
	return TA{Token: ta.Token, Sign: ta.Sign, ExpiresAt: expires}, nil
}

func parseCertificate(certPEM string) (*x509.Certificate, error) {
	rest := []byte(certPEM)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, errors.New("no CERTIFICATE block found")
		}
		if block.Type == "CERTIFICATE" {
			return x509.ParseCertificate(block.Bytes)
		}
	}
}

func parsePrivateKey(keyPEM string) (crypto.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, errors.New("no PRIVATE KEY block found")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("unsupported private key format")
}
