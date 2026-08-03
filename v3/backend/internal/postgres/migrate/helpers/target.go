package helpers

import (
	"fmt"
	"net/url"
	"regexp"

	migratemodels "github.com/devpablocristo/pymes/v3/backend/internal/postgres/migrate/models"
)

var migrationIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// PymesTarget returns the immutable Cloud SQL target for an environment.
func PymesTarget(environment string) (migratemodels.Target, error) {
	switch environment {
	case "stg", "prd":
	default:
		return migratemodels.Target{}, fmt.Errorf("migration environment must be stg or prd")
	}
	return migratemodels.Target{
		Database:      "pymes_v3_" + environment,
		Socket:        "/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db",
		SessionRole:   "pymes_v3_migrate_" + environment,
		EffectiveRole: "pymes_v3_owner_" + environment,
	}, nil
}

// ValidateTargetURL rejects any connection outside the immutable migration target.
func ValidateTargetURL(databaseURL string, target migratemodels.Target) error {
	for _, identifier := range []string{
		target.Database,
		target.SessionRole,
		target.EffectiveRole,
	} {
		if !migrationIdentifierPattern.MatchString(identifier) {
			return fmt.Errorf("migration target identifier is invalid")
		}
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("database URL is invalid")
	}
	if parsed.User == nil {
		return fmt.Errorf("database URL differs from the canonical Pymes migration target")
	}
	password, hasPassword := parsed.User.Password()
	query := parsed.Query()
	if parsed.Scheme != "postgres" ||
		parsed.Host != "" ||
		parsed.Path != "/"+target.Database ||
		parsed.User.Username() != target.SessionRole ||
		!hasPassword ||
		password == "" ||
		len(query) != 3 ||
		len(query["host"]) != 1 ||
		query.Get("host") != target.Socket ||
		len(query["sslmode"]) != 1 ||
		query.Get("sslmode") != "disable" ||
		len(query["options"]) != 1 ||
		query.Get("options") != "-c role="+target.EffectiveRole {
		return fmt.Errorf("database URL differs from the canonical Pymes migration target")
	}
	return nil
}

// ValidateTargetIdentity verifies the server-side identity before any DDL.
func ValidateTargetIdentity(
	target migratemodels.Target,
	database string,
	sessionRole string,
	effectiveRole string,
) error {
	if database != target.Database ||
		sessionRole != target.SessionRole ||
		effectiveRole != target.EffectiveRole {
		return fmt.Errorf("database identity differs from the canonical Pymes migration target")
	}
	return nil
}
