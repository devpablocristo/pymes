package ledger

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	ledgerdomain "github.com/devpablocristo/pymes/core/backend/internal/ledger/usecases/domain"
)

// buildSaleEntry traduce un SaleEvent en un asiento balanceado, resolviendo las
// cuentas por rol (account_links). Reglas:
//   - Contado (payment_status != "pending"): DR caja/banco (según método) por el
//     Total; CR ventas por el neto; CR IVA débito por alícuota.
//   - Crédito (payment_status == "pending"): DR deudores por ventas (con party_id).
//
// El IVA se ANCLA al TaxTotal ya persistido (la BD ya redondeó), repartido por
// alícuota con el residual de centavo a la de mayor base, para que el asiento
// reconcilie exacto con el documento. Si falta un account_link requerido,
// retorna ErrAccountLinkMissing (el worker marca el outbox 'failed').
func buildSaleEntry(evt SaleEvent, links map[string]uuid.UUID) (ledgerdomain.JournalEntry, error) {
	total := round2(evt.Total)
	subtotal := round2(evt.Subtotal)
	taxTotal := round2(evt.TaxTotal)

	lines := make([]ledgerdomain.JournalLine, 0, 4)

	// Débito por el total: caja/banco (contado) o deudores (crédito).
	var debitRole string
	var partyID *uuid.UUID
	if strings.EqualFold(strings.TrimSpace(evt.PaymentStatus), "pending") {
		debitRole = "receivable"
		partyID = evt.PartyID
	} else {
		debitRole = paymentMethodRole(evt.PaymentMethod)
	}
	debitAcc, err := requireLink(links, debitRole)
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	lines = append(lines, ledgerdomain.JournalLine{OrgID: evt.OrgID, AccountID: debitAcc, Debit: total, PartyID: partyID, Memo: "Venta " + evt.Number})

	// Haber: ingreso por ventas (neto).
	revAcc, err := requireLink(links, "revenue")
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	lines = append(lines, ledgerdomain.JournalLine{OrgID: evt.OrgID, AccountID: revAcc, Credit: subtotal, Memo: "Ventas " + evt.Number})

	// Haber: IVA débito fiscal por alícuota.
	for _, ir := range distributeTax(evt.Lines, taxTotal) {
		if ir.amount == 0 {
			continue
		}
		role := "vat_payable_" + rateKey(ir.rate)
		acc, err := requireLink(links, role)
		if err != nil {
			return ledgerdomain.JournalEntry{}, err
		}
		lines = append(lines, ledgerdomain.JournalLine{OrgID: evt.OrgID, AccountID: acc, Credit: ir.amount, Memo: fmt.Sprintf("IVA %s%% venta %s", trimRate(ir.rate), evt.Number)})
	}

	if err := assertBalanced(lines); err != nil {
		return ledgerdomain.JournalEntry{}, err
	}

	saleID := evt.SaleID
	return ledgerdomain.JournalEntry{
		OrgID:       evt.OrgID,
		EntryDate:   evt.OccurredAt,
		Currency:    evt.Currency,
		SourceType:  "sale",
		SourceID:    &saleID,
		SourceEvent: "sale.completed",
		Description: "Venta " + evt.Number,
		CreatedBy:   evt.Actor,
		Lines:       lines,
	}, nil
}

// buildPaymentEntry arma el asiento de un cobro de venta a CRÉDITO:
// DR caja/banco (según método) / CR deudores por ventas. El llamador ya
// verificó que la venta fue a crédito; el party de la venta va en la línea de
// deudores. Para ventas de contado no se llega acá (se marca skipped).
func buildPaymentEntry(evt PaymentEvent, links map[string]uuid.UUID, partyID *uuid.UUID) (ledgerdomain.JournalEntry, error) {
	amount := round2(evt.Amount)
	debitAcc, err := requireLink(links, paymentMethodRole(evt.Method))
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	recvAcc, err := requireLink(links, "receivable")
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	lines := []ledgerdomain.JournalLine{
		{OrgID: evt.OrgID, AccountID: debitAcc, Debit: amount, Memo: "Cobro venta"},
		{OrgID: evt.OrgID, AccountID: recvAcc, Credit: amount, PartyID: partyID, Memo: "Cobro venta"},
	}
	if err := assertBalanced(lines); err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	paymentID := evt.PaymentID
	return ledgerdomain.JournalEntry{
		OrgID:       evt.OrgID,
		EntryDate:   evt.OccurredAt,
		Currency:    evt.Currency,
		SourceType:  "payment",
		SourceID:    &paymentID,
		SourceEvent: "payment.created",
		Description: "Cobro de venta",
		CreatedBy:   evt.Actor,
		Lines:       lines,
	}, nil
}

// buildReturnEntry arma el asiento de una devolución (storno parcial de la
// venta): DR Ventas (neto devuelto) + DR IVA débito por alícuota / CR caja
// (refund cash/original) o CR pasivo por nota de crédito (refund credit_note,
// con el party del cliente).
func buildReturnEntry(evt ReturnEvent, links map[string]uuid.UUID) (ledgerdomain.JournalEntry, error) {
	total := round2(evt.Total)
	subtotal := round2(evt.Subtotal)

	lines := make([]ledgerdomain.JournalLine, 0, 4)
	revAcc, err := requireLink(links, "revenue")
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	lines = append(lines, ledgerdomain.JournalLine{OrgID: evt.OrgID, AccountID: revAcc, Debit: subtotal, Memo: "Devolución " + evt.Number})
	for _, ir := range distributeTax(evt.Lines, round2(evt.TaxTotal)) {
		if ir.amount == 0 {
			continue
		}
		acc, err := requireLink(links, "vat_payable_"+rateKey(ir.rate))
		if err != nil {
			return ledgerdomain.JournalEntry{}, err
		}
		lines = append(lines, ledgerdomain.JournalLine{OrgID: evt.OrgID, AccountID: acc, Debit: ir.amount, Memo: fmt.Sprintf("IVA %s%% devolución %s", trimRate(ir.rate), evt.Number)})
	}

	var refundRole string
	var refundParty *uuid.UUID
	if strings.EqualFold(strings.TrimSpace(evt.RefundMethod), "credit_note") {
		refundRole = "credit_note_payable"
		refundParty = evt.PartyID
	} else { // cash | original_method
		refundRole = "cash"
	}
	refundAcc, err := requireLink(links, refundRole)
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	lines = append(lines, ledgerdomain.JournalLine{OrgID: evt.OrgID, AccountID: refundAcc, Credit: total, PartyID: refundParty, Memo: "Devolución " + evt.Number})

	if err := assertBalanced(lines); err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	returnID := evt.ReturnID
	return ledgerdomain.JournalEntry{
		OrgID:       evt.OrgID,
		EntryDate:   evt.OccurredAt,
		Currency:    evt.Currency,
		SourceType:  "return",
		SourceID:    &returnID,
		SourceEvent: "return.created",
		Description: "Devolución " + evt.Number,
		CreatedBy:   evt.Actor,
		Lines:       lines,
	}, nil
}

// buildPurchasePaymentEntry arma el asiento de un pago a proveedor:
// DR Proveedores (baja el pasivo, con el party del proveedor) / CR caja-banco.
func buildPurchasePaymentEntry(evt PurchasePaymentEvent, links map[string]uuid.UUID, partyID *uuid.UUID) (ledgerdomain.JournalEntry, error) {
	amount := round2(evt.Amount)
	payAcc, err := requireLink(links, "payable")
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	cashAcc, err := requireLink(links, paymentMethodRole(evt.Method))
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	lines := []ledgerdomain.JournalLine{
		{OrgID: evt.OrgID, AccountID: payAcc, Debit: amount, PartyID: partyID, Memo: "Pago a proveedor"},
		{OrgID: evt.OrgID, AccountID: cashAcc, Credit: amount, Memo: "Pago a proveedor"},
	}
	if err := assertBalanced(lines); err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	paymentID := evt.PaymentID
	return ledgerdomain.JournalEntry{
		OrgID:       evt.OrgID,
		EntryDate:   evt.OccurredAt,
		Currency:    evt.Currency,
		SourceType:  "payment",
		SourceID:    &paymentID,
		SourceEvent: "supplier_payment.created",
		Description: "Pago a proveedor",
		CreatedBy:   evt.Actor,
		Lines:       lines,
	}, nil
}

// purchaseData es el snapshot que el worker arma leyendo la compra para postear
// su alta. IsProduct discrimina inventario (mercadería) de gasto (servicio).
type purchaseData struct {
	OrgID      uuid.UUID
	PurchaseID uuid.UUID
	Number     string
	OccurredAt time.Time
	Currency   string
	PartyID    *uuid.UUID
	Subtotal   float64
	TaxTotal   float64
	Total      float64
	Items      []purchaseLine
}

type purchaseLine struct {
	IsProduct bool
	TaxRate   float64
	Subtotal  float64
}

// buildPurchaseEntry arma el asiento de alta de una compra recibida:
// DR Mercadería (productos) + DR Gasto (servicios) + DR IVA crédito por alícuota
// / CR Proveedores (por el total, con el party del proveedor).
func buildPurchaseEntry(d purchaseData, links map[string]uuid.UUID) (ledgerdomain.JournalEntry, error) {
	total := round2(d.Total)

	var invNeto, expNeto float64
	taxLines := make([]SaleEventLine, 0, len(d.Items))
	for _, it := range d.Items {
		if it.IsProduct {
			invNeto += it.Subtotal
		} else {
			expNeto += it.Subtotal
		}
		taxLines = append(taxLines, SaleEventLine{TaxRate: it.TaxRate, Subtotal: it.Subtotal})
	}
	invNeto = round2(invNeto)
	expNeto = round2(expNeto)

	lines := make([]ledgerdomain.JournalLine, 0, 5)
	if invNeto > 0 {
		acc, err := requireLink(links, "inventory")
		if err != nil {
			return ledgerdomain.JournalEntry{}, err
		}
		lines = append(lines, ledgerdomain.JournalLine{OrgID: d.OrgID, AccountID: acc, Debit: invNeto, Memo: "Compra " + d.Number})
	}
	if expNeto > 0 {
		acc, err := requireLink(links, "purchase_expense")
		if err != nil {
			return ledgerdomain.JournalEntry{}, err
		}
		lines = append(lines, ledgerdomain.JournalLine{OrgID: d.OrgID, AccountID: acc, Debit: expNeto, Memo: "Compra " + d.Number})
	}
	for _, ir := range distributeTax(taxLines, round2(d.TaxTotal)) {
		if ir.amount == 0 {
			continue
		}
		acc, err := requireLink(links, "vat_credit_"+rateKey(ir.rate))
		if err != nil {
			return ledgerdomain.JournalEntry{}, err
		}
		lines = append(lines, ledgerdomain.JournalLine{OrgID: d.OrgID, AccountID: acc, Debit: ir.amount, Memo: fmt.Sprintf("IVA crédito %s%% compra %s", trimRate(ir.rate), d.Number)})
	}
	payAcc, err := requireLink(links, "payable")
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	lines = append(lines, ledgerdomain.JournalLine{OrgID: d.OrgID, AccountID: payAcc, Credit: total, PartyID: d.PartyID, Memo: "Proveedor " + d.Number})

	if err := assertBalanced(lines); err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	purchaseID := d.PurchaseID
	return ledgerdomain.JournalEntry{
		OrgID:       d.OrgID,
		EntryDate:   d.OccurredAt,
		Currency:    d.Currency,
		SourceType:  "purchase",
		SourceID:    &purchaseID,
		SourceEvent: "purchase.received",
		Description: "Compra " + d.Number,
		Lines:       lines,
	}, nil
}

type ivaShare struct {
	rate   float64
	base   float64
	amount float64
}

// distributeTax agrupa el neto por alícuota y reparte el TaxTotal persistido
// entre las alícuotas, empujando el residual de centavo a la de mayor base, de
// modo que la suma de IVA == TaxTotal exacto.
func distributeTax(lines []SaleEventLine, taxTotal float64) []ivaShare {
	order := make([]float64, 0, 4)
	baseByRate := make(map[float64]float64, 4)
	for _, l := range lines {
		if l.TaxRate <= 0 {
			continue
		}
		if _, ok := baseByRate[l.TaxRate]; !ok {
			order = append(order, l.TaxRate)
		}
		baseByRate[l.TaxRate] += l.Subtotal
	}
	if len(order) == 0 || taxTotal == 0 {
		return nil
	}
	shares := make([]ivaShare, 0, len(order))
	sum := 0.0
	maxIdx := 0
	maxBase := -1.0
	for i, rate := range order {
		base := baseByRate[rate]
		amt := round2(base * rate / 100.0)
		shares = append(shares, ivaShare{rate: rate, base: base, amount: amt})
		sum = round2(sum + amt)
		if base > maxBase {
			maxBase = base
			maxIdx = i
		}
	}
	if residual := round2(taxTotal - sum); residual != 0 {
		shares[maxIdx].amount = round2(shares[maxIdx].amount + residual)
	}
	return shares
}

func paymentMethodRole(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "transfer", "check", "card", "mercadopago":
		return "bank"
	default: // cash, other, ""
		return "cash"
	}
}

func requireLink(links map[string]uuid.UUID, role string) (uuid.UUID, error) {
	id, ok := links[role]
	if !ok || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: %s", ErrAccountLinkMissing, role)
	}
	return id, nil
}

// assertBalanced verifica partida doble en centavos enteros.
func assertBalanced(lines []ledgerdomain.JournalLine) error {
	cents := int64(0)
	for _, l := range lines {
		cents += int64(round2(l.Debit)*100) - int64(round2(l.Credit)*100)
	}
	if cents != 0 {
		return fmt.Errorf("%w: debit-credit imbalance of %d cents", ErrUnbalanced, cents)
	}
	return nil
}

// rateKey convierte una alícuota en el sufijo del rol: 21.0 -> "21", 10.5 -> "105".
func rateKey(rate float64) string {
	return strings.ReplaceAll(trimRate(rate), ".", "")
}

func trimRate(rate float64) string {
	return strconv.FormatFloat(rate, 'f', -1, 64)
}
