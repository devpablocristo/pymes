// Package seeds aplica SQL de demo opcional tras las migraciones (no versionado como migración).
package seeds

import (
	"context"
	"embed"
	"fmt"
	"strings"

	schedulingseeds "github.com/devpablocristo/modules/scheduling/go/seeds"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/seedtarget"
)

//go:embed *.sql
var embedded embed.FS

// Params define el tenant y si la org ya existe (Clerk) o se crea la local fija.
type Params struct {
	TargetOrgUUID uuid.UUID
	// ClerkMode: omitir 01_local_org.sql; aplicar 01_clerk_prereqs.sql (tenant_settings + API key demo).
	ClerkMode bool
}

func applySeedOrgID(sql string, orgID uuid.UUID) string {
	s := strings.ReplaceAll(sql, "__SEED_ORG_ID__", orgID.String())
	s = strings.ReplaceAll(s, seedtarget.LegacyDemoOrgUUID, orgID.String())
	return s
}

// Orden fijo: org/API key → negocio → RBAC → módulos transversales.
var legacyFileOrder = []string{
	"01_local_org.sql",
	"02_core_business.sql",
	"03_rbac.sql",
	"04_transversal_modules_demo.sql",
	"05_in_app_notifications_demo.sql",
}

var clerkFileOrder = []string{
	"01_clerk_prereqs.sql",
	"02_core_business.sql",
	"03_rbac.sql",
	"04_transversal_modules_demo.sql",
	"05_in_app_notifications_demo.sql",
}

// Run ejecuta los scripts en orden. Idempotente vía ON CONFLICT en el SQL.
func Run(ctx context.Context, db *gorm.DB, logger zerolog.Logger, p Params) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("seeds get sql db: %w", err)
	}
	order := legacyFileOrder
	if p.ClerkMode {
		order = clerkFileOrder
	}
	for _, name := range order {
		b, err := embedded.ReadFile(name)
		if err != nil {
			return fmt.Errorf("seeds read %s: %w", name, err)
		}
		body := applySeedOrgID(string(b), p.TargetOrgUUID)
		if _, err := sqlDB.ExecContext(ctx, body); err != nil {
			return fmt.Errorf("seeds exec %s: %w", name, err)
		}
		logger.Info().Str("seed_file", name).Msg("pymes core demo seed applied")
	}
	if err := schedulingseeds.RunDemo(ctx, db, p.TargetOrgUUID); err != nil {
		return err
	}
	logger.Info().Msg("scheduling module demo seed applied")
	return nil
}
