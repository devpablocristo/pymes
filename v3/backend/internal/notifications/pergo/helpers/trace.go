package helpers

import (
	"encoding/base64"
	"errors"
	"strings"
)

const tracePrefix = "pymes.v1."

// TraceID produces the tenant-aware identity returned unchanged by PerGo in
// delivery webhooks. Both components are base64url encoded so neither tenant
// routing nor notification lookup depends on a caller-controlled URL path.
func TraceID(organizationID string, notificationID string) (string, error) {
	organizationID = strings.TrimSpace(organizationID)
	notificationID = strings.TrimSpace(notificationID)
	if organizationID == "" || notificationID == "" {
		return "", errors.New("organization and notification are required")
	}
	traceID := tracePrefix +
		base64.RawURLEncoding.EncodeToString([]byte(organizationID)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(notificationID))
	if len(traceID) > 255 {
		return "", errors.New("tenant-aware trace ID is too long")
	}
	return traceID, nil
}

func ParseTraceID(traceID string) (string, string, error) {
	traceID = strings.TrimSpace(traceID)
	if !strings.HasPrefix(traceID, tracePrefix) || len(traceID) > 255 {
		return "", "", errors.New("invalid tenant-aware trace ID")
	}
	parts := strings.Split(strings.TrimPrefix(traceID, tracePrefix), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("invalid tenant-aware trace ID")
	}
	organization, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", errors.New("invalid tenant trace component")
	}
	notification, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", errors.New("invalid notification trace component")
	}
	organizationID := string(organization)
	notificationID := string(notification)
	if organizationID == "" || notificationID == "" ||
		organizationID != strings.TrimSpace(organizationID) ||
		notificationID != strings.TrimSpace(notificationID) {
		return "", "", errors.New("invalid tenant-aware trace identity")
	}
	return organizationID, notificationID, nil
}
