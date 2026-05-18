// Package status provee helpers de capa de aplicación para flujos de estado
// (FSM). Su responsabilidad es traducir errores sentinel de
// core/concurrency/go/fsm a domainerr para que la capa HTTP los mapee
// automáticamente vía httperrors.Respond.
//
// Este paquete NO depende de Gin, RBAC, GORM ni HTTP. Lo invocan los usecases
// tras `<dom>StateMachine.Validate(current, next)`.
package status

import (
	"errors"
	"fmt"

	"github.com/devpablocristo/platform/concurrency/go/fsm"
	"github.com/devpablocristo/platform/errors/go/domainerr"
)

// MapFSMError envuelve los sentinels de fsm en domainerr.Conflict para que
// httperrors.Respond los mapee a 409 con un mensaje legible. Si el error no es
// un sentinel conocido, lo devuelve sin tocar.
//
// Uso típico desde un usecase:
//
//	if err := saleStateMachine.Validate(current.Status, next); err != nil {
//	    return saledomain.Sale{}, status.MapFSMError(current.Status, next, err)
//	}
func MapFSMError(current, next string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, fsm.ErrTerminal) {
		return domainerr.Newf(domainerr.KindConflict, "status %q is terminal", current)
	}
	if errors.Is(err, fsm.ErrInvalidTransition) {
		return domainerr.Newf(domainerr.KindConflict, "status transition not allowed: %s -> %s", current, next)
	}
	return fmt.Errorf("fsm validation: %w", err)
}
