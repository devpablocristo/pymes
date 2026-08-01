package helpers

import (
	"testing"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

func TestMessageRequestMapsCloudTemplateVariablesDeterministically(t *testing.T) {
	request := MessageRequest(domain.Intent{
		ID: "notification-1", OrganizationID: "org-1",
		Kind: domain.KindReminder, RecipientE164: "+5491112345678",
		TemplateName: "booking_reminder_v1", TemplateVersion: 1,
		Locale: "es_AR", Body: "recordatorio",
		Variables: map[string]string{
			"02_time": "10:30",
			"01_name": "Cliente",
		},
		IdempotencyKey: "booking-1", CorrelationID: "correlation-1",
	}, "whatsapp_cloud", "5491100000000")
	if request.TemplateName != "booking_reminder_v1" ||
		request.Language != "es_AR" ||
		len(request.Components) != 1 ||
		len(request.Components[0].Parameters) != 2 {
		t.Fatalf("unexpected template mapping: %#v", request)
	}
	if request.From != "5491100000000" {
		t.Fatalf("sender identity = %q", request.From)
	}
	parameters := request.Components[0].Parameters
	if parameters[0].Text != "Cliente" || parameters[1].Text != "10:30" {
		t.Fatalf("parameter order = %#v", parameters)
	}
	if request.Metadata["pymes_variable_1"] != "01_name" ||
		request.Metadata["pymes_variable_2"] != "02_time" {
		t.Fatalf("variable metadata = %#v", request.Metadata)
	}
}

func TestDeliveryRouteRequiresTenantRouteUnlessPilotFallbackIsExplicit(t *testing.T) {
	intent := domain.Intent{
		DeliveryChannel: "whatsapp_cloud",
		SenderIdentity:  "5491100000000",
	}
	channel, sender, err := DeliveryRoute(intent, "whatsapp_mock", false)
	if err != nil || channel != "whatsapp_cloud" ||
		sender != "5491100000000" {
		t.Fatalf("tenant route = %q/%q err=%v", channel, sender, err)
	}
	if _, _, err = DeliveryRoute(
		domain.Intent{}, "whatsapp_mock", false,
	); err == nil {
		t.Fatal("missing tenant route accepted without pilot fallback")
	}
	channel, sender, err = DeliveryRoute(
		domain.Intent{}, "whatsapp_mock", true,
	)
	if err != nil || channel != "whatsapp_mock" || sender != "" {
		t.Fatalf("pilot fallback = %q/%q err=%v", channel, sender, err)
	}
}
