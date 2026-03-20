package dataio

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"

	"github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

const maxImportRows = 10000

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type RepositoryPort interface {
	ImportCustomers(ctx context.Context, orgID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error)
	ImportProducts(ctx context.Context, orgID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error)
	ImportSuppliers(ctx context.Context, orgID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error)
	ExportCustomers(ctx context.Context, orgID uuid.UUID) ([]string, [][]string, error)
	ExportProducts(ctx context.Context, orgID uuid.UUID) ([]string, [][]string, error)
	ExportSuppliers(ctx context.Context, orgID uuid.UUID) ([]string, [][]string, error)
	ExportSales(ctx context.Context, orgID uuid.UUID, from, to *time.Time) ([]string, [][]string, error)
	ExportCashflow(ctx context.Context, orgID uuid.UUID, from, to *time.Time) ([]string, [][]string, error)
}

type Usecases struct {
	repo    RepositoryPort
	audit   AuditPort
	tempDir string
}

type Preview struct {
	PreviewID  string              `json:"preview_id"`
	FileName   string              `json:"file_name"`
	Format     string              `json:"format"`
	TotalRows  int                 `json:"total_rows"`
	ValidRows  int                 `json:"valid_rows"`
	ErrorRows  int                 `json:"error_rows"`
	Columns    []string            `json:"columns"`
	SampleRows []map[string]string `json:"sample_rows"`
	Errors     []ImportError       `json:"errors"`
}

type ImportError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

type ImportResult struct {
	TotalRows int           `json:"total_rows"`
	Created   int           `json:"created"`
	Updated   int           `json:"updated"`
	Skipped   int           `json:"skipped"`
	Errors    []ImportError `json:"errors"`
}

type previewJob struct {
	ID        string              `json:"id"`
	Entity    string              `json:"entity"`
	Format    string              `json:"format"`
	FileName  string              `json:"file_name"`
	Columns   []string            `json:"columns"`
	Rows      []map[string]string `json:"rows"`
	CreatedAt time.Time           `json:"created_at"`
}

func NewUsecases(repo RepositoryPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, audit: audit, tempDir: "/tmp/pymes-dataio"}
}

func (u *Usecases) Preview(ctx context.Context, entity, filename string, fileData []byte) (Preview, error) {
	entity = normalizeEntity(entity)
	if !supportsImport(entity) {
		return Preview{}, apperror.NewBadInput("unsupported import entity")
	}
	format := detectFormat(filename)
	if format == "" {
		return Preview{}, apperror.NewBadInput("unsupported file format")
	}
	rows, columns, err := parseRows(format, fileData)
	if err != nil {
		return Preview{}, err
	}
	if len(rows) > maxImportRows {
		return Preview{}, apperror.NewBadInput("file exceeds 10000 rows")
	}

	preview := Preview{
		FileName: filename,
		Format:   format,
		Columns:  columns,
	}
	job := previewJob{Entity: entity, Format: format, FileName: filename, Columns: columns, CreatedAt: time.Now().UTC()}
	for idx, row := range rows {
		normalized := normalizeRow(row)
		rowErrors := validateRow(entity, normalized, idx+2)
		if len(rowErrors) > 0 {
			preview.Errors = append(preview.Errors, rowErrors...)
			preview.ErrorRows++
			continue
		}
		preview.ValidRows++
		job.Rows = append(job.Rows, normalized)
		if len(preview.SampleRows) < 5 {
			preview.SampleRows = append(preview.SampleRows, normalized)
		}
	}
	preview.TotalRows = len(rows)
	previewID, err := u.savePreview(job)
	if err != nil {
		return Preview{}, err
	}
	preview.PreviewID = previewID
	return preview, nil
}

func (u *Usecases) ConfirmImport(ctx context.Context, entity string, orgID uuid.UUID, previewID, mode, actor string) (ImportResult, error) {
	entity = normalizeEntity(entity)
	if previewID == "" {
		return ImportResult{}, apperror.NewBadInput("preview_id is required")
	}
	job, err := u.loadPreview(previewID)
	if err != nil {
		return ImportResult{}, err
	}
	if job.Entity != entity {
		return ImportResult{}, apperror.NewBadInput("preview entity mismatch")
	}
	mode = normalizeMode(mode)
	var result ImportResult
	switch entity {
	case "customers":
		result, err = u.repo.ImportCustomers(ctx, orgID, job.Rows, mode)
	case "products":
		result, err = u.repo.ImportProducts(ctx, orgID, job.Rows, mode)
	case "suppliers":
		result, err = u.repo.ImportSuppliers(ctx, orgID, job.Rows, mode)
	default:
		return ImportResult{}, apperror.NewBadInput("unsupported import entity")
	}
	if err != nil {
		return ImportResult{}, err
	}
	result.TotalRows = len(job.Rows)
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "dataio.import.confirmed", "dataio_preview", previewID, map[string]any{
			"entity":  entity,
			"mode":    mode,
			"created": result.Created,
			"updated": result.Updated,
			"skipped": result.Skipped,
			"errors":  len(result.Errors),
		})
	}
	_ = os.Remove(filepath.Join(u.tempDir, previewID+".json"))
	return result, nil
}

func (u *Usecases) Template(entity, format string) ([]byte, string, string, error) {
	entity = normalizeEntity(entity)
	headers, example, err := templateDefinition(entity)
	if err != nil {
		return nil, "", "", err
	}
	format = normalizeFormat(format)
	if format == "" {
		format = "csv"
	}
	if format == "csv" {
		content, err := buildCSV(headers, [][]string{example})
		if err != nil {
			return nil, "", "", err
		}
		return content, csvContentType, entity + "_template.csv", nil
	}
	content, err := buildXLSX(headers, [][]string{example})
	if err != nil {
		return nil, "", "", err
	}
	return content, xlsxContentType, entity + "_template.xlsx", nil
}

func (u *Usecases) Export(ctx context.Context, entity string, orgID uuid.UUID, format string, from, to *time.Time) ([]byte, string, string, error) {
	entity = normalizeEntity(entity)
	format = normalizeFormat(format)
	if format == "" {
		format = "csv"
	}
	if (entity == "sales" || entity == "cashflow") && (from == nil || to == nil) {
		return nil, "", "", apperror.NewBadInput("from and to are required for this export")
	}

	var headers []string
	var rows [][]string
	var err error
	switch entity {
	case "customers":
		headers, rows, err = u.repo.ExportCustomers(ctx, orgID)
	case "products":
		headers, rows, err = u.repo.ExportProducts(ctx, orgID)
	case "suppliers":
		headers, rows, err = u.repo.ExportSuppliers(ctx, orgID)
	case "sales":
		headers, rows, err = u.repo.ExportSales(ctx, orgID, from, to)
	case "cashflow":
		headers, rows, err = u.repo.ExportCashflow(ctx, orgID, from, to)
	default:
		return nil, "", "", apperror.NewBadInput("unsupported export entity")
	}
	if err != nil {
		return nil, "", "", err
	}

	dateSuffix := time.Now().UTC().Format("2006-01-02")
	if format == "csv" {
		content, err := buildCSV(headers, rows)
		if err != nil {
			return nil, "", "", err
		}
		return content, csvContentType, entity + "_" + dateSuffix + ".csv", nil
	}
	content, err := buildXLSX(headers, rows)
	if err != nil {
		return nil, "", "", err
	}
	return content, xlsxContentType, entity + "_" + dateSuffix + ".xlsx", nil
}

func parseRows(format string, fileData []byte) ([]map[string]string, []string, error) {
	switch format {
	case "csv":
		return parseCSV(fileData)
	case "xlsx":
		return parseXLSX(fileData)
	default:
		return nil, nil, apperror.NewBadInput("unsupported file format")
	}
}

func parseCSV(fileData []byte) ([]map[string]string, []string, error) {
	payload := stripUTF8BOM(fileData)
	if !utf8.Valid(payload) {
		decoded, err := charmap.ISO8859_1.NewDecoder().Bytes(payload)
		if err == nil {
			payload = decoded
		}
	}
	reader := csv.NewReader(bytes.NewReader(payload))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, apperror.NewBadInput("invalid csv")
	}
	return recordsToMaps(records)
}

func parseXLSX(fileData []byte) ([]map[string]string, []string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(fileData))
	if err != nil {
		return nil, nil, apperror.NewBadInput("invalid xlsx")
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, apperror.NewBadInput("empty xlsx")
	}
	records, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, nil, apperror.NewBadInput("invalid xlsx rows")
	}
	return recordsToMaps(records)
}

func recordsToMaps(records [][]string) ([]map[string]string, []string, error) {
	if len(records) == 0 {
		return nil, nil, apperror.NewBadInput("empty file")
	}
	headers := normalizeHeaders(records[0])
	if len(headers) == 0 {
		return nil, nil, apperror.NewBadInput("missing headers")
	}
	rows := make([]map[string]string, 0, max(0, len(records)-1))
	for _, record := range records[1:] {
		row := map[string]string{}
		empty := true
		for idx, header := range headers {
			if header == "" {
				continue
			}
			value := ""
			if idx < len(record) {
				value = strings.TrimSpace(record[idx])
			}
			if value != "" {
				empty = false
			}
			row[header] = value
		}
		if !empty {
			rows = append(rows, row)
		}
	}
	return rows, headers, nil
}

func normalizeHeaders(headers []string) []string {
	out := make([]string, 0, len(headers))
	for _, header := range headers {
		out = append(out, normalizeHeader(header))
	}
	return out
}

func normalizeHeader(raw string) string {
	replacer := strings.NewReplacer(" ", "_", "-", "_", ".", "_", "/", "_")
	return strings.Trim(replacer.Replace(strings.ToLower(strings.TrimSpace(raw))), "_")
}

func normalizeRow(row map[string]string) map[string]string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(row))
	for _, key := range keys {
		out[normalizeHeader(key)] = strings.TrimSpace(row[key])
	}
	return out
}

func validateRow(entity string, row map[string]string, rowNumber int) []ImportError {
	errs := make([]ImportError, 0)
	required := map[string][]string{
		"customers": {"name"},
		"products":  {"name", "price"},
		"suppliers": {"name"},
	}[entity]
	for _, field := range required {
		if strings.TrimSpace(row[field]) == "" {
			errs = append(errs, ImportError{Row: rowNumber, Column: field, Message: "required field"})
		}
	}
	if entity == "customers" {
		if v := strings.TrimSpace(row["type"]); v != "" && v != "person" && v != "company" {
			errs = append(errs, ImportError{Row: rowNumber, Column: "type", Value: v, Message: "must be person or company"})
		}
	}
	if entity == "products" {
		if v := strings.TrimSpace(row["type"]); v != "" && v != "product" && v != "service" {
			errs = append(errs, ImportError{Row: rowNumber, Column: "type", Value: v, Message: "must be product or service"})
		}
		if v := strings.TrimSpace(row["track_stock"]); v != "" && v != "true" && v != "false" && v != "1" && v != "0" {
			errs = append(errs, ImportError{Row: rowNumber, Column: "track_stock", Value: v, Message: "must be true or false"})
		}
	}
	return errs
}

func templateDefinition(entity string) ([]string, []string, error) {
	switch normalizeEntity(entity) {
	case "customers":
		return []string{"name", "type", "email", "phone", "tax_id", "address_street", "address_city", "address_state", "address_zip_code", "address_country", "notes", "tags"}, []string{"Juan Perez", "person", "juan@example.com", "+5493815551234", "20333444559", "San Martin 123", "San Miguel de Tucuman", "Tucuman", "4000", "AR", "Cliente mayorista", "vip,frecuente"}, nil
	case "products":
		return []string{"name", "type", "sku", "price", "cost_price", "unit", "tax_rate", "track_stock", "description", "tags"}, []string{"Cafe molido 500g", "product", "CAF-500", "8500", "6200", "unidad", "21", "true", "Cafe tostado molido", "almacen,cafe"}, nil
	case "suppliers":
		return []string{"name", "email", "phone", "tax_id", "contact_name", "address_street", "address_city", "address_state", "address_zip_code", "address_country", "notes", "tags"}, []string{"Distribuidora Norte", "ventas@norte.com", "+5493814440000", "30711222334", "Laura Diaz", "Ruta 9 km 12", "Tafi Viejo", "Tucuman", "4103", "AR", "Entrega semanal", "insumos,prioritario"}, nil
	default:
		return nil, nil, apperror.NewBadInput("unsupported template entity")
	}
}

func detectFormat(filename string) string {
	name := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(name, ".csv"):
		return "csv"
	case strings.HasSuffix(name, ".xlsx"):
		return "xlsx"
	default:
		return ""
	}
}

func normalizeEntity(entity string) string {
	return strings.TrimSpace(strings.ToLower(entity))
}

func normalizeMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "upsert") {
		return "upsert"
	}
	return "create_only"
}

func normalizeFormat(format string) string {
	if strings.EqualFold(strings.TrimSpace(format), "csv") {
		return "csv"
	}
	if strings.EqualFold(strings.TrimSpace(format), "xlsx") || strings.TrimSpace(format) == "" {
		return "xlsx"
	}
	return ""
}

func supportsImport(entity string) bool {
	switch normalizeEntity(entity) {
	case "customers", "products", "suppliers":
		return true
	default:
		return false
	}
}

func stripUTF8BOM(in []byte) []byte {
	if len(in) >= 3 && in[0] == 0xEF && in[1] == 0xBB && in[2] == 0xBF {
		return in[3:]
	}
	return in
}

func buildCSV(headers []string, rows [][]string) ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(buf)
	if err := writer.Write(headers); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

const (
	csvContentType  = "text/csv; charset=utf-8"
	xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

func buildXLSX(headers []string, rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	for idx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 1)
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
	}
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	_ = f.SetCellStyle(sheet, "A1", fmt.Sprintf("%s1", excelColumnName(len(headers))), style)
	for rowIdx, row := range rows {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	for idx := range headers {
		col := excelColumnName(idx + 1)
		_ = f.SetColWidth(sheet, col, col, 18)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func excelColumnName(index int) string {
	name, _ := excelize.ColumnNumberToName(index)
	return name
}
