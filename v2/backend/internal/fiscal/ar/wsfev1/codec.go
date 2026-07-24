package wsfev1

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
)

func BuildAuthorizeEnvelope(auth Auth, request Request) ([]byte, error) {
	if err := auth.validate(); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	issueDate, _ := formatARCADate(request.IssueDate)
	currency, _ := ar.CurrencyCode(request.Currency)
	receiver, _ := ar.NewReceiverDocument(request.Receiver.Type, request.Receiver.Number)
	var body strings.Builder
	body.WriteString("<ar:FECAESolicitar>")
	body.WriteString(authXML(auth))
	body.WriteString("<ar:FeCAEReq><ar:FeCabReq>")
	body.WriteString(tag("CantReg", "1"))
	body.WriteString(tag("PtoVta", strconv.Itoa(request.PointOfSale)))
	body.WriteString(tag("CbteTipo", strconv.Itoa(int(request.VoucherType))))
	body.WriteString("</ar:FeCabReq><ar:FeDetReq><ar:FECAEDetRequest>")
	body.WriteString(tag("Concepto", strconv.Itoa(int(request.Concept))))
	body.WriteString(tag("DocTipo", strconv.Itoa(int(receiver.Type))))
	body.WriteString(tag("DocNro", receiver.Number))
	body.WriteString(tag("CbteDesde", strconv.FormatInt(request.Number, 10)))
	body.WriteString(tag("CbteHasta", strconv.FormatInt(request.Number, 10)))
	body.WriteString(tag("CbteFch", issueDate))
	body.WriteString(amountTag("ImpTotal", request.Totals.Total, 2))
	body.WriteString(amountTag("ImpTotConc", request.Totals.NetUntaxed, 2))
	body.WriteString(amountTag("ImpNeto", request.Totals.NetTaxed, 2))
	body.WriteString(amountTag("ImpOpEx", request.Totals.Exempt, 2))
	body.WriteString(amountTag("ImpTrib", request.Totals.Tributes, 2))
	body.WriteString(amountTag("ImpIVA", request.Totals.VAT, 2))

	if request.Concept.NeedsServiceDates() {
		serviceFrom, _ := formatARCADate(request.ServiceFrom)
		serviceTo, _ := formatARCADate(request.ServiceTo)
		paymentDue, _ := formatARCADate(request.PaymentDue)
		body.WriteString(tag("FchServDesde", serviceFrom))
		body.WriteString(tag("FchServHasta", serviceTo))
		body.WriteString(tag("FchVtoPago", paymentDue))
	}
	body.WriteString(tag("MonId", currency))
	body.WriteString(amountTag("MonCotiz", request.ExchangeRate, 6))
	body.WriteString(tag("CondicionIVAReceptorId", strconv.Itoa(int(request.ReceiverVATCondition))))

	if request.Associated != nil {
		body.WriteString("<ar:CbtesAsoc><ar:CbteAsoc>")
		body.WriteString(tag("Tipo", strconv.Itoa(int(request.Associated.Type))))
		body.WriteString(tag("PtoVta", strconv.Itoa(request.Associated.PointOfSale)))
		body.WriteString(tag("Nro", strconv.FormatInt(request.Associated.Number, 10)))
		if request.Associated.IssuerCUIT != "" {
			body.WriteString(tag("Cuit", request.Associated.IssuerCUIT.String()))
		}
		if request.Associated.IssueDate != "" {
			associatedDate, _ := formatARCADate(request.Associated.IssueDate)
			body.WriteString(tag("CbteFch", associatedDate))
		}
		body.WriteString("</ar:CbteAsoc></ar:CbtesAsoc>")
	}
	if len(request.Tributes) > 0 {
		body.WriteString("<ar:Tributos>")
		for _, tribute := range request.Tributes {
			body.WriteString("<ar:Tributo>")
			body.WriteString(tag("Id", strconv.Itoa(tribute.ID)))
			body.WriteString(tag("Desc", tribute.Description))
			body.WriteString(amountTag("BaseImp", tribute.BaseAmount, 2))
			body.WriteString(amountTag("Alic", tribute.Rate, 4))
			body.WriteString(amountTag("Importe", tribute.Amount, 2))
			body.WriteString("</ar:Tributo>")
		}
		body.WriteString("</ar:Tributos>")
	}
	if len(request.Totals.VATLines) > 0 {
		body.WriteString("<ar:Iva>")
		for _, line := range request.Totals.VATLines {
			body.WriteString("<ar:AlicIva>")
			body.WriteString(tag("Id", strconv.Itoa(line.ID)))
			body.WriteString(amountTag("BaseImp", line.BaseAmount, 2))
			body.WriteString(amountTag("Importe", line.Amount, 2))
			body.WriteString("</ar:AlicIva>")
		}
		body.WriteString("</ar:Iva>")
	}
	if len(request.ActivityIDs) > 0 {
		body.WriteString("<ar:Actividades>")
		for _, activityID := range request.ActivityIDs {
			body.WriteString("<ar:Actividad>")
			body.WriteString(tag("Id", strconv.FormatInt(activityID, 10)))
			body.WriteString("</ar:Actividad>")
		}
		body.WriteString("</ar:Actividades>")
	}
	body.WriteString("</ar:FECAEDetRequest></ar:FeDetReq></ar:FeCAEReq></ar:FECAESolicitar>")
	return envelope(body.String()), nil
}

func BuildLastAuthorizedEnvelope(auth Auth, pointOfSale int, voucherType ar.VoucherType) ([]byte, error) {
	if err := auth.validate(); err != nil {
		return nil, err
	}
	if pointOfSale <= 0 || pointOfSale > 99999 || !voucherType.ValidMVP() {
		return nil, errors.New("invalid WSFE sequence key")
	}
	body := "<ar:FECompUltimoAutorizado>" + authXML(auth) +
		tag("PtoVta", strconv.Itoa(pointOfSale)) +
		tag("CbteTipo", strconv.Itoa(int(voucherType))) +
		"</ar:FECompUltimoAutorizado>"
	return envelope(body), nil
}

func BuildConsultEnvelope(auth Auth, pointOfSale int, voucherType ar.VoucherType, number int64) ([]byte, error) {
	if err := auth.validate(); err != nil {
		return nil, err
	}
	if pointOfSale <= 0 || pointOfSale > 99999 || !voucherType.ValidMVP() ||
		number <= 0 || number > 99999999 {
		return nil, errors.New("invalid WSFE consult key")
	}
	body := "<ar:FECompConsultar>" + authXML(auth) +
		"<ar:FeCompConsReq>" +
		tag("CbteTipo", strconv.Itoa(int(voucherType))) +
		tag("CbteNro", strconv.FormatInt(number, 10)) +
		tag("PtoVta", strconv.Itoa(pointOfSale)) +
		"</ar:FeCompConsReq></ar:FECompConsultar>"
	return envelope(body), nil
}

func envelope(body string) []byte {
	return []byte(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ar="` +
		Namespace + `"><soapenv:Header/><soapenv:Body>` + body + `</soapenv:Body></soapenv:Envelope>`)
}

func authXML(auth Auth) string {
	return "<ar:Auth>" +
		tag("Token", auth.Ticket.Token) +
		tag("Sign", auth.Ticket.Sign) +
		tag("Cuit", auth.CUIT.String()) +
		"</ar:Auth>"
}

func tag(name, value string) string {
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(value))
	return "<ar:" + name + ">" + escaped.String() + "</ar:" + name + ">"
}

func amountTag(name string, value fiscal.Decimal, scale int32) string {
	formatted, _ := value.FormatFixed(scale, fiscal.RoundHalfAwayFromZero)
	return tag(name, formatted)
}

type responseNote struct {
	Code int    `xml:"Code"`
	Msg  string `xml:"Msg"`
}

type soapFault struct {
	Code   string `xml:"faultcode"`
	Reason string `xml:"faultstring"`
}

type FaultError struct {
	Code   string
	Reason string
}

func (fault FaultError) Error() string {
	if fault.Code == "" {
		return "ARCA WSFE fault: " + fault.Reason
	}
	return "ARCA WSFE fault " + fault.Code + ": " + fault.Reason
}

func ParseLastAuthorizedResponse(raw []byte) (int64, error) {
	var response struct {
		Number int64          `xml:"Body>FECompUltimoAutorizadoResponse>FECompUltimoAutorizadoResult>CbteNro"`
		Errors []responseNote `xml:"Body>FECompUltimoAutorizadoResponse>FECompUltimoAutorizadoResult>Errors>Err"`
		Fault  soapFault      `xml:"Body>Fault"`
	}
	if err := xml.Unmarshal(raw, &response); err != nil {
		return 0, fmt.Errorf("parse FECompUltimoAutorizado: %w", err)
	}
	if response.Fault.Reason != "" {
		return 0, FaultError{Code: response.Fault.Code, Reason: response.Fault.Reason}
	}
	if len(response.Errors) > 0 {
		return 0, notesError("FECompUltimoAutorizado", response.Errors)
	}
	if response.Number < 0 {
		return 0, errors.New("FECompUltimoAutorizado returned a negative number")
	}
	return response.Number, nil
}

func ParseAuthorizeResponse(raw []byte) (AuthorizationResult, error) {
	var response struct {
		HeaderDecision string `xml:"Body>FECAESolicitarResponse>FECAESolicitarResult>FeCabResp>Resultado"`
		Detail         struct {
			Decision     string         `xml:"Resultado"`
			CAE          string         `xml:"CAE"`
			ExpiresOn    string         `xml:"CAEFchVto"`
			Number       int64          `xml:"CbteDesde"`
			ProcessedDay string         `xml:"CbteFch"`
			Observations []responseNote `xml:"Observaciones>Obs"`
		} `xml:"Body>FECAESolicitarResponse>FECAESolicitarResult>FeDetResp>FECAEDetResponse"`
		Errors []responseNote `xml:"Body>FECAESolicitarResponse>FECAESolicitarResult>Errors>Err"`
		Fault  soapFault      `xml:"Body>Fault"`
	}
	if err := xml.Unmarshal(raw, &response); err != nil {
		return AuthorizationResult{}, fmt.Errorf("parse FECAESolicitar: %w", err)
	}
	if response.Fault.Reason != "" {
		return AuthorizationResult{}, FaultError{Code: response.Fault.Code, Reason: response.Fault.Reason}
	}
	decision := strings.TrimSpace(response.Detail.Decision)
	if decision == "" {
		decision = strings.TrimSpace(response.HeaderDecision)
	}
	result := AuthorizationResult{
		CAE:          strings.TrimSpace(response.Detail.CAE),
		CAEExpiresOn: strings.TrimSpace(response.Detail.ExpiresOn),
		Number:       response.Detail.Number,
		Observations: mapNotes(response.Detail.Observations),
		Errors:       mapNotes(response.Errors),
		RawResponse:  append([]byte(nil), raw...),
	}
	switch decision {
	case "A":
		result.Decision = fiscal.DecisionAuthorized
		if result.CAE == "" {
			return AuthorizationResult{}, errors.New("FECAESolicitar approved without CAE")
		}
	case "R":
		result.Decision = fiscal.DecisionRejected
	default:
		return AuthorizationResult{}, fmt.Errorf("FECAESolicitar returned unsupported result %q", decision)
	}
	if response.Detail.ProcessedDay != "" {
		if processed, err := parseARCADate(response.Detail.ProcessedDay); err == nil {
			result.ProcessedAt = processed
		}
	}
	return result, nil
}

func ParseConsultResponse(raw []byte) (ConsultResult, error) {
	var response struct {
		Result struct {
			Concept      int            `xml:"Concepto"`
			DocumentType int            `xml:"DocTipo"`
			Document     string         `xml:"DocNro"`
			Number       int64          `xml:"CbteDesde"`
			IssueDate    string         `xml:"CbteFch"`
			Total        string         `xml:"ImpTotal"`
			Untaxed      string         `xml:"ImpTotConc"`
			Net          string         `xml:"ImpNeto"`
			Exempt       string         `xml:"ImpOpEx"`
			Tributes     string         `xml:"ImpTrib"`
			VAT          string         `xml:"ImpIVA"`
			Currency     string         `xml:"MonId"`
			ExchangeRate string         `xml:"MonCotiz"`
			Decision     string         `xml:"Resultado"`
			Code         string         `xml:"CodAutorizacion"`
			EmissionType string         `xml:"EmisionTipo"`
			ExpiresOn    string         `xml:"FchVto"`
			ProcessedAt  string         `xml:"FchProceso"`
			PointOfSale  int            `xml:"PtoVta"`
			VoucherType  int            `xml:"CbteTipo"`
			Observations []responseNote `xml:"Observaciones>Obs"`
			VATLines     []struct {
				ID     int    `xml:"Id"`
				Base   string `xml:"BaseImp"`
				Amount string `xml:"Importe"`
			} `xml:"Iva>AlicIva"`
		} `xml:"Body>FECompConsultarResponse>FECompConsultarResult>ResultGet"`
		Errors []responseNote `xml:"Body>FECompConsultarResponse>FECompConsultarResult>Errors>Err"`
		Fault  soapFault      `xml:"Body>Fault"`
	}
	if err := xml.Unmarshal(raw, &response); err != nil {
		return ConsultResult{}, fmt.Errorf("parse FECompConsultar: %w", err)
	}
	if response.Fault.Reason != "" {
		return ConsultResult{}, FaultError{Code: response.Fault.Code, Reason: response.Fault.Reason}
	}
	if response.Result.Number == 0 {
		if onlyNotFound(response.Errors) {
			return ConsultResult{Found: false, RawResponse: append([]byte(nil), raw...)}, nil
		}
		if len(response.Errors) > 0 {
			return ConsultResult{}, notesError("FECompConsultar", response.Errors)
		}
		return ConsultResult{Found: false, RawResponse: append([]byte(nil), raw...)}, nil
	}

	document, err := ar.NewReceiverDocument(ar.DocumentType(response.Result.DocumentType), response.Result.Document)
	if err != nil {
		return ConsultResult{}, fmt.Errorf("parse FECompConsultar receiver: %w", err)
	}
	net, err := parseResponseDecimal("ImpNeto", response.Result.Net)
	if err != nil {
		return ConsultResult{}, err
	}
	untaxed, err := parseResponseDecimal("ImpTotConc", response.Result.Untaxed)
	if err != nil {
		return ConsultResult{}, err
	}
	exempt, err := parseResponseDecimal("ImpOpEx", response.Result.Exempt)
	if err != nil {
		return ConsultResult{}, err
	}
	tributes, err := parseResponseDecimal("ImpTrib", response.Result.Tributes)
	if err != nil {
		return ConsultResult{}, err
	}
	vat, err := parseResponseDecimal("ImpIVA", response.Result.VAT)
	if err != nil {
		return ConsultResult{}, err
	}
	total, err := parseResponseDecimal("ImpTotal", response.Result.Total)
	if err != nil {
		return ConsultResult{}, err
	}
	exchangeRate, err := parseResponseDecimal("MonCotiz", response.Result.ExchangeRate)
	if err != nil {
		return ConsultResult{}, err
	}
	totals := ar.Totals{
		NetTaxed: net, NetUntaxed: untaxed, Exempt: exempt,
		Tributes: tributes, VAT: vat, Total: total,
	}
	for index, line := range response.Result.VATLines {
		rate, found := ar.VATRateForID(line.ID)
		if !found {
			return ConsultResult{}, fmt.Errorf("FECompConsultar VAT line %d has unknown ID %d", index, line.ID)
		}
		base, err := parseResponseDecimal("Iva.BaseImp", line.Base)
		if err != nil {
			return ConsultResult{}, err
		}
		amount, err := parseResponseDecimal("Iva.Importe", line.Amount)
		if err != nil {
			return ConsultResult{}, err
		}
		totals.VATLines = append(totals.VATLines, ar.VATBreakdown{
			ID: line.ID, Rate: rate, BaseAmount: base, Amount: amount,
		})
	}
	decision := fiscal.DecisionRejected
	if strings.TrimSpace(response.Result.Decision) == "A" {
		decision = fiscal.DecisionAuthorized
	}
	processedAt := time.Time{}
	if response.Result.ProcessedAt != "" {
		processedAt, _ = parseARCADate(response.Result.ProcessedAt)
	}
	return ConsultResult{
		Found: true, VoucherType: ar.VoucherType(response.Result.VoucherType),
		PointOfSale: response.Result.PointOfSale, Number: response.Result.Number,
		Concept: ar.Concept(response.Result.Concept), Receiver: document,
		IssueDate: response.Result.IssueDate, Totals: totals,
		Currency: strings.TrimSpace(response.Result.Currency), ExchangeRate: exchangeRate,
		Decision: decision, Code: strings.TrimSpace(response.Result.Code),
		EmissionType: strings.TrimSpace(response.Result.EmissionType),
		ExpiresOn:    strings.TrimSpace(response.Result.ExpiresOn),
		ProcessedAt:  processedAt, Observations: mapNotes(response.Result.Observations),
		Errors: mapNotes(response.Errors), RawResponse: append([]byte(nil), raw...),
	}, nil
}

func parseResponseDecimal(field, value string) (fiscal.Decimal, error) {
	if strings.TrimSpace(value) == "" {
		return fiscal.Decimal{}, nil
	}
	decimal, err := fiscal.ParseDecimal(value)
	if err != nil {
		return fiscal.Decimal{}, fmt.Errorf("parse FECompConsultar %s: %w", field, err)
	}
	return decimal, nil
}

func onlyNotFound(notes []responseNote) bool {
	return len(notes) == 1 && notes[0].Code == 602
}

func mapNotes(notes []responseNote) []Note {
	if len(notes) == 0 {
		return nil
	}
	result := make([]Note, 0, len(notes))
	for _, note := range notes {
		result = append(result, Note{Code: note.Code, Message: strings.TrimSpace(note.Msg)})
	}
	return result
}

func notesError(operation string, notes []responseNote) error {
	parts := make([]string, 0, len(notes))
	for _, note := range notes {
		parts = append(parts, fmt.Sprintf("%d: %s", note.Code, strings.TrimSpace(note.Msg)))
	}
	return fmt.Errorf("%s errors: %s", operation, strings.Join(parts, "; "))
}
