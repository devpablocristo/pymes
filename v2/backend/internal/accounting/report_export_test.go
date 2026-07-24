package accounting

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReportXLSXIsDeterministicAndKeepsExactNumbers(t *testing.T) {
	t.Parallel()

	report := deterministicReportFixture()
	var first, second bytes.Buffer
	if err := WriteReportXLSX(&first, report); err != nil {
		t.Fatal(err)
	}
	if err := WriteReportXLSX(&second, report); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("same report produced different XLSX bytes")
	}
	archive, err := zip.NewReader(bytes.NewReader(first.Bytes()), int64(first.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var worksheet string
	for _, file := range archive.File {
		if !file.Modified.Equal(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("XLSX part %s timestamp = %s", file.Name, file.Modified)
		}
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		reader, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		worksheet = string(content)
	}
	if !strings.Contains(worksheet, `<c r="C5" s="4"><v>1234.56</v></c>`) {
		t.Fatalf("worksheet did not preserve exact numeric cell:\n%s", worksheet)
	}
	if !strings.Contains(worksheet, "Honorarios &amp; servicios") {
		t.Fatalf("worksheet did not XML-escape text:\n%s", worksheet)
	}
}

func TestReportPDFIsDeterministicAndPaginatesWithoutVolatileMetadata(t *testing.T) {
	t.Parallel()

	report := deterministicReportFixture()
	for len(report.Rows) < 90 {
		report.Rows = append(report.Rows, append([]ReportCell(nil), report.Rows[0]...))
	}
	var first, second bytes.Buffer
	if err := WriteReportPDF(&first, report); err != nil {
		t.Fatal(err)
	}
	if err := WriteReportPDF(&second, report); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("same report produced different PDF bytes")
	}
	if !bytes.HasPrefix(first.Bytes(), []byte("%PDF-1.4")) {
		t.Fatalf("PDF header = %q", first.Bytes()[:min(16, first.Len())])
	}
	if bytes.Contains(first.Bytes(), []byte("CreationDate")) {
		t.Fatal("deterministic PDF contains volatile creation metadata")
	}
	if !bytes.Contains(first.Bytes(), []byte("/Count 3")) {
		t.Fatalf("PDF did not produce the expected three pages")
	}
	if !bytes.Contains(first.Bytes(), []byte("1234.56")) {
		t.Fatal("PDF did not preserve the exact decimal text")
	}
	if !bytes.HasSuffix(first.Bytes(), []byte("%%EOF\n")) {
		t.Fatal("PDF trailer is incomplete")
	}
}

func TestReportTableRejectsNonExactNumericCells(t *testing.T) {
	t.Parallel()

	report := deterministicReportFixture()
	report.Rows[0][2] = ReportCell{Value: "1e3", Numeric: true}
	if err := report.Validate(); err == nil {
		t.Fatal("scientific numeric export unexpectedly validated")
	}
}

func deterministicReportFixture() ReportTable {
	return ReportTable{
		Title:    "Estado de resultados",
		Subtitle: "Desde 2026-01-01 hasta 2026-12-31",
		Columns:  []string{"Código", "Cuenta", "Importe"},
		Rows: [][]ReportCell{{
			TextReportCell("6.1.01"),
			TextReportCell("Honorarios & servicios"),
			DecimalReportCell(MustDecimal("1234.56")),
		}},
	}
}
