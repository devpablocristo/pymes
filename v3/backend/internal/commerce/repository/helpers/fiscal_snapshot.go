package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/commerce/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func AttachAssociatedVoucher(
	noteSnapshot []byte,
	sourceVoucher domain.VoucherReference,
	sourceSnapshot []byte,
) ([]byte, error) {
	var note map[string]any
	if len(noteSnapshot) == 0 || json.Unmarshal(noteSnapshot, &note) != nil {
		return nil, fmt.Errorf("invalid note fiscal snapshot")
	}
	var source repositorymodels.FiscalSnapshot
	if len(sourceSnapshot) == 0 || json.Unmarshal(sourceSnapshot, &source) != nil ||
		source.IssueDate == "" || sourceVoucher.PointOfSale < 1 ||
		sourceVoucher.DocumentType == "" || sourceVoucher.VoucherNumber < 1 {
		return nil, fmt.Errorf("invalid source fiscal snapshot")
	}
	note["associated_voucher"] = map[string]any{
		"point_of_sale":  sourceVoucher.PointOfSale,
		"document_type":  sourceVoucher.DocumentType,
		"voucher_number": sourceVoucher.VoucherNumber,
		"issue_date":     source.IssueDate,
	}
	return json.Marshal(note)
}

func ReversalSnapshotDigest(value domain.AccountingReversal) string {
	body, _ := json.Marshal(repositorymodels.ReversalSnapshot{
		ID:           value.ID,
		DocumentKind: value.DocumentKind,
		DocumentID:   value.DocumentID,
		Reason:       value.Reason,
		EffectiveAt:  value.EffectiveAt,
	})
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func AccountingApplicationSnapshotDigest(
	value domain.PendingAccountingApplication,
) string {
	body, _ := json.Marshal(repositorymodels.AccountingApplicationSnapshot{
		ID:               value.ID,
		DebitOpenItemID:  value.DebitOpenItemID,
		CreditOpenItemID: value.CreditOpenItemID,
		Amount:           value.Amount.Amount,
		Currency:         value.Amount.Currency,
	})
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}
