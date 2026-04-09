package customer_messaging

import (
	"testing"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/customer_messaging/domain"
)

func TestParseInboundMessages_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, _, err := parseWebhookPayload([]byte(`{`))
	if err == nil {
		t.Fatal("parseWebhookPayload() error = nil, want bad input")
	}
}

func TestParseInboundMessages_Empty(t *testing.T) {
	t.Parallel()
	got, statuses, err := parseWebhookPayload([]byte(`{"object":"whatsapp_business_account","entry":[]}`))
	if err != nil {
		t.Fatalf("parseWebhookPayload() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
	if len(statuses) != 0 {
		t.Fatalf("statuses len = %d, want 0", len(statuses))
	}
}

func TestParseInboundMessages_TextMessage(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"object":"whatsapp_business_account",
		"entry":[{
			"changes":[{
				"field":"messages",
				"value":{
					"metadata":{"phone_number_id":"pnid-1"},
					"contacts":[{"wa_id":"5491111111111","profile":{"name":"Ana"}}],
					"messages":[{"id":"mid-1","from":"5491111111111","type":"text","text":{"body":" Hola mundo "}}]
				}
			}]
		}]
	}`)
	got, statuses, err := parseWebhookPayload(payload)
	if err != nil {
		t.Fatalf("parseWebhookPayload() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if len(statuses) != 0 {
		t.Fatalf("statuses len = %d, want 0", len(statuses))
	}
	m := got[0]
	if m.PhoneNumberID != "pnid-1" || m.FromPhone != "5491111111111" || m.Text != "Hola mundo" || m.MessageID != "mid-1" || m.ProfileName != "Ana" {
		t.Fatalf("message = %+v", m)
	}
}

func TestParseInboundMessages_SkipsNonText(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"object":"whatsapp_business_account",
		"entry":[{
			"changes":[{
				"field":"messages",
				"value":{
					"metadata":{"phone_number_id":"pnid-1"},
					"messages":[{"id":"m1","from":"5491111111111","type":"image","text":{"body":"x"}}]
				}
			}]
		}]
	}`)
	got, statuses, err := parseWebhookPayload(payload)
	if err != nil {
		t.Fatalf("parseWebhookPayload() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 for non-text type", len(got))
	}
	if len(statuses) != 0 {
		t.Fatalf("statuses len = %d, want 0", len(statuses))
	}
}

func TestParseInboundMessages_SkipsNonMessagesField(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"object":"whatsapp_business_account",
		"entry":[{
			"changes":[{
				"field":"statuses",
				"value":{"metadata":{"phone_number_id":"pnid-1"},"messages":[]}
			}]
		}]
	}`)
	got, statuses, err := parseWebhookPayload(payload)
	if err != nil {
		t.Fatalf("parseWebhookPayload() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 when field is not messages", len(got))
	}
	if len(statuses) != 0 {
		t.Fatalf("statuses len = %d, want 0", len(statuses))
	}
}

func TestParseWebhookPayload_StatusUpdate(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"object":"whatsapp_business_account",
		"entry":[{
			"changes":[{
				"field":"statuses",
				"value":{
					"metadata":{"phone_number_id":"pnid-1"},
					"statuses":[{"id":"wamid-123","status":"read","timestamp":"1710000000"}]
				}
			}]
		}]
	}`)
	messages, statuses, err := parseWebhookPayload(payload)
	if err != nil {
		t.Fatalf("ParseWebhookPayload() error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("messages len = %d, want 0", len(messages))
	}
	if len(statuses) != 1 {
		t.Fatalf("statuses len = %d, want 1", len(statuses))
	}
	if statuses[0].WAMessageID != "wamid-123" || statuses[0].Status != domain.StatusRead {
		t.Fatalf("status = %+v", statuses[0])
	}
}

func TestParseInboundMessages_SkipsEmptyPhoneNumberID(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"object":"whatsapp_business_account",
		"entry":[{
			"changes":[{
				"field":"messages",
				"value":{
					"metadata":{"phone_number_id":"  "},
					"messages":[{"id":"m1","from":"5491111111111","type":"text","text":{"body":"Hi"}}]
				}
			}]
		}]
	}`)
	got, statuses, err := parseWebhookPayload(payload)
	if err != nil {
		t.Fatalf("parseWebhookPayload() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 without phone_number_id", len(got))
	}
	if len(statuses) != 0 {
		t.Fatalf("statuses len = %d, want 0", len(statuses))
	}
}
