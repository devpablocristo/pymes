package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type projectedIntentRepositoryFake struct {
	intent    domain.Intent
	err       error
	calls     int
	route     domain.DeliveryRoute
	routeErr  error
	projected bool
}

func (repository *projectedIntentRepositoryFake) ResolveDeliveryRoute(
	_ context.Context,
	_ string,
) (domain.DeliveryRoute, error) {
	if repository.route.Channel == "" && repository.routeErr == nil {
		return domain.DeliveryRoute{
			Channel: "whatsapp_mock", SenderIdentity: "mock:org-1",
		}, nil
	}
	return repository.route, repository.routeErr
}

func (repository *projectedIntentRepositoryFake) Project(
	_ context.Context,
	intent domain.Intent,
) (domain.Intent, error) {
	repository.calls++
	repository.intent = intent
	repository.projected = repository.err == nil
	return intent, repository.err
}

func (repository *projectedIntentRepositoryFake) FindProjected(
	_ context.Context,
	_ string,
	_ string,
) (domain.Intent, error) {
	if !repository.projected {
		return domain.Intent{}, domain.ErrNotFound
	}
	return repository.intent, nil
}

func TestProjectSchedulingNotificationOwnsMappingAndSnapshot(t *testing.T) {
	repository := &projectedIntentRepositoryFake{}
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	usecase := ProjectSchedulingNotification{
		Repository: repository,
		Routes:     repository,
		Clock:      func() time.Time { return now },
	}
	intent, deliver, err := usecase.Execute(
		context.Background(),
		ProjectionMetadata{
			EventID:        "event-1",
			OrganizationID: "org-1",
			IdempotencyKey: "scheduling:notification:booking-1:1",
			CorrelationID:  "correlation-1",
			RequestID:      "request-1",
			ActorRef:       "user-1",
			SourceVersion:  1,
		},
		SchedulingNotification{
			Trigger:       "BookingRescheduled",
			AggregateType: "booking",
			AggregateID:   "booking-1",
			RecipientE164: "+5491112345678",
			CustomerName:  "Ada",
			ServiceName:   "Consulta",
			StartAt:       now.Add(24 * time.Hour),
			EndAt:         now.Add(25 * time.Hour),
			Timezone:      "America/Argentina/Buenos_Aires",
			ActionTokens:  map[string]string{"cancel": "opaque-cancel"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !deliver || repository.calls != 1 {
		t.Fatalf("deliver=%v calls=%d", deliver, repository.calls)
	}
	if intent.ID != "scheduling:event-1" ||
		intent.Kind != domain.KindRescheduled ||
		intent.TemplateName != "scheduling.booking_rescheduled" ||
		intent.Status != domain.StatusPending ||
		intent.SendAt != now ||
		intent.SnapshotDigest == "" ||
		intent.DeliveryChannel != "whatsapp_mock" ||
		intent.SenderIdentity != "mock:org-1" ||
		intent.Variables["action_cancel"] != "opaque-cancel" {
		t.Fatalf("projected intent is incomplete: %+v", intent)
	}

	repository.route = domain.DeliveryRoute{
		Channel: "whatsapp_cloud", SenderIdentity: "changed-route",
	}
	usecase.Clock = func() time.Time { return now.Add(time.Hour) }
	replayed, deliver, err := usecase.Execute(
		context.Background(),
		ProjectionMetadata{
			EventID:        "event-1",
			OrganizationID: "org-1",
			IdempotencyKey: "scheduling:notification:booking-1:1",
			CorrelationID:  "correlation-1",
			RequestID:      "request-1",
			ActorRef:       "user-1",
			SourceVersion:  1,
		},
		SchedulingNotification{
			Trigger:       "BookingRescheduled",
			AggregateType: "booking",
			AggregateID:   "booking-1",
			RecipientE164: "+5491112345678",
			CustomerName:  "Ada",
			ServiceName:   "Consulta",
			StartAt:       now.Add(24 * time.Hour),
			EndAt:         now.Add(25 * time.Hour),
			Timezone:      "America/Argentina/Buenos_Aires",
			ActionTokens:  map[string]string{"cancel": "opaque-cancel"},
		},
	)
	if err != nil || !deliver ||
		replayed.SnapshotDigest != intent.SnapshotDigest ||
		replayed.SenderIdentity != "mock:org-1" ||
		replayed.SendAt != now ||
		repository.calls != 1 {
		t.Fatalf(
			"replay changed projection: intent=%+v deliver=%v calls=%d err=%v",
			replayed,
			deliver,
			repository.calls,
			err,
		)
	}
}

func TestProjectSchedulingNotificationSkipsUnsupportedOrUnavailableDelivery(t *testing.T) {
	repository := &projectedIntentRepositoryFake{}
	usecase := ProjectSchedulingNotification{
		Repository: repository, Routes: repository,
	}
	metadata := ProjectionMetadata{
		EventID: "event-1", OrganizationID: "org-1",
		IdempotencyKey: "key-1", CorrelationID: "correlation-1",
		RequestID: "request-1", ActorRef: "user-1", SourceVersion: 1,
	}
	for _, input := range []SchedulingNotification{
		{
			Trigger: "BookingCompleted", AggregateType: "booking",
			AggregateID: "booking-1", RecipientE164: "+5491112345678",
		},
		{
			Trigger: "BookingConfirmed", AggregateType: "booking",
			AggregateID: "booking-1",
		},
	} {
		_, deliver, err := usecase.Execute(context.Background(), metadata, input)
		if err != nil || deliver {
			t.Fatalf("input=%+v deliver=%v err=%v", input, deliver, err)
		}
	}
	if repository.calls != 0 {
		t.Fatalf("skipped projections reached repository: calls=%d", repository.calls)
	}

	repository.err = domain.ErrDisabled
	_, deliver, err := usecase.Execute(
		context.Background(),
		metadata,
		SchedulingNotification{
			Trigger: "BookingConfirmed", AggregateType: "booking",
			AggregateID: "booking-1", RecipientE164: "+5491112345678",
		},
	)
	if err != nil || deliver {
		t.Fatalf("disabled projection deliver=%v err=%v", deliver, err)
	}
	if !errors.Is(repository.err, domain.ErrDisabled) {
		t.Fatal("test setup lost disabled result")
	}
}
