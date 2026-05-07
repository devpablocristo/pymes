package domain

import (
	"time"

	"github.com/google/uuid"
)

type Specialty struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Code        string
	Name        string
	Description string
	IsActive    bool
	IsFavorite  bool
	Tags        []string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}
