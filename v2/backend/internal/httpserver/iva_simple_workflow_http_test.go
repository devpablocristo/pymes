package httpserver

import (
	"archive/zip"
	"bytes"
	"io"
	"reflect"
	"sort"
	"testing"

	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
)

func TestFullIVARegistryBundleIsDeterministicAndContainsFourBooks(t *testing.T) {
	t.Parallel()

	files := ar.IVASimpleFiles{
		SalesVouchers:    []byte("sales\n"),
		SalesVAT:         []byte("sales-vat\n"),
		PurchaseVouchers: []byte("purchases\n"),
		PurchaseVAT:      []byte("purchases-vat\n"),
	}
	first, err := fullIVARegistryBundle(files)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fullIVARegistryBundle(files)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("IVA export ZIP is not deterministic")
	}

	reader, err := zip.NewReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		entry, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(entry)
		_ = entry.Close()
		if err != nil {
			t.Fatal(err)
		}
		got[file.Name] = string(body)
	}
	want := map[string]string{
		"ventas-comprobantes.txt":  "sales\n",
		"ventas-alicuotas.txt":     "sales-vat\n",
		"compras-comprobantes.txt": "purchases\n",
		"compras-alicuotas.txt":    "purchases-vat\n",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IVA ZIP files = %#v, want %#v", got, want)
	}
}

func TestIVARecordKeyKeepsSupplierScopedPurchaseIdentity(t *testing.T) {
	t.Parallel()

	keys := []string{
		ivaRecordKey("purchases", "30710158211", 1, 3, 10),
		ivaRecordKey("purchases", "30710158238", 1, 3, 10),
		ivaRecordKey("sales", "", 1, 3, 10),
	}
	sort.Strings(keys)
	for index := 1; index < len(keys); index++ {
		if keys[index] == keys[index-1] {
			t.Fatalf("IVA document identities collided: %q", keys[index])
		}
	}
}

func TestOptionalIVADecimalNormalizesExactStrings(t *testing.T) {
	t.Parallel()

	raw := api.DecimalAmount("001.2300")
	value, err := optionalIVADecimal(&raw)
	if err != nil {
		t.Fatal(err)
	}
	if value == nil || *value != "1.23" {
		t.Fatalf("normalized IVA balance = %v, want 1.23", value)
	}
	invalid := api.DecimalAmount("NaN")
	if _, err := optionalIVADecimal(&invalid); err == nil {
		t.Fatal("invalid IVA balance was accepted")
	}
}
