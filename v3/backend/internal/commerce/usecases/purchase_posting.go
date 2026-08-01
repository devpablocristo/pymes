package usecases

import (
	"math/big"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
)

// buildPurchasePostingCommand maps the supplier document to the minimum
// accounting vocabulary owned by Pymes: expense (5100), input VAT (1300), and
// supplier payable (2100). Functional-currency residuals caused by conversion
// are assigned to expense; VAT and the payable remain independently auditable.
func buildPurchasePostingCommand(purchase domain.Purchase) (domain.PostingCommand, error) {
	if err := purchase.ValidateAccountingAmounts(); err != nil {
		return domain.PostingCommand{}, err
	}
	effectiveAt, _ := time.Parse(time.DateOnly, purchase.IssueDate)
	total, _ := parsePostingDecimal("amount", purchase.Total.Amount, false)
	vat := new(big.Rat)
	for _, item := range purchase.VATBreakdown {
		value, _ := parsePostingDecimal("tax_amount", item.TaxAmount, true)
		vat.Add(vat, value)
	}
	rate, normalizedRate, err := normalizedPostingExchangeRate(purchase.Total.Currency, purchase.ExchangeRate)
	if err != nil {
		return domain.PostingCommand{}, err
	}

	totalMinor := roundToScale(total, postingMoneyScale)
	vatMinor := roundToScale(vat, postingMoneyScale)
	expenseMinor := new(big.Int).Sub(new(big.Int).Set(totalMinor), vatMinor)
	if expenseMinor.Sign() < 0 {
		return domain.PostingCommand{}, postingValidationError("rounded input VAT exceeds purchase total")
	}
	functionalTotal := convertMinorAmount(totalMinor, rate)
	functionalVAT := convertMinorAmount(vatMinor, rate)
	functionalExpense := new(big.Int).Sub(new(big.Int).Set(functionalTotal), functionalVAT)
	if functionalExpense.Sign() < 0 {
		return domain.PostingCommand{}, postingValidationError("functional input VAT exceeds purchase total")
	}

	zero := domain.Money{Amount: fixedDecimal(big.NewInt(0), postingMoneyScale), Currency: purchase.Total.Currency}
	expense := domain.Money{Amount: fixedDecimal(expenseMinor, postingMoneyScale), Currency: purchase.Total.Currency}
	inputVAT := domain.Money{Amount: fixedDecimal(vatMinor, postingMoneyScale), Currency: purchase.Total.Currency}
	totalMoney := domain.Money{Amount: fixedDecimal(totalMinor, postingMoneyScale), Currency: purchase.Total.Currency}
	lines := []domain.PostingLine{{
		AccountCode:      "5100",
		Debit:            expense,
		Credit:           zero,
		FunctionalAmount: fixedDecimal(functionalExpense, postingMoneyScale),
		Memo:             "Neto gravado + exento",
	}}
	if vatMinor.Sign() > 0 {
		lines = append(lines, domain.PostingLine{
			AccountCode:      "1300",
			Debit:            inputVAT,
			Credit:           zero,
			FunctionalAmount: fixedDecimal(functionalVAT, postingMoneyScale),
			Memo:             "IVA crédito fiscal",
		})
	}
	lines = append(lines, domain.PostingLine{
		AccountCode:      "2100",
		Debit:            zero,
		Credit:           totalMoney,
		FunctionalAmount: fixedDecimal(functionalTotal, postingMoneyScale),
		Memo:             "Total comprobante proveedor",
		OpenItem:         true,
		PartyRef:         purchase.SupplierRef,
	})
	sourceVersion := persistedSourceVersion(purchase.Origin)
	return domain.PostingCommand{
		CommandID:      accountingCommandID("purchase", purchase.ID, sourceVersion),
		OrganizationID: purchase.OrganizationID,
		IdempotencyKey: internalIdempotencyKey(purchase.OrganizationID, "accounting.post", purchase.ID, sourceVersion),
		SourceType:     "purchase_invoice",
		SourceID:       purchase.ID,
		SourceVersion:  sourceVersion,
		SnapshotDigest: purchase.SnapshotDigest,
		CorrelationID:  persistedCorrelationID(purchase.Origin, purchase.CorrelationID),
		EffectiveAt:    effectiveAt.UTC(),
		Description:    "Compra " + purchase.ExternalDocumentRef,
		ExchangeRate:   normalizedRate,
		Lines:          lines,
	}, nil
}
