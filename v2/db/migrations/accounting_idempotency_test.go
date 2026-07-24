package migrations

import (
	"strings"
	"testing"
)

func TestJournalPersistsCanonicalSourceEventForIdempotentReplay(t *testing.T) {
	t.Parallel()

	body, err := Files.ReadFile("000010_accounting_foundation.sql")
	if err != nil {
		t.Fatalf("read accounting foundation migration: %v", err)
	}
	sql := string(body)
	for _, required := range []string{
		"source_event text NOT NULL",
		"CHECK (btrim(source_event) <> '')",
		"CONSTRAINT accounting_journal_entries_idempotency_unique",
		"UNIQUE (org_id, idempotency_key)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("accounting idempotency SQL does not contain %q", required)
		}
	}
}
