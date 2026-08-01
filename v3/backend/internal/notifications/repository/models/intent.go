// Package models contains PostgreSQL-only notification representations.
package models

import (
	"encoding/json"
	"fmt"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type IntentRow struct {
	ID                string
	OrganizationID    string
	Kind              string
	AggregateType     string
	AggregateID       string
	RecipientE164     string
	TemplateName      string
	TemplateVersion   int
	Locale            string
	VariablesJSON     []byte
	Body              string
	SendAt            time.Time
	Status            string
	ExternalMessageID string
	IdempotencyKey    string
	CorrelationID     string
	RequestID         string
	ActorRef          string
	SourceVersion     int
	SnapshotDigest    string
	FailureCode       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (row IntentRow) Domain() (domain.Intent, error) {
	var variables map[string]string
	if err := json.Unmarshal(row.VariablesJSON, &variables); err != nil {
		return domain.Intent{}, fmt.Errorf("decode notification variables: %w", err)
	}
	return domain.Intent{
		ID: row.ID, OrganizationID: row.OrganizationID,
		Kind: domain.Kind(row.Kind), AggregateType: row.AggregateType,
		AggregateID: row.AggregateID, RecipientE164: row.RecipientE164,
		TemplateName: row.TemplateName, TemplateVersion: row.TemplateVersion,
		Locale: row.Locale, Variables: variables, Body: row.Body,
		SendAt: row.SendAt, Status: domain.Status(row.Status),
		ExternalMessageID: row.ExternalMessageID,
		IdempotencyKey:    row.IdempotencyKey, CorrelationID: row.CorrelationID,
		RequestID: row.RequestID, ActorRef: row.ActorRef,
		SourceVersion: row.SourceVersion, SnapshotDigest: row.SnapshotDigest,
		FailureCode: row.FailureCode, CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}
