package arca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"go.mozilla.org/pkcs7"
)

func selfSignedPEM(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-emisor"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	return certPEM, keyPEM
}

func TestBuildTRA(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	tra := string(BuildTRA("wsfe", now))
	for _, want := range []string{"<loginTicketRequest", "<uniqueId>", "<service>wsfe</service>", "<generationTime>", "<expirationTime>"} {
		if !strings.Contains(tra, want) {
			t.Fatalf("TRA missing %q: %s", want, tra)
		}
	}
}

func TestSignTRAProducesParseableCMS(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := selfSignedPEM(t)
	tra := BuildTRA("wsfe", time.Now())

	b64, err := SignTRA(tra, certPEM, keyPEM)
	if err != nil {
		t.Fatalf("SignTRA: %v", err)
	}
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	p7, err := pkcs7.Parse(der)
	if err != nil {
		t.Fatalf("pkcs7 parse (CMS inválido): %v", err)
	}
	// No-detached: el contenido firmado debe ser el TRA embebido.
	if string(p7.Content) != string(tra) {
		t.Fatalf("CMS content != TRA")
	}
	if len(p7.Certificates) == 0 {
		t.Fatalf("CMS sin certificado del firmante")
	}
}

func TestSignTRABadPEM(t *testing.T) {
	t.Parallel()
	if _, err := SignTRA([]byte("x"), "not a pem", "not a pem"); err == nil {
		t.Fatalf("expected error with bad PEM")
	}
}

func TestBuildQRURL(t *testing.T) {
	t.Parallel()
	url, err := BuildQRURL(QRInput{
		Fecha: "2026-07-01", CUIT: 20111111112, PtoVta: 1, TipoCmp: CbteFacturaB,
		NroCmp: 42, Importe: 121, Moneda: MonedaPesos, Ctz: 1, TipoDocRec: DocConsumidorFinal,
		NroDocRec: 0, CodAut: 74000000000001,
	})
	if err != nil {
		t.Fatalf("BuildQRURL: %v", err)
	}
	if !strings.HasPrefix(url, QRBaseURL) {
		t.Fatalf("URL sin prefijo esperado: %s", url)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, QRBaseURL))
	if err != nil {
		t.Fatalf("payload base64 inválido: %v", err)
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("payload JSON inválido: %v", err)
	}
	if p["ver"].(float64) != 1 || p["tipoCodAut"].(string) != "E" || p["codAut"].(float64) != 74000000000001 {
		t.Fatalf("payload QR incorrecto: %#v", p)
	}
}

func TestIvaIDForRate(t *testing.T) {
	t.Parallel()
	cases := map[float64]int{21: IvaID21, 10.5: IvaID105, 27: IvaID27, 0: IvaID0}
	for rate, want := range cases {
		if got, ok := IvaIDForRate(rate); !ok || got != want {
			t.Fatalf("IvaIDForRate(%v)=%d,%v want %d", rate, got, ok, want)
		}
	}
	if _, ok := IvaIDForRate(13.7); ok {
		t.Fatalf("alícuota no soportada debería dar ok=false")
	}
}

func TestParseLoginResponse(t *testing.T) {
	t.Parallel()
	body := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body>` +
		`<loginCmsResponse><loginCmsReturn>` +
		`&lt;?xml version="1.0"?&gt;&lt;loginTicketResponse&gt;` +
		`&lt;header&gt;&lt;expirationTime&gt;2030-01-01T10:00:00-03:00&lt;/expirationTime&gt;&lt;/header&gt;` +
		`&lt;credentials&gt;&lt;token&gt;TOK123&lt;/token&gt;&lt;sign&gt;SGN456&lt;/sign&gt;&lt;/credentials&gt;` +
		`&lt;/loginTicketResponse&gt;` +
		`</loginCmsReturn></loginCmsResponse></soap:Body></soap:Envelope>`
	ta, err := parseLoginResponse([]byte(body))
	if err != nil {
		t.Fatalf("parseLoginResponse: %v", err)
	}
	if ta.Token != "TOK123" || ta.Sign != "SGN456" {
		t.Fatalf("TA mal parseado: %+v", ta)
	}
	if ta.ExpiresAt.Year() != 2030 {
		t.Fatalf("expiration mal parseada: %v", ta.ExpiresAt)
	}
}

func TestParseLoginResponseFault(t *testing.T) {
	t.Parallel()
	body := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body>` +
		`<soap:Fault><faultstring>cms.sign.invalid</faultstring></soap:Fault>` +
		`</soap:Body></soap:Envelope>`
	if _, err := parseLoginResponse([]byte(body)); err == nil {
		t.Fatalf("expected fault error")
	}
}
