package migrations

import (
	"io/fs"
	"testing"
)

func TestProductMigrationsAreEmbedded(t *testing.T) {
	entries, err := fs.ReadDir(Files, ".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 7 ||
		entries[0].Name() != "000001_app_schema.sql" ||
		entries[1].Name() != "000002_iam_security.sql" ||
		entries[2].Name() != "000003_outbox_access.sql" ||
		entries[3].Name() != "000004_organization_provisioning.sql" ||
		entries[4].Name() != "000005_iam_security_upgrade.sql" ||
		entries[5].Name() != "000006_provisioning_worker.sql" ||
		entries[6].Name() != "000007_iam_worker_security.sql" {
		t.Fatalf("migration entries = %v", entries)
	}
}
