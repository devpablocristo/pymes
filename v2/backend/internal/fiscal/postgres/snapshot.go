package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/google/uuid"
)

func insertSnapshot(
	ctx context.Context,
	db DBTX,
	organizationID, voucherID, snapshotID uuid.UUID,
	snapshot fiscal.Snapshot,
	document fiscal.FiscalSnapshot,
	values normalizedSnapshot,
) error {
	issuerAddress, err := encodedAddress(document.Issuer.Address)
	if err != nil {
		return err
	}
	recipientAddress, err := encodedAddress(document.Receiver.Address)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO fiscal.voucher_snapshots (
			org_id, id, voucher_id, snapshot_version,
			issuer_tax_id, issuer_legal_name, issuer_tax_condition,
			issuer_address, issuer_activity_start_date,
			recipient_document_type, recipient_document_number,
			recipient_name, recipient_tax_condition, recipient_address,
			currency_code, exchange_rate, exchange_rate_date,
			exchange_rate_source, issue_date, service_from, service_to,
			payment_due_date, net_amount, exempt_amount, non_taxed_amount,
			vat_amount, other_tributes_amount, total_amount,
			snapshot_sha256, canonical_json
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8::jsonb, $9,
			$10, $11, $12, $13, $14::jsonb,
			$15, $16::numeric, $17, $18, $19, $20, $21, $22,
			$23::numeric, $24::numeric, $25::numeric, $26::numeric,
			$27::numeric, $28::numeric, $29, $30
		)`,
		organizationID, snapshotID, voucherID, fiscal.SnapshotVersion,
		nonEmpty(document.Issuer.TaxID, "unknown"),
		document.Issuer.Name,
		nonEmpty(document.Issuer.TaxCondition, "responsable_inscripto"),
		issuerAddress,
		values.issuerActivityDate,
		values.recipientDocType,
		values.recipientDocNumber,
		document.Receiver.Name,
		nonEmpty(document.Receiver.TaxCondition, "consumidor_final"),
		recipientAddress,
		document.Currency.Code,
		document.Currency.Rate.String(),
		values.exchangeRateDate,
		values.exchangeRateSource,
		values.issueDate,
		values.serviceFrom,
		values.serviceTo,
		values.paymentDue,
		document.Totals.NetTaxed.String(),
		document.Totals.Exempt.String(),
		document.Totals.NetUntaxed.String(),
		document.Totals.VAT.String(),
		document.Totals.OtherTaxes.String(),
		document.Totals.Total.String(),
		snapshot.Hash(),
		string(snapshot.CanonicalJSON()),
	)
	if err != nil {
		return fmt.Errorf("insert fiscal snapshot: %w", mapDatabaseError(err))
	}
	if err := insertSnapshotLines(ctx, db, organizationID, snapshotID, document.Lines); err != nil {
		return err
	}
	if err := insertSnapshotTaxes(ctx, db, organizationID, snapshotID, document); err != nil {
		return err
	}
	if document.AssociatedDocument != nil {
		associatedID, err := uuid.Parse(document.AssociatedDocument.VoucherID)
		if err != nil {
			return fmt.Errorf("parse associated fiscal voucher id: %w", err)
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO fiscal.voucher_associations (
				org_id, voucher_id, associated_voucher_id, association_type
			)
			VALUES ($1, $2, $3, 'adjusts')`,
			organizationID, voucherID, associatedID,
		); err != nil {
			return fmt.Errorf("insert fiscal voucher association: %w", mapDatabaseError(err))
		}
	}
	return nil
}

func insertSnapshotLines(
	ctx context.Context,
	db DBTX,
	organizationID, snapshotID uuid.UUID,
	lines []fiscal.FiscalLineSnapshot,
) error {
	for _, line := range lines {
		treatment := "taxable"
		net := line.NetAmount
		switch {
		case !line.ExemptAmount.IsZero():
			treatment = "exempt"
			net = line.ExemptAmount
		case !line.UntaxedAmount.IsZero():
			treatment = "non_taxed"
			net = line.UntaxedAmount
		}
		unitOfMeasure := "unit"
		_, err := db.Exec(ctx, `
			INSERT INTO fiscal.voucher_lines (
				org_id, snapshot_id, line_no, description, quantity,
				unit_of_measure, unit_price, discount_amount, tax_treatment,
				vat_rate, net_amount, vat_amount, total_amount
			)
			VALUES (
				$1, $2, $3, $4, $5::numeric,
				$6, $7::numeric, 0, $8,
				$9::numeric, $10::numeric, $11::numeric, $12::numeric
			)`,
			organizationID, snapshotID, line.Position, line.Description,
			line.Quantity.String(), unitOfMeasure, line.UnitPrice.String(),
			treatment, line.TaxRate.String(), net.String(),
			line.TaxAmount.String(), line.TotalAmount.String(),
		)
		if err != nil {
			return fmt.Errorf("insert fiscal snapshot line %d: %w", line.Position, mapDatabaseError(err))
		}
	}
	return nil
}

func insertSnapshotTaxes(
	ctx context.Context,
	db DBTX,
	organizationID, snapshotID uuid.UUID,
	document fiscal.FiscalSnapshot,
) error {
	lineNumber := 1
	otherTaxes := fiscal.Decimal{}
	for _, line := range document.Lines {
		if line.TaxAmount.IsZero() {
			continue
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO fiscal.voucher_taxes (
				org_id, snapshot_id, line_no, tax_type, authority_code,
				description, taxable_base, rate, amount
			)
			VALUES ($1, $2, $3, 'vat', $4, $5, $6::numeric, $7::numeric, $8::numeric)`,
			organizationID, snapshotID, lineNumber,
			nonEmpty(line.TaxCode, "vat"),
			nonEmpty(line.TaxCode, "VAT"),
			line.NetAmount.String(),
			line.TaxRate.String(),
			line.TaxAmount.String(),
		); err != nil {
			return fmt.Errorf("insert fiscal VAT tax %d: %w", lineNumber, mapDatabaseError(err))
		}
		lineNumber++
	}
	for _, tax := range document.Taxes {
		// VAT is represented from line-level amounts above so the database can
		// reconcile exactly once. Remaining snapshot taxes are fiscal tributes.
		if strings.EqualFold(tax.Code, "vat") || strings.HasPrefix(strings.ToUpper(tax.Code), "IVA") {
			continue
		}
		if tax.Amount.IsZero() {
			continue
		}
		otherTaxes = otherTaxes.Add(tax.Amount)
		if _, err := db.Exec(ctx, `
			INSERT INTO fiscal.voucher_taxes (
				org_id, snapshot_id, line_no, tax_type, authority_code,
				description, taxable_base, rate, amount
			)
			VALUES ($1, $2, $3, 'tribute', $4, $5, $6::numeric, $7::numeric, $8::numeric)`,
			organizationID, snapshotID, lineNumber,
			tax.Code, nonEmpty(tax.Description, tax.Code),
			tax.BaseAmount.String(), tax.Rate.String(), tax.Amount.String(),
		); err != nil {
			return fmt.Errorf("insert fiscal tribute %d: %w", lineNumber, mapDatabaseError(err))
		}
		lineNumber++
	}
	if otherTaxes.Cmp(document.Totals.OtherTaxes) < 0 {
		remainder := document.Totals.OtherTaxes.Sub(otherTaxes)
		if _, err := db.Exec(ctx, `
			INSERT INTO fiscal.voucher_taxes (
				org_id, snapshot_id, line_no, tax_type, authority_code,
				description, taxable_base, rate, amount
			)
			VALUES ($1, $2, $3, 'tribute', 'other', 'Other tributes', 0, 0, $4::numeric)`,
			organizationID, snapshotID, lineNumber, remainder.String(),
		); err != nil {
			return fmt.Errorf("insert fiscal aggregate tribute: %w", mapDatabaseError(err))
		}
	}
	if otherTaxes.Cmp(document.Totals.OtherTaxes) > 0 {
		return errors.New("fiscal snapshot taxes exceed other tax total")
	}
	return nil
}

func encodedAddress(address string) (string, error) {
	raw, err := json.Marshal(map[string]string{"formatted": strings.TrimSpace(address)})
	if err != nil {
		return "", fmt.Errorf("encode fiscal address: %w", err)
	}
	return string(raw), nil
}
