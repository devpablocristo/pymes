// Package status provides FSM application-layer helpers. As of Ola B3 the
// canonical implementation lives in platform/concurrency/go/fsm.MapDomainError;
// this file keeps a thin wrapper so existing call sites (~12 in pymes-core)
// don't need to update their imports immediately.
//
// New code should prefer:
//
//	import "github.com/devpablocristo/platform/concurrency/go/fsm"
//	err = fsm.MapDomainError(current, next, err)
package status

import (
	"github.com/devpablocristo/platform/concurrency/go/fsm"
)

// MapFSMError delegates to platform/concurrency/go/fsm.MapDomainError.
func MapFSMError(current, next string, err error) error {
	return fsm.MapDomainError(current, next, err)
}
