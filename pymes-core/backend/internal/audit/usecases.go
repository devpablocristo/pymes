package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/audit/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	Add(in domain.LogInput) domain.Entry
	List(orgID uuid.UUID, limit int) []domain.Entry
	ExportCSV(orgID uuid.UUID) (string, error)
	Verify(orgID uuid.UUID) domain.VerifyResult
}

type Usecases struct {
	repo RepositoryPort
}

func (u *Usecases) Verify(ctx context.Context, orgID string) (domain.VerifyResult, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return domain.VerifyResult{}, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	return u.repo.Verify(id), nil
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return
	}
	u.repo.Add(domain.LogInput{
		OrgID: id,
		Actor: domain.ActorRef{
			Legacy: actor,
			Type:   "user",
			Label:  actor,
		},
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Payload:      payload,
	})
}

func (u *Usecases) LogWithActor(ctx context.Context, in domain.LogInput) {
	_ = ctx
	if in.OrgID == uuid.Nil {
		return
	}
	u.repo.Add(in)
}

func (u *Usecases) List(ctx context.Context, orgID string, limit int) ([]domain.Entry, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	return u.repo.List(id, limit), nil
}

func (u *Usecases) Export(ctx context.Context, orgID, format string) (string, string, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return "", "", fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	switch strings.ToLower(format) {
	case "", "csv":
		content, err := u.repo.ExportCSV(id)
		if err != nil {
			return "", "", err
		}
		return "csv", content, nil
	case "jsonl":
		entries := u.repo.List(id, 0)
		lines := make([]string, 0, len(entries))
		for _, e := range entries {
			b, err := json.Marshal(e)
			if err != nil {
				return "", "", err
			}
			lines = append(lines, string(b))
		}
		return "jsonl", strings.Join(lines, "\n"), nil
	default:
		return "", "", fmt.Errorf("unsupported format %s: %w", format, httperrors.ErrBadInput)
	}
}
