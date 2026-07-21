package workorders

import (
	"context"

	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

// Hook permite a cada vertical engancharse al ciclo de vida de la work order
// sin duplicar el CRUD base. Cada hook se enruta por wo.AssetType y
// retornar early si no le aplica (no es su segmento).
//
// Las extensiones (auto_repair/workorders_ext, bike_shop/workorders_ext) implementan
// esta interface y se registran en el constructor del módulo base.
type Hook interface {
	// AssetType devuelve el asset_type que esta extensión maneja ("vehicle", "bicycle", ...).
	AssetType() string

	// BeforeCreate corre antes de validar/persistir una OT nueva.
	// Útil para validar el asset_id contra customer_assets o para enriquecer asset_label.
	BeforeCreate(ctx context.Context, wo *domain.WorkOrder) error

	// BeforeUpdate corre antes de persistir cambios sobre una OT existente.
	// Recibe el estado previo (current) y el siguiente (next) ya con los patches aplicados.
	BeforeUpdate(ctx context.Context, current, next *domain.WorkOrder) error

	// AfterStatusChange corre después de un cambio de estado exitoso.
	// Útil para acciones como notificar al cliente, generar reportes, etc.
	AfterStatusChange(ctx context.Context, wo *domain.WorkOrder, prevStatus string)

	// ReadyForPickupMessage devuelve el texto del WhatsApp que se envía cuando la OT
	// pasa a "ready_for_pickup". Cada vertical puede personalizar el wording según
	// su contexto (mencionar la patente, el modelo de bici, etc.).
	// Si devuelve "" el módulo base usa un texto genérico.
	ReadyForPickupMessage(wo *domain.WorkOrder) string
}

// noopHook es un hook que no hace nada. Sirve como default y como base para
// embebido cuando una extensión solo quiere implementar uno o dos métodos.
type noopHook struct {
	assetType string
}

// NewNoopHook crea un hook neutro para un asset_type específico.
func NewNoopHook(assetType string) Hook {
	return &noopHook{assetType: assetType}
}

func (h *noopHook) AssetType() string                                            { return h.assetType }
func (h *noopHook) BeforeCreate(_ context.Context, _ *domain.WorkOrder) error    { return nil }
func (h *noopHook) BeforeUpdate(_ context.Context, _, _ *domain.WorkOrder) error { return nil }
func (h *noopHook) AfterStatusChange(_ context.Context, _ *domain.WorkOrder, _ string) {
}
func (h *noopHook) ReadyForPickupMessage(_ *domain.WorkOrder) string { return "" }

// hookRegistry indexa hooks por asset_type para despacho rápido.
type hookRegistry struct {
	byType map[string]Hook
}

func newHookRegistry(hooks []Hook) *hookRegistry {
	r := &hookRegistry{byType: make(map[string]Hook, len(hooks))}
	for _, h := range hooks {
		if h == nil {
			continue
		}
		r.byType[h.AssetType()] = h
	}
	return r
}

func (r *hookRegistry) lookup(assetType string) Hook {
	if h, ok := r.byType[assetType]; ok {
		return h
	}
	return NewNoopHook(assetType)
}
