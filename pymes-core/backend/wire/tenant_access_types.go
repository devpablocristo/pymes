package wire

import "time"

type tenantAPIKeyPrincipal struct {
	TenantID string
	Scopes   []string
}

type tenantAPIKeyDTO struct {
	ID        string
	TenantID  string
	Name      string
	Scopes    []string
	CreatedAt time.Time
}

type createdTenantAPIKey struct {
	APIKey tenantAPIKeyDTO
	Secret string
}

type rotatedTenantAPIKey = createdTenantAPIKey

type tenantUserDTO struct {
	ID         string
	ExternalID string
	Email      string
	Name       string
	AvatarURL  *string
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type tenantMemberDTO struct {
	ID       string        `json:"id"`
	TenantID string        `json:"tenant_id"`
	UserID   string        `json:"user_id"`
	Role     string        `json:"role"`
	JoinedAt time.Time     `json:"joined_at"`
	User     tenantUserDTO `json:"user"`
}
