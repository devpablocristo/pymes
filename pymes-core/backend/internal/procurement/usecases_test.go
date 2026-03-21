package procurement

import (
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/procurement/usecases/domain"
)

func TestSumLinesTotal(t *testing.T) {
	t.Parallel()
	got := sumLinesTotal([]domain.RequestLine{
		{Quantity: 2, UnitPriceEstimate: 100},
		{Quantity: 1, UnitPriceEstimate: 50},
	})
	if got != 250 {
		t.Fatalf("sumLinesTotal = %v, want 250", got)
	}
}

func TestBuildPurchaseItemsFromLines(t *testing.T) {
	t.Parallel()
	req := domain.ProcurementRequest{
		ID:    uuid.MustParse("00000000-0000-0000-0000-000000000099"),
		Title: "Test",
		Lines: []domain.RequestLine{
			{Description: "A", Quantity: 2, UnitPriceEstimate: 10},
		},
	}
	items := buildPurchaseItems(req)
	if len(items) != 1 || items[0].Subtotal != 20 {
		t.Fatalf("items = %+v", items)
	}
}
