// Package notifications contains Pymes notification application workflows and
// the ports they consume.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type IntentRepository interface {
	Create(context.Context, domain.Intent) (domain.Intent, error)
	Get(context.Context, string, string) (domain.Intent, error)
}

type ProjectedIntentRepository interface {
	Project(context.Context, domain.Intent) (domain.Intent, error)
}

type RequestNotification struct {
	Repository IntentRepository
	Clock      func() time.Time
}

func (usecase RequestNotification) Execute(
	ctx context.Context,
	intent domain.Intent,
) (domain.Intent, error) {
	if ctx == nil {
		return domain.Intent{}, fmt.Errorf("request notification: context is required")
	}
	if usecase.Repository == nil {
		return domain.Intent{}, fmt.Errorf("request notification: repository is required")
	}
	now := time.Now
	if usecase.Clock != nil {
		now = usecase.Clock
	}
	if intent.Status == "" {
		intent.Status = domain.StatusPending
	}
	if intent.SendAt.IsZero() {
		intent.SendAt = now().UTC()
	} else {
		intent.SendAt = intent.SendAt.UTC()
	}
	if err := intent.Validate(); err != nil {
		return domain.Intent{}, err
	}
	digest, err := intent.Digest()
	if err != nil {
		return domain.Intent{}, err
	}
	intent.SnapshotDigest = digest
	return usecase.Repository.Create(ctx, intent)
}

type ProjectionMetadata struct {
	EventID        string
	OrganizationID string
	IdempotencyKey string
	CorrelationID  string
	RequestID      string
	ActorRef       string
	SourceVersion  int
}

type SchedulingNotification struct {
	Trigger       string
	AggregateType string
	AggregateID   string
	RecipientE164 string
	CustomerName  string
	ServiceName   string
	StartAt       time.Time
	EndAt         time.Time
	Timezone      string
	Reason        string
	ActionToken   string
	ActionTokens  map[string]string
	ExpiresAt     *time.Time
}

type ProjectSchedulingNotification struct {
	Repository ProjectedIntentRepository
	Clock      func() time.Time
}

func (usecase ProjectSchedulingNotification) Execute(
	ctx context.Context,
	metadata ProjectionMetadata,
	input SchedulingNotification,
) (domain.Intent, bool, error) {
	if ctx == nil {
		return domain.Intent{}, false, fmt.Errorf(
			"project scheduling notification: context is required",
		)
	}
	if usecase.Repository == nil {
		return domain.Intent{}, false, fmt.Errorf(
			"project scheduling notification: repository is required",
		)
	}
	kind, template, supported := schedulingTemplate(input.Trigger)
	recipient := strings.TrimSpace(input.RecipientE164)
	if !supported || recipient == "" {
		return domain.Intent{}, false, nil
	}
	now := time.Now
	if usecase.Clock != nil {
		now = usecase.Clock
	}
	variables := schedulingVariables(input)
	intent := domain.Intent{
		ID:                "scheduling:" + metadata.EventID,
		OrganizationID:    metadata.OrganizationID,
		Kind:              kind,
		AggregateType:     input.AggregateType,
		AggregateID:       input.AggregateID,
		RecipientE164:     recipient,
		TemplateName:      template,
		TemplateVersion:   1,
		Locale:            "es_AR",
		Variables:         variables,
		Body:              schedulingBody(kind, variables),
		SendAt:            now().UTC(),
		Status:            domain.StatusPending,
		IdempotencyKey:    metadata.IdempotencyKey,
		CorrelationID:     metadata.CorrelationID,
		RequestID:         metadata.RequestID,
		ActorRef:          metadata.ActorRef,
		SourceVersion:     metadata.SourceVersion,
		SnapshotDigest:    "",
		ExternalMessageID: "",
	}
	if err := intent.Validate(); err != nil {
		return domain.Intent{}, false, err
	}
	digest, err := intent.Digest()
	if err != nil {
		return domain.Intent{}, false, err
	}
	intent.SnapshotDigest = digest
	stored, err := usecase.Repository.Project(ctx, intent)
	if errors.Is(err, domain.ErrDisabled) {
		return domain.Intent{}, false, nil
	}
	if err != nil {
		return domain.Intent{}, false, err
	}
	return stored, true, nil
}

func schedulingTemplate(trigger string) (domain.Kind, string, bool) {
	switch strings.TrimSpace(trigger) {
	case "BookingCreated", "BookingConfirmed":
		return domain.KindConfirmation, "scheduling.booking_confirmation", true
	case "ReminderDue":
		return domain.KindReminder, "scheduling.booking_reminder", true
	case "BookingRescheduled":
		return domain.KindRescheduled, "scheduling.booking_rescheduled", true
	case "BookingCancelled":
		return domain.KindCancellation, "scheduling.booking_cancelled", true
	case "WaitlistOffered":
		return domain.KindWaitlist, "scheduling.waitlist_offered", true
	default:
		return "", "", false
	}
}

func schedulingVariables(input SchedulingNotification) map[string]string {
	variables := map[string]string{
		"customer_name": strings.TrimSpace(input.CustomerName),
	}
	if value := strings.TrimSpace(input.ServiceName); value != "" {
		variables["service_name"] = value
	}
	if !input.StartAt.IsZero() {
		variables["start_at"] = input.StartAt.UTC().Format(time.RFC3339)
	}
	if !input.EndAt.IsZero() {
		variables["end_at"] = input.EndAt.UTC().Format(time.RFC3339)
	}
	if value := strings.TrimSpace(input.Timezone); value != "" {
		variables["timezone"] = value
	}
	if value := strings.TrimSpace(input.Reason); value != "" {
		variables["reason"] = value
	}
	if value := strings.TrimSpace(input.ActionToken); value != "" {
		variables["action_waitlist"] = value
	}
	for purpose, token := range input.ActionTokens {
		purpose = strings.TrimSpace(purpose)
		token = strings.TrimSpace(token)
		if purpose != "" && token != "" {
			variables["action_"+purpose] = token
		}
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.IsZero() {
		variables["expires_at"] = input.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return variables
}

func schedulingBody(kind domain.Kind, variables map[string]string) string {
	customer := variables["customer_name"]
	service := variables["service_name"]
	start := variables["start_at"]
	switch kind {
	case domain.KindConfirmation:
		return fmt.Sprintf("Hola %s. Tu turno de %s para %s quedó registrado.", customer, service, start)
	case domain.KindReminder:
		return fmt.Sprintf("Hola %s. Recordatorio de tu turno de %s para %s.", customer, service, start)
	case domain.KindRescheduled:
		return fmt.Sprintf("Hola %s. Tu turno de %s fue reprogramado para %s.", customer, service, start)
	case domain.KindCancellation:
		return fmt.Sprintf("Hola %s. Tu turno de %s fue cancelado.", customer, service)
	case domain.KindWaitlist:
		return fmt.Sprintf("Hola %s. Hay un turno disponible para vos.", customer)
	default:
		return "Actualización de tu turno."
	}
}

type ReadNotification struct {
	Repository IntentRepository
}

func (usecase ReadNotification) Execute(
	ctx context.Context,
	organizationID string,
	notificationID string,
) (domain.Intent, error) {
	if ctx == nil || usecase.Repository == nil {
		return domain.Intent{}, fmt.Errorf("read notification: dependency is required")
	}
	return usecase.Repository.Get(ctx, organizationID, notificationID)
}

type DeliveryReceipt struct {
	ExternalMessageID string
	Status            string
	QueuedAt          time.Time
}

type DeliveryProvider interface {
	Send(context.Context, domain.Intent) (DeliveryReceipt, error)
}

type DeliveryEventRepository interface {
	ApplyDeliveryEvent(
		context.Context,
		string,
		domain.DeliveryEvent,
		string,
	) (bool, error)
}

type ProcessDeliveryWebhook struct {
	Repository        DeliveryEventRepository
	ExpectedWorkspace string
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (usecase ProcessDeliveryWebhook) Execute(
	ctx context.Context,
	organizationID string,
	event domain.DeliveryEvent,
	payloadHash string,
) (bool, error) {
	if ctx == nil || usecase.Repository == nil {
		return false, fmt.Errorf("process delivery webhook: dependency is required")
	}
	if organizationID == "" ||
		event.WorkspaceID != usecase.ExpectedWorkspace ||
		!digestPattern.MatchString(payloadHash) {
		return false, domain.ErrInvalidIntent
	}
	if err := domain.ValidateDeliveryIdentity(
		organizationID,
		event.NotificationID,
	); err != nil {
		return false, err
	}
	if err := domain.ValidateDeliveryEvent(event); err != nil {
		return false, err
	}
	return usecase.Repository.ApplyDeliveryEvent(
		ctx, organizationID, event, payloadHash,
	)
}

type ProviderError struct {
	StableCode string
	Retry      bool
	Unknown    bool
	Cause      error
}

func (err *ProviderError) Error() string {
	if err == nil || err.StableCode == "" {
		return "PERGO_DELIVERY_FAILED"
	}
	return err.StableCode
}

func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func AsProviderError(err error) (*ProviderError, bool) {
	var providerError *ProviderError
	return providerError, errors.As(err, &providerError)
}
