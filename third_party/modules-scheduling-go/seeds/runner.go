package seeds

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

//go:embed *.sql
var embedded embed.FS

func applyOrgID(sql string, orgID uuid.UUID) string {
	return strings.ReplaceAll(sql, "__SEED_ORG_ID__", orgID.String())
}

// demoFiles is the ordered list of seed SQL files applied by RunDemo. The
// order matters: 0002 references resources created by 0001.
var demoFiles = []string{
	"0001_demo.sql",
	"0002_catchall_service.sql",
	"0003_demo_bookings.sql",
}

func RunDemo(ctx context.Context, db *gorm.DB, orgID uuid.UUID) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("scheduling seeds get sql db: %w", err)
	}
	for _, name := range demoFiles {
		body, err := embedded.ReadFile(name)
		if err != nil {
			return fmt.Errorf("scheduling seeds read %s: %w", name, err)
		}
		if _, err := sqlDB.ExecContext(ctx, applyOrgID(string(body), orgID)); err != nil {
			return fmt.Errorf("scheduling seeds exec %s: %w", name, err)
		}
	}
	return nil
}
