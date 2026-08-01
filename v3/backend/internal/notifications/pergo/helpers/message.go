// Package helpers contains PerGo HTTP protocol mapping and verification.
package helpers

import (
	"errors"
	"sort"
	"strings"

	pergomodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

var ErrRouteNotConfigured = errors.New("PerGo delivery route is not configured")

func DeliveryRoute(
	intent domain.Intent,
	fallbackChannel string,
	allowGlobalFallback bool,
) (string, string, error) {
	route := domain.DeliveryRoute{
		Channel:        strings.TrimSpace(intent.DeliveryChannel),
		SenderIdentity: strings.TrimSpace(intent.SenderIdentity),
	}
	if err := route.Validate(); err != nil {
		return "", "", err
	}
	if route.Channel != "" {
		return route.Channel, route.SenderIdentity, nil
	}
	if !allowGlobalFallback {
		return "", "", ErrRouteNotConfigured
	}
	channel := strings.TrimSpace(fallbackChannel)
	switch channel {
	case "whatsapp", "whatsapp_cloud", "whatsapp_mock":
		return channel, "", nil
	default:
		return "", "", ErrRouteNotConfigured
	}
}

func MessageRequest(
	intent domain.Intent,
	channel string,
	senderIdentity string,
) pergomodels.MessageRequest {
	metadata := map[string]string{
		"pymes_org_id":            intent.OrganizationID,
		"pymes_message_id":        intent.ID,
		"pymes_idempotency_key":   intent.IdempotencyKey,
		"pymes_correlation_id":    intent.CorrelationID,
		"pymes_template_version":  versionString(intent.TemplateVersion),
		"pymes_notification_kind": string(intent.Kind),
	}
	request := pergomodels.MessageRequest{
		To:      strings.TrimPrefix(intent.RecipientE164, "+"),
		From:    strings.TrimSpace(senderIdentity),
		Channel: channel, Body: intent.Body, Metadata: metadata,
	}
	if channel == "whatsapp_cloud" {
		request.TemplateName = intent.TemplateName
		request.Language = intent.Locale
		keys := make([]string, 0, len(intent.Variables))
		for key := range intent.Variables {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			parameters := make(
				[]pergomodels.TemplateParameter,
				0,
				len(keys),
			)
			for index, key := range keys {
				parameters = append(
					parameters,
					pergomodels.TemplateParameter{
						Type: "text", Text: intent.Variables[key],
					},
				)
				metadata["pymes_variable_"+versionString(index+1)] = key
			}
			request.Components = []pergomodels.TemplateComponent{{
				Type: "body", Parameters: parameters,
			}}
		}
	}
	return request
}

func versionString(version int) string {
	if version < 10 {
		return string(rune('0' + version))
	}
	var reversed [20]byte
	index := len(reversed)
	for version > 0 {
		index--
		reversed[index] = byte('0' + version%10)
		version /= 10
	}
	return string(reversed[index:])
}
