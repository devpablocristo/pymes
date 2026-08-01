package commerce

import (
	"encoding/json"
	"testing"

	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func TestAttachAssociatedVoucherDerivesImmutableSourceIdentity(t *testing.T) {
	t.Parallel()
	raw, err := attachAssociatedVoucher(
		[]byte(`{"issue_date":"2026-07-31","currency":"ARS","totals":{"total":"12.10"}}`),
		domain.VoucherReference{PointOfSale: 3, DocumentType: "FA", VoucherNumber: 41},
		[]byte(`{"issue_date":"2026-07-01","currency":"ARS"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Associated struct {
			PointOfSale   int    `json:"point_of_sale"`
			DocumentType  string `json:"document_type"`
			VoucherNumber int    `json:"voucher_number"`
			IssueDate     string `json:"issue_date"`
		} `json:"associated_voucher"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Associated.PointOfSale != 3 ||
		snapshot.Associated.DocumentType != "FA" ||
		snapshot.Associated.VoucherNumber != 41 ||
		snapshot.Associated.IssueDate != "2026-07-01" {
		t.Fatalf("snapshot=%s", raw)
	}
}

func TestAttachAssociatedVoucherRejectsIncompleteSource(t *testing.T) {
	t.Parallel()
	if _, err := attachAssociatedVoucher(
		[]byte(`{"issue_date":"2026-07-31"}`),
		domain.VoucherReference{PointOfSale: 1, DocumentType: "FA"},
		[]byte(`{"issue_date":"2026-07-01"}`),
	); err == nil {
		t.Fatal("missing source voucher number must fail")
	}
}
