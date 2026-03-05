package admin

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/admin/repository/models"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/admin/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetTenantSettings(orgID uuid.UUID) domain.TenantSettings {
	var m models.TenantSettingsModel
	if err := r.db.Where("org_id = ?", orgID).First(&m).Error; err != nil {
		return domain.TenantSettings{
			OrgID:      orgID,
			PlanCode:   "starter",
			HardLimits: DefaultHardLimits("starter"),
			UpdatedAt:  time.Now().UTC(),
		}
	}
	return tenantSettingsToDomain(m)
}

func (r *Repository) UpdateTenantSettings(orgID uuid.UUID, plan string, hardLimits map[string]any, actor *string) domain.TenantSettings {
	var m models.TenantSettingsModel
	result := r.db.Where("org_id = ?", orgID).First(&m)

	now := time.Now().UTC()

	if result.Error != nil {
		effectivePlan := plan
		if effectivePlan == "" {
			effectivePlan = "starter"
		}
		m = models.TenantSettingsModel{
			OrgID:     orgID,
			PlanCode:  effectivePlan,
			CreatedAt: now,
		}
	}

	if plan != "" {
		m.PlanCode = plan
	}

	if hardLimits != nil {
		limitsJSON, _ := json.Marshal(hardLimits)
		m.HardLimits = limitsJSON
	} else {
		defaultLimits := DefaultHardLimits(m.PlanCode)
		limitsJSON, _ := json.Marshal(defaultLimits)
		m.HardLimits = limitsJSON
	}

	m.UpdatedBy = actor
	m.UpdatedAt = now

	if result.Error != nil {
		r.db.Create(&m)
	} else {
		r.db.Save(&m)
	}

	return tenantSettingsToDomain(m)
}

func (r *Repository) ListActivity(orgID uuid.UUID, limit int) []domain.ActivityEvent {
	if limit <= 0 {
		limit = 200
	}
	var rows []models.AdminActivityEventModel
	r.db.Where("org_id = ?", orgID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows)

	result := make([]domain.ActivityEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, activityToDomain(row))
	}
	return result
}

func tenantSettingsToDomain(m models.TenantSettingsModel) domain.TenantSettings {
	var limits map[string]any
	if len(m.HardLimits) > 0 {
		_ = json.Unmarshal(m.HardLimits, &limits)
	}
	if limits == nil {
		limits = DefaultHardLimits(m.PlanCode)
	}
	return domain.TenantSettings{
		OrgID:      m.OrgID,
		PlanCode:   m.PlanCode,
		HardLimits: limits,
		UpdatedBy:  m.UpdatedBy,
		UpdatedAt:  m.UpdatedAt,
	}
}

func activityToDomain(m models.AdminActivityEventModel) domain.ActivityEvent {
	var payload map[string]any
	if len(m.Payload) > 0 {
		_ = json.Unmarshal(m.Payload, &payload)
	}
	return domain.ActivityEvent{
		ID:           m.ID,
		OrgID:        m.OrgID,
		Actor:        m.Actor,
		Action:       m.Action,
		ResourceType: m.ResourceType,
		ResourceID:   m.ResourceID,
		Payload:      payload,
		CreatedAt:    m.CreatedAt,
	}
}

func DefaultHardLimits(plan string) map[string]any {
	switch strings.ToLower(plan) {
	case "growth":
		return map[string]any{"users_max": 25, "storage_mb": 5000, "api_calls_rpm": 500}
	case "enterprise":
		return map[string]any{"users_max": "unlimited", "storage_mb": 50000, "api_calls_rpm": 2000}
	default:
		return map[string]any{"users_max": 5, "storage_mb": 500, "api_calls_rpm": 100}
	}
}
