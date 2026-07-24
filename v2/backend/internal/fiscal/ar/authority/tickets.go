package authority

import (
	"context"
	"sync"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
)

// MemoryTickets keeps WSAA credentials process-local, keyed by tenant,
// environment, service, and certificate fingerprint. Tokens never reach logs,
// HTTP responses, or PostgreSQL. A worker restart safely obtains a new TA.
type MemoryTickets struct {
	mu      sync.RWMutex
	tickets map[wsaa.TicketKey]wsaa.AccessTicket
}

func NewMemoryTickets() *MemoryTickets {
	return &MemoryTickets{tickets: make(map[wsaa.TicketKey]wsaa.AccessTicket)}
}

func (repository *MemoryTickets) GetTicket(
	_ context.Context,
	key wsaa.TicketKey,
) (wsaa.AccessTicket, error) {
	repository.mu.RLock()
	defer repository.mu.RUnlock()
	ticket, found := repository.tickets[key]
	if !found {
		return wsaa.AccessTicket{}, fiscal.ErrNotFound
	}
	return ticket, nil
}

func (repository *MemoryTickets) SaveTicket(
	_ context.Context,
	key wsaa.TicketKey,
	ticket wsaa.AccessTicket,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.tickets[key] = ticket
	return nil
}
