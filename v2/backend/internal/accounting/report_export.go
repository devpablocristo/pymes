package accounting

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ReportCell preserves whether a value is textual or numeric in tabular
// exports. Numeric values are always plain exact decimal strings.
type ReportCell struct {
	Value   string `json:"value"`
	Numeric bool   `json:"numeric"`
}

func TextReportCell(value string) ReportCell {
	return ReportCell{Value: value}
}

func DecimalReportCell(value Decimal) ReportCell {
	return ReportCell{Value: value.String(), Numeric: true}
}

// ReportTable is the country-neutral intermediate format used by CSV, XLSX,
// PDF and future export adapters.
type ReportTable struct {
	Title    string         `json:"title"`
	Subtitle string         `json:"subtitle,omitempty"`
	Columns  []string       `json:"columns"`
	Rows     [][]ReportCell `json:"rows"`
}

func (report ReportTable) Validate() error {
	if strings.TrimSpace(report.Title) == "" {
		return fmt.Errorf("%w: report title is required", ErrInvalidArgument)
	}
	if len(report.Columns) == 0 {
		return fmt.Errorf("%w: report columns are required", ErrInvalidArgument)
	}
	for index, column := range report.Columns {
		if strings.TrimSpace(column) == "" {
			return fmt.Errorf("%w: report column %d is blank", ErrInvalidArgument, index+1)
		}
	}
	for rowIndex, row := range report.Rows {
		if len(row) != len(report.Columns) {
			return fmt.Errorf(
				"%w: report row %d has %d cells, expected %d",
				ErrInvalidArgument,
				rowIndex+1,
				len(row),
				len(report.Columns),
			)
		}
		for columnIndex, cell := range row {
			if !cell.Numeric {
				continue
			}
			if _, err := ParseDecimal(cell.Value); err != nil {
				return fmt.Errorf(
					"%w: report row %d column %d is not an exact number",
					ErrInvalidArgument,
					rowIndex+1,
					columnIndex+1,
				)
			}
		}
	}
	return nil
}

func TrialBalanceReportTable(trial TrialBalance) ReportTable {
	rows := make([][]ReportCell, 0, len(trial.Rows)+1)
	for _, row := range trial.Rows {
		rows = append(rows, []ReportCell{
			TextReportCell(row.Code),
			TextReportCell(row.Name),
			TextReportCell(string(row.Class)),
			DecimalReportCell(row.Debit),
			DecimalReportCell(row.Credit),
			DecimalReportCell(row.DebitBalance),
			DecimalReportCell(row.CreditBalance),
		})
	}
	rows = append(rows, []ReportCell{
		TextReportCell(""),
		TextReportCell("TOTAL"),
		TextReportCell(""),
		DecimalReportCell(trial.TotalDebit),
		DecimalReportCell(trial.TotalCredit),
		DecimalReportCell(trial.TotalDebtor),
		DecimalReportCell(trial.TotalCreditor),
	})
	return ReportTable{
		Title:    "Balance de comprobación",
		Subtitle: reportPeriod(trial.From, trial.AsOf),
		Columns:  []string{"Código", "Cuenta", "Clase", "Debe", "Haber", "Saldo deudor", "Saldo acreedor"},
		Rows:     rows,
	}
}

func GeneralLedgerReportTable(ledger GeneralLedger) ReportTable {
	rows := make([][]ReportCell, 0, len(ledger.Lines)+2)
	rows = append(rows, []ReportCell{
		TextReportCell(ledger.From.Format("2006-01-02")),
		TextReportCell(""),
		TextReportCell("Saldo inicial"),
		DecimalReportCell(Zero),
		DecimalReportCell(Zero),
		DecimalReportCell(ledger.OpeningBalance),
	})
	for _, line := range ledger.Lines {
		rows = append(rows, []ReportCell{
			TextReportCell(line.Date.Format("2006-01-02")),
			TextReportCell(strconv.FormatInt(line.EntryNumber, 10)),
			TextReportCell(line.Description),
			DecimalReportCell(line.Debit),
			DecimalReportCell(line.Credit),
			DecimalReportCell(line.Balance),
		})
	}
	rows = append(rows, []ReportCell{
		TextReportCell(ledger.To.Format("2006-01-02")),
		TextReportCell(""),
		TextReportCell("Saldo final"),
		DecimalReportCell(Zero),
		DecimalReportCell(Zero),
		DecimalReportCell(ledger.ClosingBalance),
	})
	return ReportTable{
		Title:    "Libro Mayor · " + ledger.Account.Code + " " + ledger.Account.Name,
		Subtitle: reportPeriod(ledger.From, ledger.To),
		Columns:  []string{"Fecha", "Asiento", "Concepto", "Debe", "Haber", "Saldo"},
		Rows:     rows,
	}
}

func JournalReportTable(entries []JournalEntry) ReportTable {
	ordered := append([]JournalEntry(nil), entries...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Date.Equal(ordered[j].Date) {
			if ordered[i].Number == ordered[j].Number {
				return ordered[i].ID.String() < ordered[j].ID.String()
			}
			return ordered[i].Number < ordered[j].Number
		}
		return ordered[i].Date.Before(ordered[j].Date)
	})
	rows := make([][]ReportCell, 0)
	var firstDate, lastDate time.Time
	for _, entry := range ordered {
		if firstDate.IsZero() || entry.Date.Before(firstDate) {
			firstDate = entry.Date
		}
		if lastDate.IsZero() || entry.Date.After(lastDate) {
			lastDate = entry.Date
		}
		lines := append([]JournalLine(nil), entry.Lines...)
		sort.SliceStable(lines, func(i, j int) bool {
			if lines[i].LineNo == lines[j].LineNo {
				return lines[i].ID.String() < lines[j].ID.String()
			}
			return lines[i].LineNo < lines[j].LineNo
		})
		for _, line := range lines {
			rows = append(rows, []ReportCell{
				TextReportCell(entry.Date.Format("2006-01-02")),
				TextReportCell(strconv.FormatInt(entry.Number, 10)),
				TextReportCell(line.AccountCode),
				TextReportCell(line.AccountName),
				TextReportCell(firstNonEmpty(line.Memo, entry.Description)),
				DecimalReportCell(line.Debit),
				DecimalReportCell(line.Credit),
				TextReportCell(entry.Source.Type),
			})
		}
	}
	return ReportTable{
		Title:    "Libro Diario",
		Subtitle: reportPeriod(firstDate, lastDate),
		Columns:  []string{"Fecha", "Asiento", "Código", "Cuenta", "Concepto", "Debe", "Haber", "Origen"},
		Rows:     rows,
	}
}

func BalanceSheetReportTable(statement BalanceSheet) ReportTable {
	rows := make([][]ReportCell, 0, len(statement.Assets)+len(statement.Liabilities)+len(statement.Equity)+7)
	appendStatementRows := func(section string, statementRows []StatementRow) {
		for _, row := range statementRows {
			rows = append(rows, []ReportCell{
				TextReportCell(section),
				TextReportCell(row.Code),
				TextReportCell(row.Name),
				DecimalReportCell(row.Amount),
			})
		}
	}
	appendStatementRows("Activo", statement.Assets)
	rows = append(rows, reportTotalRow("Total activo", statement.TotalAssets))
	appendStatementRows("Pasivo", statement.Liabilities)
	rows = append(rows, reportTotalRow("Total pasivo", statement.TotalLiabilities))
	appendStatementRows("Patrimonio neto", statement.Equity)
	rows = append(rows,
		reportTotalRow("Total patrimonio neto", statement.TotalEquity),
		reportTotalRow("Resultado del ejercicio", statement.CurrentResult),
		reportTotalRow("Pasivo + patrimonio neto", statement.LiabilitiesAndEquity),
		reportTotalRow("Diferencia", statement.Difference),
	)
	return ReportTable{
		Title:    "Estado de situación patrimonial",
		Subtitle: "Al " + statement.AsOf.Format("2006-01-02"),
		Columns:  []string{"Rubro", "Código", "Cuenta", "Importe"},
		Rows:     rows,
	}
}

func IncomeStatementReportTable(statement IncomeStatement) ReportTable {
	rows := make([][]ReportCell, 0, len(statement.Revenue)+len(statement.Costs)+len(statement.Expenses)+5)
	appendStatementRows := func(section string, statementRows []StatementRow) {
		for _, row := range statementRows {
			rows = append(rows, []ReportCell{
				TextReportCell(section),
				TextReportCell(row.Code),
				TextReportCell(row.Name),
				DecimalReportCell(row.Amount),
			})
		}
	}
	appendStatementRows("Ingresos", statement.Revenue)
	rows = append(rows, reportTotalRow("Total ingresos", statement.TotalRevenue))
	appendStatementRows("Costos", statement.Costs)
	rows = append(rows,
		reportTotalRow("Total costos", statement.TotalCosts),
		reportTotalRow("Resultado bruto", statement.GrossProfit),
	)
	appendStatementRows("Gastos", statement.Expenses)
	rows = append(rows,
		reportTotalRow("Total gastos", statement.TotalExpenses),
		reportTotalRow("Resultado neto", statement.NetIncome),
	)
	return ReportTable{
		Title:    "Estado de resultados",
		Subtitle: reportPeriod(statement.From, statement.To),
		Columns:  []string{"Rubro", "Código", "Cuenta", "Importe"},
		Rows:     rows,
	}
}

func AgingReportTable(asOf time.Time, aging []PartyAging) ReportTable {
	ordered := append([]PartyAging(nil), aging...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Kind == ordered[j].Kind {
			return ordered[i].PartyID.String() < ordered[j].PartyID.String()
		}
		return ordered[i].Kind < ordered[j].Kind
	})
	rows := make([][]ReportCell, 0, len(ordered))
	for _, row := range ordered {
		rows = append(rows, []ReportCell{
			TextReportCell(string(row.Kind)),
			TextReportCell(row.PartyID.String()),
			DecimalReportCell(row.Buckets.Current),
			DecimalReportCell(row.Buckets.Days1To30),
			DecimalReportCell(row.Buckets.Days31To60),
			DecimalReportCell(row.Buckets.Days61To90),
			DecimalReportCell(row.Buckets.Over90),
			DecimalReportCell(row.Buckets.Total),
		})
	}
	return ReportTable{
		Title:    "Antigüedad de saldos",
		Subtitle: "Al " + asOf.Format("2006-01-02"),
		Columns:  []string{"Tipo", "Tercero", "A vencer", "1-30", "31-60", "61-90", "Más de 90", "Total"},
		Rows:     rows,
	}
}

// WriteReportXLSX writes a deterministic, dependency-free XLSX workbook. It
// intentionally omits volatile document timestamps and uses a fixed ZIP epoch.
func WriteReportXLSX(writer io.Writer, report ReportTable) error {
	if writer == nil {
		return fmt.Errorf("%w: XLSX writer is required", ErrInvalidArgument)
	}
	if err := report.Validate(); err != nil {
		return err
	}
	archive := zip.NewWriter(writer)
	files := []struct {
		name string
		data string
	}{
		{"[Content_Types].xml", xlsxContentTypes},
		{"_rels/.rels", xlsxRootRelationships},
		{"xl/workbook.xml", xlsxWorkbook},
		{"xl/_rels/workbook.xml.rels", xlsxWorkbookRelationships},
		{"xl/styles.xml", xlsxStyles},
		{"xl/worksheets/sheet1.xml", buildReportWorksheet(report)},
	}
	for _, file := range files {
		if err := writeDeterministicZipFile(archive, file.name, file.data); err != nil {
			_ = archive.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("accounting: close XLSX: %w", err)
	}
	return nil
}

func ExportReportXLSX(report ReportTable) ([]byte, error) {
	var result bytes.Buffer
	if err := WriteReportXLSX(&result, report); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

// WriteReportPDF writes a deterministic A4-landscape PDF using a built-in
// WinAnsi Courier font. The same report always yields identical bytes.
func WriteReportPDF(writer io.Writer, report ReportTable) error {
	if writer == nil {
		return fmt.Errorf("%w: PDF writer is required", ErrInvalidArgument)
	}
	if err := report.Validate(); err != nil {
		return err
	}
	pages := reportPDFPages(report)
	document := buildPDFDocument(pages)
	if _, err := writer.Write(document); err != nil {
		return fmt.Errorf("accounting: write PDF: %w", err)
	}
	return nil
}

func ExportReportPDF(report ReportTable) ([]byte, error) {
	var result bytes.Buffer
	if err := WriteReportPDF(&result, report); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func reportTotalRow(label string, amount Decimal) []ReportCell {
	return []ReportCell{
		TextReportCell(""),
		TextReportCell(""),
		TextReportCell(label),
		DecimalReportCell(amount),
	}
}

func reportPeriod(from, to time.Time) string {
	if from.IsZero() && to.IsZero() {
		return ""
	}
	if from.IsZero() {
		return "Al " + to.Format("2006-01-02")
	}
	if to.IsZero() {
		return "Desde " + from.Format("2006-01-02")
	}
	return "Desde " + from.Format("2006-01-02") + " hasta " + to.Format("2006-01-02")
}

func writeDeterministicZipFile(archive *zip.Writer, name, data string) error {
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Store,
	}
	header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("accounting: create XLSX part %s: %w", name, err)
	}
	if _, err := io.WriteString(entry, data); err != nil {
		return fmt.Errorf("accounting: write XLSX part %s: %w", name, err)
	}
	return nil
}

func buildReportWorksheet(report ReportTable) string {
	var result strings.Builder
	result.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	result.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	result.WriteString(`<sheetViews><sheetView workbookViewId="0"><pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
	result.WriteString(`<cols>`)
	for index := range report.Columns {
		width := 18
		if index == 1 || index == 2 || index == 4 {
			width = 30
		}
		fmt.Fprintf(&result, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, index+1, index+1, width)
	}
	result.WriteString(`</cols><sheetData>`)
	writeXLSXTextRow(&result, 1, 1, report.Title, 1)
	if report.Subtitle != "" {
		writeXLSXTextRow(&result, 2, 1, report.Subtitle, 2)
	}
	result.WriteString(`<row r="4">`)
	for index, column := range report.Columns {
		writeXLSXTextCell(&result, 4, index+1, column, 3)
	}
	result.WriteString(`</row>`)
	for rowIndex, row := range report.Rows {
		excelRow := rowIndex + 5
		fmt.Fprintf(&result, `<row r="%d">`, excelRow)
		for columnIndex, cell := range row {
			if cell.Numeric {
				fmt.Fprintf(
					&result,
					`<c r="%s" s="4"><v>%s</v></c>`,
					xlsxCellReference(columnIndex+1, excelRow),
					xmlText(cell.Value),
				)
				continue
			}
			writeXLSXTextCell(&result, excelRow, columnIndex+1, cell.Value, 0)
		}
		result.WriteString(`</row>`)
	}
	result.WriteString(`</sheetData>`)
	lastColumn := xlsxColumnName(len(report.Columns))
	if len(report.Columns) > 1 {
		fmt.Fprintf(&result, `<mergeCells count="2"><mergeCell ref="A1:%s1"/><mergeCell ref="A2:%s2"/></mergeCells>`, lastColumn, lastColumn)
	}
	fmt.Fprintf(&result, `<autoFilter ref="A4:%s%d"/>`, lastColumn, max(4, len(report.Rows)+4))
	result.WriteString(`</worksheet>`)
	return result.String()
}

func writeXLSXTextRow(result *strings.Builder, row, column int, value string, style int) {
	fmt.Fprintf(result, `<row r="%d">`, row)
	writeXLSXTextCell(result, row, column, value, style)
	result.WriteString(`</row>`)
}

func writeXLSXTextCell(result *strings.Builder, row, column int, value string, style int) {
	fmt.Fprintf(
		result,
		`<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`,
		xlsxCellReference(column, row),
		style,
		xmlText(value),
	)
}

func xlsxCellReference(column, row int) string {
	return xlsxColumnName(column) + strconv.Itoa(row)
}

func xlsxColumnName(column int) string {
	if column <= 0 {
		return "A"
	}
	var reversed []byte
	for column > 0 {
		column--
		reversed = append(reversed, byte('A'+column%26))
		column /= 26
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return string(reversed)
}

func xmlText(value string) string {
	var cleaned strings.Builder
	for _, current := range value {
		if current == '\t' || current == '\n' || current == '\r' ||
			(current >= 0x20 && current <= 0xD7FF) ||
			(current >= 0xE000 && current <= 0xFFFD) ||
			(current >= 0x10000 && current <= 0x10FFFF) {
			cleaned.WriteRune(current)
		}
	}
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(cleaned.String()))
	return escaped.String()
}

func reportPDFPages(report ReportTable) [][]string {
	const rowsPerPage = 42
	widths := reportColumnWidths(report)
	header := formatPDFReportRow(reportHeaderCells(report.Columns), widths)
	allRows := make([]string, 0, len(report.Rows))
	for _, row := range report.Rows {
		allRows = append(allRows, formatPDFReportRow(row, widths))
	}
	if len(allRows) == 0 {
		allRows = append(allRows, "")
	}
	pageCount := (len(allRows) + rowsPerPage - 1) / rowsPerPage
	pages := make([][]string, 0, pageCount)
	for page := 0; page < pageCount; page++ {
		start := page * rowsPerPage
		end := min(start+rowsPerPage, len(allRows))
		lines := []string{
			report.Title,
			report.Subtitle,
			header,
			strings.Repeat("-", min(125, len(header))),
		}
		lines = append(lines, allRows[start:end]...)
		if pageCount > 1 {
			lines = append(lines, fmt.Sprintf("Página %d de %d", page+1, pageCount))
		}
		pages = append(pages, lines)
	}
	return pages
}

func reportHeaderCells(columns []string) []ReportCell {
	result := make([]ReportCell, 0, len(columns))
	for _, column := range columns {
		result = append(result, TextReportCell(column))
	}
	return result
}

func reportColumnWidths(report ReportTable) []int {
	const available = 180
	columns := len(report.Columns)
	widths := make([]int, columns)
	base := max(8, (available-(columns-1)*3)/columns)
	for index := range widths {
		widths[index] = base
	}
	for index, column := range report.Columns {
		normalized := strings.ToLower(column)
		if strings.Contains(normalized, "cuenta") ||
			strings.Contains(normalized, "concepto") ||
			strings.Contains(normalized, "descripción") {
			widths[index] += 8
		}
	}
	for _, row := range report.Rows {
		for index, cell := range row {
			if !cell.Numeric {
				continue
			}
			widths[index] = max(widths[index], utf8.RuneCountInString(cell.Value))
		}
	}
	total := (columns - 1) * 3
	for _, width := range widths {
		total += width
	}
	for total > available {
		changed := false
		for index := range widths {
			if total <= available {
				break
			}
			if widths[index] > 8 {
				widths[index]--
				total--
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return widths
}

func formatPDFReportRow(cells []ReportCell, widths []int) string {
	parts := make([]string, 0, len(cells))
	for index, cell := range cells {
		width := widths[index]
		value := strings.Join(strings.Fields(cell.Value), " ")
		if cell.Numeric {
			parts = append(parts, strings.Repeat(" ", max(0, width-utf8.RuneCountInString(value)))+value)
		} else {
			value = truncateRunes(value, width)
			parts = append(parts, value+strings.Repeat(" ", max(0, width-utf8.RuneCountInString(value))))
		}
	}
	return strings.Join(parts, " | ")
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func buildPDFDocument(pages [][]string) []byte {
	objectCount := 3 + len(pages)*2
	objects := make([][]byte, objectCount+1)
	objects[1] = []byte(`<< /Type /Catalog /Pages 2 0 R >>`)
	var kids strings.Builder
	for index := range pages {
		fmt.Fprintf(&kids, "%d 0 R ", 4+index*2)
	}
	objects[2] = []byte(fmt.Sprintf(
		`<< /Type /Pages /Kids [ %s] /Count %d >>`,
		kids.String(),
		len(pages),
	))
	objects[3] = []byte(`<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>`)
	for index, lines := range pages {
		pageObject := 4 + index*2
		contentObject := pageObject + 1
		objects[pageObject] = []byte(fmt.Sprintf(
			`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 842 595] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>`,
			contentObject,
		))
		content := buildPDFPageContent(lines)
		objects[contentObject] = []byte(fmt.Sprintf(
			"<< /Length %d >>\nstream\n%s\nendstream",
			len(content),
			content,
		))
	}

	var document bytes.Buffer
	document.WriteString("%PDF-1.4\n%\xD0\xD4\xC5\xD8\n")
	offsets := make([]int, objectCount+1)
	for index := 1; index <= objectCount; index++ {
		offsets[index] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n", index)
		document.Write(objects[index])
		document.WriteString("\nendobj\n")
	}
	xref := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n", objectCount+1)
	document.WriteString("0000000000 65535 f \n")
	for index := 1; index <= objectCount; index++ {
		fmt.Fprintf(&document, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(
		&document,
		"trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		objectCount+1,
		xref,
	)
	return document.Bytes()
}

func buildPDFPageContent(lines []string) string {
	var content strings.Builder
	content.WriteString("BT\n")
	for index, line := range lines {
		fontSize := 7
		y := 558 - index*12
		if index == 0 {
			fontSize = 14
		} else if index == 1 {
			fontSize = 9
		}
		fmt.Fprintf(
			&content,
			"/F1 %d Tf\n1 0 0 1 36 %d Tm\n(%s) Tj\n",
			fontSize,
			y,
			pdfText(line),
		)
	}
	content.WriteString("ET")
	return content.String()
}

func pdfText(value string) string {
	var result strings.Builder
	for _, current := range value {
		encoded := pdfWinAnsi(current)
		switch encoded {
		case '\\', '(', ')':
			result.WriteByte('\\')
			result.WriteByte(encoded)
		default:
			if encoded < 0x20 {
				result.WriteByte('?')
			} else {
				result.WriteByte(encoded)
			}
		}
	}
	return result.String()
}

func pdfWinAnsi(value rune) byte {
	if value >= 0x20 && value <= 0xFF {
		return byte(value)
	}
	switch value {
	case '€':
		return 128
	case '‚':
		return 130
	case '“':
		return 147
	case '”':
		return 148
	case '–':
		return 150
	case '—':
		return 151
	case '…':
		return 133
	default:
		return '?'
	}
}

const xlsxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
	`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
	`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` +
	`</Types>`

const xlsxRootRelationships = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
	`</Relationships>`

const xlsxWorkbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
	`<sheets><sheet name="Reporte" sheetId="1" r:id="rId1"/></sheets>` +
	`</workbook>`

const xlsxWorkbookRelationships = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
	`</Relationships>`

const xlsxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<numFmts count="1"><numFmt numFmtId="164" formatCode="0.####################"/></numFmts>` +
	`<fonts count="2"><font><sz val="10"/><name val="Calibri"/></font><font><b/><sz val="10"/><name val="Calibri"/></font></fonts>` +
	`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FFD9EAF7"/><bgColor indexed="64"/></patternFill></fill></fills>` +
	`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
	`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
	`<cellXfs count="5">` +
	`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
	`<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/>` +
	`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
	`<xf numFmtId="0" fontId="1" fillId="1" borderId="0" xfId="0" applyFont="1" applyFill="1"/>` +
	`<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>` +
	`</cellXfs>` +
	`</styleSheet>`
