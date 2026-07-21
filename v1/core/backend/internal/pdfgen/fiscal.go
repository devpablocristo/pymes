package pdfgen

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	admindomain "github.com/devpablocristo/pymes/core/backend/internal/admin/usecases/domain"
	fiscaldomain "github.com/devpablocristo/pymes/core/backend/internal/fiscal/usecases/domain"
	saledomain "github.com/devpablocristo/pymes/core/backend/internal/sales/usecases/domain"
)

// FiscalPort lee el comprobante fiscal a renderizar (adapter sobre fiscal.Usecases).
type FiscalPort interface {
	GetVoucher(ctx context.Context, orgID, voucherID uuid.UUID) (fiscaldomain.FiscalVoucher, error)
}

// RenderFiscalVoucher genera el PDF del comprobante fiscal (factura/NC) con la
// letra, numeración, importes, CAE y el QR (RG 4892) como imagen. El detalle de
// ítems se toma de la venta asociada si está disponible.
func (u *Usecases) RenderFiscalVoucher(ctx context.Context, tenantID, voucherID uuid.UUID) ([]byte, string, error) {
	if u.fiscal == nil || u.admin == nil {
		return nil, "", domainerr.Validation("fiscal pdf service unavailable")
	}
	v, err := u.fiscal.GetVoucher(ctx, tenantID, voucherID)
	if err != nil {
		return nil, "", err
	}
	settings, err := u.admin.GetTenantSettings(ctx, tenantID.String())
	if err != nil {
		return nil, "", err
	}
	var items []saledomain.SaleItem
	if v.SaleID != nil && u.sales != nil {
		if sale, serr := u.sales.GetByID(ctx, tenantID, *v.SaleID); serr == nil {
			items = sale.Items
		}
	}

	symbol := currencySymbol(v.Currency)
	pdf := newPDF()
	renderFiscalHeader(pdf, settings, v)
	renderFiscalReceptor(pdf, v)
	if len(items) > 0 {
		renderSaleItems(pdf, items, symbol)
	}
	renderFiscalTotals(pdf, v, symbol)
	renderFiscalCAE(pdf, v)
	return output(pdf), fiscalFilename(v), nil
}

// renderFiscalHeader dibuja el emisor y el recuadro de la letra + tipo/número/fecha.
func renderFiscalHeader(pdf *fpdf.Fpdf, settings admindomain.TenantSettings, v fiscaldomain.FiscalVoucher) {
	letra := voucherLetter(v.VoucherType)
	// Recuadro central con la letra (identificación del comprobante).
	pdf.SetFont("Arial", "B", 26)
	pdf.SetXY(96, 14)
	pdf.CellFormat(18, 14, letra, "1", 0, "C", false, 0, "")
	pdf.SetFont("Arial", "", 7)
	pdf.SetXY(96, 28)
	pdf.CellFormat(18, 4, "COD. "+strconv.Itoa(v.VoucherType), "", 0, "C", false, 0, "")

	// Emisor (izquierda).
	name := strings.TrimSpace(settings.BusinessName)
	if name == "" {
		name = "Mi negocio"
	}
	pdf.SetXY(14, 14)
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(80, 8, name, "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	for _, line := range []string{
		settings.BusinessAddress,
		joinCompact("Tel", settings.BusinessPhone),
		joinCompact("Email", settings.BusinessEmail),
		joinCompact("CUIT", settings.BusinessTaxID),
		"Condicion IVA: " + emisorConditionLabel(letra),
	} {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pdf.CellFormat(80, 5, line, "", 2, "L", false, 0, "")
	}

	// Tipo, número y fecha (derecha).
	pdf.SetXY(120, 14)
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(76, 8, voucherTitle(v.VoucherType), "", 2, "R", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(76, 6, "N° "+fiscalNumber(v), "", 2, "R", false, 0, "")
	pdf.CellFormat(76, 6, "Fecha: "+fiscalIssueDate(v), "", 2, "R", false, 0, "")

	pdf.SetY(46)
	pdf.SetFont("Arial", "", 11)
}

func renderFiscalReceptor(pdf *fpdf.Fpdf, v fiscaldomain.FiscalVoucher) {
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, "Receptor", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, docLabel(v.DocTipo)+": "+receptorDoc(v.DocNro), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Condicion IVA: "+condIVALabel(v.CondicionIVAReceptor), "", 1, "L", false, 0, "")
	pdf.Ln(2)
}

func renderFiscalTotals(pdf *fpdf.Fpdf, v fiscaldomain.FiscalVoucher, symbol string) {
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "", "", 1, "L", false, 0, "")
	if v.ImpIVA > 0 {
		pdf.CellFormat(0, 6, "Neto gravado: "+formatMoney(symbol, v.ImpNeto), "", 1, "R", false, 0, "")
		pdf.CellFormat(0, 6, "IVA: "+formatMoney(symbol, v.ImpIVA), "", 1, "R", false, 0, "")
	}
	pdf.CellFormat(0, 8, "TOTAL: "+formatMoney(symbol, v.ImpTotal), "", 1, "R", false, 0, "")
}

// renderFiscalCAE dibuja el bloque CAE + vencimiento y el QR (izquierda) exigido
// por RG 4892. Si el comprobante no está autorizado, informa el estado.
func renderFiscalCAE(pdf *fpdf.Fpdf, v fiscaldomain.FiscalVoucher) {
	pdf.Ln(6)
	if v.Status != "authorized" || v.CAE == "" {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(0, 6, "Comprobante no autorizado (estado: "+v.Status+")", "", 1, "L", false, 0, "")
		return
	}
	y := pdf.GetY()
	if v.QRURL != "" {
		if png, err := qrcode.Encode(v.QRURL, qrcode.Medium, 256); err == nil {
			opt := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
			name := "qr-" + v.ID.String()
			pdf.RegisterImageOptionsReader(name, opt, bytes.NewReader(png))
			pdf.ImageOptions(name, 14, y, 32, 32, false, opt, 0, "")
		}
	}
	pdf.SetXY(50, y+6)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "CAE N°: "+v.CAE, "", 2, "L", false, 0, "")
	pdf.SetX(50)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, "Vto. CAE: "+fiscalCAEVto(v), "", 2, "L", false, 0, "")
}

// --- helpers de presentación (mapeos de códigos ARCA a etiquetas legibles) ---

func voucherLetter(t int) string {
	switch t {
	case 1, 2, 3:
		return "A"
	case 6, 7, 8:
		return "B"
	case 11, 12, 13:
		return "C"
	case 19:
		return "E"
	default:
		return "X"
	}
}

func voucherTitle(t int) string {
	switch t {
	case 3, 8, 13:
		return "NOTA DE CREDITO"
	case 2, 7, 12:
		return "NOTA DE DEBITO"
	default:
		return "FACTURA"
	}
}

func emisorConditionLabel(letra string) string {
	switch letra {
	case "A":
		return "Responsable Inscripto"
	case "C":
		return "Monotributo / Exento"
	default:
		return "Responsable Inscripto"
	}
}

func docLabel(docTipo int) string {
	switch docTipo {
	case 80:
		return "CUIT"
	case 86:
		return "CUIL"
	case 96:
		return "DNI"
	default:
		return "Consumidor Final"
	}
}

func receptorDoc(nro string) string {
	nro = strings.TrimSpace(nro)
	if nro == "" || nro == "0" {
		return "-"
	}
	return nro
}

func condIVALabel(cond int) string {
	switch cond {
	case 1:
		return "Responsable Inscripto"
	case 4:
		return "Exento"
	case 6:
		return "Monotributo"
	default:
		return "Consumidor Final"
	}
}

func fiscalNumber(v fiscaldomain.FiscalVoucher) string {
	return fmt.Sprintf("%04d-%08d", v.PointOfSale, v.CbteNro)
}

func fiscalFilename(v fiscaldomain.FiscalVoucher) string {
	return fmt.Sprintf("comprobante-%s-%04d-%08d.pdf", voucherLetter(v.VoucherType), v.PointOfSale, v.CbteNro)
}

func fiscalIssueDate(v fiscaldomain.FiscalVoucher) string {
	if v.EmittedAt != nil && !v.EmittedAt.IsZero() {
		return formatDate(*v.EmittedAt)
	}
	return formatDate(v.CreatedAt)
}

func fiscalCAEVto(v fiscaldomain.FiscalVoucher) string {
	if t, err := time.Parse("20060102", strings.TrimSpace(v.CAEVto)); err == nil {
		return t.Format("02/01/2006")
	}
	return v.CAEVto
}
