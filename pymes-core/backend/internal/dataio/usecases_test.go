package dataio

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type fakeRepo struct {
	importRows []map[string]string
	mode       string
}

func (f *fakeRepo) ImportCustomers(ctx context.Context, tenantID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error) {
	_ = ctx
	_ = tenantID
	f.importRows = rows
	f.mode = mode
	return ImportResult{TotalRows: len(rows), Created: len(rows)}, nil
}

func (f *fakeRepo) ImportProducts(ctx context.Context, tenantID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error) {
	return f.ImportCustomers(ctx, tenantID, rows, mode)
}

func (f *fakeRepo) ImportSuppliers(ctx context.Context, tenantID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error) {
	return f.ImportCustomers(ctx, tenantID, rows, mode)
}

func (f *fakeRepo) ExportCustomers(ctx context.Context, tenantID uuid.UUID) ([]string, [][]string, error) {
	_ = ctx
	_ = tenantID
	return []string{"name", "type"}, [][]string{{"Juan", "person"}}, nil
}

func (f *fakeRepo) ExportProducts(ctx context.Context, tenantID uuid.UUID) ([]string, [][]string, error) {
	return f.ExportCustomers(ctx, tenantID)
}

func (f *fakeRepo) ExportSuppliers(ctx context.Context, tenantID uuid.UUID) ([]string, [][]string, error) {
	return f.ExportCustomers(ctx, tenantID)
}

func (f *fakeRepo) ExportSales(ctx context.Context, tenantID uuid.UUID, from, to *time.Time) ([]string, [][]string, error) {
	_ = from
	_ = to
	return f.ExportCustomers(ctx, tenantID)
}

func (f *fakeRepo) ExportCashflow(ctx context.Context, tenantID uuid.UUID, from, to *time.Time) ([]string, [][]string, error) {
	_ = from
	_ = to
	return f.ExportCustomers(ctx, tenantID)
}

type fakeAudit struct{ called bool }

func (f *fakeAudit) Log(ctx context.Context, tenantID string, actor, action, resourceType, resourceID string, payload map[string]any) {
	_ = ctx
	_ = tenantID
	_ = actor
	_ = action
	_ = resourceType
	_ = resourceID
	_ = payload
	f.called = true
}

func TestPreviewAndConfirmImport(t *testing.T) {
	repo := &fakeRepo{}
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)
	tmpDir := t.TempDir()
	uc.tempDir = tmpDir

	preview, err := uc.Preview(context.Background(), "customers", "customers.csv", []byte("name,type,email\nJuan,person,juan@example.com\n"))
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if preview.PreviewID == "" {
		t.Fatal("Preview() returned empty preview_id")
	}
	if preview.ValidRows != 1 {
		t.Fatalf("Preview().ValidRows = %d, want 1", preview.ValidRows)
	}

	result, err := uc.ConfirmImport(context.Background(), "customers", uuid.MustParse("00000000-0000-0000-0000-000000000001"), preview.PreviewID, "upsert", "tester")
	if err != nil {
		t.Fatalf("ConfirmImport() error = %v", err)
	}
	if result.Created != 1 {
		t.Fatalf("ConfirmImport().Created = %d, want 1", result.Created)
	}
	if repo.mode != "upsert" {
		t.Fatalf("repo.mode = %q, want upsert", repo.mode)
	}
	if !audit.called {
		t.Fatal("expected audit log to be called")
	}
	if _, err := os.Stat(tmpDir + "/" + preview.PreviewID + ".json"); !os.IsNotExist(err) {
		t.Fatal("expected preview file to be removed after confirm")
	}
}

func TestTemplateAndExportXLSX(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil)

	tpl, contentType, filename, err := uc.Template("customers", "xlsx")
	if err != nil {
		t.Fatalf("Template() error = %v", err)
	}
	if contentType != xlsxContentType {
		t.Fatalf("Template() contentType = %q, want %q", contentType, xlsxContentType)
	}
	if filename != "customers_template.xlsx" {
		t.Fatalf("Template() filename = %q", filename)
	}
	wb, err := excelize.OpenReader(bytes.NewReader(tpl))
	if err != nil {
		t.Fatalf("OpenReader(template) error = %v", err)
	}
	defer wb.Close()

	exported, exportedType, exportedName, err := uc.Export(context.Background(), "customers", uuid.New(), "xlsx", nil, nil)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if exportedType != xlsxContentType {
		t.Fatalf("Export() contentType = %q, want %q", exportedType, xlsxContentType)
	}
	if !strings.HasSuffix(exportedName, ".xlsx") {
		t.Fatalf("Export() filename = %q, want .xlsx", exportedName)
	}
	if _, err := excelize.OpenReader(bytes.NewReader(exported)); err != nil {
		t.Fatalf("OpenReader(exported) error = %v", err)
	}
}
