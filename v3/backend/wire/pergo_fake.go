package wire

import (
	"net/http"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	"github.com/devpablocristo/pymes/v3/backend/internal/notifications"
)

func InitializePerGoFake(config config.PerGoFake) http.Handler {
	return notifications.NewPerGoFake(notifications.PerGoFakeConfig{
		APIKey: config.APIKey, WorkspaceID: config.WorkspaceID,
		WebhookURL:    config.WebhookURL,
		WebhookSecret: []byte(config.WebhookSecret),
		Delay:         config.Delay,
	}).Handler()
}
