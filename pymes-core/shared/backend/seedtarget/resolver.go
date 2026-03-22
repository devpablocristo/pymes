// Package seedtarget resuelve el UUID de org para seeds de demo (local fijo o Clerk external_id).
package seedtarget

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LegacyDemoOrgUUID es la org fija histórica de seeds sin Clerk.
const LegacyDemoOrgUUID = "00000000-0000-0000-0000-000000000001"

// ResolveDemoOrgUUID devuelve el tenant donde aplicar seeds.
// Si externalID está vacío → org local fija (Compose / API key demo).
// Si externalID es no vacío (ej. org_2abc de Clerk) → busca orgs.external_id; la fila debe existir (primer login o webhook).
func ResolveDemoOrgUUID(ctx context.Context, db *gorm.DB, externalID string) (uuid.UUID, error) {
	ext := strings.TrimSpace(externalID)
	if ext == "" {
		return uuid.MustParse(LegacyDemoOrgUUID), nil
	}
	var idStr string
	if err := db.WithContext(ctx).Raw("SELECT id::text FROM orgs WHERE external_id = ?", ext).Scan(&idStr).Error; err != nil {
		return uuid.Nil, fmt.Errorf("seed target org lookup: %w", err)
	}
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		return uuid.Nil, fmt.Errorf(
			"seed target org: no row with external_id=%q — create the org first (Clerk sign-in or webhook), then restart with PYMES_SEED_DEMO_ORG_EXTERNAL_ID",
			ext,
		)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("seed target org: invalid uuid %q: %w", idStr, err)
	}
	return id, nil
}
