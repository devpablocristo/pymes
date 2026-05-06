package paymentgateway

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (u *Usecases) resolveReference(
	ctx context.Context,
	orgID uuid.UUID,
	refType string,
	refID uuid.UUID,
) (amount float64, currency string, description string, err error) {
	switch refType {
	case "sale":
		sale, e := u.repo.GetSaleSnapshot(ctx, orgID, refID)
		if e != nil {
			return 0, "", "", e
		}
		return sale.Total, coalesce(sale.Currency, "ARS"), fmt.Sprintf("Venta %s - %s", sale.Number, coalesce(sale.CustomerName, "Cliente")), nil
	case "quote":
		quote, e := u.repo.GetQuoteSnapshot(ctx, orgID, refID)
		if e != nil {
			return 0, "", "", e
		}
		return quote.Total, coalesce(quote.Currency, "ARS"), fmt.Sprintf("Presupuesto %s - %s", quote.Number, coalesce(quote.CustomerName, "Cliente")), nil
	default:
		return 0, "", "", ErrInvalidReference
	}
}
