package procurement

import (
	"context"

	"github.com/google/uuid"
)

type webhookPort interface {
	Enqueue(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error
}

// Option configura usecases (webhooks, etc.).
type Option func(*Usecases)

func WithWebhooks(w webhookPort) Option {
	return func(u *Usecases) {
		u.webhooks = w
	}
}
