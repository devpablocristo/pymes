// Package ar contains Argentina-specific fiscal rules and catalogs. It depends
// on the country-neutral fiscal domain, while the neutral domain never imports
// this package.
package ar

import (
	"errors"
	"fmt"
	"strings"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

type VoucherType int

const (
	InvoiceA    VoucherType = 1
	DebitNoteA  VoucherType = 2
	CreditNoteA VoucherType = 3
	InvoiceB    VoucherType = 6
	DebitNoteB  VoucherType = 7
	CreditNoteB VoucherType = 8
	InvoiceC    VoucherType = 11
	DebitNoteC  VoucherType = 12
	CreditNoteC VoucherType = 13
)

func (voucherType VoucherType) ValidMVP() bool {
	switch voucherType {
	case InvoiceA, DebitNoteA, CreditNoteA,
		InvoiceB, DebitNoteB, CreditNoteB,
		InvoiceC, DebitNoteC, CreditNoteC:
		return true
	default:
		return false
	}
}

func (voucherType VoucherType) Letter() (string, error) {
	switch voucherType {
	case InvoiceA, DebitNoteA, CreditNoteA:
		return "A", nil
	case InvoiceB, DebitNoteB, CreditNoteB:
		return "B", nil
	case InvoiceC, DebitNoteC, CreditNoteC:
		return "C", nil
	default:
		return "", fmt.Errorf("unsupported Argentina voucher type %d", voucherType)
	}
}

func (voucherType VoucherType) Operation() (fiscal.Operation, error) {
	switch voucherType {
	case InvoiceA, InvoiceB, InvoiceC:
		return fiscal.OperationInvoice, nil
	case CreditNoteA, CreditNoteB, CreditNoteC:
		return fiscal.OperationCreditNote, nil
	case DebitNoteA, DebitNoteB, DebitNoteC:
		return fiscal.OperationDebitNote, nil
	default:
		return "", fmt.Errorf("unsupported Argentina voucher type %d", voucherType)
	}
}

func (voucherType VoucherType) IsTypeC() bool {
	return voucherType == InvoiceC || voucherType == CreditNoteC || voucherType == DebitNoteC
}

type DocumentType int

const (
	DocumentCUIT          DocumentType = 80
	DocumentCUIL          DocumentType = 86
	DocumentDNI           DocumentType = 96
	DocumentConsumerFinal DocumentType = 99
)

type Concept int

const (
	ConceptProducts Concept = 1
	ConceptServices Concept = 2
	ConceptMixed    Concept = 3
)

func (concept Concept) Valid() bool {
	return concept == ConceptProducts || concept == ConceptServices || concept == ConceptMixed
}

func (concept Concept) NeedsServiceDates() bool {
	return concept == ConceptServices || concept == ConceptMixed
}

type VATCondition int

const (
	VATRegistered     VATCondition = 1
	VATExempt         VATCondition = 4
	VATConsumerFinal  VATCondition = 5
	VATMonotax        VATCondition = 6
	VATNotResponsible VATCondition = 15
)

func ParseVATCondition(value string) (VATCondition, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "responsable_inscripto", "responsable inscripto", "ri":
		return VATRegistered, nil
	case "exento", "iva_exento", "iva exento":
		return VATExempt, nil
	case "consumidor_final", "consumidor final", "cf":
		return VATConsumerFinal, nil
	case "monotributo", "monotributista", "responsable_monotributo":
		return VATMonotax, nil
	case "no_responsable", "no responsable":
		return VATNotResponsible, nil
	default:
		return 0, fmt.Errorf("unsupported Argentina VAT condition %q", value)
	}
}

func VoucherTypeFor(
	operation fiscal.Operation,
	issuerCondition, receiverCondition VATCondition,
) (VoucherType, error) {
	if !operation.Valid() {
		return 0, fmt.Errorf("unsupported fiscal operation %q", operation)
	}
	var invoice VoucherType
	switch issuerCondition {
	case VATRegistered:
		if receiverCondition == VATRegistered {
			invoice = InvoiceA
		} else {
			invoice = InvoiceB
		}
	case VATMonotax, VATExempt:
		invoice = InvoiceC
	default:
		return 0, fmt.Errorf("VAT condition %d cannot issue A/B/C vouchers", issuerCondition)
	}

	switch operation {
	case fiscal.OperationInvoice:
		return invoice, nil
	case fiscal.OperationCreditNote:
		switch invoice {
		case InvoiceA:
			return CreditNoteA, nil
		case InvoiceB:
			return CreditNoteB, nil
		case InvoiceC:
			return CreditNoteC, nil
		}
	case fiscal.OperationDebitNote:
		switch invoice {
		case InvoiceA:
			return DebitNoteA, nil
		case InvoiceB:
			return DebitNoteB, nil
		case InvoiceC:
			return DebitNoteC, nil
		}
	}
	return 0, errors.New("could not derive Argentina voucher type")
}

func NoteTypeFor(original VoucherType, operation fiscal.Operation) (VoucherType, error) {
	if operation != fiscal.OperationCreditNote && operation != fiscal.OperationDebitNote {
		return 0, errors.New("associated operation must be a credit or debit note")
	}
	switch original {
	case InvoiceA:
		if operation == fiscal.OperationCreditNote {
			return CreditNoteA, nil
		}
		return DebitNoteA, nil
	case InvoiceB:
		if operation == fiscal.OperationCreditNote {
			return CreditNoteB, nil
		}
		return DebitNoteB, nil
	case InvoiceC:
		if operation == fiscal.OperationCreditNote {
			return CreditNoteC, nil
		}
		return DebitNoteC, nil
	default:
		return 0, fmt.Errorf("voucher type %d is not an associable A/B/C invoice", original)
	}
}

const (
	CurrencyPES = "PES"
	CurrencyDOL = "DOL"
	CurrencyEUR = "060"
)

func CurrencyCode(value string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ARS", CurrencyPES:
		return CurrencyPES, nil
	case "USD", CurrencyDOL:
		return CurrencyDOL, nil
	case "EUR", CurrencyEUR:
		return CurrencyEUR, nil
	default:
		return "", fmt.Errorf("unsupported ARCA currency %q", value)
	}
}

const (
	VATIDZero        = 3
	VATIDTenAndHalf  = 4
	VATIDTwentyOne   = 5
	VATIDTwentySeven = 6
	VATIDFive        = 8
	VATIDTwoAndHalf  = 9
)

var vatRateIDs = map[string]int{
	"0":    VATIDZero,
	"2.5":  VATIDTwoAndHalf,
	"5":    VATIDFive,
	"10.5": VATIDTenAndHalf,
	"21":   VATIDTwentyOne,
	"27":   VATIDTwentySeven,
}

func VATIDForRate(rate fiscal.Decimal) (int, bool) {
	id, found := vatRateIDs[rate.String()]
	return id, found
}

func VATRateForID(id int) (fiscal.Decimal, bool) {
	for rate, candidate := range vatRateIDs {
		if candidate == id {
			return fiscal.MustDecimal(rate), true
		}
	}
	return fiscal.Decimal{}, false
}

func VATRegisterRateCode(rate fiscal.Decimal) (string, error) {
	if _, supported := VATIDForRate(rate); !supported {
		return "", fmt.Errorf("unsupported VAT rate %s", rate)
	}
	scaled, err := rate.ScaledInteger(2, fiscal.RoundHalfAwayFromZero)
	if err != nil {
		return "", err
	}
	if len(scaled) > 4 {
		return "", fmt.Errorf("VAT rate %s does not fit registry code", rate)
	}
	return strings.Repeat("0", 4-len(scaled)) + scaled, nil
}
