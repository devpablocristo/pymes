package dto

import identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"

type SessionOrganization struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

type CurrentSession struct {
	ActorID      string              `json:"actor_id"`
	Organization SessionOrganization `json:"organization"`
	Role         string              `json:"role"`
	Permissions  []string            `json:"permissions"`
}

func CurrentSessionFromPrincipal(value identitydomain.Principal) CurrentSession {
	return CurrentSession{
		ActorID: value.ActorID,
		Organization: SessionOrganization{
			ID:     value.OrganizationID,
			Name:   value.OrganizationName,
			Slug:   value.OrganizationSlug,
			Status: value.OrganizationStatus,
		},
		Role:        string(value.Role),
		Permissions: append([]string(nil), value.Permissions...),
	}
}
