package fiscal

import (
	"bytes"
	"encoding/json"
	"testing"
)

func snapshotFixture(t *testing.T) Snapshot {
	t.Helper()
	snapshot, err := NewSnapshot(FiscalSnapshot{
		Version:     SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   "2026-07-24",
		Issuer:      PartySnapshot{Name: "Emisor SA", TaxID: "30000000007"},
		Receiver:    PartySnapshot{Name: "Cliente SA", TaxID: "30710158211"},
		Currency:    CurrencySnapshot{Code: "PES", Rate: MustDecimal("1")},
		Lines: []FiscalLineSnapshot{{
			Position: 1, Description: "Servicio", Quantity: MustDecimal("1"),
			UnitPrice: MustDecimal("100"), NetAmount: MustDecimal("100"),
			TaxCode: "IVA21", TaxRate: MustDecimal("21"), TaxAmount: MustDecimal("21"),
			TotalAmount: MustDecimal("121"),
		}},
		Totals: FiscalTotalsSnapshot{
			NetTaxed: MustDecimal("100"), VAT: MustDecimal("21"), Total: MustDecimal("121"),
			Functional: MustDecimal("121"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestSnapshotIsCanonicalHashedAndDefensivelyCopied(t *testing.T) {
	t.Parallel()

	snapshot := snapshotFixture(t)
	first := snapshot.CanonicalJSON()
	first[0] = 'X'
	if bytes.Equal(first, snapshot.CanonicalJSON()) {
		t.Fatal("snapshot exposed mutable canonical bytes")
	}

	wire, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Hash() != snapshot.Hash() {
		t.Fatalf("hash changed: got %s want %s", decoded.Hash(), snapshot.Hash())
	}
}

func TestSnapshotRejectsTampering(t *testing.T) {
	t.Parallel()

	snapshot := snapshotFixture(t)
	raw := snapshot.CanonicalJSON()
	raw = bytes.Replace(raw, []byte(`"121"`), []byte(`"122"`), 1)
	if _, err := ParseSnapshot(raw, snapshot.Hash()); err == nil {
		t.Fatal("expected hash mismatch")
	}
}
