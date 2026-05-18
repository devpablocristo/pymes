package publicapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	schedulingpublic "github.com/devpablocristo/modules/scheduling/go/publicapi"
)

func mapSchedulingErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		return ErrInvalidInput
	case strings.Contains(msg, "validation"):
		return ErrInvalidInput
	case strings.Contains(msg, "required"):
		return ErrInvalidInput
	case strings.Contains(msg, "invalid"):
		return ErrInvalidInput
	case strings.Contains(msg, "slot not available"):
		return ErrSlotUnavailable
	case strings.Contains(msg, "conflict"):
		return ErrSlotUnavailable
	default:
		return err
	}
}

func bookingFromSchedulingBooking(item schedulingdomain.Booking) BookingPublic {
	return BookingPublic{
		ID:            item.ID,
		CustomerName:  item.CustomerName,
		CustomerPhone: item.CustomerPhone,
		CustomerEmail: item.CustomerEmail,
		Title:         item.Reference,
		Status:        string(item.Status),
		StartAt:       item.StartAt.UTC(),
		EndAt:         item.EndAt.UTC(),
		Duration:      int(item.EndAt.Sub(item.StartAt).Minutes()),
	}
}

func buildActionLinks(tokens map[schedulingdomain.BookingActionType]schedulingdomain.BookingActionToken) schedulingpublic.ActionLinks {
	out := schedulingpublic.ActionLinks{}
	if token, ok := tokens[schedulingdomain.BookingActionConfirm]; ok {
		out.ConfirmToken = token.Token
		out.ConfirmPath = "/scheduling/bookings/actions/confirm?token=" + token.Token
	}
	if token, ok := tokens[schedulingdomain.BookingActionCancel]; ok {
		out.CancelToken = token.Token
		out.CancelPath = "/scheduling/bookings/actions/cancel?token=" + token.Token
	}
	return out
}

func uuidPtrFromPayload(payload map[string]any, key string) (*uuid.UUID, error) {
	value := firstStringFromPayload(payload, key)
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func uuidValueFromPayload(payload map[string]any, key string) (uuid.UUID, error) {
	id, err := uuidPtrFromPayload(payload, key)
	if err != nil {
		return uuid.Nil, err
	}
	if id == nil {
		return uuid.Nil, ErrInvalidInput
	}
	return *id, nil
}

func firstStringFromPayload(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(value.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func timeValueFromPayload(payload map[string]any, key string) (time.Time, error) {
	raw := firstStringFromPayload(payload, key)
	if raw == "" {
		return time.Time{}, ErrInvalidInput
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func optionalTimeValueFromPayload(payload map[string]any, key string) (*time.Time, error) {
	raw := firstStringFromPayload(payload, key)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func intValueFromPayload(payload map[string]any, key string) int {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return 0
	}
	switch value := raw.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case string:
		if value = strings.TrimSpace(value); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func ensureMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return cloneMap(m)
	}
	return map[string]any{}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func digitsOnly(v string) string {
	var b strings.Builder
	for _, r := range v {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isCatchAllService reports whether a scheduling service is the owner-side
// catch-all (used to anote ad-hoc bookings from the internal calendar). Such
// services are flagged with metadata.catchall = true at seed time and must be
// hidden from the public catalog.
func isCatchAllService(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	switch v := metadata["catchall"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}
