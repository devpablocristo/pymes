package workorders

import (
	"fmt"
	"strings"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

// normalizeWorkOrderStatus mapea valores legacy y devuelve el estado canónico almacenado en API/DB.
func normalizeWorkOrderStatus(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch s {
	case "", "received":
		return "received"
	case "diagnosis", "diagnosing":
		return "diagnosing"
	case "quote_pending":
		return "quote_pending"
	case "awaiting_parts":
		return "awaiting_parts"
	case "in_progress":
		return "in_progress"
	case "quality_check":
		return "quality_check"
	case "ready", "ready_for_pickup":
		return "ready_for_pickup"
	case "delivered":
		return "delivered"
	case "invoiced":
		return "invoiced"
	case "cancelled":
		return "cancelled"
	case "on_hold":
		return "on_hold"
	default:
		return "received"
	}
}

// allowedWorkOrderTransitions define transiciones válidas entre estados canónicos (taller auto_repair).
var allowedWorkOrderTransitions = map[string][]string{
	"received":           {"diagnosing", "on_hold", "cancelled"},
	"diagnosing":         {"quote_pending", "received", "on_hold", "cancelled"},
	"quote_pending":      {"awaiting_parts", "in_progress", "diagnosing", "on_hold", "cancelled"},
	"awaiting_parts":     {"in_progress", "quote_pending", "on_hold", "cancelled"},
	"in_progress":        {"quality_check", "awaiting_parts", "on_hold", "cancelled"},
	"quality_check":      {"ready_for_pickup", "in_progress", "on_hold", "cancelled"},
	"ready_for_pickup":   {"delivered", "in_progress", "on_hold", "cancelled"},
	"delivered":          {"invoiced", "ready_for_pickup", "on_hold", "cancelled"},
	"on_hold":            {"received", "diagnosing", "quote_pending", "awaiting_parts", "in_progress", "quality_check", "ready_for_pickup", "cancelled"},
	"invoiced":           {},
	"cancelled":          {},
}

func validateWorkOrderStatusTransition(fromRaw, toRaw string) error {
	from := normalizeWorkOrderStatus(fromRaw)
	to := normalizeWorkOrderStatus(toRaw)
	if from == to {
		return nil
	}
	if from == "invoiced" || from == "cancelled" {
		return fmt.Errorf("work order status is terminal (%s): %w", from, httperrors.ErrConflict)
	}
	for _, allowed := range allowedWorkOrderTransitions[from] {
		if allowed == to {
			return nil
		}
	}
	// Transición a facturado desde flujo comercial (además de delivered).
	if to == "invoiced" {
		for _, allowed := range []string{"delivered", "ready_for_pickup", "in_progress", "quality_check", "quote_pending"} {
			if from == allowed {
				return nil
			}
		}
	}
	return fmt.Errorf("invalid work order status transition from %q to %q: %w", from, to, httperrors.ErrBadInput)
}
