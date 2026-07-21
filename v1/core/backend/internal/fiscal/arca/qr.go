package arca

import (
	"encoding/base64"
	"encoding/json"
)

// QRInput son los datos del comprobante para el QR obligatorio (RG 4892).
type QRInput struct {
	Fecha      string  // YYYY-MM-DD (fecha del comprobante)
	CUIT       int64   // CUIT emisor (sin guiones)
	PtoVta     int     // punto de venta
	TipoCmp    int     // CbteTipo
	NroCmp     int64   // número de comprobante
	Importe    float64 // total del comprobante
	Moneda     string  // código moneda (PES, DOL, …)
	Ctz        float64 // cotización
	TipoDocRec int     // tipo doc receptor
	NroDocRec  int64   // número doc receptor
	CodAut     int64   // CAE (o CAEA)
}

// qrPayload es el JSON que ARCA espera codificado en base64 dentro del QR.
// El orden/nombres de campos siguen la especificación RG 4892.
type qrPayload struct {
	Ver        int     `json:"ver"`
	Fecha      string  `json:"fecha"`
	CUIT       int64   `json:"cuit"`
	PtoVta     int     `json:"ptoVta"`
	TipoCmp    int     `json:"tipoCmp"`
	NroCmp     int64   `json:"nroCmp"`
	Importe    float64 `json:"importe"`
	Moneda     string  `json:"moneda"`
	Ctz        float64 `json:"ctz"`
	TipoDocRec int     `json:"tipoDocRec"`
	NroDocRec  int64   `json:"nroDocRec"`
	TipoCodAut string  `json:"tipoCodAut"`
	CodAut     int64   `json:"codAut"`
}

// QRBaseURL es el visor de comprobantes de ARCA (dominio afip.gov.ar real).
const QRBaseURL = "https://www.afip.gob.ar/fe/qr/?p="

// BuildQRURL arma la URL del QR (RG 4892): base64(JSON) sobre QRBaseURL.
// tipoCodAut "E" = CAE.
func BuildQRURL(in QRInput) (string, error) {
	payload := qrPayload{
		Ver: 1, Fecha: in.Fecha, CUIT: in.CUIT, PtoVta: in.PtoVta, TipoCmp: in.TipoCmp,
		NroCmp: in.NroCmp, Importe: in.Importe, Moneda: in.Moneda, Ctz: in.Ctz,
		TipoDocRec: in.TipoDocRec, NroDocRec: in.NroDocRec, TipoCodAut: "E", CodAut: in.CodAut,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return QRBaseURL + base64.StdEncoding.EncodeToString(raw), nil
}
