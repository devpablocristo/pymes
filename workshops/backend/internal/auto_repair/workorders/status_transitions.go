package workorders

import (
	"errors"
	"fmt"
	"strings"

	"github.com/devpablocristo/core/concurrency/go/fsm"

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

// workOrderStatusSM: Kanban entre columnas operativas; invoiced/cancelled terminales;
// invoiced solo desde subconjunto explícito; cancelación desde cualquier no terminal.
var workOrderStatusSM = newWorkOrderStatusMachine()

func newWorkOrderStatusMachine() *fsm.StringMachine {
	kanbanOpen := []string{
		"received", "diagnosing", "quote_pending", "awaiting_parts",
		"in_progress", "quality_check", "ready_for_pickup", "delivered", "on_hold",
	}
	return fsm.NewBuilder().
		Terminal("invoiced", "cancelled").
		FreeTransitionsAmong(kanbanOpen...).
		AllowFromStatesTo("invoiced", "delivered", "ready_for_pickup", "in_progress", "quality_check", "quote_pending").
		AllowAnyTo("cancelled").
		Build()
}

func validateWorkOrderStatusTransition(fromRaw, toRaw string) error {
	from := normalizeWorkOrderStatus(fromRaw)
	to := normalizeWorkOrderStatus(toRaw)
	err := workOrderStatusSM.Validate(from, to)
	if err == nil {
		return nil
	}
	if errors.Is(err, fsm.ErrTerminal) {
		return fmt.Errorf("work order status is terminal (%s): %w", from, httperrors.ErrConflict)
	}
	if errors.Is(err, fsm.ErrInvalidTransition) {
		return fmt.Errorf("invalid work order status transition from %q to %q: %w", from, to, httperrors.ErrBadInput)
	}
	return err
}
