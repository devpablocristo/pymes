package whatsapp

import (
	"testing"
)

func TestParseInboundMessages_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := parseInboundMessages([]byte(`{`))
	if err == nil {
		t.Fatal("parseInboundMessages() error = nil, want bad input")
	}
}

func TestParseInboundMessages_Empty(t *testing.T) {
	t.Parallel()
	got, err := parseInboundMessages([]byte(`{"object":"whatsapp_business_account","entry":[]}`))
	if err != nil {
		t.Fatalf("parseInboundMessages() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
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
	got, err := parseInboundMessages(payload)
	if err != nil {
		t.Fatalf("parseInboundMessages() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
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
	got, err := parseInboundMessages(payload)
	if err != nil {
		t.Fatalf("parseInboundMessages() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 for non-text type", len(got))
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
	got, err := parseInboundMessages(payload)
	if err != nil {
		t.Fatalf("parseInboundMessages() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 when field is not messages", len(got))
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
	got, err := parseInboundMessages(payload)
	if err != nil {
		t.Fatalf("parseInboundMessages() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 without phone_number_id", len(got))
	}
}
