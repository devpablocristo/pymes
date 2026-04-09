package models

import (
	"time"

	"github.com/google/uuid"
)

type CandidateModel struct {
	ID              uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	OrgID           uuid.UUID  `gorm:"column:org_id;type:uuid;not null"`
	Kind            string     `gorm:"column:kind;not null"`
	EventType       string     `gorm:"column:event_type;not null"`
	EntityType      string     `gorm:"column:entity_type;not null"`
	EntityID        string     `gorm:"column:entity_id;not null"`
	Fingerprint     string     `gorm:"column:fingerprint;not null"`
	Severity        string     `gorm:"column:severity;not null"`
	Status          string     `gorm:"column:status;not null"`
	Title           string     `gorm:"column:title;not null"`
	Body            string     `gorm:"column:body;not null"`
	EvidenceJSON    []byte     `gorm:"column:evidence_json;type:jsonb;not null"`
	OccurrenceCount int        `gorm:"column:occurrence_count;not null"`
	FirstSeenAt     time.Time  `gorm:"column:first_seen_at;not null"`
	LastSeenAt      time.Time  `gorm:"column:last_seen_at;not null"`
	FirstNotifiedAt *time.Time `gorm:"column:first_notified_at"`
	LastNotifiedAt  *time.Time `gorm:"column:last_notified_at"`
	ResolvedAt      *time.Time `gorm:"column:resolved_at"`
	LastActor       string     `gorm:"column:last_actor;not null"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null"`
}

func (CandidateModel) TableName() string {
	return "pymes_business_insight_candidates"
}
