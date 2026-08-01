// architecture:adapter external
package scheduling

import (
	"context"
	"strings"

	commercedomain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	partyhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/parties/helpers"
	partymodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/parties/models"
	schedulingdomain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
)

// PartyCommands is the consumer-owned boundary towards Commerce use cases.
// Scheduling never reaches the Commerce repository.
type PartyCommands interface {
	GetParty(context.Context, string, string) (commercedomain.Party, error)
	CreatePartyIdempotent(
		context.Context,
		commercedomain.IdempotencyCommand,
		commercedomain.Party,
	) (commercedomain.Party, error)
}

type PartyDirectoryAdapter struct {
	commands PartyCommands
}

func NewPartyDirectoryAdapter(commands PartyCommands) *PartyDirectoryAdapter {
	return &PartyDirectoryAdapter{commands: commands}
}

func (a *PartyDirectoryAdapter) EnsureCustomer(
	ctx context.Context,
	metadata schedulingdomain.CommandMetadata,
	customer PublicCustomer,
) (CustomerIdentity, error) {
	input := partymodels.Customer{
		PartyID: strings.TrimSpace(customer.PartyID),
		Name:    strings.TrimSpace(customer.Name),
		Email:   strings.TrimSpace(customer.Email),
		Phone:   strings.TrimSpace(customer.Phone),
	}
	if input.PartyID != "" {
		party, err := a.commands.GetParty(ctx, metadata.OrganizationID, input.PartyID)
		if err != nil {
			return CustomerIdentity{}, partyhelpers.MapError(err)
		}
		if party.OrganizationID != metadata.OrganizationID ||
			(party.Kind != "customer" && party.Kind != "both") {
			return CustomerIdentity{}, partyhelpers.InvalidCustomer()
		}
		return CustomerIdentity{
			PartyID: party.ID,
			Name:    party.DisplayName,
			Email:   input.Email,
			Phone:   input.Phone,
		}, nil
	}
	if input.Name == "" {
		return CustomerIdentity{}, partyhelpers.InvalidCustomer()
	}
	party := partyhelpers.NewCustomer(metadata, input)
	command := partyhelpers.IdempotencyCommand(metadata, party)
	created, err := a.commands.CreatePartyIdempotent(ctx, command, party)
	if err != nil {
		return CustomerIdentity{}, partyhelpers.MapError(err)
	}
	return CustomerIdentity{
		PartyID: created.ID,
		Name:    created.DisplayName,
		Email:   input.Email,
		Phone:   input.Phone,
	}, nil
}
