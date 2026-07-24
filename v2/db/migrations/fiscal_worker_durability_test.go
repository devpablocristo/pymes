package migrations

import (
	"strings"
	"testing"
)

func TestFiscalWorkerDurabilityMigrationSerializesAndReclaimsWork(t *testing.T) {
	raw, err := Files.ReadFile("000017_fiscal_worker_durability.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, required := range []string{
		"CREATE UNIQUE INDEX fiscal_vouchers_one_unresolved_series_uidx",
		"WHERE status IN ('processing', 'uncertain')",
		"voucher.status = 'processing'",
		"voucher.lease_until <= now()",
		"CREATE OR REPLACE FUNCTION fiscal.pending_organizations",
		"SECURITY DEFINER",
		"REVOKE ALL",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("fiscal worker durability migration does not contain %q", required)
		}
	}
}
