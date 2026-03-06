package service_links

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	domain "github.com/devpablocristo/pymes/professionals/backend/internal/service_links/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/professionals/backend/internal/shared/httperrors"
)

type RepositoryPort interface {
	ListByProfile(ctx context.Context, orgID, profileID uuid.UUID) ([]domain.ServiceLink, error)
	ReplaceForProfile(ctx context.Context, orgID, profileID uuid.UUID, links []domain.ServiceLink) ([]domain.ServiceLink, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.ServiceLink, error)
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

func (u *Usecases) ListByProfile(ctx context.Context, orgID, profileID uuid.UUID) ([]domain.ServiceLink, error) {
	return u.repo.ListByProfile(ctx, orgID, profileID)
}

func (u *Usecases) ReplaceForProfile(ctx context.Context, orgID, profileID uuid.UUID, links []domain.ServiceLink, actor string) ([]domain.ServiceLink, error) {
	for i, link := range links {
		if link.ProductID == uuid.Nil {
			return nil, fmt.Errorf("product_id is required for link at position %d: %w", i, httperrors.ErrBadInput)
		}
		links[i].PublicDescription = strings.TrimSpace(link.PublicDescription)
		if links[i].Metadata == nil {
			links[i].Metadata = map[string]any{}
		}
	}

	out, err := u.repo.ReplaceForProfile(ctx, orgID, profileID, links)
	if err != nil {
		return nil, err
	}

	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "service_links.replaced", "professional_profile", profileID.String(), map[string]any{"count": len(links)})
	}
	return out, nil
}

func (u *Usecases) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.ServiceLink, error) {
	return u.repo.ListByOrg(ctx, orgID)
}
