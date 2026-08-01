package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	commercedomain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	partymodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/parties/models"
	schedulingdomain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func NewCustomer(
	metadata schedulingdomain.CommandMetadata,
	customer partymodels.Customer,
) commercedomain.Party {
	id := uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("pymes:scheduling:customer:"+metadata.OrganizationID+":"+metadata.SourceID),
	)
	return commercedomain.Party{
		ID:             id.String(),
		OrganizationID: metadata.OrganizationID,
		Kind:           "customer",
		DisplayName:    customer.Name,
	}
}

func IdempotencyCommand(
	metadata schedulingdomain.CommandMetadata,
	party commercedomain.Party,
) commercedomain.IdempotencyCommand {
	payload, _ := json.Marshal(struct {
		ID             string `json:"id"`
		OrganizationID string `json:"organization_id"`
		Kind           string `json:"kind"`
		DisplayName    string `json:"display_name"`
	}{
		ID:             party.ID,
		OrganizationID: party.OrganizationID,
		Kind:           party.Kind,
		DisplayName:    party.DisplayName,
	})
	digest := sha256.Sum256(payload)
	return commercedomain.IdempotencyCommand{
		Key:            metadata.IdempotencyKey,
		OrganizationID: metadata.OrganizationID,
		Operation:      commercedomain.OperationCreateParty,
		SourceID:       party.ID,
		SourceVersion:  metadata.SourceVersion,
		PayloadHash:    hex.EncodeToString(digest[:]),
		RequestID:      metadata.RequestID,
		CorrelationID:  metadata.CorrelationID,
		ActorRef:       metadata.ActorID,
	}
}

func MapError(err error) error {
	if errors.Is(err, commercedomain.ErrIdempotencyKeyReused) {
		return schedulingdomain.WrapError(
			schedulingdomain.CodeIdempotencyKeyReused,
			"idempotency key was reused with different customer data",
			err,
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return schedulingdomain.WrapError(
			schedulingdomain.CodeNotFound,
			"customer was not found",
			err,
		)
	}
	if strings.Contains(err.Error(), "VALIDATION_ERROR") {
		return schedulingdomain.WrapError(
			schedulingdomain.CodeValidation,
			"customer data is invalid",
			err,
		)
	}
	return err
}

func InvalidCustomer() error {
	return schedulingdomain.NewError(
		schedulingdomain.CodeValidation,
		"customer name or an existing customer is required",
	)
}
