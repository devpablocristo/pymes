// Package workordersext implementa las extensiones de bike_shop sobre el módulo
// base workshops/internal/workorders. Punto de extensión futuro para validaciones
// específicas (cuadros en lista de robados, alertas de cadena por uso, etc.).
package workordersext

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	bicyclesdomain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/usecases/domain"
	workorders "github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

// Hook es la implementación de workorders.Hook para target_type="bicycle".
type Hook struct {
	assets bicycleLookupPort
}

type bicycleLookupPort interface {
	GetByID(ctx context.Context, orgID, id uuid.UUID) (bicyclesdomain.Bicycle, error)
}

// New construye un hook nuevo para bike_shop.
func New(assets bicycleLookupPort) workorders.Hook { return &Hook{assets: assets} }

// TargetType identifica este hook para el registry.
func (h *Hook) TargetType() string { return "bicycle" }

// BeforeCreate asegura que metadata.segment esté seteado para reportes y queries.
func (h *Hook) BeforeCreate(ctx context.Context, wo *domain.WorkOrder) error {
	if wo.Metadata == nil {
		wo.Metadata = map[string]any{}
	}
	if _, ok := wo.Metadata["segment"]; !ok {
		wo.Metadata["segment"] = "bike_shop"
	}
	if _, ok := wo.Metadata["vertical"]; !ok {
		wo.Metadata["vertical"] = "workshops"
	}
	if err := h.syncBicycle(ctx, wo); err != nil {
		return err
	}
	return nil
}

// BeforeUpdate solo revalida el asset si el target cambia.
func (h *Hook) BeforeUpdate(ctx context.Context, current, next *domain.WorkOrder) error {
	if current.TargetID == next.TargetID {
		return nil
	}
	return h.syncBicycle(ctx, next)
}

// AfterStatusChange punto de extensión futuro.
func (h *Hook) AfterStatusChange(_ context.Context, _ *domain.WorkOrder, _ string) {}

// ReadyForPickupMessage devuelve el texto del WhatsApp para "listo para retirar".
// Menciona el modelo/etiqueta de la bicicleta si está disponible.
func (h *Hook) ReadyForPickupMessage(wo *domain.WorkOrder) string {
	label := strings.TrimSpace(wo.TargetLabel)
	number := strings.TrimSpace(wo.Number)
	if label != "" {
		return fmt.Sprintf("Hola: su bicicleta está lista para retirar. Orden %s · %s. Coordiná la entrega con la bicicletería.", number, label)
	}
	return fmt.Sprintf("Hola: su bicicleta está lista para retirar. Orden %s. Coordiná la entrega con la bicicletería.", number)
}

func (h *Hook) syncBicycle(ctx context.Context, wo *domain.WorkOrder) error {
	if h.assets == nil {
		return nil
	}
	asset, err := h.assets.GetByID(ctx, wo.OrgID, wo.TargetID)
	if err != nil {
		if errors.Is(err, httperrors.ErrNotFound) {
			return fmt.Errorf("target_id is invalid: %w", httperrors.ErrBadInput)
		}
		return err
	}
	if strings.TrimSpace(wo.TargetLabel) == "" {
		wo.TargetLabel = asset.DisplayLabel()
	}
	if wo.CustomerID == nil && asset.CustomerID != nil {
		value := *asset.CustomerID
		wo.CustomerID = &value
	}
	if strings.TrimSpace(wo.CustomerName) == "" {
		wo.CustomerName = asset.CustomerName
	}
	return nil
}
