// Package iamoutbox processes IAM provider effects queued by Pymes v2.
package iamoutbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	ProvisionOrganizationTopic = "iam.organization.provision.requested.v1"
	ProviderClerk              = "clerk"
	LocalOwnerRole             = "owner"
	ClerkAdministratorRole     = "org:admin"
)

var (
	ErrInvalidEvent     = errors.New("iam outbox: invalid event")
	ErrUnsupportedTopic = errors.New("iam outbox: unsupported topic")

	provisioningSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// ProvisionOrganizationEvent is the immutable provider request written by the
// privileged provisioning command.
type ProvisionOrganizationEvent struct {
	SchemaVersion  int    `json:"schema_version"`
	RequestID      string `json:"request_id"`
	Provider       string `json:"provider"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	OwnerEmail     string `json:"owner_email"`
	OwnerRole      string `json:"owner_role"`
	ProviderRole   string `json:"provider_role"`
}

func decodeProvisionOrganizationEvent(raw []byte) (ProvisionOrganizationEvent, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	event := ProvisionOrganizationEvent{}
	if err := decoder.Decode(&event); err != nil {
		return ProvisionOrganizationEvent{}, fmt.Errorf("%w: decode payload: %v", ErrInvalidEvent, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return ProvisionOrganizationEvent{}, err
	}
	if err := event.validate(); err != nil {
		return ProvisionOrganizationEvent{}, err
	}
	return event, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: decode trailing payload: %v", ErrInvalidEvent, err)
	}
	return fmt.Errorf("%w: payload contains more than one JSON value", ErrInvalidEvent)
}

func (event ProvisionOrganizationEvent) validate() error {
	if event.SchemaVersion != 1 {
		return fmt.Errorf("%w: unsupported schema version %d", ErrInvalidEvent, event.SchemaVersion)
	}
	if _, err := uuid.Parse(event.RequestID); err != nil {
		return fmt.Errorf("%w: request_id must be a UUID", ErrInvalidEvent)
	}
	if _, err := uuid.Parse(event.OrganizationID); err != nil {
		return fmt.Errorf("%w: organization_id must be a UUID", ErrInvalidEvent)
	}
	if event.Provider != ProviderClerk {
		return fmt.Errorf("%w: provider must be %q", ErrInvalidEvent, ProviderClerk)
	}
	if event.Name == "" || event.Name != strings.TrimSpace(event.Name) || len(event.Name) > 200 {
		return fmt.Errorf("%w: organization name is not canonical", ErrInvalidEvent)
	}
	if len(event.Slug) > 63 || !provisioningSlugPattern.MatchString(event.Slug) {
		return fmt.Errorf("%w: organization slug is not canonical", ErrInvalidEvent)
	}
	email := strings.ToLower(strings.TrimSpace(event.OwnerEmail))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || event.OwnerEmail != email {
		return fmt.Errorf("%w: owner email is not canonical", ErrInvalidEvent)
	}
	if event.OwnerRole != LocalOwnerRole {
		return fmt.Errorf("%w: owner role must be %q", ErrInvalidEvent, LocalOwnerRole)
	}
	if event.ProviderRole != ClerkAdministratorRole {
		return fmt.Errorf("%w: provider role must be %q", ErrInvalidEvent, ClerkAdministratorRole)
	}
	return nil
}
