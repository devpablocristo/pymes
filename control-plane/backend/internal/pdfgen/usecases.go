package pdfgen

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"

	admindomain "github.com/devpablocristo/pymes/control-plane/backend/internal/admin/usecases/domain"
	quotedomain "github.com/devpablocristo/pymes/control-plane/backend/internal/quotes/usecases/domain"
	saledomain "github.com/devpablocristo/pymes/control-plane/backend/internal/sales/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

type QuotePort interface {
	GetByID(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error)
}

type SalePort interface {
	GetByID(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error)
}

type AdminPort interface {
	GetTenantSettings(ctx context.Context, orgID string) (admindomain.TenantSettings, error)
}

type Usecases struct {
	quotes QuotePort
	sales  SalePort
	admin  AdminPort
}

func NewUsecases(quotes QuotePort, sales SalePort, admin AdminPort) *Usecases {
	return &Usecases{quotes: quotes, sales: sales, admin: admin}
}

func (u *Usecases) RenderQuotePDF(ctx context.Context, orgID, quoteID uuid.UUID) ([]byte, string, error) {
	if u.quotes == nil || u.admin == nil {
		return nil, "", apperror.NewBadInput("pdf service unavailable")
	}
	quote, err := u.quotes.GetByID(ctx, orgID, quoteID)
	if err != nil {
		return nil, "", err
	}
	settings, err := u.admin.GetTenantSettings(ctx, orgID.String())
	if err != nil {
		return nil, "", err
	}
	pdf := newPDF()
	renderHeader(pdf, settings, "PRESUPUESTO", quote.Number)
	renderPartyBlock(pdf, quote.CustomerName, quote.CreatedAt, quote.ValidUntil)
	renderQuoteItems(pdf, quote.Items, currencySymbol(settings.Currency))
	renderTotals(pdf, quote.Subtotal, quote.TaxTotal, quote.Total, currencySymbol(settings.Currency))
	renderNotes(pdf, quote.Notes)
	return output(pdf), quote.Number + ".pdf", nil
}

func (u *Usecases) RenderSaleReceipt(ctx context.Context, orgID, saleID uuid.UUID) ([]byte, string, error) {
	if u.sales == nil || u.admin == nil {
		return nil, "", apperror.NewBadInput("pdf service unavailable")
	}
	sale, err := u.sales.GetByID(ctx, orgID, saleID)
	if err != nil {
		return nil, "", err
	}
	settings, err := u.admin.GetTenantSettings(ctx, orgID.String())
	if err != nil {
		return nil, "", err
	}
	pdf := newPDF()
	renderHeader(pdf, settings, "COMPROBANTE DE VENTA", sale.Number)
	renderPartyBlock(pdf, sale.CustomerName, sale.CreatedAt, nil)
	renderSaleMeta(pdf, sale.PaymentMethod)
	renderSaleItems(pdf, sale.Items, currencySymbol(settings.Currency))
	renderTotals(pdf, sale.Subtotal, sale.TaxTotal, sale.Total, currencySymbol(settings.Currency))
	renderNotes(pdf, sale.Notes)
	return output(pdf), sale.Number + ".pdf", nil
}

func newPDF() *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(14, 14, 14)
	pdf.SetAutoPageBreak(true, 14)
	pdf.AddPage()
	pdf.SetFont("Arial", "", 11)
	return pdf
}

func renderHeader(pdf *fpdf.Fpdf, settings admindomain.TenantSettings, title, number string) {
	name := strings.TrimSpace(settings.BusinessName)
	if name == "" {
		name = "Mi negocio"
	}
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 8, name, "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	for _, line := range []string{settings.BusinessAddress, joinCompact("Tel", settings.BusinessPhone), joinCompact("Email", settings.BusinessEmail), joinCompact("CUIT", settings.BusinessTaxID)} {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)
	pdf.SetFont("Arial", "B", 15)
	pdf.CellFormat(0, 8, fmt.Sprintf("%s N° %s", title, number), "", 1, "L", false, 0, "")
	pdf.Ln(2)
}

func renderPartyBlock(pdf *fpdf.Fpdf, partyName string, createdAt time.Time, validUntil *time.Time) {
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, "Fecha: "+formatDate(createdAt), "", 1, "L", false, 0, "")
	if validUntil != nil {
		pdf.CellFormat(0, 6, "Valido hasta: "+formatDate(*validUntil), "", 1, "L", false, 0, "")
	}
	if strings.TrimSpace(partyName) != "" {
		pdf.CellFormat(0, 6, "Cliente: "+strings.TrimSpace(partyName), "", 1, "L", false, 0, "")
	}
	pdf.Ln(2)
}

func renderSaleMeta(pdf *fpdf.Fpdf, paymentMethod string) {
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, "Metodo de pago: "+strings.TrimSpace(paymentMethod), "", 1, "L", false, 0, "")
	pdf.Ln(2)
}

func renderQuoteItems(pdf *fpdf.Fpdf, items []quotedomain.QuoteItem, symbol string) {
	renderTableHeader(pdf)
	pdf.SetFont("Arial", "", 10)
	for idx, item := range items {
		pdf.CellFormat(12, 7, fmt.Sprintf("%d", idx+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(92, 7, item.Description, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, formatQty(item.Quantity), "1", 0, "R", false, 0, "")
		pdf.CellFormat(28, 7, formatMoney(symbol, item.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(24, 7, formatMoney(symbol, item.Subtotal), "1", 1, "R", false, 0, "")
	}
	pdf.Ln(2)
}

func renderSaleItems(pdf *fpdf.Fpdf, items []saledomain.SaleItem, symbol string) {
	renderTableHeader(pdf)
	pdf.SetFont("Arial", "", 10)
	for idx, item := range items {
		pdf.CellFormat(12, 7, fmt.Sprintf("%d", idx+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(92, 7, item.Description, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, formatQty(item.Quantity), "1", 0, "R", false, 0, "")
		pdf.CellFormat(28, 7, formatMoney(symbol, item.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(24, 7, formatMoney(symbol, item.Subtotal), "1", 1, "R", false, 0, "")
	}
	pdf.Ln(2)
}

func renderTableHeader(pdf *fpdf.Fpdf) {
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(12, 7, "#", "1", 0, "C", false, 0, "")
	pdf.CellFormat(92, 7, "Descripcion", "1", 0, "L", false, 0, "")
	pdf.CellFormat(20, 7, "Cant", "1", 0, "R", false, 0, "")
	pdf.CellFormat(28, 7, "P.Unit", "1", 0, "R", false, 0, "")
	pdf.CellFormat(24, 7, "Subtotal", "1", 1, "R", false, 0, "")
}

func renderTotals(pdf *fpdf.Fpdf, subtotal, taxTotal, total float64, symbol string) {
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Subtotal: "+formatMoney(symbol, subtotal), "", 1, "R", false, 0, "")
	pdf.CellFormat(0, 6, "Impuestos: "+formatMoney(symbol, taxTotal), "", 1, "R", false, 0, "")
	pdf.CellFormat(0, 8, "TOTAL: "+formatMoney(symbol, total), "", 1, "R", false, 0, "")
}

func renderNotes(pdf *fpdf.Fpdf, notes string) {
	notes = strings.TrimSpace(notes)
	if notes == "" {
		return
	}
	pdf.Ln(3)
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 6, "Notas: "+notes, "", "L", false)
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("02/01/2006")
}

func formatQty(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

func formatMoney(symbol string, amount float64) string {
	return fmt.Sprintf("%s %.2f", symbol, amount)
}

func currencySymbol(currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "BRL":
		return "R$"
	case "PEN":
		return "S/"
	default:
		return "$"
	}
}

func joinCompact(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return label + ": " + value
}

func output(pdf *fpdf.Fpdf) []byte {
	var b bytes.Buffer
	_ = pdf.Output(&b)
	return b.Bytes()
}
