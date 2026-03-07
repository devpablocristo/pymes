package org

import (
	"context"
	"fmt"
	"strings"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/org/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type RepositoryPort interface {
	CreateOrg(name, slug, externalID, actor string) domain.Organization
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, audit: audit}
}

func (u *Usecases) Create(ctx context.Context, name, slug, externalID, actor string) (domain.Organization, error) {
	_ = ctx
	if strings.TrimSpace(name) == "" {
		return domain.Organization{}, fmt.Errorf("name is required: %w", httperrors.ErrBadInput)
	}
	org := u.repo.CreateOrg(name, slug, externalID, actor)
	if u.audit != nil {
		u.audit.Log(ctx, org.ID.String(), actor, "org.created", "org", org.ID.String(), map[string]any{"name": org.Name})
	}
	return org, nil
}
