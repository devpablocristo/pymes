package usecases

import (
	"context"
	"fmt"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
)

// CommandStore is the persistence port for synchronous commerce commands.
// It keeps the inbound HTTP adapter independent from the database adapter.
type CommandStore interface {
	Ping(context.Context) error
	CreateParty(context.Context, domain.Party) (domain.Party, error)
	GetParty(context.Context, string, string) (domain.Party, error)
	CreatePurchaseAndQueue(context.Context, domain.Purchase) error
	GetPurchase(context.Context, string, string) (domain.Purchase, error)
	CreatePaymentAndApplications(context.Context, domain.Payment, []domain.OpenItemApplication) error
	GetPayment(context.Context, string, string) (domain.Payment, error)
	CreateSaleAndQueueFiscal(context.Context, domain.Sale, string) (domain.Sale, error)
	CreateAccountingReversal(context.Context, domain.AccountingReversal) (domain.AccountingReversal, error)
	GetSale(context.Context, string, string) (domain.Sale, error)
}

type Commands struct {
	Store CommandStore
	Now   func() time.Time
}

func (u Commands) Ready(ctx context.Context) error {
	if u.Store == nil {
		return fmt.Errorf("commerce store is not configured")
	}
	return u.Store.Ping(ctx)
}

func (u Commands) Clock() time.Time {
	if u.Now == nil {
		return time.Now()
	}
	return u.Now()
}
func (u Commands) CreateParty(ctx context.Context, party domain.Party) (domain.Party, error) {
	if u.Store == nil || party.ID == "" || party.OrganizationID == "" || party.DisplayName == "" {
		return domain.Party{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreateParty(ctx, party)
}
func (u Commands) GetParty(ctx context.Context, organizationID, partyID string) (domain.Party, error) {
	if u.Store == nil {
		return domain.Party{}, fmt.Errorf("commerce store is not configured")
	}
	return u.Store.GetParty(ctx, organizationID, partyID)
}
func (u Commands) CreatePurchaseAndQueue(ctx context.Context, purchase domain.Purchase) error {
	if u.Store == nil || purchase.ID == "" || purchase.OrganizationID == "" || !purchase.Total.Valid() {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreatePurchaseAndQueue(ctx, purchase)
}
func (u Commands) CreatePaymentAndApplications(ctx context.Context, payment domain.Payment, applications []domain.OpenItemApplication) error {
	if u.Store == nil || payment.ID == "" || payment.OrganizationID == "" || !payment.Total.Valid() {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreatePaymentAndApplications(ctx, payment, applications)
}
func (u Commands) GetPurchase(ctx context.Context, organizationID, purchaseID string) (domain.Purchase, error) {
	if u.Store == nil {
		return domain.Purchase{}, fmt.Errorf("commerce store is not configured")
	}
	return u.Store.GetPurchase(ctx, organizationID, purchaseID)
}
func (u Commands) GetPayment(ctx context.Context, organizationID, paymentID string) (domain.Payment, error) {
	if u.Store == nil {
		return domain.Payment{}, fmt.Errorf("commerce store is not configured")
	}
	return u.Store.GetPayment(ctx, organizationID, paymentID)
}
func (u Commands) CreateSaleAndQueueFiscal(ctx context.Context, sale domain.Sale, credentialRef string) (domain.Sale, error) {
	if u.Store == nil || sale.ID == "" || sale.OrganizationID == "" || credentialRef == "" || !sale.Total.Valid() {
		return domain.Sale{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreateSaleAndQueueFiscal(ctx, sale, credentialRef)
}
func (u Commands) GetSale(ctx context.Context, organizationID, saleID string) (domain.Sale, error) {
	if u.Store == nil {
		return domain.Sale{}, fmt.Errorf("commerce store is not configured")
	}
	return u.Store.GetSale(ctx, organizationID, saleID)
}

func (u Commands) CreateAccountingReversal(ctx context.Context, value domain.AccountingReversal) (domain.AccountingReversal, error) {
	if u.Store == nil || value.ID == "" || value.OrganizationID == "" || value.DocumentID == "" ||
		value.EffectiveAt.IsZero() || value.Reason == "" {
		return domain.AccountingReversal{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreateAccountingReversal(ctx, value)
}
