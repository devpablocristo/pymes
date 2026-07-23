// Package iamsync reconciles durable Pymes IAM commands with Clerk.
package iamsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	OrganizationUpdateTopic = "iam.organization.update.requested.v1"
	MemberRoleChangeTopic   = "iam.membership.role-change.requested.v1"
	MemberRemoveTopic       = "iam.membership.remove.requested.v1"
	OwnershipTransferTopic  = "iam.ownership.transfer.requested.v1"
	InvitationCreateTopic   = "iam.invitation.create.requested.v1"
	InvitationResendTopic   = "iam.invitation.resend.requested.v1"
	InvitationRevokeTopic   = "iam.invitation.revoke.requested.v1"

	organizationUpdateOperation = "organization.update"
	memberRoleChangeOperation   = "membership.role-change"
	memberRemoveOperation       = "membership.remove"
	ownershipTransferOperation  = "ownership.transfer"
	invitationCreateOperation   = "invitation.create"
	invitationResendOperation   = "invitation.resend"
	invitationRevokeOperation   = "invitation.revoke"

	providerClerk          = "clerk"
	clerkAdministratorRole = "org:admin"
	clerkMemberRole        = "org:member"
)

var (
	ErrInvalidEvent     = errors.New("iam sync: invalid event")
	ErrUnsupportedTopic = errors.New("iam sync: unsupported topic")
	ErrDurableConflict  = errors.New("iam sync: durable state conflict")
	ErrStateChanged     = errors.New("iam sync: durable state changed during reconciliation")
)

var topicOperations = map[string]string{
	OrganizationUpdateTopic: organizationUpdateOperation,
	MemberRoleChangeTopic:   memberRoleChangeOperation,
	MemberRemoveTopic:       memberRemoveOperation,
	OwnershipTransferTopic:  ownershipTransferOperation,
	InvitationCreateTopic:   invitationCreateOperation,
	InvitationResendTopic:   invitationResendOperation,
	InvitationRevokeTopic:   invitationRevokeOperation,
}

var syncTopics = []string{
	OrganizationUpdateTopic,
	MemberRoleChangeTopic,
	MemberRemoveTopic,
	OwnershipTransferTopic,
	InvitationCreateTopic,
	InvitationResendTopic,
	InvitationRevokeTopic,
}

// Event is the immutable schema emitted by the IAM HTTP command handlers.
// Provider credentials, invitation tickets and tokens are deliberately absent.
type Event struct {
	SchemaVersion          int        `json:"schema_version"`
	Operation              string     `json:"operation"`
	OrganizationID         string     `json:"organization_id"`
	ExternalOrganizationID string     `json:"external_organization_id,omitempty"`
	ActorUserID            string     `json:"actor_user_id"`
	ActorMembershipID      string     `json:"actor_membership_id"`
	ResourceID             string     `json:"resource_id"`
	ExternalResourceID     string     `json:"external_resource_id,omitempty"`
	Name                   string     `json:"name,omitempty"`
	Email                  string     `json:"email,omitempty"`
	Role                   string     `json:"role,omitempty"`
	PreviousRole           string     `json:"previous_role,omitempty"`
	ExpiresAt              *time.Time `json:"expires_at,omitempty"`
	AppliedLocally         bool       `json:"applied_locally"`
}

func decodeEvent(topic string, raw []byte) (Event, error) {
	expectedOperation, ok := topicOperations[topic]
	if !ok {
		return Event{}, fmt.Errorf("%w: %q", ErrUnsupportedTopic, topic)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	event := Event{}
	if err := decoder.Decode(&event); err != nil {
		return Event{}, fmt.Errorf("%w: decode payload: %v", ErrInvalidEvent, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return Event{}, fmt.Errorf("%w: decode trailing payload: %v", ErrInvalidEvent, err)
		}
		return Event{}, fmt.Errorf("%w: payload contains more than one JSON value", ErrInvalidEvent)
	}
	event.normalize()
	if err := event.validate(topic, expectedOperation); err != nil {
		return Event{}, err
	}
	return event, nil
}

func (event *Event) normalize() {
	event.Operation = strings.TrimSpace(event.Operation)
	event.OrganizationID = strings.TrimSpace(event.OrganizationID)
	event.ExternalOrganizationID = strings.TrimSpace(event.ExternalOrganizationID)
	event.ActorUserID = strings.TrimSpace(event.ActorUserID)
	event.ActorMembershipID = strings.TrimSpace(event.ActorMembershipID)
	event.ResourceID = strings.TrimSpace(event.ResourceID)
	event.ExternalResourceID = strings.TrimSpace(event.ExternalResourceID)
	event.Name = strings.TrimSpace(event.Name)
	event.Email = strings.ToLower(strings.TrimSpace(event.Email))
	event.Role = strings.TrimSpace(event.Role)
	event.PreviousRole = strings.TrimSpace(event.PreviousRole)
	if event.ExpiresAt != nil {
		normalized := event.ExpiresAt.UTC()
		event.ExpiresAt = &normalized
	}
}

func (event Event) validate(topic, expectedOperation string) error {
	if event.SchemaVersion != 1 {
		return fmt.Errorf("%w: unsupported schema version %d", ErrInvalidEvent, event.SchemaVersion)
	}
	if event.Operation != expectedOperation {
		return fmt.Errorf(
			"%w: operation %q does not match topic %q",
			ErrInvalidEvent,
			event.Operation,
			topic,
		)
	}
	for name, value := range map[string]string{
		"organization_id":     event.OrganizationID,
		"actor_user_id":       event.ActorUserID,
		"actor_membership_id": event.ActorMembershipID,
		"resource_id":         event.ResourceID,
	} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("%w: %s must be a UUID", ErrInvalidEvent, name)
		}
	}
	if event.ExternalOrganizationID == "" {
		return fmt.Errorf("%w: external_organization_id is required", ErrInvalidEvent)
	}

	switch topic {
	case OrganizationUpdateTopic:
		if event.ResourceID != event.OrganizationID {
			return fmt.Errorf("%w: organization update resource must be the organization", ErrInvalidEvent)
		}
		if event.Name == "" || len([]rune(event.Name)) > 120 || !event.AppliedLocally {
			return fmt.Errorf("%w: organization update fields are inconsistent", ErrInvalidEvent)
		}
	case MemberRoleChangeTopic:
		if !localRole(event.Role) || !localRole(event.PreviousRole) || event.Role == "owner" {
			return fmt.Errorf("%w: membership role change contains unsupported roles", ErrInvalidEvent)
		}
		applied := roleRank(event.Role) <= roleRank(event.PreviousRole)
		if event.AppliedLocally != applied {
			return fmt.Errorf("%w: membership role application flag is inconsistent", ErrInvalidEvent)
		}
	case MemberRemoveTopic:
		if !localRole(event.PreviousRole) || event.PreviousRole == "owner" || !event.AppliedLocally {
			return fmt.Errorf("%w: membership removal fields are inconsistent", ErrInvalidEvent)
		}
	case OwnershipTransferTopic:
		if event.Role != "owner" ||
			(event.PreviousRole != "admin" && event.PreviousRole != "member") {
			return fmt.Errorf("%w: ownership transfer roles are inconsistent", ErrInvalidEvent)
		}
		if event.AppliedLocally {
			return fmt.Errorf("%w: ownership transfer must be provider-first", ErrInvalidEvent)
		}
	case InvitationCreateTopic, InvitationResendTopic, InvitationRevokeTopic:
		if err := validateInvitationFields(event); err != nil {
			return err
		}
		expectedApplied := topic != InvitationResendTopic
		if event.AppliedLocally != expectedApplied {
			return fmt.Errorf("%w: invitation application flag is inconsistent", ErrInvalidEvent)
		}
	}
	return nil
}

func validateInvitationFields(event Event) error {
	address, err := mail.ParseAddress(event.Email)
	if err != nil || address.Address != event.Email {
		return fmt.Errorf("%w: invitation email is not canonical", ErrInvalidEvent)
	}
	if event.Role != "admin" && event.Role != "member" {
		return fmt.Errorf("%w: invitation role must be admin or member", ErrInvalidEvent)
	}
	if event.ExpiresAt == nil || event.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: invitation expiry is required", ErrInvalidEvent)
	}
	return nil
}

func localRole(role string) bool {
	return role == "owner" || role == "admin" || role == "member"
}

func roleRank(role string) int {
	switch role {
	case "owner":
		return 3
	case "admin":
		return 2
	case "member":
		return 1
	default:
		return 0
	}
}

func providerRole(local string) (string, error) {
	switch local {
	case "owner", "admin":
		return clerkAdministratorRole, nil
	case "member":
		return clerkMemberRole, nil
	default:
		return "", fmt.Errorf("%w: unsupported local role %q", ErrDurableConflict, local)
	}
}

func validIdempotencyKey(key string, event Event) bool {
	prefix := "iam:" + event.OrganizationID + ":" + event.Operation + ":"
	return strings.HasPrefix(strings.TrimSpace(key), prefix) &&
		len(strings.TrimPrefix(strings.TrimSpace(key), prefix)) >= 8
}
