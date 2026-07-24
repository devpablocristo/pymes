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
	if len(entries) != 18 ||
		entries[0].Name() != "000001_app_schema.sql" ||
		entries[1].Name() != "000002_iam_security.sql" ||
		entries[2].Name() != "000003_outbox_access.sql" ||
		entries[3].Name() != "000004_organization_provisioning.sql" ||
		entries[4].Name() != "000005_iam_security_upgrade.sql" ||
		entries[5].Name() != "000006_provisioning_worker.sql" ||
		entries[6].Name() != "000007_iam_worker_security.sql" ||
		entries[7].Name() != "000008_global_owner.sql" ||
		entries[8].Name() != "000009_administration_lifecycle.sql" ||
		entries[9].Name() != "000010_accounting_foundation.sql" ||
		entries[10].Name() != "000011_accounting_operations.sql" ||
		entries[11].Name() != "000012_fiscal_core.sql" ||
		entries[12].Name() != "000013_fiscal_iva.sql" ||
		entries[13].Name() != "000014_argentina_catalogs.sql" ||
		entries[14].Name() != "000015_fiscal_worker_runtime.sql" ||
		entries[15].Name() != "000016_homologation.sql" ||
		entries[16].Name() != "000017_fiscal_worker_durability.sql" ||
		entries[17].Name() != "000018_accounting_bootstrap.sql" {
		t.Fatalf("migration entries = %v", entries)
	}
}
