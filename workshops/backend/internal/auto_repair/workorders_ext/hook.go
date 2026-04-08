// Package workordersext implementa las extensiones de auto_repair sobre el módulo
// base workshops/internal/workorders. Hoy es un hook no-op: instala el patrón base+extensión
// para que validaciones específicas (VIN, kilometraje, integraciones OBD, etc.) puedan
// agregarse acá sin tocar el módulo base.
package workordersext

import (
	"context"
	"fmt"
	"strings"

	workorders "github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

// Hook es la implementación de workorders.Hook para target_type="vehicle".
type Hook struct{}

// New construye un hook nuevo para auto_repair.
func New() workorders.Hook { return &Hook{} }

// TargetType identifica este hook para el registry.
func (h *Hook) TargetType() string { return "vehicle" }

// BeforeCreate corre antes de validar/persistir una OT nueva de auto_repair.
// Asegura que metadata.segment esté seteado para reportes y queries cross-vertical.
func (h *Hook) BeforeCreate(_ context.Context, wo *domain.WorkOrder) error {
	if wo.Metadata == nil {
		wo.Metadata = map[string]any{}
	}
	if _, ok := wo.Metadata["segment"]; !ok {
		wo.Metadata["segment"] = "auto_repair"
	}
	if _, ok := wo.Metadata["vertical"]; !ok {
		wo.Metadata["vertical"] = "workshops"
	}
	return nil
}

// BeforeUpdate corre antes de persistir cambios. Por ahora no hace nada extra.
func (h *Hook) BeforeUpdate(_ context.Context, _, _ *domain.WorkOrder) error { return nil }

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
