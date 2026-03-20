package domain

import (
	"time"

	"github.com/google/uuid"
)

type Permission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

type Role struct {
	ID          uuid.UUID    `json:"id"`
	OrgID       uuid.UUID    `json:"org_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `json:"is_system"`
	Permissions []Permission `json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type UserRole struct {
	UserID     uuid.UUID `json:"user_id"`
	OrgID      uuid.UUID `json:"org_id"`
	RoleID     uuid.UUID `json:"role_id"`
	AssignedBy string    `json:"assigned_by"`
	AssignedAt time.Time `json:"assigned_at"`
}
