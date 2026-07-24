package accounting

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestParseStatementXLSXUsesExactDecimalStrings(t *testing.T) {
	t.Parallel()

	var content bytes.Buffer
	archive := zip.NewWriter(&content)
	worksheet, err := archive.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = worksheet.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row>
      <c r="A1" t="inlineStr"><is><t>fecha</t></is></c>
      <c r="B1" t="inlineStr"><is><t>descripcion</t></is></c>
      <c r="C1" t="inlineStr"><is><t>importe</t></is></c>
    </row>
    <row>
      <c r="A2" t="inlineStr"><is><t>2026-07-24</t></is></c>
      <c r="B2" t="inlineStr"><is><t>Transferencia</t></is></c>
      <c r="C2"><v>1234567890.12</v></c>
    </row>
  </sheetData>
</worksheet>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	movements, err := ParseStatementXLSX(content.Bytes(), MustCurrency("ARS"))
	if err != nil {
		t.Fatal(err)
	}
	if len(movements) != 1 {
		t.Fatalf("movement count = %d, want 1", len(movements))
	}
	if got := movements[0].Amount.String(); got != "1234567890.12" {
		t.Fatalf("amount = %s, want exact 1234567890.12", got)
	}
	if got := movements[0].Description; got != "Transferencia" {
		t.Fatalf("description = %q", got)
	}
}
