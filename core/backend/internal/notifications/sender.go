package notifications

import (
	"context"
	"log/slog"
	"strings"

	corenotifications "github.com/devpablocristo/core/notifications/go"
	"github.com/rs/zerolog"
)

type emailSenderAdapter struct {
	sender corenotifications.EmailSender
}

func (s *emailSenderAdapter) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	return s.sender.Send(ctx, corenotifications.EmailMessage{
		To:       to,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	})
}

func NewNoopSender(_ zerolog.Logger) EmailSender {
	return &emailSenderAdapter{sender: corenotifications.NewNoopEmailSender(slog.Default())}
}

func NewEmailSender(backend string, _ zerolog.Logger) (EmailSender, error) {
	config := corenotifications.EmailConfigFromEnv("")
	if trimmed := strings.TrimSpace(backend); trimmed != "" {
		config.Backend = trimmed
	}

	sender, err := corenotifications.NewEmailSender(context.Background(), config, slog.Default())
	if err != nil {
		return nil, err
	}
	return &emailSenderAdapter{sender: sender}, nil
}
