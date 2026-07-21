package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type ExamModel struct {
	ID              uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	OrgID        uuid.UUID      `gorm:"column:org_id;type:uuid;index"`
	PatientName     string         `gorm:"column:patient_name"`
	PatientDocument string         `gorm:"column:patient_document"`
	EmployerName    string         `gorm:"column:employer_name"`
	ClientName      string         `gorm:"column:client_name"`
	PaymentMethod   string         `gorm:"column:payment_method"`
	ExamType        string         `gorm:"column:exam_type"`
	Status          string         `gorm:"column:status"`
	ScheduledAt     *time.Time     `gorm:"column:scheduled_at"`
	CompletedAt     *time.Time     `gorm:"column:completed_at"`
	Result          string         `gorm:"column:result"`
	Notes           string         `gorm:"column:notes"`
	IsFavorite      bool           `gorm:"column:is_favorite"`
	Tags            pq.StringArray `gorm:"column:tags;type:text[]"`
	ImageURLs       pq.StringArray `gorm:"column:image_urls;type:text[]"`
	CreatedBy       string         `gorm:"column:created_by"`
	UpdatedBy       string         `gorm:"column:updated_by"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (ExamModel) TableName() string {
	return "medical.occupational_health_exams"
}
