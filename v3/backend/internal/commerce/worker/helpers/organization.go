// Package helpers contains delivery-envelope validation for the commerce worker.
package helpers

import "fmt"

// RequireEventOrganization prevents a leased event from crossing tenant state.
func RequireEventOrganization(eventID, eventOrganizationID, aggregateOrganizationID string) error {
	if eventOrganizationID == "" || aggregateOrganizationID == "" ||
		eventOrganizationID != aggregateOrganizationID {
		return fmt.Errorf(
			"OUTBOX_ORGANIZATION_MISMATCH: event %s organization %q, aggregate organization %q",
			eventID,
			eventOrganizationID,
			aggregateOrganizationID,
		)
	}
	return nil
}
