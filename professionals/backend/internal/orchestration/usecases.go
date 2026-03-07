package orchestration

import (
	"context"
	"fmt"
	"strings"

	httperrors "github.com/devpablocristo/pymes/pkgs/go-pkg/httperrors"
)

type controlPlanePort interface {
	CreateAppointment(ctx context.Context, payload map[string]any) (map[string]any, error)
	CreateQuote(ctx context.Context, payload map[string]any) (map[string]any, error)
	CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error)
}

type Usecases struct {
	cp controlPlanePort
}

func NewUsecases(cp controlPlanePort) *Usecases {
	return &Usecases{cp: cp}
}

func (u *Usecases) CreateAppointment(ctx context.Context, orgID string, payload map[string]any) (map[string]any, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, fmt.Errorf("org_id is required: %w", httperrors.ErrBadInput)
	}
	return u.cp.CreateAppointment(ctx, withOrgID(orgID, payload))
}

func (u *Usecases) CreateQuote(ctx context.Context, orgID string, payload map[string]any) (map[string]any, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, fmt.Errorf("org_id is required: %w", httperrors.ErrBadInput)
	}
	return u.cp.CreateQuote(ctx, withOrgID(orgID, payload))
}

func (u *Usecases) CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error) {
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(saleID) == "" {
		return nil, fmt.Errorf("org_id and sale_id are required: %w", httperrors.ErrBadInput)
	}
	return u.cp.CreateSalePaymentLink(ctx, orgID, saleID)
}

func withOrgID(orgID string, payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		out[key] = value
	}
	out["org_id"] = orgID
	return out
}
