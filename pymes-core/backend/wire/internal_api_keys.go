package wire

import (
	"sort"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/users"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/utils"
)

type internalAPIKeyResolver struct {
	db *gorm.DB
}

func newInternalAPIKeyResolver(db *gorm.DB) *internalAPIKeyResolver {
	return &internalAPIKeyResolver{db: db}
}

func (r *internalAPIKeyResolver) ResolveAPIKey(raw string) (users.ResolvedAPIKey, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || r.db == nil {
		return users.ResolvedAPIKey{}, false
	}
	hash := utils.SHA256Hex(raw)
	type row struct {
		ID    uuid.UUID `gorm:"column:id"`
		OrgID uuid.UUID `gorm:"column:org_id"`
	}
	var kr row
	if err := r.db.Table("org_api_keys").Select("id", "org_id").Where("api_key_hash = ?", hash).Take(&kr).Error; err != nil {
		return users.ResolvedAPIKey{}, false
	}
	var scopes []string
	_ = r.db.Table("org_api_key_scopes").Where("api_key_id = ?", kr.ID).Order("scope").Pluck("scope", &scopes)
	sort.Strings(scopes)
	return users.ResolvedAPIKey{ID: kr.ID, OrgID: kr.OrgID, Scopes: scopes}, true
}
