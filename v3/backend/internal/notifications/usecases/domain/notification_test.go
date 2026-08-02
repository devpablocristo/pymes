package domain

import (
	"errors"
	"testing"
	"time"
)

func validIntent() Intent {
	return Intent{
		ID: "notification-1", OrganizationID: "org-1",
		Kind: KindConfirmation, AggregateType: "booking",
		AggregateID: "booking-1", RecipientE164: "+5491112345678",
		TemplateName: "booking.confirmation", TemplateVersion: 2,
		Locale: "es_AR", Variables: map[string]string{"customer": "Pablo"},
		Body: "Tu turno está confirmado.", SendAt: time.Unix(100, 0).UTC(),
		Status: StatusPending, IdempotencyKey: "booking-1:confirmation:v2",
		CorrelationID: "correlation-1", RequestID: "request-1",
		ActorRef: "system:scheduling", SourceVersion: 1,
	}
}

func TestIntentValidationAndDigestAreDeterministic(t *testing.T) {
	intent := validIntent()
	if err := intent.Validate(); err != nil {
		t.Fatalf("valid intent rejected: %v", err)
	}
	first, err := intent.Digest()
	if err != nil {
		t.Fatal(err)
	}
	intent.Variables = map[string]string{"customer": "Pablo"}
	second, err := intent.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("digest mismatch %q %q", first, second)
	}
	intent.RecipientE164 = "1112345678"
	if !errors.Is(intent.Validate(), ErrInvalidIntent) {
		t.Fatal("expected invalid E.164 recipient")
	}
}

func TestIntentDeliveryRouteIsValidatedAndFrozenIntoDigest(t *testing.T) {
	intent := validIntent()
	intent.DeliveryChannel = "whatsapp_cloud"
	intent.SenderIdentity = "5491100000000"
	first, err := intent.Digest()
	if err != nil {
		t.Fatal(err)
	}
	intent.SenderIdentity = "5491199999999"
	second, err := intent.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("delivery route was not frozen into the intent digest")
	}
	intent.SenderIdentity = ""
	if !errors.Is(intent.Validate(), ErrInvalidIntent) {
		t.Fatal("partial delivery route was accepted")
	}
	intent.DeliveryChannel = "email"
	intent.SenderIdentity = "sender"
	if !errors.Is(intent.Validate(), ErrInvalidIntent) {
		t.Fatal("unsupported delivery channel was accepted")
	}
}

func TestNextStatusNeverRegresses(t *testing.T) {
	status, err := NextStatus(StatusQueued, "delivered")
	if err != nil || status != StatusDelivered {
		t.Fatalf("delivered transition = %q, %v", status, err)
	}
	legacyStatus, legacyErr := NextStatus(StatusQueued, "message.delivered")
	if legacyErr != nil || legacyStatus != StatusDelivered {
		t.Fatalf("legacy delivered transition = %q, %v", legacyStatus, legacyErr)
	}
	if _, err := NextStatus(StatusDelivered, "sent"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("expected stale sent event to be rejected")
	}
	if _, err := NextStatus(StatusRead, "failed"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("expected terminal read state")
	}
}

func TestDeliveryFailureCodePreservesOnlyStableUncertainty(t *testing.T) {
	if code := DeliveryFailureCode("DELIVERY_UNCERTAIN"); code !=
		FailureCodeDeliveryUncertain {
		t.Fatalf("uncertain failure code = %q", code)
	}
	if code := DeliveryFailureCode("provider leaked phone +5491112345678"); code !=
		FailureCodeDeliveryFailed {
		t.Fatalf("untrusted provider failure code = %q", code)
	}
}
