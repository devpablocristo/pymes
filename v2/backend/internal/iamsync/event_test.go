package iamsync

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestDecodeEventAcceptsEveryIAMCommandContract(t *testing.T) {
	t.Parallel()
	expiry := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		topic string
		event Event
	}{
		{
			topic: OrganizationUpdateTopic,
			event: eventFixture(organizationUpdateOperation, func(event *Event) {
				event.ResourceID = event.OrganizationID
				event.Name = "Acme renovada"
				event.AppliedLocally = true
			}),
		},
		{
			topic: MemberRoleChangeTopic,
			event: eventFixture(memberRoleChangeOperation, func(event *Event) {
				event.Role = "member"
				event.PreviousRole = "admin"
				event.AppliedLocally = true
			}),
		},
		{
			topic: MemberRemoveTopic,
			event: eventFixture(memberRemoveOperation, func(event *Event) {
				event.PreviousRole = "member"
				event.AppliedLocally = true
			}),
		},
		{
			topic: OwnershipTransferTopic,
			event: eventFixture(ownershipTransferOperation, func(event *Event) {
				event.Role = "owner"
				event.PreviousRole = "member"
			}),
		},
		{
			topic: InvitationCreateTopic,
			event: invitationEventFixture(invitationCreateOperation, expiry, true),
		},
		{
			topic: InvitationResendTopic,
			event: invitationEventFixture(invitationResendOperation, expiry, false),
		},
		{
			topic: InvitationRevokeTopic,
			event: invitationEventFixture(invitationRevokeOperation, expiry, true),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.topic, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(test.event)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			decoded, err := decodeEvent(test.topic, raw)
			if err != nil {
				t.Fatalf("decodeEvent() error = %v", err)
			}
			if decoded.Operation != test.event.Operation ||
				decoded.OrganizationID != test.event.OrganizationID ||
				decoded.ResourceID != test.event.ResourceID {
				t.Fatalf("decoded = %#v", decoded)
			}
		})
	}
}

func TestDecodeEventRejectsCrossTopicAndUnknownFields(t *testing.T) {
	t.Parallel()
	event := eventFixture(organizationUpdateOperation, func(event *Event) {
		event.ResourceID = event.OrganizationID
		event.Name = "Acme"
		event.AppliedLocally = true
	})
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if _, err := decodeEvent(MemberRemoveTopic, raw); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("cross-topic error = %v, want ErrInvalidEvent", err)
	}
	raw = append(raw[:len(raw)-1], []byte(`,"ticket":"secret"}`)...)
	if _, err := decodeEvent(OrganizationUpdateTopic, raw); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("unknown-field error = %v, want ErrInvalidEvent", err)
	}
}

func eventFixture(operation string, mutate func(*Event)) Event {
	event := Event{
		SchemaVersion:          1,
		Operation:              operation,
		OrganizationID:         "10000000-0000-4000-8000-000000000001",
		ExternalOrganizationID: "org_acme",
		ActorUserID:            "20000000-0000-4000-8000-000000000001",
		ActorMembershipID:      "30000000-0000-4000-8000-000000000001",
		ResourceID:             "40000000-0000-4000-8000-000000000001",
	}
	if mutate != nil {
		mutate(&event)
	}
	return event
}

func invitationEventFixture(operation string, expiry time.Time, applied bool) Event {
	return eventFixture(operation, func(event *Event) {
		event.Email = "member@example.test"
		event.Role = "member"
		event.ExpiresAt = &expiry
		event.AppliedLocally = applied
	})
}
