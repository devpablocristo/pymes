package httpserver

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestJournalLineReportKeyCarriesEntryAndLineIdentity(t *testing.T) {
	t.Parallel()

	entryID := uuid.New()
	lineID := uuid.New()
	key := journalLineReportKey(entryID, lineID)

	if key != entryID.String()+":"+lineID.String() {
		t.Fatalf("report key = %q", key)
	}
	if !strings.HasPrefix(key, entryID.String()+":") ||
		!strings.HasSuffix(key, ":"+lineID.String()) {
		t.Fatalf("report key does not preserve drill-down ids: %q", key)
	}
}
