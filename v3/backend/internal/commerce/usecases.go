package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

// CommandStore is the persistence port for synchronous commerce commands.
// It keeps the inbound HTTP adapter independent from the database adapter.
type CommandStore interface {
	Ping(context.Context) error
	CreateParty(context.Context, domain.Party) (domain.Party, error)
	CreatePartyIdempotent(context.Context, domain.IdempotencyCommand, domain.Party) (domain.Party, error)
	GetParty(context.Context, string, string) (domain.Party, error)
	CreatePurchaseAndQueue(context.Context, domain.Purchase) error
	CreatePurchaseAndQueueIdempotent(context.Context, domain.IdempotencyCommand, domain.Purchase) (domain.Purchase, error)
	GetPurchase(context.Context, string, string) (domain.Purchase, error)
	CreatePaymentAndApplications(context.Context, domain.Payment, []domain.OpenItemApplication) error
	CreatePaymentAndApplicationsIdempotent(context.Context, domain.IdempotencyCommand, domain.Payment, []domain.OpenItemApplication) (domain.Payment, error)
	GetPayment(context.Context, string, string) (domain.Payment, error)
	CreateSaleAndQueueFiscal(context.Context, domain.Sale, string) (domain.Sale, error)
	CreateSaleAndQueueFiscalIdempotent(context.Context, domain.IdempotencyCommand, domain.Sale, string) (domain.Sale, error)
	CreateAccountingReversal(context.Context, domain.AccountingReversal) (domain.AccountingReversal, error)
	CreateAccountingReversalIdempotent(context.Context, domain.IdempotencyCommand, domain.AccountingReversal) (domain.AccountingReversal, error)
	GetSale(context.Context, string, string) (domain.Sale, error)
}

type AccountingAdjustmentStore interface {
	GetAccountingFailure(context.Context, string, string) (domain.AccountingFailure, error)
	RequestAccountingAdjustmentIdempotent(
		context.Context,
		domain.IdempotencyCommand,
		string,
		domain.AccountingAdjustment,
	) (domain.AccountingAdjustment, error)
}

type Commands struct {
	Store                 CommandStore
	AccountingAdjustments AccountingAdjustmentStore
	Now                   func() time.Time
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

func (u Commands) CreatePartyIdempotent(ctx context.Context, command domain.IdempotencyCommand, party domain.Party) (domain.Party, error) {
	if u.Store == nil || party.ID == "" || party.OrganizationID == "" || party.DisplayName == "" ||
		!validIdempotencyCommand(command, party.OrganizationID, domain.OperationCreateParty, party.ID) {
		return domain.Party{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreatePartyIdempotent(ctx, command, party)
}

func (u Commands) GetParty(ctx context.Context, organizationID, partyID string) (domain.Party, error) {
	if u.Store == nil {
		return domain.Party{}, fmt.Errorf("commerce store is not configured")
	}
	return u.Store.GetParty(ctx, organizationID, partyID)
}
func (u Commands) CreatePurchaseAndQueue(ctx context.Context, purchase domain.Purchase) error {
	if u.Store == nil || purchase.ID == "" || purchase.OrganizationID == "" ||
		purchase.ValidateAccountingAmounts() != nil {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreatePurchaseAndQueue(ctx, purchase)
}

func (u Commands) CreatePurchaseAndQueueIdempotent(ctx context.Context, command domain.IdempotencyCommand, purchase domain.Purchase) (domain.Purchase, error) {
	if u.Store == nil || purchase.ID == "" || purchase.OrganizationID == "" ||
		purchase.ValidateAccountingAmounts() != nil ||
		!validIdempotencyCommand(command, purchase.OrganizationID, domain.OperationCreatePurchase, purchase.ID) {
		return domain.Purchase{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreatePurchaseAndQueueIdempotent(ctx, command, purchase)
}

func (u Commands) CreatePaymentAndApplications(ctx context.Context, payment domain.Payment, applications []domain.OpenItemApplication) error {
	if u.Store == nil || payment.ID == "" || payment.OrganizationID == "" || !payment.Total.Valid() {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreatePaymentAndApplications(ctx, payment, applications)
}

func (u Commands) CreatePaymentAndApplicationsIdempotent(ctx context.Context, command domain.IdempotencyCommand, payment domain.Payment, applications []domain.OpenItemApplication) (domain.Payment, error) {
	if u.Store == nil || payment.ID == "" || payment.OrganizationID == "" || !payment.Total.Valid() ||
		!validIdempotencyCommand(command, payment.OrganizationID, domain.OperationCreatePayment, payment.ID) {
		return domain.Payment{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreatePaymentAndApplicationsIdempotent(ctx, command, payment, applications)
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

func (u Commands) CreateSaleAndQueueFiscalIdempotent(ctx context.Context, command domain.IdempotencyCommand, sale domain.Sale, credentialRef string) (domain.Sale, error) {
	if u.Store == nil || sale.ID == "" || sale.OrganizationID == "" || credentialRef == "" || !sale.Total.Valid() ||
		!validIdempotencyCommand(command, sale.OrganizationID, domain.OperationCreateSale, sale.ID) {
		return domain.Sale{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreateSaleAndQueueFiscalIdempotent(ctx, command, sale, credentialRef)
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

func (u Commands) CreateAccountingReversalIdempotent(ctx context.Context, command domain.IdempotencyCommand, value domain.AccountingReversal) (domain.AccountingReversal, error) {
	if u.Store == nil || value.ID == "" || value.OrganizationID == "" || value.DocumentID == "" ||
		value.EffectiveAt.IsZero() || value.Reason == "" ||
		!validIdempotencyCommand(command, value.OrganizationID, domain.OperationCreateAccountingReversal, value.ID) {
		return domain.AccountingReversal{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.Store.CreateAccountingReversalIdempotent(ctx, command, value)
}

func (u Commands) GetAccountingFailure(
	ctx context.Context,
	organizationID, failureID string,
) (domain.AccountingFailure, error) {
	if u.AccountingAdjustments == nil || organizationID == "" || failureID == "" {
		return domain.AccountingFailure{}, fmt.Errorf("commerce accounting adjustments are not configured")
	}
	return u.AccountingAdjustments.GetAccountingFailure(ctx, organizationID, failureID)
}

func (u Commands) RequestAccountingAdjustmentIdempotent(
	ctx context.Context,
	command domain.IdempotencyCommand,
	failureID string,
	value domain.AccountingAdjustment,
) (domain.AccountingAdjustment, error) {
	if u.AccountingAdjustments == nil ||
		failureID == "" || value.ID == "" || value.FailureID != failureID ||
		value.OrganizationID == "" || value.EffectiveAt.IsZero() ||
		strings.TrimSpace(value.Reason) == "" || len(value.Reason) > 500 ||
		!validIdempotencyCommand(
			command,
			value.OrganizationID,
			domain.OperationCreateAccountingAdjustment,
			value.ID,
		) {
		return domain.AccountingAdjustment{}, fmt.Errorf("VALIDATION_ERROR")
	}
	return u.AccountingAdjustments.RequestAccountingAdjustmentIdempotent(
		ctx, command, failureID, value,
	)
}

func validIdempotencyCommand(command domain.IdempotencyCommand, organizationID, operation, sourceID string) bool {
	decodedHash, err := hex.DecodeString(command.PayloadHash)
	trimmedKey := strings.TrimSpace(command.Key)
	return trimmedKey != "" &&
		trimmedKey == command.Key &&
		len(command.Key) <= 255 &&
		command.OrganizationID == organizationID &&
		command.Operation == operation &&
		command.SourceID == sourceID &&
		command.SourceVersion > 0 &&
		err == nil &&
		len(decodedHash) == sha256.Size
}
