// Package seeds aplica SQL de demo opcional para auto_repair (no migración).
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

// Run idempotente (ON CONFLICT en el script). targetOrg debe coincidir con el tenant usado en seeds del core.
func Run(ctx context.Context, db *gorm.DB, logger zerolog.Logger, targetOrg uuid.UUID) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("seeds get sql db: %w", err)
	}
	b, err := sqlFiles.ReadFile("auto_repair_demo.sql")
	if err != nil {
		return fmt.Errorf("seeds read auto_repair_demo.sql: %w", err)
	}
	body := strings.ReplaceAll(string(b), seedtarget.LegacyDemoOrgUUID, targetOrg.String())
	if _, err := sqlDB.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("seeds exec auto_repair_demo: %w", err)
	}
	logger.Info().Msg("workshops auto_repair demo seed applied")
	return nil
}
