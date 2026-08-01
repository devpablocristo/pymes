// Package domain owns notification intent, lifecycle and invariants.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Kind string

const (
	KindConfirmation Kind = "confirmation"
	KindReminder     Kind = "reminder"
	KindRescheduled  Kind = "rescheduled"
	KindCancellation Kind = "cancellation"
	KindWaitlist     Kind = "waitlist"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusUncertain Status = "uncertain"
	StatusQueued    Status = "queued"
	StatusSent      Status = "sent"
	StatusDelivered Status = "delivered"
	StatusRead      Status = "read"
	StatusFailed    Status = "failed"
)

var (
	ErrDisabled             = errors.New("WHATSAPP_DISABLED")
	ErrNotFound             = errors.New("NOTIFICATION_NOT_FOUND")
	ErrIdempotencyKeyReused = errors.New("IDEMPOTENCY_KEY_REUSED")
	ErrInvalidIntent        = errors.New("NOTIFICATION_INVALID")
	ErrInvalidTransition    = errors.New("NOTIFICATION_STATE_INVALID")
)

var (
	e164Pattern     = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
	identifierShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:_./-]*$`)
	templateShape   = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)
	localeShape     = regexp.MustCompile(`^[a-z]{2}(?:_[A-Z]{2})?$`)
)

// Intent is Pymes' source of truth. Provider credentials and transport remain
// PerGo responsibilities; external identifiers are only delivery projections.
type Intent struct {
	ID                string            `json:"id"`
	OrganizationID    string            `json:"organization_id"`
	Kind              Kind              `json:"kind"`
	AggregateType     string            `json:"aggregate_type"`
	AggregateID       string            `json:"aggregate_id"`
	RecipientE164     string            `json:"-"`
	TemplateName      string            `json:"template_name"`
	TemplateVersion   int               `json:"template_version"`
	Locale            string            `json:"locale"`
	Variables         map[string]string `json:"-"`
	Body              string            `json:"-"`
	SendAt            time.Time         `json:"send_at"`
	Status            Status            `json:"status"`
	ExternalMessageID string            `json:"external_message_id,omitempty"`
	IdempotencyKey    string            `json:"idempotency_key"`
	CorrelationID     string            `json:"correlation_id"`
	RequestID         string            `json:"request_id"`
	ActorRef          string            `json:"actor_ref"`
	SourceVersion     int               `json:"source_version"`
	SnapshotDigest    string            `json:"snapshot_digest"`
	FailureCode       string            `json:"failure_code,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type DeliveryEvent struct {
	Event          string
	TraceID        string
	NotificationID string
	MessageID      string
	Channel        string
	Timestamp      time.Time
	WorkspaceID    string
	ErrorCode      string
}

func (i Intent) Validate() error {
	switch i.Kind {
	case KindConfirmation, KindReminder, KindRescheduled, KindCancellation, KindWaitlist:
	default:
		return invalid("kind")
	}
	if !validIdentifier(i.ID, 255) ||
		!validIdentifier(i.OrganizationID, 255) ||
		!validIdentifier(i.AggregateType, 80) ||
		!validIdentifier(i.AggregateID, 255) ||
		!validIdentifier(i.IdempotencyKey, 255) ||
		!validIdentifier(i.CorrelationID, 255) ||
		!validIdentifier(i.RequestID, 255) ||
		!validIdentifier(i.ActorRef, 255) {
		return invalid("identity")
	}
	if !e164Pattern.MatchString(i.RecipientE164) {
		return invalid("recipient")
	}
	if len(i.TemplateName) > 120 || !templateShape.MatchString(i.TemplateName) {
		return invalid("template")
	}
	if i.TemplateVersion < 1 || i.SourceVersion < 1 {
		return invalid("version")
	}
	if !localeShape.MatchString(i.Locale) {
		return invalid("locale")
	}
	if i.SendAt.IsZero() {
		return invalid("send_at")
	}
	if len(i.Body) < 1 || len(i.Body) > 4096 {
		return invalid("body")
	}
	if len(i.Variables) > 50 {
		return invalid("variables")
	}
	for key, value := range i.Variables {
		if !validIdentifier(key, 80) || len(value) > 1000 {
			return invalid("variables")
		}
	}
	return nil
}

func (i Intent) Digest() (string, error) {
	type snapshot struct {
		ID              string            `json:"id"`
		OrganizationID  string            `json:"organization_id"`
		Kind            Kind              `json:"kind"`
		AggregateType   string            `json:"aggregate_type"`
		AggregateID     string            `json:"aggregate_id"`
		RecipientE164   string            `json:"recipient_e164"`
		TemplateName    string            `json:"template_name"`
		TemplateVersion int               `json:"template_version"`
		Locale          string            `json:"locale"`
		Variables       map[string]string `json:"variables"`
		Body            string            `json:"body"`
		SendAt          string            `json:"send_at"`
		IdempotencyKey  string            `json:"idempotency_key"`
		CorrelationID   string            `json:"correlation_id"`
		RequestID       string            `json:"request_id"`
		ActorRef        string            `json:"actor_ref"`
		SourceVersion   int               `json:"source_version"`
	}
	body, err := json.Marshal(snapshot{
		ID: i.ID, OrganizationID: i.OrganizationID, Kind: i.Kind,
		AggregateType: i.AggregateType, AggregateID: i.AggregateID,
		RecipientE164: i.RecipientE164, TemplateName: i.TemplateName,
		TemplateVersion: i.TemplateVersion, Locale: i.Locale,
		Variables: i.Variables, Body: i.Body,
		SendAt:         i.SendAt.UTC().Format(time.RFC3339Nano),
		IdempotencyKey: i.IdempotencyKey, CorrelationID: i.CorrelationID,
		RequestID: i.RequestID, ActorRef: i.ActorRef,
		SourceVersion: i.SourceVersion,
	})
	if err != nil {
		return "", fmt.Errorf("encode notification snapshot: %w", err)
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:]), nil
}

func (i Intent) CanDispatch() bool {
	return i.Status == StatusPending || i.Status == StatusUncertain
}

func (i Intent) TerminalForDispatch() bool {
	switch i.Status {
	case StatusQueued, StatusSent, StatusDelivered, StatusRead, StatusFailed:
		return true
	default:
		return false
	}
}

func NextStatus(current Status, event string) (Status, error) {
	target := Status("")
	switch strings.ToLower(strings.TrimSpace(event)) {
	case "message.queued", "queued":
		target = StatusQueued
	case "message.sent", "sent":
		target = StatusSent
	case "message.delivered", "delivered":
		target = StatusDelivered
	case "message.read", "read":
		target = StatusRead
	case "message.failed", "failed":
		target = StatusFailed
	default:
		return "", fmt.Errorf("%w: unknown delivery event", ErrInvalidTransition)
	}
	if current == target {
		return target, nil
	}
	if current == StatusFailed || current == StatusRead {
		return "", ErrInvalidTransition
	}
	if target == StatusFailed {
		if current == StatusDelivered || current == StatusRead {
			return "", ErrInvalidTransition
		}
		return target, nil
	}
	rank := map[Status]int{
		StatusPending: 0, StatusUncertain: 0, StatusQueued: 1,
		StatusSent: 2, StatusDelivered: 3, StatusRead: 4,
	}
	if rank[target] < rank[current] {
		return "", ErrInvalidTransition
	}
	return target, nil
}

func ValidateDeliveryEvent(event DeliveryEvent) error {
	if event.Event == "" || !validIdentifier(event.TraceID, 255) ||
		!validIdentifier(event.NotificationID, 255) ||
		!validIdentifier(event.MessageID, 255) ||
		!validIdentifier(event.WorkspaceID, 255) ||
		event.Timestamp.IsZero() {
		return invalid("delivery_event")
	}
	if event.Channel != "whatsapp" &&
		event.Channel != "whatsapp_cloud" &&
		event.Channel != "whatsapp_mock" {
		return invalid("channel")
	}
	_, err := NextStatus(StatusPending, event.Event)
	return err
}

func ValidateDeliveryIdentity(
	organizationID string,
	notificationID string,
) error {
	if !validIdentifier(organizationID, 255) ||
		!validIdentifier(notificationID, 255) {
		return invalid("delivery_identity")
	}
	return nil
}

func invalid(field string) error {
	return fmt.Errorf("%w: %s", ErrInvalidIntent, field)
}

func validIdentifier(value string, maximum int) bool {
	return len(value) > 0 && len(value) <= maximum &&
		value == strings.TrimSpace(value) &&
		identifierShape.MatchString(value)
}
