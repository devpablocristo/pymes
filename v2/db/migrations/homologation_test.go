package migrations

import (
	"strings"
	"testing"
)

func TestHomologationMigrationIsReadOnlyTenantScopedEvidence(t *testing.T) {
	raw, err := Files.ReadFile("000016_homologation.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, required := range []string{
		"ALTER TABLE fiscal.homologation_runs FORCE ROW LEVEL SECURITY",
		"ALTER TABLE fiscal.homologation_checks FORCE ROW LEVEL SECURITY",
		"org_id = app.current_org_id()",
		"finalized homologation runs are immutable",
		"homologation checks are immutable",
		"evidence jsonb NOT NULL",
		"GRANT SELECT, INSERT, UPDATE",
		"GRANT SELECT, INSERT",
		"Evidencia técnica",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("homologation migration does not contain %q", required)
		}
	}
	if strings.Contains(sql, "GRANT DELETE") {
		t.Fatal("homologation evidence must not grant DELETE")
	}
}
