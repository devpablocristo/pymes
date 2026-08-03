package helpers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	accountingapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/models"
	accountingmodels "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/accounting/models"
	"github.com/google/uuid"
)

const reportCursorVersion = 1

func GeneralLedgerEntryFromPosting(
	organizationID string,
	body accountingapi.PostingCommand,
	journalEntryID uuid.UUID,
	occurredAt time.Time,
) accountingapi.GeneralLedgerEntry {
	tenantID := StableUUID("tenant", organizationID).String()
	lines := make([]accountingapi.GeneralLedgerEntryLine, 0, len(body.Lines))
	for index, line := range body.Lines {
		lines = append(lines, accountingapi.GeneralLedgerEntryLine{
			AccountId:      StableUUID("account", organizationID, line.AccountCode),
			BaseCredit:     line.Credit,
			BaseDebit:      line.Debit,
			CreditAmount:   line.Credit,
			Currency:       line.Currency,
			DebitAmount:    line.Debit,
			Description:    line.Memo,
			ExchangeRate:   "1",
			Id:             StableUUID("journal-entry-line", journalEntryID.String(), fmt.Sprint(index)),
			JournalEntryId: journalEntryID,
			TenantId:       tenantID,
			VatRate:        "0",
		})
	}
	reference := body.Source.Id
	sourceID := body.Source.Id
	sourceType := body.Source.Type
	postedBy := "fake-accounting"
	return accountingapi.GeneralLedgerEntry{
		CreatedAt:        occurredAt,
		CreatedBy:        postedBy,
		Description:      body.Description,
		EntryDate:        body.EffectiveAt,
		EntryNumber:      "FAKE-" + strings.ToUpper(journalEntryID.String()[:8]),
		Id:               journalEntryID,
		Lines:            lines,
		PostedAt:         &occurredAt,
		PostedBy:         &postedBy,
		Reference:        &reference,
		RequiresEvidence: false,
		SourceId:         &sourceID,
		SourceType:       &sourceType,
		Status:           accountingapi.POSTED,
		TenantId:         tenantID,
	}
}

func EncodeReportCursor(organizationID, asOf, entryID string) (string, error) {
	if strings.TrimSpace(organizationID) == "" ||
		strings.TrimSpace(asOf) == "" ||
		strings.TrimSpace(entryID) == "" {
		return "", fmt.Errorf("report cursor boundary is incomplete")
	}
	payload, err := json.Marshal(accountingmodels.ReportCursor{
		Version:        reportCursorVersion,
		OrganizationID: organizationID,
		AsOf:           asOf,
		EntryID:        entryID,
	})
	if err != nil {
		return "", fmt.Errorf("encode report cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecodeReportCursor(value, organizationID, asOf string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode report cursor: %w", err)
	}
	var payload accountingmodels.ReportCursor
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return "", fmt.Errorf("decode report cursor payload: %w", err)
	}
	if payload.Version != reportCursorVersion ||
		payload.OrganizationID != organizationID ||
		payload.AsOf != asOf ||
		strings.TrimSpace(payload.EntryID) == "" {
		return "", fmt.Errorf("report cursor does not match the request")
	}
	return payload.EntryID, nil
}
