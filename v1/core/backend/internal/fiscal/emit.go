package fiscal

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/core/backend/internal/fiscal/arca"
	fiscaldomain "github.com/devpablocristo/pymes/core/backend/internal/fiscal/usecases/domain"
)

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// currencyToArca mapea el código de moneda de pymes al MonId de ARCA.
func currencyToArca(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "", "ARS", "PES":
		return arca.MonedaPesos
	case "USD", "DOL":
		return arca.MonedaDolar
	case "EUR", "060":
		return "060"
	default:
		return arca.MonedaPesos
	}
}

// toArcaDate convierte "YYYY-MM-DD" a "YYYYMMDD"; si viene vacío usa el default.
func toArcaDate(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("20060102")
	}
	// Si ya viene como YYYYMMDD (8 dígitos) se acepta tal cual.
	if len(s) == 8 && isNumeric(s) {
		return s
	}
	return def
}

// mapCondIVA normaliza la condición IVA (texto) al código ARCA.
func mapCondIVA(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "responsable_inscripto", "ri", "inscripto":
		return arca.CondIVAResponsableInscripto
	case "monotributo", "monotributista":
		return arca.CondIVAMonotributo
	case "exento":
		return arca.CondIVAExento
	case "consumidor_final", "cf", "":
		return arca.CondIVAConsumidorFinal
	default:
		return arca.CondIVAConsumidorFinal
	}
}

// resolveReceptor deriva el documento del receptor: CUIT (11 dígitos) -> DocTipo
// 80; si no, Consumidor Final (99, "0"). Devuelve también la condición IVA.
func resolveReceptor(taxID, condIVA string) (docTipo int, docNro string, condCode int) {
	id := strings.TrimSpace(taxID)
	if len(id) == 11 && isNumeric(id) {
		cond := mapCondIVA(condIVA)
		if cond == arca.CondIVAConsumidorFinal {
			cond = arca.CondIVAResponsableInscripto // con CUIT, por defecto RI
		}
		return arca.DocCUIT, id, cond
	}
	return arca.DocConsumidorFinal, "0", arca.CondIVAConsumidorFinal
}

// computeVoucherType determina el tipo de comprobante según la condición del
// emisor y del receptor: emisor monotributo -> C; emisor RI -> A si el receptor
// es RI, si no B.
func computeVoucherType(emisorCond string, receptorCond int) int {
	switch mapCondIVA(emisorCond) {
	case arca.CondIVAMonotributo:
		return arca.CbteFacturaC
	case arca.CondIVAResponsableInscripto:
		if receptorCond == arca.CondIVAResponsableInscripto {
			return arca.CbteFacturaA
		}
		return arca.CbteFacturaB
	default:
		return arca.CbteFacturaB
	}
}

// isInvoiceType indica si el tipo es una FACTURA (no NC/ND). Se usa para ubicar
// el comprobante original de una nota de crédito.
func isInvoiceType(t int) bool {
	switch t {
	case arca.CbteFacturaA, arca.CbteFacturaB, arca.CbteFacturaC, arca.CbteFacturaE:
		return true
	default:
		return false
	}
}

// ncTypeFor devuelve el tipo de nota de crédito para una factura dada.
func ncTypeFor(invoiceType int) (int, bool) {
	switch invoiceType {
	case arca.CbteFacturaA:
		return arca.CbteNotaCreditoA, true
	case arca.CbteFacturaB:
		return arca.CbteNotaCreditoB, true
	case arca.CbteFacturaC:
		return arca.CbteNotaCreditoC, true
	default:
		return 0, false
	}
}

// isTypeC indica si el comprobante NO discrimina IVA (monotributo / tipo C).
func isTypeC(voucherType int) bool {
	switch voucherType {
	case arca.CbteFacturaC, arca.CbteNotaDebitoC, arca.CbteNotaCreditoC:
		return true
	default:
		return false
	}
}

// buildImports arma los importes del comprobante y el desglose de IVA a partir de
// los datos de la venta. Para tipo C no discrimina IVA (neto = total, IVA = 0).
// Consistencia ARCA: ImpTotal = ImpNeto + ImpIVA; ImpIVA = Σ Iva.Importe.
func buildImports(voucherType int, sale SaleFiscalData) (neto, iva, total float64, lines []fiscaldomain.IvaLine, err error) {
	if isTypeC(voucherType) {
		total = round2(sale.Total)
		return total, 0, total, nil, nil
	}
	baseByRate := map[float64]float64{}
	order := []float64{}
	for _, it := range sale.Items {
		if _, ok := baseByRate[it.TaxRate]; !ok {
			order = append(order, it.TaxRate)
		}
		baseByRate[it.TaxRate] += it.Subtotal
	}
	for _, rate := range order {
		base := round2(baseByRate[rate])
		id, ok := arca.IvaIDForRate(rate)
		if !ok {
			return 0, 0, 0, nil, fmt.Errorf("alícuota IVA no soportada por ARCA: %.2f%%", rate)
		}
		importe := round2(base * rate / 100.0)
		neto = round2(neto + base)
		iva = round2(iva + importe)
		lines = append(lines, fiscaldomain.IvaLine{ID: id, BaseImp: base, Importe: importe})
	}
	total = round2(neto + iva)
	return neto, iva, total, lines, nil
}

func ivaLinesToArca(lines []fiscaldomain.IvaLine) []arca.AliqIva {
	out := make([]arca.AliqIva, 0, len(lines))
	for _, l := range lines {
		out = append(out, arca.AliqIva{ID: l.ID, BaseImp: l.BaseImp, Importe: l.Importe})
	}
	return out
}
