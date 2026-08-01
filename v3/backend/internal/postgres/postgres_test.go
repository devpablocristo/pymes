package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestOpenFailsBeforeDialingForInvalidConfiguration(t *testing.T) {
	t.Setenv("PYMES_POSTGRES_MAX_CONNS", "0")
	_, err := Open(
		context.Background(),
		"postgres://unused",
		"pymes-v3-test",
	)
	if err == nil ||
		!strings.Contains(err.Error(), "PYMES_POSTGRES_MAX_CONNS must be > 0") {
		t.Fatalf("Open() error = %v", err)
	}
}

func TestOpenRequiresExplicitCompositionInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		ctx             context.Context
		databaseURL     string
		applicationName string
		want            string
	}{
		{
			name:            "context",
			databaseURL:     "postgres://unused",
			applicationName: "pymes-v3-test",
			want:            "postgres context is required",
		},
		{
			name:            "database URL",
			ctx:             context.Background(),
			applicationName: "pymes-v3-test",
			want:            "postgres database URL is required",
		},
		{
			name:        "application name",
			ctx:         context.Background(),
			databaseURL: "postgres://unused",
			want:        "postgres application name is required",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Open(
				test.ctx,
				test.databaseURL,
				test.applicationName,
			)
			if err == nil || err.Error() != test.want {
				t.Fatalf("Open() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestOpenUsesPlatformPoolPolicy(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	t.Setenv("PYMES_POSTGRES_MIN_CONNS", "0")
	t.Setenv("PYMES_POSTGRES_MAX_CONNS", "2")
	t.Setenv("PYMES_POSTGRES_STATEMENT_TIMEOUT", "3s")

	database, err := Open(
		context.Background(),
		databaseURL,
		"pymes-v3-platform-postgres-test",
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)

	var applicationName, statementTimeout string
	if err := database.Pool().QueryRow(
		context.Background(),
		"SELECT current_setting('application_name'), current_setting('statement_timeout')",
	).Scan(&applicationName, &statementTimeout); err != nil {
		t.Fatal(err)
	}
	if applicationName != "pymes-v3-platform-postgres-test" {
		t.Fatalf("application_name = %q", applicationName)
	}
	if statementTimeout != "3s" {
		t.Fatalf("statement_timeout = %q", statementTimeout)
	}
}
