package verticalaudit

import (
	"context"

	"github.com/rs/zerolog"
)

// Logger is a lightweight audit implementation for vertical services.
type Logger struct {
	logger zerolog.Logger
}

func NewLogger(logger zerolog.Logger) *Logger {
	return &Logger{logger: logger}
}

func (a *Logger) Log(_ context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any) {
	if a == nil {
		return
	}
	a.logger.Info().
		Str("org_id", orgID).
		Str("actor", actor).
		Str("action", action).
		Str("resource_type", resourceType).
		Str("resource_id", resourceID).
		Any("payload", payload).
		Msg("audit")
}
