package migrate

import (
	"context"
	"os"
	"testing"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
)

func TestUpFromEmptyDatabaseIsRepeatableAcrossReconnect(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database := openDatabase(t, ctx, databaseURL)
	resetDatabase(t, ctx, database)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetDatabase(t, cleanupCtx, database)
		database.Close()
	})

	if err := Up(ctx, database); err != nil {
		t.Fatalf("first Up() error = %v", err)
	}
	if err := Up(ctx, database); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	assertSchemaState(t, ctx, database)

	database.Close()
	database = openDatabase(t, ctx, databaseURL)
	if err := Up(ctx, database); err != nil {
		t.Fatalf("Up() after reconnect error = %v", err)
	}
	assertSchemaState(t, ctx, database)
}

func openDatabase(t *testing.T, ctx context.Context, databaseURL string) *postgres.DB {
	t.Helper()
	database, err := postgres.OpenWithConfig(ctx, databaseURL, postgres.DefaultConfig("pymes-v2-db-test"))
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	return database
}

func resetDatabase(t *testing.T, ctx context.Context, database *postgres.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, "DROP SCHEMA IF EXISTS app CASCADE"); err != nil {
		t.Fatalf("drop app schema: %v", err)
	}
	if _, err := database.Exec(ctx, "DROP TABLE IF EXISTS schema_migrations"); err != nil {
		t.Fatalf("drop migration table: %v", err)
	}
}

func assertSchemaState(t *testing.T, ctx context.Context, database *postgres.DB) {
	t.Helper()
	var schemaExists bool
	if err := database.QueryRow(ctx, "SELECT to_regnamespace('app') IS NOT NULL").Scan(&schemaExists); err != nil {
		t.Fatalf("query app schema: %v", err)
	}
	if !schemaExists {
		t.Fatal("app schema does not exist")
	}

	var migrationCount int
	if err := database.QueryRow(ctx, "SELECT count(*) FROM schema_migrations WHERE scope = $1", Scope).Scan(&migrationCount); err != nil {
		t.Fatalf("query migrations: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("migration count = %d, want 1", migrationCount)
	}
}
