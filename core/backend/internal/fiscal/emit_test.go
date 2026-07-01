package fiscal

import (
	"math"
	"testing"

	"github.com/devpablocristo/pymes/core/backend/internal/fiscal/arca"
)

func TestComputeVoucherType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		emisor   string
		receptor int
		want     int
	}{
		{"responsable_inscripto", arca.CondIVAResponsableInscripto, arca.CbteFacturaA},
		{"responsable_inscripto", arca.CondIVAConsumidorFinal, arca.CbteFacturaB},
		{"responsable_inscripto", arca.CondIVAMonotributo, arca.CbteFacturaB},
		{"monotributo", arca.CondIVAConsumidorFinal, arca.CbteFacturaC},
	}
	for _, c := range cases {
		if got := computeVoucherType(c.emisor, c.receptor); got != c.want {
			t.Fatalf("computeVoucherType(%s,%d)=%d want %d", c.emisor, c.receptor, got, c.want)
		}
	}
}

func TestNcTypeFor(t *testing.T) {
	t.Parallel()
	cases := map[int]int{arca.CbteFacturaA: arca.CbteNotaCreditoA, arca.CbteFacturaB: arca.CbteNotaCreditoB, arca.CbteFacturaC: arca.CbteNotaCreditoC}
	for inv, want := range cases {
		if got, ok := ncTypeFor(inv); !ok || got != want {
			t.Fatalf("ncTypeFor(%d)=%d,%v want %d", inv, got, ok, want)
		}
	}
	if !isInvoiceType(arca.CbteFacturaB) || isInvoiceType(arca.CbteNotaCreditoB) {
		t.Fatalf("isInvoiceType wrong")
	}
}

func TestCurrencyToArca(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"": arca.MonedaPesos, "ARS": arca.MonedaPesos, "PES": arca.MonedaPesos,
		"usd": arca.MonedaDolar, "DOL": arca.MonedaDolar, "EUR": "060", "xyz": arca.MonedaPesos,
	}
	for in, want := range cases {
		if got := currencyToArca(in); got != want {
			t.Fatalf("currencyToArca(%q)=%q want %q", in, got, want)
		}
	}
}

func TestToArcaDate(t *testing.T) {
	t.Parallel()
	if got := toArcaDate("2026-06-01", "X"); got != "20260601" {
		t.Fatalf("iso date: got %q", got)
	}
	if got := toArcaDate("20260601", "X"); got != "20260601" {
		t.Fatalf("compact date: got %q", got)
	}
	if got := toArcaDate("", "20260701"); got != "20260701" {
		t.Fatalf("empty → default: got %q", got)
	}
	if got := toArcaDate("no-date", "20260701"); got != "20260701" {
		t.Fatalf("garbage → default: got %q", got)
	}
}

func TestResolveReceptor(t *testing.T) {
	t.Parallel()
	// CUIT de 11 dígitos.
	dt, dn, cond := resolveReceptor("20111111112", "responsable_inscripto")
	if dt != arca.DocCUIT || dn != "20111111112" || cond != arca.CondIVAResponsableInscripto {
		t.Fatalf("CUIT receptor mal resuelto: %d %s %d", dt, dn, cond)
	}
	// Sin identificación → Consumidor Final.
	dt, dn, cond = resolveReceptor("", "")
	if dt != arca.DocConsumidorFinal || dn != "0" || cond != arca.CondIVAConsumidorFinal {
		t.Fatalf("CF mal resuelto: %d %s %d", dt, dn, cond)
	}
}

func TestBuildImports(t *testing.T) {
	t.Parallel()

	// Factura B, una alícuota.
	neto, iva, total, lines, err := buildImports(arca.CbteFacturaB, SaleFiscalData{
		Subtotal: 100, TaxTotal: 21, Total: 121, Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}},
	})
	if err != nil || neto != 100 || iva != 21 || total != 121 || len(lines) != 1 || lines[0].ID != arca.IvaID21 {
		t.Fatalf("single-rate: neto=%.2f iva=%.2f total=%.2f lines=%d err=%v", neto, iva, total, len(lines), err)
	}

	// Multi-alícuota.
	neto, iva, total, lines, err = buildImports(arca.CbteFacturaA, SaleFiscalData{
		Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}, {TaxRate: 10.5, Subtotal: 100}},
	})
	if err != nil || neto != 200 || math.Abs(iva-31.5) > 0.001 || math.Abs(total-231.5) > 0.001 || len(lines) != 2 {
		t.Fatalf("multi-rate: neto=%.2f iva=%.2f total=%.2f lines=%d err=%v", neto, iva, total, len(lines), err)
	}

	// Tipo C: no discrimina IVA.
	neto, iva, total, lines, err = buildImports(arca.CbteFacturaC, SaleFiscalData{Total: 121, Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}}})
	if err != nil || neto != 121 || iva != 0 || total != 121 || len(lines) != 0 {
		t.Fatalf("tipo C: neto=%.2f iva=%.2f total=%.2f lines=%d err=%v", neto, iva, total, len(lines), err)
	}

	// Alícuota no soportada → error.
	if _, _, _, _, err := buildImports(arca.CbteFacturaB, SaleFiscalData{Items: []SaleFiscalItem{{TaxRate: 13.7, Subtotal: 100}}}); err == nil {
		t.Fatalf("expected error for unsupported IVA rate")
	}
}
