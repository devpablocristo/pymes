// Package dto contains notification HTTP payloads and public projections.
package dto

import (
	"time"

	pergomodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type Notification struct {
	ID                string        `json:"id"`
	OrganizationID    string        `json:"organization_id"`
	Kind              domain.Kind   `json:"kind"`
	AggregateType     string        `json:"aggregate_type"`
	AggregateID       string        `json:"aggregate_id"`
	TemplateName      string        `json:"template_name"`
	TemplateVersion   int           `json:"template_version"`
	Locale            string        `json:"locale"`
	SendAt            time.Time     `json:"send_at"`
	Status            domain.Status `json:"status"`
	ExternalMessageID string        `json:"external_message_id,omitempty"`
	CorrelationID     string        `json:"correlation_id"`
	FailureCode       string        `json:"failure_code,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type Error struct {
	Code string `json:"code"`
}

func Public(intent domain.Intent) Notification {
	return Notification{
		ID: intent.ID, OrganizationID: intent.OrganizationID,
		Kind: intent.Kind, AggregateType: intent.AggregateType,
		AggregateID: intent.AggregateID, TemplateName: intent.TemplateName,
		TemplateVersion: intent.TemplateVersion, Locale: intent.Locale,
		SendAt: intent.SendAt, Status: intent.Status,
		ExternalMessageID: intent.ExternalMessageID,
		CorrelationID:     intent.CorrelationID, FailureCode: intent.FailureCode,
		CreatedAt: intent.CreatedAt, UpdatedAt: intent.UpdatedAt,
	}
}

type PerGoWebhook = pergomodels.WebhookEvent
