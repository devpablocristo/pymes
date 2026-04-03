// Package seeds aplica SQL de demo opcional para workshops (no migración).
package seeds

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/seedtarget"
)

//go:embed *.sql
var sqlFiles embed.FS

var seedFiles = []struct {
	file string
	name string
}{
	{"auto_repair_demo.sql", "auto_repair"},
	{"bike_shop_demo.sql", "bike_shop"},
}

// Run idempotente (ON CONFLICT en el script). targetOrg debe coincidir con el tenant usado en seeds del core.
func Run(ctx context.Context, db *gorm.DB, logger zerolog.Logger, targetOrg uuid.UUID) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("seeds get sql db: %w", err)
	}
	for _, sf := range seedFiles {
		b, err := sqlFiles.ReadFile(sf.file)
		if err != nil {
			return fmt.Errorf("seeds read %s: %w", sf.file, err)
		}
		body := strings.ReplaceAll(string(b), seedtarget.LegacyDemoOrgUUID, targetOrg.String())
		if _, err := sqlDB.ExecContext(ctx, body); err != nil {
			return fmt.Errorf("seeds exec %s: %w", sf.name, err)
		}
		logger.Info().Msgf("workshops %s demo seed applied", sf.name)
	}
	return nil
}
