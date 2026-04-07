// Package workshops contiene primitivas compartidas entre subdominios del vertical workshops.
package workshops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/devpablocristo/core/concurrency/go/fsm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

// WorkOrderFSM define las transiciones válidas para órdenes de trabajo.
// Los 9 estados operativos permiten transición libre (kanban); invoiced/cancelled son terminales.
var WorkOrderFSM = fsm.NewBuilder().
	Terminal("invoiced", "cancelled").
	FreeTransitionsAmong(
		"received", "diagnosing", "quote_pending", "awaiting_parts",
		"in_progress", "quality_check", "ready_for_pickup", "delivered", "on_hold",
	).
	AllowFromStatesTo("invoiced", "delivered", "ready_for_pickup", "in_progress", "quality_check", "quote_pending").
	AllowAnyTo("cancelled").
	Build()

// NormalizeStatus mapea valores legacy y devuelve el estado canónico.
func NormalizeStatus(raw string) string {
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

// ValidateStatusTransition valida y devuelve error HTTP apropiado.
func ValidateStatusTransition(fromRaw, toRaw string) error {
	from := NormalizeStatus(fromRaw)
	to := NormalizeStatus(toRaw)
	err := WorkOrderFSM.Validate(from, to)
	if err == nil {
		return nil
	}
	if errors.Is(err, fsm.ErrTerminal) {
		return fmt.Errorf("work order status is terminal (%s): %w", from, httperrors.ErrConflict)
	}
	if errors.Is(err, fsm.ErrInvalidTransition) {
		return fmt.Errorf("invalid status transition from %q to %q: %w", from, to, httperrors.ErrBadInput)
	}
	return err
}
