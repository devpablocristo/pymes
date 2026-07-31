package usecases

import (
	"context"
	"fmt"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
)

// UncertainFiscalStore is the outbound port required to reconcile a lost
// authority response. The use-case deliberately has no SQL or pgx dependency.
type UncertainFiscalStore interface {
	ListUncertainSales(context.Context, int) ([]domain.PendingFiscal, error)
	ApplyFiscalResult(context.Context, domain.Sale, domain.FiscalResult) error
}
type ReconcileFiscal struct {
	Store  UncertainFiscalStore
	Fiscal domain.FiscalClient
}

func (u ReconcileFiscal) Execute(ctx context.Context) error {
	if u.Store == nil || u.Fiscal == nil {
		return fmt.Errorf("reconcile fiscal dependencies are not configured")
	}
	pending, err := u.Store.ListUncertainSales(ctx, 20)
	if err != nil {
		return err
	}
	for _, item := range pending {
		sale := item.Sale
		result, err := u.Fiscal.Consult(ctx, domain.FiscalRequest{RequestID: "fiscal:" + sale.ID + ":1", OrganizationID: sale.OrganizationID, CredentialRef: item.CredentialRef, Voucher: sale.Voucher, Total: sale.Total, SnapshotDigest: sale.SnapshotDigest, CorrelationID: sale.CorrelationID, FiscalSnapshot: sale.FiscalSnapshot})
		if err != nil {
			return err
		}
		if err := u.Store.ApplyFiscalResult(ctx, sale, result); err != nil {
			return err
		}
	}
	return nil
}
