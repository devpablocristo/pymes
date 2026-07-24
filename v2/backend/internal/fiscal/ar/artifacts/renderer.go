package artifacts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render is deliberately snapshot-only: the mutable sale/customer records are
// never consulted after the fiscal request has been queued.
func (renderer *Renderer) Render(
	_ context.Context,
	snapshot fiscal.Snapshot,
	authorization fiscal.Authorization,
) ([]fiscal.RenderedArtifact, error) {
	if renderer == nil {
		return nil, errors.New("nil Argentina fiscal artifact renderer")
	}
	if authorization.Decision != fiscal.DecisionAuthorized {
		return nil, errors.New("fiscal artifacts require an authorized voucher")
	}
	document, err := snapshot.Document()
	if err != nil {
		return nil, err
	}
	voucherType, pointOfSale, err := voucherIdentity(document.Metadata)
	if err != nil {
		return nil, err
	}
	qrURL, err := fiscalQRURL(document, voucherType, pointOfSale, authorization)
	if err != nil {
		return nil, err
	}
	qrPNG, err := qrcode.Encode(qrURL, qrcode.Medium, 320)
	if err != nil {
		return nil, fmt.Errorf("render fiscal QR: %w", err)
	}
	pdfBody, err := fiscalPDF(document, voucherType, pointOfSale, authorization, qrPNG)
	if err != nil {
		return nil, err
	}
	return []fiscal.RenderedArtifact{
		{Kind: "qr", ContentType: "image/png", Body: qrPNG},
		{Kind: "pdf", ContentType: "application/pdf", Body: pdfBody},
	}, nil
}

func voucherIdentity(metadata map[string]string) (ar.VoucherType, int, error) {
	voucherTypeValue, err := strconv.Atoi(strings.TrimSpace(metadata["voucher_type"]))
	if err != nil || !ar.VoucherType(voucherTypeValue).ValidMVP() {
		return 0, 0, errors.New("fiscal snapshot lacks a valid voucher_type")
	}
	pointOfSale, err := strconv.Atoi(strings.TrimSpace(metadata["point_of_sale"]))
	if err != nil || pointOfSale < 1 || pointOfSale > 99999 {
		return 0, 0, errors.New("fiscal snapshot lacks a valid point_of_sale")
	}
	return ar.VoucherType(voucherTypeValue), pointOfSale, nil
}

func fiscalQRURL(
	document fiscal.FiscalSnapshot,
	voucherType ar.VoucherType,
	pointOfSale int,
	authorization fiscal.Authorization,
) (string, error) {
	issuerCUIT, err := ar.ParseCUIT(document.Issuer.TaxID)
	if err != nil {
		return "", err
	}
	var receiver *ar.ReceiverDocument
	if document.Receiver.DocumentType != "" {
		rawType, parseErr := strconv.Atoi(document.Receiver.DocumentType)
		if parseErr != nil {
			return "", errors.New("invalid receiver document type in fiscal snapshot")
		}
		value, parseErr := ar.NewReceiverDocument(
			ar.DocumentType(rawType), document.Receiver.DocumentNumber,
		)
		if parseErr != nil {
			return "", parseErr
		}
		receiver = &value
	}
	return ar.BuildQRURL(ar.QRInput{
		IssueDate:         document.IssueDate,
		IssuerCUIT:        issuerCUIT,
		PointOfSale:       pointOfSale,
		VoucherType:       voucherType,
		VoucherNumber:     authorization.Number,
		Total:             document.Totals.Total,
		Currency:          document.Currency.Code,
		ExchangeRate:      document.Currency.Rate,
		ReceiverDocument:  receiver,
		AuthorizationCode: authorization.Code,
	})
}

func fiscalPDF(
	document fiscal.FiscalSnapshot,
	voucherType ar.VoucherType,
	pointOfSale int,
	authorization fiscal.Authorization,
	qrPNG []byte,
) ([]byte, error) {
	letter, err := voucherType.Letter()
	if err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCatalogSort(true)
	pdf.SetMargins(14, 14, 14)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetCompression(true)
	pdf.SetTitle(voucherTitle(voucherType), true)
	pdf.SetAuthor(document.Issuer.Name, true)
	creationDate := authorization.ProcessedAt.UTC()
	if creationDate.IsZero() {
		creationDate, _ = time.Parse("2006-01-02", document.IssueDate)
	}
	pdf.SetCreationDate(creationDate)
	pdf.SetModificationDate(creationDate)
	pdf.AddPage()
	text := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(82, 8, text(document.Issuer.Name), "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 25)
	pdf.CellFormat(18, 14, letter, "1", 0, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(82, 8, text(voucherTitle(voucherType)), "", 1, "R", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(82, 5, text(document.Issuer.Address), "", 0, "L", false, 0, "")
	pdf.CellFormat(18, 5, fmt.Sprintf("COD. %02d", voucherType), "", 0, "C", false, 0, "")
	pdf.CellFormat(
		82, 5,
		fmt.Sprintf("N° %05d-%08d", pointOfSale, authorization.Number),
		"", 1, "R", false, 0, "",
	)
	pdf.CellFormat(
		100, 5,
		text("CUIT: "+document.Issuer.TaxID+" · IVA: "+taxConditionLabel(document.Issuer.TaxCondition)),
		"", 0, "L", false, 0, "",
	)
	pdf.CellFormat(82, 5, text("Fecha: "+displayDate(document.IssueDate)), "", 1, "R", false, 0, "")
	if document.Issuer.ActivityStartDay != "" {
		pdf.CellFormat(
			0, 5, text("Inicio de actividades: "+displayDate(document.Issuer.ActivityStartDay)),
			"", 1, "L", false, 0, "",
		)
	}
	pdf.Ln(4)

	pdf.SetFillColor(235, 241, 249)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 7, text("RECEPTOR"), "1", 1, "L", true, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(0, 6, text(document.Receiver.Name), "LR", 1, "L", false, 0, "")
	receiverIdentity := strings.TrimSpace(document.Receiver.DocumentNumber)
	if receiverIdentity == "" {
		receiverIdentity = "Sin documento"
	}
	pdf.CellFormat(
		0, 6,
		text(receiverIdentity+" · IVA: "+taxConditionLabel(document.Receiver.TaxCondition)),
		"LRB", 1, "L", false, 0, "",
	)
	pdf.Ln(4)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(17, 33, 57)
	pdf.SetTextColor(255, 255, 255)
	for _, heading := range []struct {
		width float64
		label string
		align string
	}{
		{92, "Descripción", "L"},
		{20, "Cant.", "R"},
		{32, "P. unitario", "R"},
		{38, "Total", "R"},
	} {
		pdf.CellFormat(heading.width, 7, text(heading.label), "1", 0, heading.align, true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(23, 38, 60)
	pdf.SetFont("Arial", "", 8)
	for _, line := range document.Lines {
		pdf.CellFormat(92, 7, text(truncate(line.Description, 54)), "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, decimal(line.Quantity, 3), "1", 0, "R", false, 0, "")
		pdf.CellFormat(32, 7, decimal(line.UnitPrice, 2), "1", 0, "R", false, 0, "")
		pdf.CellFormat(38, 7, decimal(line.TotalAmount, 2), "1", 1, "R", false, 0, "")
	}
	pdf.Ln(3)

	pdf.SetFont("Arial", "", 9)
	writeTotal := func(label string, amount fiscal.Decimal, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		pdf.SetFont("Arial", style, 10)
		pdf.CellFormat(
			0, 6,
			text(label+": "+currencySymbol(document.Currency.Code)+" "+decimal(amount, 2)),
			"", 1, "R", false, 0, "",
		)
	}
	if !voucherType.IsTypeC() {
		writeTotal("Neto gravado", document.Totals.NetTaxed, false)
		if !document.Totals.Exempt.IsZero() {
			writeTotal("Exento", document.Totals.Exempt, false)
		}
		if !document.Totals.NetUntaxed.IsZero() {
			writeTotal("No gravado", document.Totals.NetUntaxed, false)
		}
		writeTotal("IVA", document.Totals.VAT, false)
	}
	if !document.Totals.OtherTaxes.IsZero() {
		writeTotal("Otros tributos", document.Totals.OtherTaxes, false)
	}
	writeTotal("TOTAL", document.Totals.Total, true)

	pdf.Ln(7)
	y := pdf.GetY()
	options := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
	pdf.RegisterImageOptionsReader("arca-qr", options, bytes.NewReader(qrPNG))
	pdf.ImageOptions("arca-qr", 14, y, 36, 36, false, options, 0, "")
	pdf.SetXY(56, y+5)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 7, "CAE N°: "+authorization.Code, "", 2, "L", false, 0, "")
	pdf.SetX(56)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(
		0, 6, text("Vencimiento CAE: "+displayDate(authorization.ExpiresOn)),
		"", 2, "L", false, 0, "",
	)
	pdf.SetX(56)
	pdf.SetFont("Arial", "", 8)
	pdf.MultiCell(
		0, 5,
		text("Comprobante autorizado por ARCA. El QR contiene los datos fiscales del snapshot inmutable."),
		"", "L", false,
	)

	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, fmt.Errorf("render fiscal PDF: %w", err)
	}
	return output.Bytes(), nil
}

func voucherTitle(voucherType ar.VoucherType) string {
	switch voucherType {
	case ar.CreditNoteA, ar.CreditNoteB, ar.CreditNoteC:
		return "NOTA DE CRÉDITO"
	case ar.DebitNoteA, ar.DebitNoteB, ar.DebitNoteC:
		return "NOTA DE DÉBITO"
	default:
		return "FACTURA"
	}
}

func taxConditionLabel(value string) string {
	condition, err := ar.ParseVATCondition(value)
	if err != nil {
		return value
	}
	switch condition {
	case ar.VATRegistered:
		return "Responsable inscripto"
	case ar.VATMonotax:
		return "Monotributo"
	case ar.VATExempt:
		return "Exento"
	case ar.VATConsumerFinal:
		return "Consumidor final"
	case ar.VATNotResponsible:
		return "No responsable"
	default:
		return value
	}
}

func currencySymbol(code string) string {
	switch code {
	case ar.CurrencyPES, "ARS":
		return "$"
	case ar.CurrencyDOL, "USD":
		return "US$"
	case ar.CurrencyEUR, "EUR":
		return "€"
	default:
		return code
	}
}

func decimal(value fiscal.Decimal, scale int32) string {
	formatted, err := value.FormatFixed(scale, fiscal.RoundHalfAwayFromZero)
	if err != nil {
		return value.String()
	}
	return formatted
}

func displayDate(value string) string {
	for _, layout := range []string{"2006-01-02", "20060102"} {
		if date, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return date.Format("02/01/2006")
		}
	}
	return value
}

func truncate(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit-1]) + "…"
}
