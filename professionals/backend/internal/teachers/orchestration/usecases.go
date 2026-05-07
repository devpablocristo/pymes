package orchestration

import (
	"context"
	"fmt"
	"strings"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type controlPlanePort interface {
	CreateBooking(ctx context.Context, payload map[string]any) (map[string]any, error)
	CreateQuote(ctx context.Context, payload map[string]any) (map[string]any, error)
	CreateSalePaymentLink(ctx context.Context, tenantID, saleID string) (map[string]any, error)
	GetBusinessInfo(ctx context.Context, orgRef string) (map[string]any, error)
}

type Usecases struct {
	cp controlPlanePort
}

func NewUsecases(cp controlPlanePort) *Usecases {
	return &Usecases{cp: cp}
}

func (u *Usecases) CreateBooking(ctx context.Context, tenantID string, payload map[string]any) (map[string]any, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("tenant_id is required: %w", httperrors.ErrBadInput)
	}
	return u.cp.CreateBooking(ctx, withOrgID(tenantID, payload))
}

func (u *Usecases) CreateQuote(ctx context.Context, tenantID string, payload map[string]any) (map[string]any, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("tenant_id is required: %w", httperrors.ErrBadInput)
	}
	return u.cp.CreateQuote(ctx, withOrgID(tenantID, payload))
}

func (u *Usecases) CreateSalePaymentLink(ctx context.Context, tenantID, saleID string) (map[string]any, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(saleID) == "" {
		return nil, fmt.Errorf("tenant_id and sale_id are required: %w", httperrors.ErrBadInput)
	}
	return u.cp.CreateSalePaymentLink(ctx, tenantID, saleID)
}

func (u *Usecases) GetPublicPreviewBootstrap(ctx context.Context, tenantID string) (map[string]any, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("tenant_id is required: %w", httperrors.ErrBadInput)
	}
	return u.cp.GetBusinessInfo(ctx, tenantID)
}

func withOrgID(tenantID string, payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		out[key] = value
	}
	out["tenant_id"] = tenantID
	return out
}
