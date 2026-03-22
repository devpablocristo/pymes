package pymescore

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SendInternalWhatsAppText llama al endpoint interno de pymes-core (auth por token de servicio).
func (c *Client) SendInternalWhatsAppText(ctx context.Context, orgID string, partyID uuid.UUID, body string) error {
	if c == nil || c.Client == nil {
		return fmt.Errorf("pymes-core client not configured")
	}
	_, err := c.Post(ctx, "/v1/internal/v1/whatsapp/send-text", orgID, map[string]string{
		"party_id": partyID.String(),
		"body":     body,
	})
	if err != nil {
		return fmt.Errorf("whatsapp send: %w", err)
	}
	return nil
}
