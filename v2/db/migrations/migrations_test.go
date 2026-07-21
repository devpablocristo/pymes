package migrations

import (
	"io/fs"
	"testing"
)

func TestInitialMigrationIsEmbedded(t *testing.T) {
	entries, err := fs.ReadDir(Files, ".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "000001_app_schema.sql" {
		t.Fatalf("migration entries = %v", entries)
	}
}
