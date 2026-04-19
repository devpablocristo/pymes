// Package workordersext implementa las extensiones de auto_repair sobre el módulo
// base workshops/internal/workorders. Hoy es un hook no-op: instala el patrón base+extensión
// para que validaciones específicas (VIN, kilometraje, integraciones OBD, etc.) puedan
// agregarse acá sin tocar el módulo base.
package workordersext

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	vehiclesdomain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/usecases/domain"
	workorders "github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

// Hook es la implementación de workorders.Hook para target_type="vehicle".
type Hook struct {
	assets vehicleLookupPort
}

type vehicleLookupPort interface {
	GetByID(ctx context.Context, orgID, id uuid.UUID) (vehiclesdomain.Vehicle, error)
}

// New construye un hook nuevo para auto_repair.
func New(assets vehicleLookupPort) workorders.Hook { return &Hook{assets: assets} }

// TargetType identifica este hook para el registry.
func (h *Hook) TargetType() string { return "vehicle" }

// BeforeCreate corre antes de validar/persistir una OT nueva de auto_repair.
// Asegura que metadata.segment esté seteado para reportes y queries cross-vertical.
func (h *Hook) BeforeCreate(ctx context.Context, wo *domain.WorkOrder) error {
	if wo.Metadata == nil {
		wo.Metadata = map[string]any{}
	}
	if _, ok := wo.Metadata["segment"]; !ok {
		wo.Metadata["segment"] = "auto_repair"
	}
	if _, ok := wo.Metadata["vertical"]; !ok {
		wo.Metadata["vertical"] = "workshops"
	}
	if err := h.syncVehicle(ctx, wo); err != nil {
		return err
	}
	return nil
}

// BeforeUpdate solo revalida el asset si el target cambia.
func (h *Hook) BeforeUpdate(ctx context.Context, current, next *domain.WorkOrder) error {
	if current.TargetID == next.TargetID {
		return nil
	}
	return h.syncVehicle(ctx, next)
}

// AfterStatusChange corre después de un cambio de estado. Punto de extensión futuro
// para reportes / notificaciones específicas de talleres mecánicos.
func (h *Hook) AfterStatusChange(_ context.Context, _ *domain.WorkOrder, _ string) {}

// ReadyForPickupMessage devuelve el texto del WhatsApp para "listo para retirar".
// Menciona la patente del vehículo si está disponible.
func (h *Hook) ReadyForPickupMessage(wo *domain.WorkOrder) string {
	plate := strings.TrimSpace(wo.TargetLabel)
	number := strings.TrimSpace(wo.Number)
	if plate != "" {
		return fmt.Sprintf("Hola: su vehículo está listo para retirar. Orden %s · Patente %s. Coordiná la entrega con el taller.", number, plate)
	}
	return fmt.Sprintf("Hola: su vehículo está listo para retirar. Orden %s. Coordiná la entrega con el taller.", number)
}

func (h *Hook) syncVehicle(ctx context.Context, wo *domain.WorkOrder) error {
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
		wo.TargetLabel = asset.LicensePlate
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
