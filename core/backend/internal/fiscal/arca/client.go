package arca

import (
	"net/http"
	"time"
)

// Client es el adapter HTTP/SOAP de los web services de ARCA. Es stateless: las
// credenciales y el ambiente (homo/prod) se pasan por llamada, porque son por org.
type Client struct {
	http    *http.Client
	timeout time.Duration
}

func NewClient() *Client {
	timeout := 30 * time.Second
	return &Client{http: &http.Client{Timeout: timeout}, timeout: timeout}
}

// Credentials son los datos del emisor para autenticar y facturar.
type Credentials struct {
	CUIT       int64
	CertPEM    string
	KeyPEM     string
	Production bool
}

// TA es el Ticket de Acceso devuelto por el WSAA (token + firma, con vencimiento).
type TA struct {
	Token     string
	Sign      string
	ExpiresAt time.Time
}
