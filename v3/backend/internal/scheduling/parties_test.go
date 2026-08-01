package scheduling

import (
	"context"
	"testing"

	commercedomain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	schedulingdomain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
)

type partyCommandsFake struct {
	parties map[string]commercedomain.Party
	creates int
}

func (f *partyCommandsFake) GetParty(
	_ context.Context,
	organizationID, partyID string,
) (commercedomain.Party, error) {
	party, ok := f.parties[organizationID+":"+partyID]
	if !ok {
		return commercedomain.Party{}, schedulingdomain.NewError(
			schedulingdomain.CodeNotFound,
			"not found",
		)
	}
	return party, nil
}

func (f *partyCommandsFake) CreatePartyIdempotent(
	_ context.Context,
	_ commercedomain.IdempotencyCommand,
	party commercedomain.Party,
) (commercedomain.Party, error) {
	f.creates++
	return party, nil
}

func TestPartyAdapterCreatesDeterministicCustomerThroughCommerceUsecase(t *testing.T) {
	commands := &partyCommandsFake{parties: map[string]commercedomain.Party{}}
	adapter := NewPartyDirectoryAdapter(commands)
	metadata := testMetadata("org-a", "public-booking", "booking-a")
	first, err := adapter.EnsureCustomer(
		context.Background(),
		metadata,
		PublicCustomer{Name: "Ada"},
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.EnsureCustomer(
		context.Background(),
		metadata,
		PublicCustomer{Name: "Ada"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.PartyID == "" || first != second || first.Name != "Ada" || commands.creates != 2 {
		t.Fatalf(
			"deterministic identity=%+v second=%+v creates=%d",
			first,
			second,
			commands.creates,
		)
	}
}

func TestPartyAdapterRejectsSupplierAsSchedulingCustomer(t *testing.T) {
	commands := &partyCommandsFake{parties: map[string]commercedomain.Party{
		"org-a:supplier-a": {
			ID: "supplier-a", OrganizationID: "org-a", Kind: "supplier",
		},
	}}
	adapter := NewPartyDirectoryAdapter(commands)
	_, err := adapter.EnsureCustomer(
		context.Background(),
		testMetadata("org-a", "existing-party", "booking-a"),
		PublicCustomer{PartyID: "supplier-a"},
	)
	if schedulingdomain.ErrorCodeOf(err) != schedulingdomain.CodeValidation {
		t.Fatalf("supplier accepted as customer: %v", err)
	}
}
