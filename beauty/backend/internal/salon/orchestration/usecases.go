package orchestration

import (
	"context"
	"fmt"
	"strings"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type controlPlanePort interface {
	CreateBooking(ctx context.Context, payload map[string]any) (map[string]any, error)
}

type Usecases struct {
	cp controlPlanePort
}

func NewUsecases(cp controlPlanePort) *Usecases {
	return &Usecases{cp: cp}
}

func (u *Usecases) CreateBooking(ctx context.Context, orgID string, payload map[string]any) (map[string]any, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, fmt.Errorf("org_id is required: %w", httperrors.ErrBadInput)
	}
	out := copyMap(payload)
	out["org_id"] = orgID
	return u.cp.CreateBooking(ctx, out)
}

func copyMap(payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		result[key] = value
	}
	return result
}
