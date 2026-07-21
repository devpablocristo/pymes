package arca

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// wsfeNS es el namespace de WSFEv1 (dominio real de ARCA, no se renombra).
const wsfeNS = "http://ar.gov.afip.dif.FEV1/"

// AliqIva es una alícuota de IVA del comprobante (Id ARCA, base imponible, importe).
type AliqIva struct {
	ID      int
	BaseImp float64
	Importe float64
}

// CAERequest son los datos del comprobante para solicitar CAE (FECAESolicitar).
type CAERequest struct {
	PtoVta                 int
	CbteTipo               int
	Concepto               int
	DocTipo                int
	DocNro                 string
	CbteNro                int64
	CbteFch                string // YYYYMMDD
	ImpTotal               float64
	ImpTotConc             float64
	ImpNeto                float64
	ImpOpEx                float64
	ImpTrib                float64
	ImpIVA                 float64
	MonID                  string
	MonCotiz               float64
	CondicionIVAReceptorID int
	Iva                    []AliqIva
	// Servicios (Concepto 2/3):
	FchServDesde string
	FchServHasta string
	FchVtoPago   string
	// Comprobantes asociados (para NC/ND).
	CbtesAsoc []CbteAsoc
}

// CbteAsoc referencia el comprobante original de una nota de crédito/débito.
type CbteAsoc struct {
	Tipo   int
	PtoVta int
	Nro    int64
}

// Note es una observación o error de ARCA (código + mensaje).
type Note struct {
	Code int
	Msg  string
}

// CAEResult es el resultado de FECAESolicitar.
type CAEResult struct {
	Resultado    string // A=aprobado, R=rechazado, P=parcial
	CAE          string
	CAEFchVto    string // YYYYMMDD
	CbteNro      int64
	Observations []Note
	Errors       []Note
}

func authXML(ta TA, cuit int64) string {
	return "<ar:Auth><ar:Token>" + xmlEscape(ta.Token) + "</ar:Token><ar:Sign>" + xmlEscape(ta.Sign) +
		"</ar:Sign><ar:Cuit>" + strconv.FormatInt(cuit, 10) + "</ar:Cuit></ar:Auth>"
}

func envelope(body string) []byte {
	return []byte(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ar="` + wsfeNS + `">` +
		`<soapenv:Header/><soapenv:Body>` + body + `</soapenv:Body></soapenv:Envelope>`)
}

// LastAuthorized consulta el último número de comprobante autorizado para
// (PtoVta, CbteTipo). El siguiente a emitir es este + 1.
func (c *Client) LastAuthorized(ctx context.Context, prod bool, ta TA, cuit int64, ptoVta, cbteTipo int) (int64, error) {
	body := "<ar:FECompUltimoAutorizado>" + authXML(ta, cuit) +
		"<ar:PtoVta>" + strconv.Itoa(ptoVta) + "</ar:PtoVta>" +
		"<ar:CbteTipo>" + strconv.Itoa(cbteTipo) + "</ar:CbteTipo>" +
		"</ar:FECompUltimoAutorizado>"
	raw, err := c.doSOAP(ctx, EndpointsFor(prod).WSFE, wsfeNS+"FECompUltimoAutorizado", envelope(body))
	if err != nil {
		return 0, err
	}
	var resp struct {
		CbteNro int64  `xml:"Body>FECompUltimoAutorizadoResponse>FECompUltimoAutorizadoResult>CbteNro"`
		ErrMsg  string `xml:"Body>FECompUltimoAutorizadoResponse>FECompUltimoAutorizadoResult>Errors>Err>Msg"`
	}
	if err := xml.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("parse FECompUltimoAutorizado: %w", err)
	}
	if strings.TrimSpace(resp.ErrMsg) != "" {
		return 0, fmt.Errorf("wsfe error: %s", resp.ErrMsg)
	}
	return resp.CbteNro, nil
}

// RequestCAE solicita el CAE de un comprobante (FECAESolicitar).
func (c *Client) RequestCAE(ctx context.Context, prod bool, ta TA, cuit int64, req CAERequest) (CAEResult, error) {
	raw, err := c.doSOAP(ctx, EndpointsFor(prod).WSFE, wsfeNS+"FECAESolicitar", envelope(buildCAESolicitar(ta, cuit, req)))
	if err != nil {
		return CAEResult{}, err
	}
	return parseCAEResult(raw)
}

func buildCAESolicitar(ta TA, cuit int64, r CAERequest) string {
	var b strings.Builder
	b.WriteString("<ar:FECAESolicitar>")
	b.WriteString(authXML(ta, cuit))
	b.WriteString("<ar:FeCAEReq>")
	b.WriteString("<ar:FeCabReq><ar:CantReg>1</ar:CantReg><ar:PtoVta>")
	b.WriteString(strconv.Itoa(r.PtoVta))
	b.WriteString("</ar:PtoVta><ar:CbteTipo>")
	b.WriteString(strconv.Itoa(r.CbteTipo))
	b.WriteString("</ar:CbteTipo></ar:FeCabReq>")
	b.WriteString("<ar:FeDetReq><ar:FECAEDetRequest>")
	b.WriteString(tag("Concepto", strconv.Itoa(r.Concepto)))
	b.WriteString(tag("DocTipo", strconv.Itoa(r.DocTipo)))
	b.WriteString(tag("DocNro", r.DocNro))
	b.WriteString(tag("CbteDesde", strconv.FormatInt(r.CbteNro, 10)))
	b.WriteString(tag("CbteHasta", strconv.FormatInt(r.CbteNro, 10)))
	b.WriteString(tag("CbteFch", r.CbteFch))
	b.WriteString(tag("ImpTotal", amount(r.ImpTotal)))
	b.WriteString(tag("ImpTotConc", amount(r.ImpTotConc)))
	b.WriteString(tag("ImpNeto", amount(r.ImpNeto)))
	b.WriteString(tag("ImpOpEx", amount(r.ImpOpEx)))
	b.WriteString(tag("ImpTrib", amount(r.ImpTrib)))
	b.WriteString(tag("ImpIVA", amount(r.ImpIVA)))
	if r.Concepto == ConceptoServicios || r.Concepto == ConceptoAmbos {
		b.WriteString(tag("FchServDesde", r.FchServDesde))
		b.WriteString(tag("FchServHasta", r.FchServHasta))
		b.WriteString(tag("FchVtoPago", r.FchVtoPago))
	}
	b.WriteString(tag("MonId", r.MonID))
	b.WriteString(tag("MonCotiz", amount(r.MonCotiz)))
	if r.CondicionIVAReceptorID > 0 {
		b.WriteString(tag("CondicionIVAReceptorId", strconv.Itoa(r.CondicionIVAReceptorID)))
	}
	if len(r.CbtesAsoc) > 0 {
		b.WriteString("<ar:CbtesAsoc>")
		for _, a := range r.CbtesAsoc {
			b.WriteString("<ar:CbteAsoc>")
			b.WriteString(tag("Tipo", strconv.Itoa(a.Tipo)))
			b.WriteString(tag("PtoVta", strconv.Itoa(a.PtoVta)))
			b.WriteString(tag("Nro", strconv.FormatInt(a.Nro, 10)))
			b.WriteString("</ar:CbteAsoc>")
		}
		b.WriteString("</ar:CbtesAsoc>")
	}
	if len(r.Iva) > 0 {
		b.WriteString("<ar:Iva>")
		for _, a := range r.Iva {
			b.WriteString("<ar:AlicIva>")
			b.WriteString(tag("Id", strconv.Itoa(a.ID)))
			b.WriteString(tag("BaseImp", amount(a.BaseImp)))
			b.WriteString(tag("Importe", amount(a.Importe)))
			b.WriteString("</ar:AlicIva>")
		}
		b.WriteString("</ar:Iva>")
	}
	b.WriteString("</ar:FECAEDetRequest></ar:FeDetReq>")
	b.WriteString("</ar:FeCAEReq></ar:FECAESolicitar>")
	return b.String()
}

func parseCAEResult(raw []byte) (CAEResult, error) {
	var resp struct {
		Resultado string `xml:"Body>FECAESolicitarResponse>FECAESolicitarResult>FeCabResp>Resultado"`
		Det       struct {
			Resultado string `xml:"Resultado"`
			CAE       string `xml:"CAE"`
			CAEFchVto string `xml:"CAEFchVto"`
			CbteDesde int64  `xml:"CbteDesde"`
			Obs       []struct {
				Code int    `xml:"Code"`
				Msg  string `xml:"Msg"`
			} `xml:"Observaciones>Obs"`
		} `xml:"Body>FECAESolicitarResponse>FECAESolicitarResult>FeDetResp>FECAEDetResponse"`
		Errs []struct {
			Code int    `xml:"Code"`
			Msg  string `xml:"Msg"`
		} `xml:"Body>FECAESolicitarResponse>FECAESolicitarResult>Errors>Err"`
		Fault string `xml:"Body>Fault>faultstring"`
	}
	if err := xml.Unmarshal(raw, &resp); err != nil {
		return CAEResult{}, fmt.Errorf("parse FECAESolicitar: %w", err)
	}
	if strings.TrimSpace(resp.Fault) != "" {
		return CAEResult{}, fmt.Errorf("wsfe fault: %s", resp.Fault)
	}
	out := CAEResult{
		Resultado: firstNonEmpty(resp.Det.Resultado, resp.Resultado),
		CAE:       resp.Det.CAE,
		CAEFchVto: resp.Det.CAEFchVto,
		CbteNro:   resp.Det.CbteDesde,
	}
	for _, o := range resp.Det.Obs {
		out.Observations = append(out.Observations, Note{Code: o.Code, Msg: o.Msg})
	}
	for _, e := range resp.Errs {
		out.Errors = append(out.Errors, Note{Code: e.Code, Msg: e.Msg})
	}
	return out, nil
}

func tag(name, val string) string { return "<ar:" + name + ">" + xmlEscape(val) + "</ar:" + name + ">" }

func amount(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
