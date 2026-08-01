// Package helpers contains PostgreSQL protocol helpers for notifications.
package helpers

import (
	"context"
	"errors"

	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
	"github.com/jackc/pgx/v5"
)

type Scanner interface {
	Scan(...any) error
}

func SetOrganization(ctx context.Context, tx pgx.Tx, organizationID string) error {
	if tx == nil || organizationID == "" {
		return errors.New("notification tenant transaction is required")
	}
	_, err := tx.Exec(
		ctx,
		"SELECT set_config('app.org_id',$1,true)",
		organizationID,
	)
	return err
}

func ScanIntent(scanner Scanner) (domain.Intent, error) {
	var row repositorymodels.IntentRow
	err := scanner.Scan(
		&row.ID, &row.OrganizationID, &row.Kind, &row.AggregateType,
		&row.AggregateID, &row.RecipientE164, &row.TemplateName,
		&row.TemplateVersion, &row.Locale, &row.VariablesJSON, &row.Body,
		&row.SendAt, &row.Status, &row.ExternalMessageID,
		&row.IdempotencyKey, &row.CorrelationID, &row.RequestID,
		&row.ActorRef, &row.SourceVersion, &row.SnapshotDigest,
		&row.FailureCode, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return domain.Intent{}, err
	}
	return row.Domain()
}
