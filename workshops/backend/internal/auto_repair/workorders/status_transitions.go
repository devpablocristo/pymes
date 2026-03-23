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

// isWorkOrderKanbanOpenStatus son columnas del tablero operativo (movimiento libre estilo Trello/Kanban).
func isWorkOrderKanbanOpenStatus(s string) bool {
	switch normalizeWorkOrderStatus(s) {
	case "received", "diagnosing", "quote_pending", "awaiting_parts",
		"in_progress", "quality_check", "ready_for_pickup", "delivered", "on_hold":
		return true
	default:
		return false
	}
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
	// Kanban: cualquier salto entre columnas operativas (arrastre en UI).
	if isWorkOrderKanbanOpenStatus(from) && isWorkOrderKanbanOpenStatus(to) {
		return nil
	}
	if to == "cancelled" {
		return nil
	}
	if to == "invoiced" {
		for _, allowed := range []string{"delivered", "ready_for_pickup", "in_progress", "quality_check", "quote_pending"} {
			if from == allowed {
				return nil
			}
		}
		return fmt.Errorf("invalid work order status transition from %q to %q: %w", from, to, httperrors.ErrBadInput)
	}
	return fmt.Errorf("invalid work order status transition from %q to %q: %w", from, to, httperrors.ErrBadInput)
}
