package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	accountingpg "github.com/devpablocristo/pymes/v2/backend/internal/accounting/postgres"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type validatedPurchaseLine struct {
	Description   string
	Quantity      fiscal.Decimal
	Unit          string
	UnitPrice     fiscal.Decimal
	Discount      fiscal.Decimal
	TaxTreatment  api.FiscalPurchaseLineInputTaxTreatment
	VATRate       fiscal.Decimal
	Net           fiscal.Decimal
	VAT           fiscal.Decimal
	Total         fiscal.Decimal
	Inventory     bool
	accountingNet accounting.Decimal
}

type validatedPurchaseTax struct {
	Kind          api.FiscalPurchaseTaxInputKind
	DatabaseKind  string
	AuthorityCode string
	Jurisdiction  *string
	Description   string
	TaxableBase   fiscal.Decimal
	Rate          fiscal.Decimal
	Amount        fiscal.Decimal
	Creditable    bool
}

type purchaseAmounts struct {
	Net, Exempt, NonTaxed, VAT, OtherTaxes fiscal.Decimal
	Withholding, Perception, Total         fiscal.Decimal
	CreditableVAT                          fiscal.Decimal
}

type validatedPurchase struct {
	Input         api.FiscalPurchaseVoucherInput
	SupplierCUIT  string
	Currency      string
	ARCurrency    string
	ExchangeRate  fiscal.Decimal
	Lines         []validatedPurchaseLine
	Taxes         []validatedPurchaseTax
	Amounts       purchaseAmounts
	CanonicalJSON string
	SnapshotHash  string
	Association   *associatedPurchase
}

type associatedPurchase struct {
	ID                  uuid.UUID
	OpenItemID          uuid.UUID
	RemainingCurrency   accounting.Decimal
	RemainingFunctional accounting.Decimal
}

type purchaseCanonicalSnapshot struct {
	Version            int                     `json:"version"`
	CountryCode        string                  `json:"country_code"`
	Environment        string                  `json:"environment"`
	SupplierID         string                  `json:"supplier_id"`
	SupplierTaxID      string                  `json:"supplier_tax_id"`
	SupplierName       string                  `json:"supplier_name"`
	VoucherType        int                     `json:"voucher_type"`
	PointOfSale        int                     `json:"point_of_sale"`
	VoucherNumber      int64                   `json:"voucher_number"`
	IssueDate          string                  `json:"issue_date"`
	DueDate            *string                 `json:"due_date,omitempty"`
	Currency           string                  `json:"currency"`
	ExchangeRate       string                  `json:"exchange_rate"`
	ExchangeRateDate   string                  `json:"exchange_rate_date"`
	ExchangeRateSource string                  `json:"exchange_rate_source"`
	SourceType         string                  `json:"source_type"`
	SourceID           string                  `json:"source_id"`
	SourceReference    *string                 `json:"source_reference,omitempty"`
	AssociatedPurchase *string                 `json:"associated_purchase_voucher_id,omitempty"`
	Lines              []purchaseCanonicalLine `json:"lines"`
	Taxes              []purchaseCanonicalTax  `json:"taxes"`
	Totals             purchaseCanonicalTotals `json:"totals"`
}

type purchaseCanonicalLine struct {
	LineNo         int    `json:"line_no"`
	Description    string `json:"description"`
	Quantity       string `json:"quantity"`
	UnitOfMeasure  string `json:"unit_of_measure"`
	UnitPrice      string `json:"unit_price"`
	DiscountAmount string `json:"discount_amount"`
	TaxTreatment   string `json:"tax_treatment"`
	VATRate        string `json:"vat_rate"`
	NetAmount      string `json:"net_amount"`
	VATAmount      string `json:"vat_amount"`
	TotalAmount    string `json:"total_amount"`
	Inventory      bool   `json:"inventory"`
}

type purchaseCanonicalTax struct {
	LineNo        int     `json:"line_no"`
	Kind          string  `json:"kind"`
	AuthorityCode string  `json:"authority_code"`
	Jurisdiction  *string `json:"jurisdiction,omitempty"`
	Description   string  `json:"description"`
	TaxableBase   string  `json:"taxable_base"`
	Rate          string  `json:"rate"`
	Amount        string  `json:"amount"`
	Creditable    bool    `json:"creditable"`
}

type purchaseCanonicalTotals struct {
	Net           string `json:"net"`
	Exempt        string `json:"exempt"`
	NonTaxed      string `json:"non_taxed"`
	VAT           string `json:"vat"`
	CreditableVAT string `json:"creditable_vat"`
	OtherTaxes    string `json:"other_taxes"`
	Withholding   string `json:"withholding"`
	Perception    string `json:"perception"`
	Total         string `json:"total"`
}

func (h *IAMAPI) listFiscalPurchaseVouchers(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListFiscalPurchaseVouchersParams,
) {
	cursor, err := decodeKeysetCursor((*string)(params.Cursor))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var firstDay, nextMonth time.Time
	if params.Period != nil && strings.TrimSpace(*params.Period) != "" {
		_, firstDay, nextMonth, err = fiscalPeriod(*params.Period)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
			return
		}
	}
	limit := normalizedLimit((*int)(params.Limit))
	query := ""
	if params.Query != nil {
		query = strings.TrimSpace(*params.Query)
	}
	response := api.FiscalPurchaseVoucherList{Items: make([]api.FiscalPurchaseVoucher, 0)}
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalView,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			organizationID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active fiscal organization: %w", err)
			}
			if err := tx.QueryRow(ctx, `
				SELECT count(*)
				  FROM fiscal.purchase_vouchers AS purchase
				 WHERE purchase.org_id = $1
				   AND ($2::date IS NULL OR (
						purchase.issue_date >= $2::date
						AND purchase.issue_date < $3::date
				   ))
				   AND (
						$4 = ''
						OR purchase.supplier_name ILIKE '%' || $4 || '%'
						OR purchase.supplier_tax_id::text ILIKE '%' || $4 || '%'
						OR purchase.voucher_number::text ILIKE '%' || $4 || '%'
						OR coalesce(purchase.source_reference, '') ILIKE '%' || $4 || '%'
				   )`,
				organizationID,
				nullablePurchasePeriodDate(firstDay),
				nullablePurchasePeriodDate(nextMonth),
				query,
			).Scan(&response.Page.Total); err != nil {
				return fmt.Errorf("count fiscal purchase vouchers: %w", err)
			}
			rows, err := tx.Query(ctx, fiscalPurchaseVoucherSelect+`
				 WHERE purchase.org_id = $1
				   AND ($2::date IS NULL OR (
						purchase.issue_date >= $2::date
						AND purchase.issue_date < $3::date
				   ))
				   AND (
						$4 = ''
						OR purchase.supplier_name ILIKE '%' || $4 || '%'
						OR purchase.supplier_tax_id::text ILIKE '%' || $4 || '%'
						OR purchase.voucher_number::text ILIKE '%' || $4 || '%'
						OR coalesce(purchase.source_reference, '') ILIKE '%' || $4 || '%'
				   )
				   AND (
						$5 = ''
						OR (purchase.issue_date, purchase.id)
						   < ($5::date, $6::uuid)
				   )
				 ORDER BY purchase.issue_date DESC, purchase.id DESC
				 LIMIT $7`,
				organizationID,
				nullablePurchasePeriodDate(firstDay),
				nullablePurchasePeriodDate(nextMonth),
				query,
				cursor.Sort,
				nullableCursorID(cursor.ID),
				limit+1,
			)
			if err != nil {
				return fmt.Errorf("list fiscal purchase vouchers: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				item, err := scanFiscalPurchaseVoucherAPI(rows)
				if err != nil {
					return err
				}
				response.Items = append(response.Items, item)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate fiscal purchase vouchers: %w", err)
			}
			if len(response.Items) > limit {
				last := response.Items[limit-1]
				response.Items = response.Items[:limit]
				response.Page.NextCursor = encodeKeysetCursor(
					last.IssueDate.Time.Format("2006-01-02"),
					last.Id.String(),
				)
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) createFiscalPurchaseVoucher(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateFiscalPurchaseVoucherParams,
) {
	var input api.FiscalPurchaseVoucherInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	validated, err := validateFiscalPurchase(input)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var response api.FiscalPurchaseVoucher
	created := false
	if !h.withinBusinessTx(
		w,
		r,
		productiam.PermissionFiscalManage,
		func(
			ctx context.Context,
			tx pgx.Tx,
			active platformiam.ActiveMembership,
			_ clerkadapter.SessionClaims,
		) error {
			organizationID, err := uuid.Parse(active.OrganizationID)
			if err != nil {
				return fmt.Errorf("parse active fiscal organization: %w", err)
			}
			idempotencyKey := strings.TrimSpace(string(params.IdempotencyKey))
			existing, existingHash, found, err := findPurchaseByIdempotency(
				ctx, tx, organizationID, idempotencyKey,
			)
			if err != nil {
				return err
			}
			if found {
				if existingHash != validated.SnapshotHash {
					return fmt.Errorf("%w: fiscal purchase request differs", errBusinessIdempotency)
				}
				response = existing
				return nil
			}
			if err := loadAndValidateAssociatedPurchase(
				ctx,
				tx,
				organizationID,
				&validated,
			); err != nil {
				return err
			}

			purchaseID := uuid.New()
			inserted, err := insertFiscalPurchase(
				ctx,
				tx,
				organizationID,
				purchaseID,
				active.UserID,
				idempotencyKey,
				validated,
			)
			if err != nil {
				return err
			}
			if !inserted {
				existing, existingHash, found, err = findPurchaseByIdempotency(
					ctx, tx, organizationID, idempotencyKey,
				)
				if err != nil {
					return err
				}
				if found {
					if existingHash != validated.SnapshotHash {
						return fmt.Errorf("%w: fiscal purchase request differs", errBusinessIdempotency)
					}
					response = existing
					return nil
				}
				return fmt.Errorf("%w: supplier voucher or source", errBusinessDuplicate)
			}
			if err := insertFiscalPurchaseDetails(
				ctx, tx, organizationID, purchaseID, validated,
			); err != nil {
				return err
			}
			journalEntryID, err := postFiscalPurchase(
				ctx,
				tx,
				organizationID,
				purchaseID,
				active.UserID,
				idempotencyKey,
				validated,
			)
			if err != nil {
				return mapAccountingError(err)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO fiscal.purchase_voucher_accounting_links (
					org_id,
					purchase_voucher_id,
					journal_entry_id,
					created_by
				)
				VALUES ($1, $2, $3, $4)`,
				organizationID, purchaseID, journalEntryID, active.UserID,
			); err != nil {
				return fmt.Errorf("link fiscal purchase accounting entry: %w", err)
			}
			response, _, err = scanPurchaseByID(ctx, tx, organizationID, purchaseID)
			created = err == nil
			return err
		},
	) {
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, response)
}

func validateFiscalPurchase(input api.FiscalPurchaseVoucherInput) (validatedPurchase, error) {
	if !input.Environment.Valid() ||
		!input.VoucherType.Valid() ||
		input.SupplierId == uuid.Nil ||
		input.SourceId == uuid.Nil ||
		input.PointOfSale < 1 ||
		input.PointOfSale > 99999 ||
		input.VoucherNumber <= 0 ||
		input.IssueDate.Time.IsZero() ||
		input.ExchangeRateDate.Time.IsZero() ||
		strings.TrimSpace(input.SupplierName) == "" ||
		strings.TrimSpace(input.ExchangeRateSource) == "" ||
		len(input.Lines) == 0 {
		return validatedPurchase{}, errors.New("required purchase voucher fields are invalid")
	}
	if input.DueDate != nil && input.DueDate.Time.Before(input.IssueDate.Time) {
		return validatedPurchase{}, errors.New("due date cannot precede issue date")
	}
	cuit, err := ar.ParseCUIT(input.SupplierTaxId)
	if err != nil {
		return validatedPurchase{}, fmt.Errorf("invalid supplier CUIT: %w", err)
	}
	operation, err := ar.VoucherType(input.VoucherType).Operation()
	if err != nil {
		return validatedPurchase{}, err
	}
	switch operation {
	case fiscal.OperationInvoice:
		if input.AssociatedPurchaseVoucherId != nil {
			return validatedPurchase{}, errors.New(
				"an invoice cannot reference another purchase voucher",
			)
		}
	case fiscal.OperationCreditNote, fiscal.OperationDebitNote:
		if input.AssociatedPurchaseVoucherId == nil ||
			uuid.UUID(*input.AssociatedPurchaseVoucherId) == uuid.Nil {
			return validatedPurchase{}, errors.New(
				"credit and debit notes require the original purchase invoice",
			)
		}
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	arCurrency, err := ar.CurrencyCode(currency)
	if err != nil {
		return validatedPurchase{}, err
	}
	exchangeRate, err := exactPurchaseDecimal(input.ExchangeRate, 10, true)
	if err != nil {
		return validatedPurchase{}, fmt.Errorf("invalid exchange rate: %w", err)
	}
	if currency == "ARS" && !exchangeRate.Equal(fiscal.NewDecimalFromInt(1)) {
		return validatedPurchase{}, errors.New("ARS exchange rate must equal one")
	}

	result := validatedPurchase{
		Input:        input,
		SupplierCUIT: cuit.String(),
		Currency:     currency,
		ARCurrency:   arCurrency,
		ExchangeRate: exchangeRate,
		Lines:        make([]validatedPurchaseLine, 0, len(input.Lines)),
		Taxes:        make([]validatedPurchaseTax, 0, len(input.Taxes)),
	}
	lineVATByRate := make(map[string]purchaseTaxAggregate)
	for index, raw := range input.Lines {
		line, err := validatePurchaseLine(raw)
		if err != nil {
			return validatedPurchase{}, fmt.Errorf("line %d: %w", index+1, err)
		}
		result.Lines = append(result.Lines, line)
		switch line.TaxTreatment {
		case api.FiscalPurchaseLineInputTaxTreatmentTaxable:
			result.Amounts.Net = result.Amounts.Net.Add(line.Net)
		case api.FiscalPurchaseLineInputTaxTreatmentExempt:
			result.Amounts.Exempt = result.Amounts.Exempt.Add(line.Net)
		case api.FiscalPurchaseLineInputTaxTreatmentNonTaxed:
			result.Amounts.NonTaxed = result.Amounts.NonTaxed.Add(line.Net)
		}
		result.Amounts.VAT = result.Amounts.VAT.Add(line.VAT)
		if line.TaxTreatment == api.FiscalPurchaseLineInputTaxTreatmentTaxable {
			key := line.VATRate.String()
			group := lineVATByRate[key]
			group.Base = group.Base.Add(line.Net)
			group.Amount = group.Amount.Add(line.VAT)
			lineVATByRate[key] = group
		}
	}

	taxVATByRate := make(map[string]purchaseTaxAggregate)
	for index, raw := range input.Taxes {
		tax, err := validatePurchaseTax(raw)
		if err != nil {
			return validatedPurchase{}, fmt.Errorf("tax %d: %w", index+1, err)
		}
		result.Taxes = append(result.Taxes, tax)
		switch tax.Kind {
		case api.FiscalPurchaseTaxInputKindVat:
			group := taxVATByRate[tax.Rate.String()]
			group.Base = group.Base.Add(tax.TaxableBase)
			group.Amount = group.Amount.Add(tax.Amount)
			taxVATByRate[tax.Rate.String()] = group
			if tax.Creditable {
				result.Amounts.CreditableVAT = result.Amounts.CreditableVAT.Add(tax.Amount)
			}
		case api.FiscalPurchaseTaxInputKindOtherTax:
			result.Amounts.OtherTaxes = result.Amounts.OtherTaxes.Add(tax.Amount)
		case api.FiscalPurchaseTaxInputKindWithholding:
			result.Amounts.Withholding = result.Amounts.Withholding.Add(tax.Amount)
		case api.FiscalPurchaseTaxInputKindPerception:
			result.Amounts.Perception = result.Amounts.Perception.Add(tax.Amount)
		}
	}
	if err := comparePurchaseVATAggregates(lineVATByRate, taxVATByRate); err != nil {
		return validatedPurchase{}, err
	}
	result.Amounts.Total = result.Amounts.Net.
		Add(result.Amounts.Exempt).
		Add(result.Amounts.NonTaxed).
		Add(result.Amounts.VAT).
		Add(result.Amounts.OtherTaxes).
		Add(result.Amounts.Perception)
	if result.Amounts.Total.IsZero() {
		return validatedPurchase{}, errors.New("purchase voucher total must be positive")
	}
	canonical, hash, err := canonicalizePurchase(result)
	if err != nil {
		return validatedPurchase{}, err
	}
	result.CanonicalJSON, result.SnapshotHash = canonical, hash
	return result, nil
}

func loadAndValidateAssociatedPurchase(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	input *validatedPurchase,
) error {
	operation, err := ar.VoucherType(input.Input.VoucherType).Operation()
	if err != nil {
		return fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}
	if operation == fiscal.OperationInvoice {
		return nil
	}
	if input.Input.AssociatedPurchaseVoucherId == nil {
		return fmt.Errorf(
			"%w: purchase adjustment requires its original invoice",
			errBusinessInvalidRequest,
		)
	}
	associatedID := uuid.UUID(*input.Input.AssociatedPurchaseVoucherId)
	expectedInvoiceType, err := originalPurchaseInvoiceType(
		ar.VoucherType(input.Input.VoucherType),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", errBusinessInvalidRequest, err)
	}

	var (
		environment         string
		supplierID          string
		supplierTaxID       string
		voucherType         int
		issueDate           time.Time
		currency            string
		openItemID          *uuid.UUID
		remainingCurrency   *string
		remainingFunctional *string
	)
	err = tx.QueryRow(ctx, `
		SELECT
			purchase.environment,
			purchase.supplier_id,
			purchase.supplier_tax_id::text,
			purchase.voucher_type,
			purchase.issue_date,
			purchase.currency_code::text,
			open_item.open_item_id,
			open_item.remaining_currency_amount::text,
			open_item.remaining_functional_amount::text
		  FROM fiscal.purchase_vouchers AS purchase
		  LEFT JOIN LATERAL (
				SELECT
					balance.open_item_id,
					balance.remaining_currency_amount,
					balance.remaining_functional_amount
				  FROM accounting.open_item_balances_view AS balance
				 WHERE balance.org_id = purchase.org_id
				   AND balance.item_type = 'payable'
				   AND balance.document_id = purchase.id::text
				 ORDER BY balance.issued_at, balance.open_item_id
				 LIMIT 1
		  ) AS open_item ON true
		 WHERE purchase.org_id = $1
		   AND purchase.id = $2
		 FOR KEY SHARE OF purchase`,
		organizationID,
		associatedID,
	).Scan(
		&environment,
		&supplierID,
		&supplierTaxID,
		&voucherType,
		&issueDate,
		&currency,
		&openItemID,
		&remainingCurrency,
		&remainingFunctional,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: original purchase invoice", errBusinessNotFound)
	}
	if err != nil {
		return fmt.Errorf("load original purchase invoice: %w", err)
	}
	if environment != string(input.Input.Environment) ||
		supplierID != input.Input.SupplierId.String() ||
		supplierTaxID != input.SupplierCUIT ||
		voucherType != int(expectedInvoiceType) ||
		currency != input.Currency ||
		issueDate.After(input.Input.IssueDate.Time) {
		return fmt.Errorf(
			"%w: original invoice must match supplier, environment, letter, currency, and date",
			errBusinessInvalidRequest,
		)
	}
	association := &associatedPurchase{ID: associatedID}
	if operation == fiscal.OperationCreditNote {
		if openItemID == nil || remainingCurrency == nil ||
			remainingFunctional == nil {
			return fmt.Errorf(
				"%w: original purchase invoice has no payable balance",
				errBusinessInvalidTransition,
			)
		}
		association.OpenItemID = *openItemID
		association.RemainingCurrency, err = accounting.ParseAmount(*remainingCurrency)
		if err != nil {
			return fmt.Errorf("parse original payable currency balance: %w", err)
		}
		association.RemainingFunctional, err = accounting.ParseAmount(*remainingFunctional)
		if err != nil {
			return fmt.Errorf("parse original payable functional balance: %w", err)
		}
		noteAmount, parseErr := accounting.ParseAmount(input.Amounts.Total.String())
		if parseErr != nil {
			return parseErr
		}
		if association.RemainingCurrency.Sign() <= 0 ||
			association.RemainingFunctional.Sign() <= 0 ||
			noteAmount.Cmp(association.RemainingCurrency) > 0 {
			return fmt.Errorf(
				"%w: credit note exceeds the original payable balance",
				errBusinessInvalidTransition,
			)
		}
	}
	input.Association = association
	return nil
}

func originalPurchaseInvoiceType(noteType ar.VoucherType) (ar.VoucherType, error) {
	switch noteType {
	case ar.CreditNoteA, ar.DebitNoteA:
		return ar.InvoiceA, nil
	case ar.CreditNoteB, ar.DebitNoteB:
		return ar.InvoiceB, nil
	case ar.CreditNoteC, ar.DebitNoteC:
		return ar.InvoiceC, nil
	default:
		return 0, fmt.Errorf("voucher type %d is not a purchase adjustment", noteType)
	}
}

type purchaseTaxAggregate struct {
	Base, Amount fiscal.Decimal
}

func validatePurchaseLine(input api.FiscalPurchaseLineInput) (validatedPurchaseLine, error) {
	if !input.TaxTreatment.Valid() ||
		strings.TrimSpace(input.Description) == "" ||
		strings.TrimSpace(input.UnitOfMeasure) == "" {
		return validatedPurchaseLine{}, errors.New("description, unit and tax treatment are required")
	}
	quantity, err := exactPurchaseDecimal(input.Quantity, 6, true)
	if err != nil {
		return validatedPurchaseLine{}, fmt.Errorf("invalid quantity: %w", err)
	}
	unitPrice, err := exactPurchaseDecimal(input.UnitPrice, 6, false)
	if err != nil {
		return validatedPurchaseLine{}, fmt.Errorf("invalid unit price: %w", err)
	}
	discountRaw := "0"
	if input.DiscountAmount != nil {
		discountRaw = *input.DiscountAmount
	}
	discount, err := exactPurchaseDecimal(discountRaw, 6, false)
	if err != nil {
		return validatedPurchaseLine{}, fmt.Errorf("invalid discount: %w", err)
	}
	net, err := exactPurchaseDecimal(input.NetAmount, 6, false)
	if err != nil {
		return validatedPurchaseLine{}, fmt.Errorf("invalid net amount: %w", err)
	}
	vat, err := exactPurchaseDecimal(input.VatAmount, 6, false)
	if err != nil {
		return validatedPurchaseLine{}, fmt.Errorf("invalid VAT amount: %w", err)
	}
	total, err := exactPurchaseDecimal(input.TotalAmount, 6, false)
	if err != nil {
		return validatedPurchaseLine{}, fmt.Errorf("invalid total amount: %w", err)
	}
	rate, err := exactPurchaseDecimal(input.VatRate, 6, false)
	if err != nil {
		return validatedPurchaseLine{}, fmt.Errorf("invalid VAT rate: %w", err)
	}
	gross, err := quantity.Mul(unitPrice).Quantize(6, fiscal.RoundHalfAwayFromZero)
	if err != nil || !gross.Sub(discount).Equal(net) {
		return validatedPurchaseLine{}, errors.New("quantity × unit price − discount must equal net amount")
	}
	if !net.Add(vat).Equal(total) {
		return validatedPurchaseLine{}, errors.New("net amount + VAT must equal total amount")
	}
	if input.TaxTreatment != api.FiscalPurchaseLineInputTaxTreatmentTaxable {
		if !rate.IsZero() || !vat.IsZero() {
			return validatedPurchaseLine{}, errors.New("exempt and non-taxed lines cannot include VAT")
		}
	} else {
		if _, valid := ar.VATIDForRate(rate); !valid {
			return validatedPurchaseLine{}, fmt.Errorf("unsupported VAT rate %s", rate)
		}
		expectedVAT, err := net.Mul(rate).
			Quo(fiscal.NewDecimalFromInt(100), 2, fiscal.RoundHalfAwayFromZero)
		if err != nil || !expectedVAT.Equal(vat) {
			return validatedPurchaseLine{}, errors.New("VAT amount does not match net amount and rate")
		}
	}
	accountingNet, err := accounting.ParseAmount(net.String())
	if err != nil {
		return validatedPurchaseLine{}, err
	}
	inventory := input.Inventory != nil && *input.Inventory
	return validatedPurchaseLine{
		Description: strings.TrimSpace(input.Description),
		Quantity:    quantity, Unit: strings.TrimSpace(input.UnitOfMeasure),
		UnitPrice: unitPrice, Discount: discount,
		TaxTreatment: input.TaxTreatment, VATRate: rate,
		Net: net, VAT: vat, Total: total, Inventory: inventory,
		accountingNet: accountingNet,
	}, nil
}

func validatePurchaseTax(input api.FiscalPurchaseTaxInput) (validatedPurchaseTax, error) {
	if !input.Kind.Valid() ||
		strings.TrimSpace(input.AuthorityCode) == "" ||
		strings.TrimSpace(input.Description) == "" {
		return validatedPurchaseTax{}, errors.New("kind, authority code and description are required")
	}
	base, err := exactPurchaseDecimal(input.TaxableBase, 6, false)
	if err != nil {
		return validatedPurchaseTax{}, fmt.Errorf("invalid taxable base: %w", err)
	}
	rate, err := exactPurchaseDecimal(input.Rate, 6, false)
	if err != nil {
		return validatedPurchaseTax{}, fmt.Errorf("invalid rate: %w", err)
	}
	amount, err := exactPurchaseDecimal(input.Amount, 6, false)
	if err != nil {
		return validatedPurchaseTax{}, fmt.Errorf("invalid amount: %w", err)
	}
	creditable := input.Creditable != nil && *input.Creditable
	if creditable && input.Kind != api.FiscalPurchaseTaxInputKindVat {
		return validatedPurchaseTax{}, errors.New("only VAT can be creditable")
	}
	databaseKind := string(input.Kind)
	if input.Kind == api.FiscalPurchaseTaxInputKindOtherTax {
		databaseKind = "tribute"
	}
	if input.Kind == api.FiscalPurchaseTaxInputKindVat {
		if _, valid := ar.VATIDForRate(rate); !valid {
			return validatedPurchaseTax{}, fmt.Errorf("unsupported VAT rate %s", rate)
		}
	}
	var jurisdiction *string
	if input.Jurisdiction != nil {
		value := strings.TrimSpace(*input.Jurisdiction)
		if value == "" {
			return validatedPurchaseTax{}, errors.New("jurisdiction cannot be blank")
		}
		jurisdiction = &value
	}
	return validatedPurchaseTax{
		Kind: input.Kind, DatabaseKind: databaseKind,
		AuthorityCode: strings.TrimSpace(input.AuthorityCode),
		Jurisdiction:  jurisdiction, Description: strings.TrimSpace(input.Description),
		TaxableBase: base, Rate: rate, Amount: amount, Creditable: creditable,
	}, nil
}

func comparePurchaseVATAggregates(
	lines, taxes map[string]purchaseTaxAggregate,
) error {
	for rate, line := range lines {
		tax := taxes[rate]
		if !line.Base.Equal(tax.Base) || !line.Amount.Equal(tax.Amount) {
			return fmt.Errorf("VAT tax breakdown does not match taxable lines at rate %s", rate)
		}
	}
	for rate, tax := range taxes {
		line := lines[rate]
		if !line.Base.Equal(tax.Base) || !line.Amount.Equal(tax.Amount) {
			return fmt.Errorf("VAT tax breakdown does not match taxable lines at rate %s", rate)
		}
	}
	return nil
}

func exactPurchaseDecimal(raw string, scale int32, positive bool) (fiscal.Decimal, error) {
	value, err := fiscal.ParseDecimal(raw)
	if err != nil {
		return fiscal.Decimal{}, err
	}
	quantized, err := value.Quantize(scale, fiscal.RoundHalfAwayFromZero)
	if err != nil || !quantized.Equal(value) {
		return fiscal.Decimal{}, fmt.Errorf("value must have at most %d fractional digits", scale)
	}
	if value.IsNegative() || (positive && value.IsZero()) {
		return fiscal.Decimal{}, errors.New("value must be positive")
	}
	if scale <= 6 {
		if _, err := accounting.ParseAmount(value.String()); err != nil {
			return fiscal.Decimal{}, err
		}
	} else if _, err := accounting.ParseExchangeRate(value.String()); err != nil {
		return fiscal.Decimal{}, err
	}
	return value, nil
}

func canonicalizePurchase(input validatedPurchase) (string, string, error) {
	var dueDate *string
	if input.Input.DueDate != nil {
		value := input.Input.DueDate.Time.Format("2006-01-02")
		dueDate = &value
	}
	sourceReference := normalizedOptionalString(input.Input.SourceReference)
	var associatedPurchaseID *string
	if input.Input.AssociatedPurchaseVoucherId != nil {
		value := uuid.UUID(*input.Input.AssociatedPurchaseVoucherId).String()
		associatedPurchaseID = &value
	}
	snapshot := purchaseCanonicalSnapshot{
		Version: 1, CountryCode: "AR",
		Environment:   string(input.Input.Environment),
		SupplierID:    input.Input.SupplierId.String(),
		SupplierTaxID: input.SupplierCUIT,
		SupplierName:  strings.TrimSpace(input.Input.SupplierName),
		VoucherType:   int(input.Input.VoucherType),
		PointOfSale:   input.Input.PointOfSale,
		VoucherNumber: input.Input.VoucherNumber,
		IssueDate:     input.Input.IssueDate.Time.Format("2006-01-02"),
		DueDate:       dueDate, Currency: input.Currency,
		ExchangeRate:       input.ExchangeRate.String(),
		ExchangeRateDate:   input.Input.ExchangeRateDate.Time.Format("2006-01-02"),
		ExchangeRateSource: strings.TrimSpace(input.Input.ExchangeRateSource),
		SourceType:         "purchase", SourceID: input.Input.SourceId.String(),
		SourceReference:    sourceReference,
		AssociatedPurchase: associatedPurchaseID,
		Lines:              make([]purchaseCanonicalLine, 0, len(input.Lines)),
		Taxes:              make([]purchaseCanonicalTax, 0, len(input.Taxes)),
		Totals: purchaseCanonicalTotals{
			Net: input.Amounts.Net.String(), Exempt: input.Amounts.Exempt.String(),
			NonTaxed: input.Amounts.NonTaxed.String(), VAT: input.Amounts.VAT.String(),
			CreditableVAT: input.Amounts.CreditableVAT.String(),
			OtherTaxes:    input.Amounts.OtherTaxes.String(),
			Withholding:   input.Amounts.Withholding.String(),
			Perception:    input.Amounts.Perception.String(), Total: input.Amounts.Total.String(),
		},
	}
	for index, line := range input.Lines {
		snapshot.Lines = append(snapshot.Lines, purchaseCanonicalLine{
			LineNo: index + 1, Description: line.Description,
			Quantity: line.Quantity.String(), UnitOfMeasure: line.Unit,
			UnitPrice: line.UnitPrice.String(), DiscountAmount: line.Discount.String(),
			TaxTreatment: string(line.TaxTreatment), VATRate: line.VATRate.String(),
			NetAmount: line.Net.String(), VATAmount: line.VAT.String(),
			TotalAmount: line.Total.String(), Inventory: line.Inventory,
		})
	}
	for index, tax := range input.Taxes {
		snapshot.Taxes = append(snapshot.Taxes, purchaseCanonicalTax{
			LineNo: index + 1, Kind: string(tax.Kind),
			AuthorityCode: tax.AuthorityCode, Jurisdiction: tax.Jurisdiction,
			Description: tax.Description, TaxableBase: tax.TaxableBase.String(),
			Rate: tax.Rate.String(), Amount: tax.Amount.String(), Creditable: tax.Creditable,
		})
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "", "", fmt.Errorf("marshal fiscal purchase snapshot: %w", err)
	}
	digest := sha256.Sum256(raw)
	return string(raw), hex.EncodeToString(digest[:]), nil
}

func insertFiscalPurchase(
	ctx context.Context,
	tx pgx.Tx,
	organizationID, purchaseID uuid.UUID,
	actor, idempotencyKey string,
	input validatedPurchase,
) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO fiscal.purchase_vouchers (
			org_id, id, environment,
			supplier_id, supplier_tax_id, supplier_name,
			voucher_type, point_of_sale, voucher_number,
			issue_date, due_date,
			currency_code, exchange_rate, exchange_rate_date, exchange_rate_source,
			net_amount, exempt_amount, non_taxed_amount, vat_amount,
			other_taxes_amount, withholding_amount, perception_amount, total_amount,
			source_type, source_id, source_reference, associated_purchase_voucher_id,
			idempotency_key, canonical_json, snapshot_sha256, created_by
		)
		VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8, $9,
			$10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19,
			$20, $21, $22, $23,
			'purchase', $24, $25, $26,
			$27, $28, $29, $30
		)
		ON CONFLICT DO NOTHING`,
		organizationID, purchaseID, string(input.Input.Environment),
		input.Input.SupplierId.String(), input.SupplierCUIT,
		strings.TrimSpace(input.Input.SupplierName),
		int(input.Input.VoucherType), input.Input.PointOfSale, input.Input.VoucherNumber,
		input.Input.IssueDate.Time, nullablePurchaseDate(input.Input.DueDate),
		input.Currency, input.ExchangeRate.String(), input.Input.ExchangeRateDate.Time,
		strings.TrimSpace(input.Input.ExchangeRateSource),
		input.Amounts.Net.String(), input.Amounts.Exempt.String(),
		input.Amounts.NonTaxed.String(), input.Amounts.VAT.String(),
		input.Amounts.OtherTaxes.String(), input.Amounts.Withholding.String(),
		input.Amounts.Perception.String(), input.Amounts.Total.String(),
		input.Input.SourceId.String(), normalizedOptionalString(input.Input.SourceReference),
		input.Input.AssociatedPurchaseVoucherId,
		idempotencyKey, input.CanonicalJSON, input.SnapshotHash, actor,
	)
	if err != nil {
		return false, fmt.Errorf("insert fiscal purchase voucher: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func insertFiscalPurchaseDetails(
	ctx context.Context,
	tx pgx.Tx,
	organizationID, purchaseID uuid.UUID,
	input validatedPurchase,
) error {
	for index, line := range input.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fiscal.purchase_voucher_lines (
				org_id, purchase_voucher_id, line_no,
				description, quantity, unit_of_measure, unit_price, discount_amount,
				tax_treatment, vat_rate, net_amount, vat_amount, total_amount
			)
			VALUES (
				$1, $2, $3,
				$4, $5, $6, $7, $8,
				$9, $10, $11, $12, $13
			)`,
			organizationID, purchaseID, index+1,
			line.Description, line.Quantity.String(), line.Unit, line.UnitPrice.String(),
			line.Discount.String(), string(line.TaxTreatment), line.VATRate.String(),
			line.Net.String(), line.VAT.String(), line.Total.String(),
		); err != nil {
			return fmt.Errorf("insert fiscal purchase line %d: %w", index+1, err)
		}
	}
	for index, tax := range input.Taxes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fiscal.purchase_voucher_taxes (
				org_id, purchase_voucher_id, line_no, tax_type,
				authority_code, jurisdiction, description,
				taxable_base, rate, amount, creditable
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			organizationID, purchaseID, index+1, tax.DatabaseKind,
			tax.AuthorityCode, tax.Jurisdiction, tax.Description,
			tax.TaxableBase.String(), tax.Rate.String(), tax.Amount.String(), tax.Creditable,
		); err != nil {
			return fmt.Errorf("insert fiscal purchase tax %d: %w", index+1, err)
		}
	}
	return nil
}

func postFiscalPurchase(
	ctx context.Context,
	tx pgx.Tx,
	organizationID, purchaseID uuid.UUID,
	actor, idempotencyKey string,
	input validatedPurchase,
) (uuid.UUID, error) {
	transactor, err := accountingpg.NewTxTransactor(tx)
	if err != nil {
		return uuid.Nil, err
	}
	service, err := accounting.NewService(transactor)
	if err != nil {
		return uuid.Nil, err
	}
	scope := accounting.Scope{
		OrganizationID: organizationID, ActorID: actor,
		CanManageAccounting: true,
	}
	mappings, err := service.ListAccountMappings(ctx, scope)
	if err != nil {
		return uuid.Nil, err
	}
	mappingByRole := make(map[string]accounting.AccountMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByRole[mapping.Role] = mapping
	}
	functionalCode := "ARS"
	err = tx.QueryRow(ctx, `
		SELECT functional_currency::text
		  FROM accounting.organization_settings
		 WHERE org_id = $1`,
		organizationID,
	).Scan(&functionalCode)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("load accounting functional currency: %w", err)
	}
	currency, err := accounting.NewCurrency(input.Currency)
	if err != nil {
		return uuid.Nil, err
	}
	functionalCurrency, err := accounting.NewCurrency(functionalCode)
	if err != nil {
		return uuid.Nil, err
	}
	exchangeRate, err := accounting.ParseExchangeRate(input.ExchangeRate.String())
	if err != nil {
		return uuid.Nil, err
	}
	subtotalFiscal := input.Amounts.Total.Sub(input.Amounts.CreditableVAT)
	subtotal, err := accounting.ParseAmount(subtotalFiscal.String())
	if err != nil {
		return uuid.Nil, err
	}
	creditableVAT, err := accounting.ParseAmount(input.Amounts.CreditableVAT.String())
	if err != nil {
		return uuid.Nil, err
	}
	total, err := accounting.ParseAmount(input.Amounts.Total.String())
	if err != nil {
		return uuid.Nil, err
	}
	lines := make([]accounting.PurchaseLine, 0, len(input.Lines)+1)
	var representedSubtotal fiscal.Decimal
	creditableRates := make(map[string]bool)
	for _, tax := range input.Taxes {
		if tax.Kind == api.FiscalPurchaseTaxInputKindVat && tax.Creditable && !tax.Amount.IsZero() {
			creditableRates[tax.Rate.String()] = true
		}
	}
	for _, line := range input.Lines {
		taxRate := accounting.Zero
		if creditableRates[line.VATRate.String()] {
			taxRate, err = accounting.ParseAmount(line.VATRate.String())
			if err != nil {
				return uuid.Nil, err
			}
		}
		lines = append(lines, accounting.PurchaseLine{
			NetAmount:   line.accountingNet,
			TaxRate:     taxRate,
			IsInventory: line.Inventory,
		})
		representedSubtotal = representedSubtotal.Add(line.Net)
	}
	extra := subtotalFiscal.Sub(representedSubtotal)
	if extra.IsNegative() {
		return uuid.Nil, fmt.Errorf("%w: purchase accounting subtotal", accounting.ErrUnbalancedEntry)
	}
	if !extra.IsZero() {
		extraAmount, err := accounting.ParseAmount(extra.String())
		if err != nil {
			return uuid.Nil, err
		}
		lines = append(lines, accounting.PurchaseLine{NetAmount: extraAmount})
	}
	dueDate := input.Input.IssueDate.Time
	if input.Input.DueDate != nil {
		dueDate = input.Input.DueDate.Time
	}
	supplierID := uuid.UUID(input.Input.SupplierId)
	engine := accounting.NewPostingEngine(mappingByRole)
	plan, err := engine.BuildPurchase(accounting.PurchaseEvent{
		ID: purchaseID,
		Number: fmt.Sprintf(
			"%05d-%08d", input.Input.PointOfSale, input.Input.VoucherNumber,
		),
		Date: input.Input.IssueDate.Time, DueDate: dueDate,
		Currency: currency, FunctionalCurrency: functionalCurrency,
		ExchangeRate:       exchangeRate,
		ExchangeRateDate:   input.Input.ExchangeRateDate.Time,
		ExchangeRateSource: strings.TrimSpace(input.Input.ExchangeRateSource),
		PartyID:            &supplierID, Subtotal: subtotal,
		TaxTotal: creditableVAT, Total: total, Lines: lines,
		Actor: actor, IdempotencyKey: "fiscal-purchase:" + idempotencyKey,
	})
	if err != nil {
		return uuid.Nil, err
	}
	operation, _ := ar.VoucherType(input.Input.VoucherType).Operation()
	if operation == fiscal.OperationCreditNote {
		reversePurchasePlan(&plan)
		if err := applyPurchaseCreditNote(
			&plan,
			input.Association,
			total,
			functionalCurrency,
			mappingByRole,
			input.Input.IssueDate.Time,
			actor,
		); err != nil {
			return uuid.Nil, err
		}
	}
	posted, err := service.PostPlan(ctx, scope, plan)
	if err != nil {
		return uuid.Nil, err
	}
	return posted.Entry.ID, nil
}

func reversePurchasePlan(plan *accounting.PostingPlan) {
	plan.Entry.Kind = accounting.EntryAdjustment
	plan.Entry.Source.Event = "purchase.credit_note.received"
	plan.Entry.Description = "Nota de crédito de " + plan.Entry.Description
	for index := range plan.Entry.Lines {
		line := &plan.Entry.Lines[index]
		line.Debit, line.Credit = line.Credit, line.Debit
		line.TransactionDebit, line.TransactionCredit =
			line.TransactionCredit, line.TransactionDebit
	}
	// A supplier credit note reduces an existing payable. The contract does not
	// create an independent negative payable; its application is attached below
	// after the original purchase and open-item balance have been validated.
	plan.OpenItems = nil
}

func applyPurchaseCreditNote(
	plan *accounting.PostingPlan,
	associated *associatedPurchase,
	amount accounting.Decimal,
	functionalCurrency accounting.Currency,
	mappings map[string]accounting.AccountMapping,
	appliedAt time.Time,
	actor string,
) error {
	if plan == nil || associated == nil ||
		associated.OpenItemID == uuid.Nil ||
		amount.Sign() <= 0 ||
		associated.RemainingCurrency.Sign() <= 0 ||
		associated.RemainingFunctional.Sign() <= 0 {
		return fmt.Errorf(
			"%w: original payable balance is required",
			accounting.ErrInvalidArgument,
		)
	}
	if amount.Cmp(associated.RemainingCurrency) > 0 {
		return fmt.Errorf(
			"%w: credit note exceeds original payable balance",
			accounting.ErrConflict,
		)
	}
	bookFunctional := associated.RemainingFunctional
	if !amount.Equal(associated.RemainingCurrency) {
		proportional, err := associated.RemainingFunctional.
			Mul(amount).
			Quo(associated.RemainingCurrency, 12)
		if err != nil {
			return err
		}
		bookFunctional = functionalCurrency.Round(proportional)
	}
	if bookFunctional.Sign() <= 0 ||
		bookFunctional.Cmp(associated.RemainingFunctional) > 0 {
		return fmt.Errorf(
			"%w: invalid functional payable reduction",
			accounting.ErrInvalidArgument,
		)
	}
	payableMapping, ok := mappings[accounting.RolePayable]
	if !ok || payableMapping.AccountID == uuid.Nil {
		return fmt.Errorf(
			"%w: %s",
			accounting.ErrMappingMissing,
			accounting.RolePayable,
		)
	}
	payableLine := -1
	currentFunctional := accounting.Zero
	for index := range plan.Entry.Lines {
		line := &plan.Entry.Lines[index]
		if line.AccountID == payableMapping.AccountID && line.PartyID != nil {
			payableLine = index
			currentFunctional = line.Debit
			break
		}
	}
	if payableLine < 0 || currentFunctional.Sign() <= 0 {
		return fmt.Errorf(
			"%w: supplier payable line",
			accounting.ErrNotFound,
		)
	}
	control := &plan.Entry.Lines[payableLine]
	control.Debit = bookFunctional
	control.TransactionDebit = bookFunctional
	control.Currency = functionalCurrency
	control.ExchangeRate = accounting.One
	control.ExchangeRateDate = appliedAt
	control.ExchangeRateSource = "functional_currency"
	difference := currentFunctional.Sub(bookFunctional)
	if !difference.IsZero() {
		role := accounting.RoleFXGain
		debit, credit := accounting.Zero, difference.Abs()
		if difference.Sign() > 0 {
			role = accounting.RoleFXLoss
			debit, credit = difference, accounting.Zero
		}
		mapping, found := mappings[role]
		if !found || mapping.AccountID == uuid.Nil {
			return fmt.Errorf("%w: %s", accounting.ErrMappingMissing, role)
		}
		plan.Entry.Lines = append(plan.Entry.Lines, accounting.JournalLine{
			ID:                 uuid.New(),
			AccountID:          mapping.AccountID,
			Debit:              debit,
			Credit:             credit,
			TransactionDebit:   debit,
			TransactionCredit:  credit,
			Currency:           functionalCurrency,
			ExchangeRate:       accounting.One,
			ExchangeRateDate:   appliedAt,
			ExchangeRateSource: "functional_currency",
			Memo:               "Diferencia de cambio por nota de crédito de compra",
		})
	}
	for index := range plan.Entry.Lines {
		plan.Entry.Lines[index].LineNo = index + 1
	}
	plan.Applications = []accounting.OpenItemApplication{{
		ID:                 uuid.New(),
		OpenItemID:         associated.OpenItemID,
		AppliedAt:          appliedAt,
		Amount:             amount,
		FunctionalAmount:   bookFunctional,
		ExchangeDifference: difference,
		CreatedBy:          actor,
	}}
	return plan.Entry.ValidateForPosting()
}

const fiscalPurchaseVoucherSelect = `
	SELECT
		purchase.id,
		purchase.environment,
		purchase.supplier_id,
		purchase.supplier_tax_id::text,
		purchase.supplier_name,
		purchase.voucher_type,
		purchase.point_of_sale,
		purchase.voucher_number,
		purchase.issue_date,
		purchase.due_date,
		purchase.currency_code::text,
		purchase.exchange_rate::text,
		purchase.net_amount::text,
		purchase.exempt_amount::text,
		purchase.non_taxed_amount::text,
		purchase.vat_amount::text,
		purchase.other_taxes_amount::text,
		purchase.withholding_amount::text,
		purchase.perception_amount::text,
		purchase.total_amount::text,
		purchase.associated_purchase_voucher_id,
		purchase.version,
		accounting_link.journal_entry_id,
		purchase.created_at
	  FROM fiscal.purchase_vouchers AS purchase
	  LEFT JOIN fiscal.purchase_voucher_accounting_links AS accounting_link
	    ON accounting_link.org_id = purchase.org_id
	   AND accounting_link.purchase_voucher_id = purchase.id`

type purchaseRow interface {
	Scan(dest ...any) error
}

func scanFiscalPurchaseVoucherAPI(row purchaseRow) (api.FiscalPurchaseVoucher, error) {
	var (
		response   api.FiscalPurchaseVoucher
		supplierID string
		associated *uuid.UUID
		dueDate    *time.Time
		journalID  *uuid.UUID
		issueDate  time.Time
	)
	if err := row.Scan(
		&response.Id,
		&response.Environment,
		&supplierID,
		&response.SupplierTaxId,
		&response.SupplierName,
		&response.VoucherType,
		&response.PointOfSale,
		&response.VoucherNumber,
		&issueDate,
		&dueDate,
		&response.Currency,
		&response.ExchangeRate,
		&response.NetAmount,
		&response.ExemptAmount,
		&response.NonTaxedAmount,
		&response.VatAmount,
		&response.OtherTaxesAmount,
		&response.WithholdingAmount,
		&response.PerceptionAmount,
		&response.TotalAmount,
		&associated,
		&response.Version,
		&journalID,
		&response.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.FiscalPurchaseVoucher{}, fmt.Errorf("%w: fiscal purchase voucher", errBusinessNotFound)
		}
		return api.FiscalPurchaseVoucher{}, fmt.Errorf("scan fiscal purchase voucher: %w", err)
	}
	parsedSupplierID, err := uuid.Parse(supplierID)
	if err != nil {
		return api.FiscalPurchaseVoucher{}, fmt.Errorf("parse fiscal purchase supplier ID: %w", err)
	}
	response.SupplierId = parsedSupplierID
	if associated != nil {
		value := openapi_types.UUID(*associated)
		response.AssociatedPurchaseVoucherId = &value
	}
	response.IssueDate = openapi_types.Date{Time: issueDate}
	if dueDate != nil {
		value := openapi_types.Date{Time: *dueDate}
		response.DueDate = &value
	}
	if journalID != nil {
		value := openapi_types.UUID(*journalID)
		response.JournalEntryId = &value
	}
	return response, nil
}

func findPurchaseByIdempotency(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	idempotencyKey string,
) (api.FiscalPurchaseVoucher, string, bool, error) {
	var hash string
	response, err := scanFiscalPurchaseVoucherAPI(tx.QueryRow(
		ctx,
		fiscalPurchaseVoucherSelect+`
		 WHERE purchase.org_id = $1
		   AND purchase.idempotency_key = $2`,
		organizationID,
		idempotencyKey,
	))
	if errors.Is(err, errBusinessNotFound) {
		return api.FiscalPurchaseVoucher{}, "", false, nil
	}
	if err != nil {
		return api.FiscalPurchaseVoucher{}, "", false, err
	}
	if err := tx.QueryRow(ctx, `
		SELECT snapshot_sha256::text
		  FROM fiscal.purchase_vouchers
		 WHERE org_id = $1
		   AND id = $2`,
		organizationID, response.Id,
	).Scan(&hash); err != nil {
		return api.FiscalPurchaseVoucher{}, "", false, fmt.Errorf("load fiscal purchase hash: %w", err)
	}
	return response, hash, true, nil
}

func scanPurchaseByID(
	ctx context.Context,
	tx pgx.Tx,
	organizationID, purchaseID uuid.UUID,
) (api.FiscalPurchaseVoucher, string, error) {
	response, err := scanFiscalPurchaseVoucherAPI(tx.QueryRow(
		ctx,
		fiscalPurchaseVoucherSelect+`
		 WHERE purchase.org_id = $1
		   AND purchase.id = $2`,
		organizationID,
		purchaseID,
	))
	return response, "", err
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullablePurchaseDate(value *openapi_types.Date) any {
	if value == nil {
		return nil
	}
	return value.Time
}

func nullablePurchasePeriodDate(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func purchaseVoucherSign(voucherType int) fiscal.Decimal {
	switch ar.VoucherType(voucherType) {
	case ar.CreditNoteA, ar.CreditNoteB, ar.CreditNoteC:
		return fiscal.NewDecimalFromInt(-1)
	default:
		return fiscal.NewDecimalFromInt(1)
	}
}

type purchaseIVATaxJSON struct {
	Kind         string `json:"kind"`
	Authority    string `json:"authority"`
	Jurisdiction string `json:"jurisdiction"`
	Base         string `json:"base"`
	Rate         string `json:"rate"`
	Amount       string `json:"amount"`
	Creditable   bool   `json:"creditable"`
}

func loadIVAPurchaseRecords(
	ctx context.Context,
	tx pgx.Tx,
	firstDay, nextMonth time.Time,
	environment string,
) ([]ar.IVARecord, ivaTotals, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			purchase.issue_date,
			purchase.supplier_tax_id::text,
			purchase.supplier_name,
			purchase.voucher_type,
			purchase.point_of_sale,
			purchase.voucher_number,
			purchase.currency_code::text,
			purchase.exchange_rate::text,
			purchase.total_amount::text,
			purchase.non_taxed_amount::text,
			purchase.exempt_amount::text,
			purchase.vat_amount::text,
			purchase.other_taxes_amount::text,
			purchase.withholding_amount::text,
			purchase.perception_amount::text,
			coalesce((
				SELECT jsonb_agg(
					jsonb_build_object(
						'kind', tax.tax_type,
						'authority', tax.authority_code,
						'jurisdiction', coalesce(tax.jurisdiction, ''),
						'base', tax.taxable_base::text,
						'rate', tax.rate::text,
						'amount', tax.amount::text,
						'creditable', tax.creditable
					)
					ORDER BY tax.line_no
				)
				  FROM fiscal.purchase_voucher_taxes AS tax
				 WHERE tax.org_id = purchase.org_id
				   AND tax.purchase_voucher_id = purchase.id
			), '[]'::jsonb)
		  FROM fiscal.purchase_vouchers AS purchase
		 WHERE purchase.issue_date >= $1
		   AND purchase.issue_date < $2
		   AND purchase.environment = $3
		 ORDER BY
			purchase.issue_date,
			purchase.point_of_sale,
			purchase.voucher_number,
			purchase.id`,
		firstDay, nextMonth, environment,
	)
	if err != nil {
		return nil, ivaTotals{}, fmt.Errorf("query IVA Simple purchase vouchers: %w", err)
	}
	defer rows.Close()
	records := make([]ar.IVARecord, 0)
	var totals ivaTotals
	for rows.Next() {
		var (
			issueDate                                       time.Time
			supplierTaxID, supplierName, currency           string
			exchangeRaw, totalRaw, untaxedRaw, exemptRaw    string
			vatRaw, otherRaw, withholdingRaw, perceptionRaw string
			voucherType, pointOfSale                        int
			voucherNumber                                   int64
			taxJSON                                         []byte
		)
		if err := rows.Scan(
			&issueDate,
			&supplierTaxID,
			&supplierName,
			&voucherType,
			&pointOfSale,
			&voucherNumber,
			&currency,
			&exchangeRaw,
			&totalRaw,
			&untaxedRaw,
			&exemptRaw,
			&vatRaw,
			&otherRaw,
			&withholdingRaw,
			&perceptionRaw,
			&taxJSON,
		); err != nil {
			return nil, ivaTotals{}, fmt.Errorf("scan IVA Simple purchase voucher: %w", err)
		}
		counterparty, err := ar.NewReceiverDocument(ar.DocumentCUIT, supplierTaxID)
		if err != nil {
			return nil, ivaTotals{}, fmt.Errorf("validate IVA purchase supplier: %w", err)
		}
		exchangeRate, err := fiscal.ParseDecimal(exchangeRaw)
		if err != nil {
			return nil, ivaTotals{}, err
		}
		total, err := fiscal.ParseDecimal(totalRaw)
		if err != nil {
			return nil, ivaTotals{}, err
		}
		untaxed, err := fiscal.ParseDecimal(untaxedRaw)
		if err != nil {
			return nil, ivaTotals{}, err
		}
		exempt, err := fiscal.ParseDecimal(exemptRaw)
		if err != nil {
			return nil, ivaTotals{}, err
		}
		vat, err := fiscal.ParseDecimal(vatRaw)
		if err != nil {
			return nil, ivaTotals{}, err
		}
		otherTaxes, err := fiscal.ParseDecimal(otherRaw)
		if err != nil {
			return nil, ivaTotals{}, err
		}
		withholding, err := fiscal.ParseDecimal(withholdingRaw)
		if err != nil {
			return nil, ivaTotals{}, err
		}
		perception, err := fiscal.ParseDecimal(perceptionRaw)
		if err != nil {
			return nil, ivaTotals{}, err
		}
		var taxes []purchaseIVATaxJSON
		if err := json.Unmarshal(taxJSON, &taxes); err != nil {
			return nil, ivaTotals{}, fmt.Errorf("decode IVA purchase taxes: %w", err)
		}
		record := ar.IVARecord{
			Direction: ar.IVAPurchase, Authorized: true,
			IssueDate:   issueDate.Format("2006-01-02"),
			VoucherType: ar.VoucherType(voucherType),
			PointOfSale: pointOfSale, Number: voucherNumber, NumberTo: voucherNumber,
			CounterpartyDocument: counterparty, CounterpartyName: supplierName,
			Currency: currency, ExchangeRate: exchangeRate,
			Total: total, Untaxed: untaxed, Exempt: exempt, VAT: vat,
			OtherTaxes: otherTaxes,
		}
		vatByRate := make(map[string]ar.VATBreakdown)
		rateOrder := make([]string, 0)
		for _, tax := range taxes {
			amount, err := fiscal.ParseDecimal(tax.Amount)
			if err != nil {
				return nil, ivaTotals{}, err
			}
			switch tax.Kind {
			case "vat":
				base, err := fiscal.ParseDecimal(tax.Base)
				if err != nil {
					return nil, ivaTotals{}, err
				}
				rate, err := fiscal.ParseDecimal(tax.Rate)
				if err != nil {
					return nil, ivaTotals{}, err
				}
				vatID, valid := ar.VATIDForRate(rate)
				if !valid {
					return nil, ivaTotals{}, fmt.Errorf("unsupported IVA purchase rate %s", rate)
				}
				key := rate.String()
				line, found := vatByRate[key]
				if !found {
					line.ID, line.Rate = vatID, rate
					rateOrder = append(rateOrder, key)
				}
				line.BaseAmount = line.BaseAmount.Add(base)
				line.Amount = line.Amount.Add(amount)
				vatByRate[key] = line
				if tax.Creditable {
					record.ComputableVATCredit = record.ComputableVATCredit.Add(amount)
				}
			case "perception":
				switch {
				case strings.TrimSpace(tax.Jurisdiction) != "",
					strings.Contains(strings.ToUpper(tax.Authority), "IIBB"):
					record.GrossIncomePerceptions =
						record.GrossIncomePerceptions.Add(amount)
				case strings.Contains(strings.ToUpper(tax.Authority), "IVA"):
					record.VATPerceptions = record.VATPerceptions.Add(amount)
				default:
					record.NationalPerceptions =
						record.NationalPerceptions.Add(amount)
				}
			}
		}
		for _, rate := range rateOrder {
			record.VATLines = append(record.VATLines, vatByRate[rate])
		}
		records = append(records, record)
		sign := purchaseVoucherSign(voucherType)
		netTaxed := total.
			Sub(untaxed).
			Sub(exempt).
			Sub(vat).
			Sub(otherTaxes).
			Sub(perception)
		totals.purchasesNet = totals.purchasesNet.
			Add(netTaxed.Mul(sign)).
			Add(untaxed.Mul(sign)).
			Add(exempt.Mul(sign))
		totals.inputVAT = totals.inputVAT.
			Add(record.ComputableVATCredit.Mul(sign))
		totals.withholdings = totals.withholdings.Add(withholding.Mul(sign))
		totals.perceptions = totals.perceptions.Add(perception.Mul(sign))
	}
	if err := rows.Err(); err != nil {
		return nil, ivaTotals{}, fmt.Errorf("iterate IVA Simple purchase vouchers: %w", err)
	}
	return records, totals, nil
}
