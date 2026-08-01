package commerce

import (
	"context"
	"fmt"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

// UncertainFiscalStore is the outbound port required to reconcile a lost
// authority response. The use-case deliberately has no SQL or pgx dependency.
type UncertainFiscalStore interface {
	ListUncertainSales(context.Context, int) ([]domain.PendingFiscal, error)
	ApplyFiscalResult(context.Context, domain.Sale, domain.FiscalResult) error
	ReserveFiscalConsultAttempt(context.Context, string, string) (int, error)
}
type ReconcileFiscal struct {
	Store  UncertainFiscalStore
	Fiscal FiscalClient
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
		attempt, err := u.Store.ReserveFiscalConsultAttempt(ctx, sale.OrganizationID, sale.ID)
		if err != nil {
			return err
		}
		sourceVersion := persistedSourceVersion(sale.Origin)
		correlationID := persistedCorrelationID(sale.Origin, sale.CorrelationID)
		sourceID := fmt.Sprintf("%s:%d", sale.ID, attempt)
		request := domain.FiscalRequest{
			RequestID:      fmt.Sprintf("fiscal-consult:%s:%d", sale.ID, attempt),
			OrganizationID: sale.OrganizationID,
			IdempotencyKey: internalIdempotencyKey(sale.OrganizationID, "fiscal.consult", sourceID, sourceVersion),
			SourceVersion:  sourceVersion,
			CredentialRef:  item.CredentialRef,
			Voucher:        sale.Voucher,
			Total:          sale.Total,
			SnapshotDigest: sale.SnapshotDigest,
			CorrelationID:  correlationID,
			FiscalSnapshot: sale.FiscalSnapshot,
		}
		consultContext := aggregateDeliveryContext(
			ctx, sale.OrganizationID, sale.Origin, request.RequestID, sale.CorrelationID,
		)
		result, err := u.Fiscal.Consult(consultContext, request)
		if err != nil {
			return err
		}
		if err := validateFiscalResult(request, result); err != nil {
			return err
		}
		if err := u.Store.ApplyFiscalResult(ctx, sale, result); err != nil {
			return err
		}
	}
	return nil
}
