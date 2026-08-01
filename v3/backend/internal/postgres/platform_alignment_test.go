package postgres

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPublishedPlatformVersionsRemainPinned(t *testing.T) {
	t.Parallel()
	module, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(module), "\n")
	for _, requirement := range []string{
		"github.com/devpablocristo/platform/databases/postgres/go v0.5.0",
		"github.com/devpablocristo/platform/observability/go v0.2.1",
		"github.com/devpablocristo/platform/sdks/clerk/go v0.5.1",
	} {
		found := false
		for _, line := range lines {
			if strings.TrimSpace(line) == requirement {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("go.mod lacks pinned platform requirement %q", requirement)
		}
	}
}

func TestTenantDurabilityDoesNotImportIncompatiblePlatformStores(t *testing.T) {
	t.Parallel()
	forbidden := map[string]bool{
		"github.com/devpablocristo/platform/outbox/go":      true,
		"github.com/devpablocristo/platform/idempotency/go": true,
	}
	const platformPostgres = "github.com/devpablocristo/platform/databases/postgres/go"
	err := filepath.WalkDir(
		"../..",
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() ||
				filepath.Ext(path) != ".go" ||
				strings.HasSuffix(path, "_test.go") {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			file, err := parser.ParseFile(
				token.NewFileSet(),
				path,
				source,
				parser.ImportsOnly,
			)
			if err != nil {
				return err
			}
			for _, imported := range file.Imports {
				importPath, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				if forbidden[importPath] {
					t.Errorf(
						"%s imports incompatible tenant durability store %q",
						path,
						importPath,
					)
				}
				if importPath == platformPostgres &&
					!strings.HasSuffix(
						filepath.ToSlash(path),
						"/internal/postgres/postgres.go",
					) {
					t.Errorf(
						"%s bypasses the local PostgreSQL infrastructure adapter",
						path,
					)
				}
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLocalDurabilitySchemaKeepsRequiredTenantSemantics(t *testing.T) {
	t.Parallel()
	files := map[string][]string{
		"../../../db/migrations/001_core.sql": {
			"PRIMARY KEY (org_id, operation, source_id, source_version)",
			"payload_hash char(64) NOT NULL",
			"UNIQUE (org_id, topic, idempotency_key)",
			"CREATE POLICY idempotency_org_isolation",
			"CREATE POLICY outbox_org_isolation",
		},
		"../../../db/migrations/006_outbox_dead_letters.sql": {
			"CREATE TABLE IF NOT EXISTS app.outbox_dead_letters",
			"CREATE POLICY outbox_dead_letters_org_isolation",
		},
		"../../../db/migrations/008_public_idempotency_and_tenant_keys.sql": {
			"ON app.idempotency_records (org_id, operation, idempotency_key)",
			"CHECK (source_version > 0)",
		},
		"../../../db/migrations/010_service_response_inbox.sql": {
			"CREATE TABLE IF NOT EXISTS app.service_response_inbox",
			"PRIMARY KEY (org_id, service, request_id)",
			"UNIQUE (org_id, service, operation, idempotency_key)",
			"service response inbox is immutable",
		},
		"../../../db/migrations/013_outbox_origin_metadata.sql": {
			"ADD COLUMN IF NOT EXISTS request_id text",
			"ADD COLUMN IF NOT EXISTS actor_ref text",
			"ADD COLUMN IF NOT EXISTS source_version integer",
			"ADD COLUMN IF NOT EXISTS snapshot_digest char(64)",
		},
	}
	for path, required := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range required {
			if !strings.Contains(string(source), fragment) {
				t.Errorf("%s lacks durability contract %q", path, fragment)
			}
		}
	}
}
