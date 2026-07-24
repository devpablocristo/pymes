package iamoutbox

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDecodeProvisionOrganizationEventIsStrict(t *testing.T) {
	valid, err := json.Marshal(validProvisionEvent())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if _, err := decodeProvisionOrganizationEvent(valid); err != nil {
		t.Fatalf("decode valid event: %v", err)
	}

	tests := []struct {
		name string
		raw  string
	}{
		{name: "unknown field", raw: strings.TrimSuffix(string(valid), "}") + `,"ticket":"secret"}`},
		{name: "trailing value", raw: string(valid) + ` {}`},
		{name: "unsupported version", raw: strings.Replace(string(valid), `"schema_version":1`, `"schema_version":2`, 1)},
		{name: "invalid request ID", raw: strings.Replace(string(valid), validProvisionEvent().RequestID, "not-a-uuid", 1)},
		{name: "non canonical email", raw: strings.Replace(string(valid), "owner@example.test", "OWNER@example.test", 1)},
		{name: "wrong local role", raw: strings.Replace(string(valid), `"owner_role":"admin"`, `"owner_role":"owner"`, 1)},
		{name: "wrong provider role", raw: strings.Replace(string(valid), `"provider_role":"org:admin"`, `"provider_role":"org:member"`, 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeProvisionOrganizationEvent([]byte(test.raw))
			if !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("error = %v, want ErrInvalidEvent", err)
			}
		})
	}
}

func validProvisionEvent() ProvisionOrganizationEvent {
	return ProvisionOrganizationEvent{
		SchemaVersion:  1,
		RequestID:      "4f0c4582-32c1-43f3-82b1-52662be6fafd",
		Provider:       ProviderClerk,
		OrganizationID: "0604cfee-4422-4c29-9d08-460773bd6ed2",
		Name:           "Acme Argentina",
		Slug:           "acme-argentina",
		OwnerEmail:     "owner@example.test",
		OwnerRole:      LocalOwnerRole,
		ProviderRole:   ClerkAdministratorRole,
	}
}
