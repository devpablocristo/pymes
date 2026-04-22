package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type EmployeeModel struct {
	ID         uuid.UUID      `gorm:"column:id;primaryKey"`
	OrgID      uuid.UUID      `gorm:"column:org_id"`
	FirstName  string         `gorm:"column:first_name"`
	LastName   string         `gorm:"column:last_name"`
	Email      string         `gorm:"column:email"`
	Phone      string         `gorm:"column:phone"`
	Position   string         `gorm:"column:position"`
	Status     string         `gorm:"column:status"`
	HireDate   *time.Time     `gorm:"column:hire_date"`
	EndDate    *time.Time     `gorm:"column:end_date"`
	UserID     *uuid.UUID     `gorm:"column:user_id"`
	Notes      string         `gorm:"column:notes"`
	IsFavorite bool           `gorm:"column:is_favorite"`
	Tags       pq.StringArray `gorm:"column:tags;type:text[]"`
	CreatedBy  string         `gorm:"column:created_by"`
	CreatedAt  time.Time      `gorm:"column:created_at"`
	UpdatedAt  time.Time      `gorm:"column:updated_at"`
	DeletedAt  *time.Time     `gorm:"column:deleted_at"`
}

func (EmployeeModel) TableName() string { return "employees" }
