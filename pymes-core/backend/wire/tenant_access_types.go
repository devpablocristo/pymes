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
	ID         string     `json:"id"`
	ExternalID string     `json:"external_id"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	GivenName  string     `json:"given_name,omitempty"`
	FamilyName string     `json:"family_name,omitempty"`
	AvatarURL  *string    `json:"avatar_url,omitempty"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type tenantMemberDTO struct {
	ID       string        `json:"id"`
	TenantID string        `json:"tenant_id"`
	UserID   string        `json:"user_id"`
	Role     string        `json:"role"`
	Status   string        `json:"status"`
	JoinedAt time.Time     `json:"joined_at"`
	User     tenantUserDTO `json:"user"`
}
