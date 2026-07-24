package accounting

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type xlsxSharedStrings struct {
	Items []xlsxSharedString `xml:"si"`
}

type xlsxSharedString struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

type xlsxWorksheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Reference string `xml:"r,attr"`
	Type      string `xml:"t,attr"`
	Value     string `xml:"v"`
	Inline    struct {
		Text string `xml:"t"`
		Runs []struct {
			Text string `xml:"t"`
		} `xml:"r"`
	} `xml:"is"`
}

// ParseStatementXLSX reads the first worksheet of a conventional XLSX file and
// delegates header/amount validation to ParseStatementCSV.
func ParseStatementXLSX(content []byte, defaultCurrency Currency) ([]StatementMovement, error) {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid XLSX archive", ErrInvalidArgument)
	}
	files := make(map[string]*zip.File, len(archive.File))
	var worksheets []string
	for _, file := range archive.File {
		name := filepath.ToSlash(file.Name)
		files[name] = file
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") {
			worksheets = append(worksheets, name)
		}
	}
	if len(worksheets) == 0 {
		return nil, fmt.Errorf("%w: XLSX has no worksheets", ErrInvalidArgument)
	}
	sort.Strings(worksheets)
	shared, err := readXLSXSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}
	sheet, err := readXLSXWorksheet(files[worksheets[0]])
	if err != nil {
		return nil, err
	}
	records, err := xlsxRecords(sheet, shared)
	if err != nil {
		return nil, err
	}
	if len(records) > 1 {
		dateColumns := xlsxDateColumns(records[0])
		for row := 1; row < len(records); row++ {
			for column := range dateColumns {
				if column >= len(records[row]) {
					continue
				}
				if converted, parseErr := excelSerialDate(records[row][column]); parseErr == nil {
					records[row][column] = converted.Format("2006-01-02")
				}
			}
		}
	}
	var encoded bytes.Buffer
	writer := csv.NewWriter(&encoded)
	if err := writer.WriteAll(records); err != nil {
		return nil, fmt.Errorf("%w: normalize XLSX rows", ErrInvalidArgument)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("%w: normalize XLSX rows", ErrInvalidArgument)
	}
	return ParseStatementCSV(&encoded, defaultCurrency)
}

func readXLSXSharedStrings(file *zip.File) ([]string, error) {
	if file == nil {
		return nil, nil
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: open XLSX shared strings", ErrInvalidArgument)
	}
	defer reader.Close()
	var document xlsxSharedStrings
	if err := xml.NewDecoder(reader).Decode(&document); err != nil {
		return nil, fmt.Errorf("%w: decode XLSX shared strings", ErrInvalidArgument)
	}
	result := make([]string, 0, len(document.Items))
	for _, item := range document.Items {
		var text strings.Builder
		text.WriteString(item.Text)
		for _, run := range item.Runs {
			text.WriteString(run.Text)
		}
		result = append(result, text.String())
	}
	return result, nil
}

func readXLSXWorksheet(file *zip.File) (xlsxWorksheet, error) {
	if file == nil {
		return xlsxWorksheet{}, fmt.Errorf("%w: XLSX worksheet is missing", ErrInvalidArgument)
	}
	reader, err := file.Open()
	if err != nil {
		return xlsxWorksheet{}, fmt.Errorf("%w: open XLSX worksheet", ErrInvalidArgument)
	}
	defer reader.Close()
	var sheet xlsxWorksheet
	if err := xml.NewDecoder(io.LimitReader(reader, 64<<20)).Decode(&sheet); err != nil {
		return xlsxWorksheet{}, fmt.Errorf("%w: decode XLSX worksheet", ErrInvalidArgument)
	}
	return sheet, nil
}

func xlsxRecords(sheet xlsxWorksheet, shared []string) ([][]string, error) {
	records := make([][]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		record := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			column, err := xlsxColumnIndex(cell.Reference)
			if err != nil {
				return nil, err
			}
			for len(record) <= column {
				record = append(record, "")
			}
			value := cell.Value
			switch cell.Type {
			case "s":
				index, parseErr := strconv.Atoi(strings.TrimSpace(cell.Value))
				if parseErr != nil || index < 0 || index >= len(shared) {
					return nil, fmt.Errorf("%w: invalid XLSX shared string index", ErrInvalidArgument)
				}
				value = shared[index]
			case "inlineStr":
				var text strings.Builder
				text.WriteString(cell.Inline.Text)
				for _, run := range cell.Inline.Runs {
					text.WriteString(run.Text)
				}
				value = text.String()
			}
			record[column] = value
		}
		records = append(records, record)
	}
	return records, nil
}

func xlsxColumnIndex(reference string) (int, error) {
	if reference == "" {
		return 0, nil
	}
	column := 0
	letters := 0
	for _, char := range reference {
		if char < 'A' || char > 'Z' {
			break
		}
		column = column*26 + int(char-'A'+1)
		letters++
	}
	if letters == 0 {
		return 0, fmt.Errorf("%w: invalid XLSX cell reference", ErrInvalidArgument)
	}
	return column - 1, nil
}

func xlsxDateColumns(header []string) map[int]struct{} {
	result := make(map[int]struct{})
	for index, value := range header {
		switch normalizeHeader(value) {
		case "date", "fecha", "booked_at", "value_date", "fecha_valor":
			result[index] = struct{}{}
		}
	}
	return result
}

func excelSerialDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		value = value[:dot]
	}
	wholeDays, err := strconv.Atoi(value)
	if err != nil || wholeDays <= 0 {
		return time.Time{}, fmt.Errorf("%w: invalid XLSX date serial", ErrInvalidArgument)
	}
	// Excel includes the fictitious 1900-02-29. 1899-12-30 is the standard
	// compatibility epoch for serials after that point.
	return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, wholeDays), nil
}
