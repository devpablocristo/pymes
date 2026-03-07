package wire

import (
	"sort"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pkgs/go-pkg/auth"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/utils"
)

type apiKeyResolver struct {
	db *gorm.DB
}

type apiKeyModel struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID uuid.UUID `gorm:"type:uuid;index;not null"`
}

func (apiKeyModel) TableName() string { return "org_api_keys" }

type apiKeyScopeModel struct {
	KeyID uuid.UUID `gorm:"type:uuid;index;not null"`
	Scope string    `gorm:"not null"`
}

func (apiKeyScopeModel) TableName() string { return "org_api_key_scopes" }

func newAPIKeyResolver(db *gorm.DB) auth.APIKeyResolver {
	return &apiKeyResolver{db: db}
}

func (r *apiKeyResolver) ResolveAPIKey(raw string) (auth.ResolvedKey, bool) {
	hash := utils.SHA256Hex(raw)
	var key apiKeyModel
	if err := r.db.Where("key_hash = ?", hash).First(&key).Error; err != nil {
		return auth.ResolvedKey{}, false
	}

	var scopeModels []apiKeyScopeModel
	r.db.Where("key_id = ?", key.ID).Find(&scopeModels)
	scopes := make([]string, 0, len(scopeModels))
	for _, scope := range scopeModels {
		scopes = append(scopes, scope.Scope)
	}
	sort.Strings(scopes)

	return auth.ResolvedKey{
		ID:     key.ID,
		OrgID:  key.OrgID,
		Scopes: scopes,
	}, true
}
