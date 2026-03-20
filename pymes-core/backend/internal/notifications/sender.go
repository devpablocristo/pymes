package notifications

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/rs/zerolog"
)

type NoopSender struct {
	logger zerolog.Logger
}

func NewNoopSender(logger zerolog.Logger) *NoopSender {
	return &NoopSender{logger: logger}
}

func (s *NoopSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	_ = ctx
	_ = htmlBody
	_ = textBody
	s.logger.Info().Str("to", to).Str("subject", subject).Msg("noop email sender")
	return nil
}

type SMTPSender struct {
	host     string
	port     int
	user     string
	password string
	from     string
}

func NewSMTPSender(host string, port int, user, password, from string) *SMTPSender {
	return &SMTPSender{host: host, port: port, user: user, password: password, from: from}
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	_ = ctx
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	boundary := "pymes-cp-boundary"
	msg := strings.Join([]string{
		"From: " + s.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=" + boundary,
		"",
		"--" + boundary,
		"Content-Type: text/plain; charset=UTF-8",
		"",
		textBody,
		"--" + boundary,
		"Content-Type: text/html; charset=UTF-8",
		"",
		htmlBody,
		"--" + boundary + "--",
	}, "\r\n")

	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.password, s.host)
	}
	if err := smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

type SESSender struct {
	client *sesv2.Client
	from   string
}

func NewSESSender(client *sesv2.Client, from string) *SESSender {
	return &SESSender{client: client, from: from}
}

func (s *SESSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	if _, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: &s.from,
		Destination: &sestypes.Destination{
			ToAddresses: []string{to},
		},
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{Data: &subject},
				Body: &sestypes.Body{
					Text: &sestypes.Content{Data: &textBody},
					Html: &sestypes.Content{Data: &htmlBody},
				},
			},
		},
	}); err != nil {
		return fmt.Errorf("ses send: %w", err)
	}
	return nil
}

func NewEmailSender(backend string, logger zerolog.Logger) (EmailSender, error) {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", "noop":
		return NewNoopSender(logger), nil
	case "smtp":
		host := envOrDefault("SMTP_HOST", "localhost")
		port := envIntOrDefault("SMTP_PORT", 1025)
		user := os.Getenv("SMTP_USER")
		password := os.Getenv("SMTP_PASSWORD")
		from := envOrDefault("SMTP_FROM_EMAIL", envOrDefault("AWS_SES_FROM_EMAIL", "noreply@example.com"))
		return NewSMTPSender(host, port, user, password, from), nil
	case "ses":
		region := envOrDefault("AWS_REGION", "us-east-1")
		from := envOrDefault("AWS_SES_FROM_EMAIL", "noreply@example.com")
		cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
		if err != nil {
			return nil, fmt.Errorf("load aws config: %w", err)
		}
		return NewSESSender(sesv2.NewFromConfig(cfg), from), nil
	default:
		return nil, fmt.Errorf("unsupported NOTIFICATION_BACKEND: %s", backend)
	}
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
