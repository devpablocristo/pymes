package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID         uuid.UUID  `json:"id"`
	ExternalID string     `json:"external_id"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	AvatarURL  string     `json:"avatar_url"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type APIKey struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	Scopes    []string   `json:"scopes"`
	CreatedBy string     `json:"created_by,omitempty"`
	RotatedAt *time.Time `json:"rotated_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Member struct {
	UserID     uuid.UUID `json:"user_id"`
	ExternalID string    `json:"external_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
}
