package workorders

import "github.com/devpablocristo/pymes/workshops/backend/internal/shared/workshops"

// normalizeWorkOrderStatus delega a la FSM compartida.
func normalizeWorkOrderStatus(raw string) string {
	return workshops.NormalizeStatus(raw)
}
