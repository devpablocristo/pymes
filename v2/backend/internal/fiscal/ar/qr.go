package ar

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

const QRBaseURL = "https://www.arca.gob.ar/fe/qr/?p="

type QRInput struct {
	IssueDate         string
	IssuerCUIT        CUIT
	PointOfSale       int
	VoucherType       VoucherType
	VoucherNumber     int64
	Total             fiscal.Decimal
	Currency          string
	ExchangeRate      fiscal.Decimal
	ReceiverDocument  *ReceiverDocument
	AuthorizationCode string
}

type qrPayload struct {
	Version           int             `json:"ver"`
	IssueDate         string          `json:"fecha"`
	IssuerCUIT        json.RawMessage `json:"cuit"`
	PointOfSale       int             `json:"ptoVta"`
	VoucherType       int             `json:"tipoCmp"`
	VoucherNumber     int64           `json:"nroCmp"`
	Total             json.RawMessage `json:"importe"`
	Currency          string          `json:"moneda"`
	ExchangeRate      json.RawMessage `json:"ctz"`
	ReceiverDocType   *int            `json:"tipoDocRec,omitempty"`
	ReceiverDocNumber json.RawMessage `json:"nroDocRec,omitempty"`
	AuthorizationType string          `json:"tipoCodAut"`
	AuthorizationCode json.RawMessage `json:"codAut"`
}

func BuildQRPayload(input QRInput) ([]byte, error) {
	if _, err := ParseCUIT(input.IssuerCUIT.String()); err != nil {
		return nil, fmt.Errorf("issuer CUIT: %w", err)
	}
	if _, err := time.Parse("2006-01-02", input.IssueDate); err != nil {
		return nil, errors.New("QR issue date must use YYYY-MM-DD")
	}
	if input.PointOfSale <= 0 || input.PointOfSale > 99999 {
		return nil, errors.New("QR point of sale must be between 1 and 99999")
	}
	if !input.VoucherType.ValidMVP() {
		return nil, errors.New("QR voucher type is not supported")
	}
	if input.VoucherNumber <= 0 || input.VoucherNumber > 99999999 {
		return nil, errors.New("QR voucher number must be between 1 and 99999999")
	}
	if input.Total.IsNegative() {
		return nil, errors.New("QR total cannot be negative")
	}
	currency, err := CurrencyCode(input.Currency)
	if err != nil {
		return nil, err
	}
	if input.ExchangeRate.Cmp(fiscal.Decimal{}) <= 0 {
		return nil, errors.New("QR exchange rate must be positive")
	}
	if currency == CurrencyPES && !input.ExchangeRate.Equal(fiscal.NewDecimalFromInt(1)) {
		return nil, errors.New("PES exchange rate must equal 1")
	}
	authorizationCode := strings.TrimSpace(input.AuthorizationCode)
	if len(authorizationCode) == 0 || len(authorizationCode) > 14 || !onlyDigits(authorizationCode) {
		return nil, errors.New("QR authorization code must contain up to 14 digits")
	}
	total, err := input.Total.FormatFixed(2, fiscal.RoundHalfAwayFromZero)
	if err != nil {
		return nil, err
	}
	exchangeRate, err := input.ExchangeRate.FormatFixed(6, fiscal.RoundHalfAwayFromZero)
	if err != nil {
		return nil, err
	}

	payload := qrPayload{
		Version:           1,
		IssueDate:         input.IssueDate,
		IssuerCUIT:        json.RawMessage(input.IssuerCUIT.String()),
		PointOfSale:       input.PointOfSale,
		VoucherType:       int(input.VoucherType),
		VoucherNumber:     input.VoucherNumber,
		Total:             json.RawMessage(total),
		Currency:          currency,
		ExchangeRate:      json.RawMessage(exchangeRate),
		AuthorizationType: "E",
		AuthorizationCode: json.RawMessage(authorizationCode),
	}
	if input.ReceiverDocument != nil {
		document, err := NewReceiverDocument(input.ReceiverDocument.Type, input.ReceiverDocument.Number)
		if err != nil {
			return nil, fmt.Errorf("QR receiver document: %w", err)
		}
		documentType := int(document.Type)
		payload.ReceiverDocType = &documentType
		payload.ReceiverDocNumber = json.RawMessage(document.Number)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal ARCA QR payload: %w", err)
	}
	return raw, nil
}

func BuildQRURL(input QRInput) (string, error) {
	payload, err := BuildQRPayload(input)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(payload)
	if _, err := url.Parse(QRBaseURL + encoded); err != nil {
		return "", err
	}
	return QRBaseURL + encoded, nil
}

func ParseQRURL(rawURL string) (map[string]json.RawMessage, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	encoded := parsed.Query().Get("p")
	if encoded == "" {
		return nil, errors.New("QR URL does not contain payload")
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, err
	}
	return document, nil
}

func DocumentNumberInt64(document ReceiverDocument) (int64, error) {
	value, err := strconv.ParseInt(document.Number, 10, 64)
	if err != nil {
		return 0, errors.New("receiver document does not fit an integer")
	}
	return value, nil
}
