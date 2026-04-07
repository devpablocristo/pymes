// Package workordersext implementa las extensiones de bike_shop sobre el módulo
// base workshops/internal/workorders. Punto de extensión futuro para validaciones
// específicas (cuadros en lista de robados, alertas de cadena por uso, etc.).
package workordersext

import (
	"context"
	"fmt"
	"strings"

	workorders "github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

// Hook es la implementación de workorders.Hook para target_type="bicycle".
type Hook struct{}

// New construye un hook nuevo para bike_shop.
func New() workorders.Hook { return &Hook{} }

// TargetType identifica este hook para el registry.
func (h *Hook) TargetType() string { return "bicycle" }

// BeforeCreate asegura que metadata.segment esté seteado para reportes y queries.
func (h *Hook) BeforeCreate(_ context.Context, wo *domain.WorkOrder) error {
	if wo.Metadata == nil {
		wo.Metadata = map[string]any{}
	}
	if _, ok := wo.Metadata["segment"]; !ok {
		wo.Metadata["segment"] = "bike_shop"
	}
	if _, ok := wo.Metadata["vertical"]; !ok {
		wo.Metadata["vertical"] = "workshops"
	}
	return nil
}

// BeforeUpdate punto de extensión futuro.
func (h *Hook) BeforeUpdate(_ context.Context, _, _ *domain.WorkOrder) error { return nil }

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
