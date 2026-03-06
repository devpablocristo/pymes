package users

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/users/repository/models"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/users/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/utils"
)

type ResolvedAPIKey struct {
	ID     uuid.UUID
	OrgID  uuid.UUID
	Scopes []string
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByExternalID(externalID string) (domain.User, bool) {
	var m models.UserModel
	if err := r.db.Where("external_id = ? AND deleted_at IS NULL", externalID).First(&m).Error; err != nil {
		return domain.User{}, false
	}
	return userToDomain(m), true
}

func (r *Repository) UpsertUser(externalID, email, name, avatarURL string) domain.User {
	now := time.Now().UTC()
	var m models.UserModel
	result := r.db.Where("external_id = ?", externalID).First(&m)

	if result.Error != nil {
		m = models.UserModel{
			ID:         uuid.New(),
			ExternalID: externalID,
			Email:      email,
			Name:       name,
			AvatarURL:  avatarURL,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		r.db.Create(&m)
	} else {
		r.db.Model(&m).Updates(map[string]any{
			"email":      email,
			"name":       name,
			"avatar_url": avatarURL,
			"deleted_at": nil,
			"updated_at": now,
		})
		m.Email = email
		m.Name = name
		m.AvatarURL = avatarURL
		m.DeletedAt = nil
		m.UpdatedAt = now
	}

	return userToDomain(m)
}

func (r *Repository) SoftDeleteUser(externalID string) bool {
	now := time.Now().UTC()
	result := r.db.Model(&models.UserModel{}).
		Where("external_id = ? AND deleted_at IS NULL", externalID).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	return result.RowsAffected > 0
}

func (r *Repository) AddMembership(orgID, userID uuid.UUID, role string) {
	var m models.OrgMemberModel
	result := r.db.Where("org_id = ? AND user_id = ?", orgID, userID).First(&m)

	if result.Error != nil {
		m = models.OrgMemberModel{
			ID:        uuid.New(),
			OrgID:     orgID,
			UserID:    userID,
			Role:      role,
			CreatedAt: time.Now().UTC(),
		}
		r.db.Create(&m)
	} else {
		r.db.Model(&m).Update("role", role)
	}
}

func (r *Repository) RemoveMembership(orgID, userID uuid.UUID) {
	r.db.Where("org_id = ? AND user_id = ?", orgID, userID).Delete(&models.OrgMemberModel{})
}

func (r *Repository) ListMembers(orgID uuid.UUID) []domain.Member {
	type memberRow struct {
		UserID     uuid.UUID `gorm:"column:user_id"`
		ExternalID string    `gorm:"column:external_id"`
		Email      string    `gorm:"column:email"`
		Name       string    `gorm:"column:name"`
		Role       string    `gorm:"column:role"`
	}

	var rows []memberRow
	r.db.Table("org_members").
		Select("org_members.user_id, users.external_id, users.email, users.name, org_members.role").
		Joins("JOIN users ON users.id = org_members.user_id").
		Where("org_members.org_id = ? AND users.deleted_at IS NULL", orgID).
		Order("LOWER(users.email)").
		Find(&rows)

	result := make([]domain.Member, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.Member{
			UserID:     row.UserID,
			ExternalID: row.ExternalID,
			Email:      row.Email,
			Name:       row.Name,
			Role:       row.Role,
		})
	}
	return result
}

func (r *Repository) CreateAPIKey(orgID uuid.UUID, name, createdBy string, scopes []string, rawKey string) domain.APIKey {
	now := time.Now().UTC()
	hash := utils.SHA256Hex(rawKey)
	prefix := rawKey[:min(len(rawKey), 12)]
	cleanedScopes := cleanScopes(scopes)

	keyID := uuid.New()
	m := models.APIKeyModel{
		ID:        keyID,
		OrgID:     orgID,
		Name:      name,
		KeyHash:   hash,
		KeyPrefix: prefix,
		CreatedBy: createdBy,
		CreatedAt: now,
	}
	r.db.Create(&m)

	for _, scope := range cleanedScopes {
		r.db.Create(&models.APIKeyScopeModel{
			ID:    uuid.New(),
			KeyID: keyID,
			Scope: scope,
		})
	}

	return domain.APIKey{
		ID:        m.ID,
		OrgID:     m.OrgID,
		Name:      m.Name,
		KeyPrefix: m.KeyPrefix,
		Scopes:    cleanedScopes,
		CreatedBy: m.CreatedBy,
		RotatedAt: m.RotatedAt,
		CreatedAt: m.CreatedAt,
	}
}

func (r *Repository) RotateAPIKey(orgID, keyID uuid.UUID, rawKey string) (domain.APIKey, bool) {
	var m models.APIKeyModel
	if err := r.db.Where("id = ? AND org_id = ?", keyID, orgID).First(&m).Error; err != nil {
		return domain.APIKey{}, false
	}

	now := time.Now().UTC()
	hash := utils.SHA256Hex(rawKey)
	prefix := rawKey[:min(len(rawKey), 12)]

	r.db.Model(&m).Updates(map[string]any{
		"key_hash":   hash,
		"key_prefix": prefix,
		"rotated_at": now,
	})
	m.KeyHash = hash
	m.KeyPrefix = prefix
	m.RotatedAt = &now

	scopes := r.loadScopes(keyID)

	return domain.APIKey{
		ID:        m.ID,
		OrgID:     m.OrgID,
		Name:      m.Name,
		KeyPrefix: m.KeyPrefix,
		Scopes:    scopes,
		CreatedBy: m.CreatedBy,
		RotatedAt: m.RotatedAt,
		CreatedAt: m.CreatedAt,
	}, true
}

func (r *Repository) DeleteAPIKey(orgID, keyID uuid.UUID) bool {
	result := r.db.Where("id = ? AND org_id = ?", keyID, orgID).Delete(&models.APIKeyModel{})
	if result.RowsAffected == 0 {
		return false
	}
	r.db.Where("key_id = ?", keyID).Delete(&models.APIKeyScopeModel{})
	return true
}

func (r *Repository) ListAPIKeys(orgID uuid.UUID) []domain.APIKey {
	var keys []models.APIKeyModel
	r.db.Where("org_id = ?", orgID).
		Order("created_at DESC").
		Find(&keys)

	result := make([]domain.APIKey, 0, len(keys))
	for _, k := range keys {
		scopes := r.loadScopes(k.ID)
		result = append(result, domain.APIKey{
			ID:        k.ID,
			OrgID:     k.OrgID,
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			Scopes:    scopes,
			CreatedBy: k.CreatedBy,
			RotatedAt: k.RotatedAt,
			CreatedAt: k.CreatedAt,
		})
	}
	return result
}

func (r *Repository) ResolveAPIKey(raw string) (ResolvedAPIKey, bool) {
	hash := utils.SHA256Hex(raw)
	var m models.APIKeyModel
	if err := r.db.Where("key_hash = ?", hash).First(&m).Error; err != nil {
		return ResolvedAPIKey{}, false
	}

	scopes := r.loadScopes(m.ID)

	return ResolvedAPIKey{
		ID:     m.ID,
		OrgID:  m.OrgID,
		Scopes: scopes,
	}, true
}

func (r *Repository) loadScopes(keyID uuid.UUID) []string {
	var scopeModels []models.APIKeyScopeModel
	r.db.Where("key_id = ?", keyID).Find(&scopeModels)

	scopes := make([]string, 0, len(scopeModels))
	for _, s := range scopeModels {
		scopes = append(scopes, s.Scope)
	}
	sort.Strings(scopes)
	return scopes
}

func userToDomain(m models.UserModel) domain.User {
	return domain.User{
		ID:         m.ID,
		ExternalID: m.ExternalID,
		Email:      m.Email,
		Name:       m.Name,
		AvatarURL:  m.AvatarURL,
		DeletedAt:  m.DeletedAt,
	}
}

func cleanScopes(scopes []string) []string {
	m := make(map[string]struct{})
	res := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := m[s]; ok {
			continue
		}
		m[s] = struct{}{}
		res = append(res, s)
	}
	sort.Strings(res)
	return res
}
