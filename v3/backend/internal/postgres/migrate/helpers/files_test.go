package helpers

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsSQLMigrationsInDirectoryOrder(t *testing.T) {
	directory := t.TempDir()
	for name, body := range map[string]string{
		"002_second.sql": "SELECT 2",
		"001_first.sql":  "SELECT 1",
		"README.md":      "ignored",
	} {
		if err := os.WriteFile(
			filepath.Join(directory, name),
			[]byte(body),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	migrations, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 {
		t.Fatalf("migrations = %v", migrations)
	}
	if migrations[0].Name != "001_first.sql" ||
		migrations[0].SQL != "SELECT 1" ||
		migrations[1].Name != "002_second.sql" ||
		migrations[1].SQL != "SELECT 2" {
		t.Fatalf("migrations = %#v", migrations)
	}
}

func TestLoadClassifiesUnavailableDirectory(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, ErrDirectoryUnavailable) {
		t.Fatalf("error = %v", err)
	}
}
