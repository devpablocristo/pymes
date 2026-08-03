// architecture:adapter external
package fakeservice

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	accountingapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/models"
	accountinghelpers "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/accounting/helpers"
	accountingmodels "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/accounting/models"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ accountingapi.ServerInterface = (*accountingFakeServer)(nil)

type accountingFakeServer struct {
	mu            sync.Mutex
	provisioned   map[string]bool
	events        map[string]accountingapi.AccountingEvent
	periods       map[string]map[uuid.UUID]accountingapi.Period
	periodByKey   map[string]uuid.UUID
	reportPages   []accountingmodels.ReportPageRequest
	reportEntries map[string]map[uuid.UUID]accountingapi.GeneralLedgerEntry
}

func newAccountingFakeServer() *accountingFakeServer {
	return &accountingFakeServer{
		provisioned:   make(map[string]bool),
		events:        make(map[string]accountingapi.AccountingEvent),
		periods:       make(map[string]map[uuid.UUID]accountingapi.Period),
		periodByKey:   make(map[string]uuid.UUID),
		reportEntries: make(map[string]map[uuid.UUID]accountingapi.GeneralLedgerEntry),
	}
}

func (s *accountingFakeServer) AccountingHealth(w http.ResponseWriter, _ *http.Request) {
	accountinghelpers.WriteJSON(w, http.StatusOK, accountingapi.HealthStatus{Status: accountingapi.Ok})
}

func (s *accountingFakeServer) AccountingReadiness(w http.ResponseWriter, _ *http.Request) {
	accountinghelpers.WriteJSON(w, http.StatusOK, accountingapi.ReadinessStatus{Status: accountingapi.ReadinessStatusStatusReady})
}

func (s *accountingFakeServer) AccountingMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("pymes_accounting_fake_ready 1\n"))
}

func (s *accountingFakeServer) ProvisionOrganization(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.ProvisionOrganizationParams,
) {
	var body accountingapi.ProvisionOrganizationJSONRequestBody
	if !accountinghelpers.DecodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}
	if body.OrganizationId != organizationID {
		accountinghelpers.WriteProblem(w, http.StatusBadRequest, params.XCorrelationID, "VALIDATION_ERROR", "organization mismatch", "path and body organization_id must match")
		return
	}

	s.mu.Lock()
	_, found := s.provisioned[organizationID]
	s.provisioned[organizationID] = true
	s.mu.Unlock()

	status := http.StatusCreated
	if found {
		status = http.StatusOK
	}
	accountinghelpers.WriteJSON(w, status, accountingapi.ProvisioningResult{
		OrganizationId: organizationID,
		Status:         accountingapi.ProvisioningResultStatusReady,
	})
}

func (s *accountingFakeServer) ListAccounts(
	w http.ResponseWriter,
	_ *http.Request,
	_ accountingapi.OrganizationId,
	_ accountingapi.ListAccountsParams,
) {
	accountinghelpers.WriteJSON(w, http.StatusOK, []accountingapi.Account{})
}

func (s *accountingFakeServer) SubmitPostingCommand(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.SubmitPostingCommandParams,
) {
	var body accountingapi.PostingCommand
	if !accountinghelpers.DecodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateAccountingMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body.OrganizationId, body.IdempotencyKey, body.CorrelationId) {
		return
	}

	journalEntryID := accountinghelpers.StableUUID("journal-entry", organizationID, body.CommandId.String())
	openItemIDs := make([]uuid.UUID, 0)
	for index, line := range body.Lines {
		if line.OpenItem != nil && *line.OpenItem {
			openItemIDs = append(openItemIDs, accountinghelpers.StableUUID("open-item", organizationID, body.CommandId.String(), fmt.Sprint(index)))
		}
	}
	occurredAt := time.Now().UTC()
	event := accountingapi.AccountingEvent{
		CommandId:      body.CommandId,
		CorrelationId:  body.CorrelationId,
		EventId:        accountinghelpers.StableUUID("event", organizationID, body.CommandId.String()),
		IdempotencyKey: body.IdempotencyKey,
		JournalEntryId: &journalEntryID,
		OccurredAt:     occurredAt,
		OpenItemIds:    &openItemIDs,
		OrganizationId: organizationID,
		SnapshotDigest: body.SnapshotDigest,
		Source:         &body.Source,
		SourceVersion:  body.SourceVersion,
		Status:         accountingapi.Posted,
	}
	reportEntry := accountinghelpers.GeneralLedgerEntryFromPosting(
		organizationID,
		body,
		journalEntryID,
		occurredAt,
	)
	s.writeAccountingEvent(
		w,
		"posting",
		organizationID,
		body.IdempotencyKey,
		event,
		&reportEntry,
	)
}

func (s *accountingFakeServer) ReverseJournalEntry(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.ReverseJournalEntryParams,
) {
	var body accountingapi.ReversalCommand
	if !accountinghelpers.DecodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateAccountingMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body.OrganizationId, body.IdempotencyKey, body.CorrelationId) {
		return
	}

	journalEntryID := accountinghelpers.StableUUID("reversal", organizationID, body.CommandId.String())
	event := accountingapi.AccountingEvent{
		CommandId:      body.CommandId,
		CorrelationId:  body.CorrelationId,
		EventId:        accountinghelpers.StableUUID("event", organizationID, body.CommandId.String()),
		IdempotencyKey: body.IdempotencyKey,
		JournalEntryId: &journalEntryID,
		OccurredAt:     time.Now().UTC(),
		OrganizationId: organizationID,
		SnapshotDigest: body.SnapshotDigest,
		Source:         body.SourceDocumentRef,
		SourceVersion:  body.SourceVersion,
		Status:         accountingapi.Reversed,
	}
	s.writeAccountingEvent(w, "reversal", organizationID, body.IdempotencyKey, event, nil)
}

func (s *accountingFakeServer) ApplyOpenItem(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.ApplyOpenItemParams,
) {
	var body accountingapi.OpenItemApplicationCommand
	if !accountinghelpers.DecodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateAccountingMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body.OrganizationId, body.IdempotencyKey, body.CorrelationId) {
		return
	}

	applicationID := accountinghelpers.StableUUID("application", organizationID, body.CommandId.String())
	event := accountingapi.AccountingEvent{
		ApplicationId:  &applicationID,
		CommandId:      body.CommandId,
		CorrelationId:  body.CorrelationId,
		EventId:        accountinghelpers.StableUUID("event", organizationID, body.CommandId.String()),
		IdempotencyKey: body.IdempotencyKey,
		OccurredAt:     time.Now().UTC(),
		OrganizationId: organizationID,
		SnapshotDigest: body.SnapshotDigest,
		Source:         body.SourceDocumentRef,
		SourceVersion:  body.SourceVersion,
		Status:         accountingapi.Applied,
	}
	s.writeAccountingEvent(w, "application", organizationID, body.IdempotencyKey, event, nil)
}

func (s *accountingFakeServer) ReverseOpenItemApplication(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.ReverseOpenItemApplicationParams,
) {
	var body accountingapi.OpenItemApplicationReversalCommand
	if !accountinghelpers.DecodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateAccountingMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body.OrganizationId, body.IdempotencyKey, body.CorrelationId) {
		return
	}

	event := accountingapi.AccountingEvent{
		ApplicationId:  &body.ApplicationId,
		CommandId:      body.CommandId,
		CorrelationId:  body.CorrelationId,
		EventId:        accountinghelpers.StableUUID("event", organizationID, body.CommandId.String()),
		IdempotencyKey: body.IdempotencyKey,
		OccurredAt:     time.Now().UTC(),
		OrganizationId: organizationID,
		SnapshotDigest: body.SnapshotDigest,
		SourceVersion:  body.SourceVersion,
		Status:         accountingapi.Reversed,
	}
	s.writeAccountingEvent(
		w,
		"application-reversal",
		organizationID,
		body.IdempotencyKey,
		event,
		nil,
	)
}

func (s *accountingFakeServer) ListPeriods(
	w http.ResponseWriter,
	_ *http.Request,
	organizationID accountingapi.OrganizationId,
	_ accountingapi.ListPeriodsParams,
) {
	s.mu.Lock()
	periodMap := s.periods[organizationID]
	periods := make([]accountingapi.Period, 0, len(periodMap))
	for _, period := range periodMap {
		periods = append(periods, period)
	}
	s.mu.Unlock()
	accountinghelpers.WriteJSON(w, http.StatusOK, periods)
}

func (s *accountingFakeServer) CreatePeriod(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.CreatePeriodParams,
) {
	var body accountingapi.PeriodInput
	if !accountinghelpers.DecodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}
	key := organizationID + "\x00period\x00" + params.IdempotencyKey

	s.mu.Lock()
	id, found := s.periodByKey[key]
	if !found {
		id = accountinghelpers.StableUUID("period", organizationID, params.IdempotencyKey)
		s.periodByKey[key] = id
		if s.periods[organizationID] == nil {
			s.periods[organizationID] = make(map[uuid.UUID]accountingapi.Period)
		}
		s.periods[organizationID][id] = accountingapi.Period{
			EndsOn:   body.EndsOn,
			Id:       id,
			StartsOn: body.StartsOn,
			Status:   accountingapi.PeriodStatusOpen,
		}
	}
	period := s.periods[organizationID][id]
	s.mu.Unlock()

	status := http.StatusCreated
	if found {
		status = http.StatusOK
	}
	accountinghelpers.WriteJSON(w, status, period)
}

func (s *accountingFakeServer) TransitionPeriod(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	periodID openapi_types.UUID,
	params accountingapi.TransitionPeriodParams,
) {
	var body accountingapi.TransitionPeriodJSONRequestBody
	if !accountinghelpers.DecodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}

	s.mu.Lock()
	period, found := s.periods[organizationID][periodID]
	if found {
		period.Status = accountingapi.PeriodStatus(body.TargetStatus)
		s.periods[organizationID][periodID] = period
	}
	s.mu.Unlock()
	if !found {
		accountinghelpers.WriteProblem(w, http.StatusBadRequest, params.XCorrelationID, "VALIDATION_ERROR", "period not found", periodID.String())
		return
	}
	accountinghelpers.WriteJSON(w, http.StatusOK, period)
}

func (s *accountingFakeServer) GetReport(
	w http.ResponseWriter,
	_ *http.Request,
	organizationID accountingapi.OrganizationId,
	report accountingapi.GetReportParamsReport,
	params accountingapi.GetReportParams,
) {
	if report != accountingapi.GeneralLedger &&
		(params.Limit != nil || params.Cursor != nil) {
		accountinghelpers.WriteProblem(
			w,
			http.StatusBadRequest,
			params.XCorrelationID,
			"VALIDATION_ERROR",
			"pagination is only supported for general_ledger",
			"limit and cursor require report=general_ledger",
		)
		return
	}

	response := map[string]any{
		"as_of":           params.AsOf,
		"organization_id": organizationID,
		"report":          report,
		"rows":            []any{},
	}
	if report == accountingapi.GeneralLedger {
		limit := 200
		if params.Limit != nil {
			limit = *params.Limit
		}
		if limit < 1 || limit > 500 {
			accountinghelpers.WriteProblem(
				w,
				http.StatusBadRequest,
				params.XCorrelationID,
				"VALIDATION_ERROR",
				"invalid general_ledger limit",
				"limit must be between 1 and 500",
			)
			return
		}
		cursor := ""
		afterEntryID := ""
		if params.Cursor != nil {
			cursor = strings.TrimSpace(*params.Cursor)
			if cursor == "" || utf8.RuneCountInString(cursor) > 2048 {
				accountinghelpers.WriteProblem(
					w,
					http.StatusBadRequest,
					params.XCorrelationID,
					"VALIDATION_ERROR",
					"invalid general_ledger cursor",
					"cursor must contain between 1 and 2048 characters",
				)
				return
			}
			var err error
			afterEntryID, err = accountinghelpers.DecodeReportCursor(
				cursor,
				organizationID,
				params.AsOf.Time.UTC().Format("2006-01-02"),
			)
			if err != nil {
				accountinghelpers.WriteProblem(
					w,
					http.StatusUnprocessableEntity,
					params.XCorrelationID,
					"VALIDATION_ERROR",
					"invalid general_ledger cursor",
					"cursor does not match this report request",
				)
				return
			}
		}

		asOf := params.AsOf.Time.UTC().Format("2006-01-02")
		s.mu.Lock()
		s.reportPages = append(s.reportPages, accountingmodels.ReportPageRequest{
			OrganizationID: organizationID,
			Report:         string(report),
			Limit:          limit,
			Cursor:         cursor,
		})
		entries := make([]accountingapi.GeneralLedgerEntry, 0, len(s.reportEntries[organizationID]))
		for _, entry := range s.reportEntries[organizationID] {
			if entry.EntryDate.UTC().Format("2006-01-02") <= asOf {
				entries = append(entries, entry)
			}
		}
		s.mu.Unlock()
		sort.Slice(entries, func(left, right int) bool {
			if !entries[left].EntryDate.Equal(entries[right].EntryDate) {
				return entries[left].EntryDate.After(entries[right].EntryDate)
			}
			if !entries[left].CreatedAt.Equal(entries[right].CreatedAt) {
				return entries[left].CreatedAt.After(entries[right].CreatedAt)
			}
			return entries[left].Id.String() > entries[right].Id.String()
		})

		start := 0
		if afterEntryID != "" {
			found := false
			for index := range entries {
				if entries[index].Id.String() == afterEntryID {
					start = index + 1
					found = true
					break
				}
			}
			if !found {
				accountinghelpers.WriteProblem(
					w,
					http.StatusUnprocessableEntity,
					params.XCorrelationID,
					"VALIDATION_ERROR",
					"invalid general_ledger cursor",
					"cursor boundary is no longer available",
				)
				return
			}
		}
		end := start + limit
		if end > len(entries) {
			end = len(entries)
		}
		pageEntries := append([]accountingapi.GeneralLedgerEntry(nil), entries[start:end]...)
		hasMore := end < len(entries)
		generatedAt := time.Now().UTC()
		organization := string(organizationID)
		reportName := accountingapi.AccountingReportReportGeneralLedger
		reportResponse := accountingapi.AccountingReport{
			AsOf:           &params.AsOf,
			Entries:        &pageEntries,
			GeneratedAt:    &generatedAt,
			HasMore:        &hasMore,
			OrganizationId: &organization,
			Report:         &reportName,
		}
		if hasMore {
			nextCursor, err := accountinghelpers.EncodeReportCursor(
				organizationID,
				asOf,
				pageEntries[len(pageEntries)-1].Id.String(),
			)
			if err != nil {
				accountinghelpers.WriteProblem(
					w,
					http.StatusInternalServerError,
					params.XCorrelationID,
					"INTERNAL_ERROR",
					"failed to encode general_ledger cursor",
					"cursor generation failed",
				)
				return
			}
			reportResponse.NextCursor = &nextCursor
		}
		accountinghelpers.WriteJSON(w, http.StatusOK, reportResponse)
		return
	}
	accountinghelpers.WriteJSON(w, http.StatusOK, response)
}

func (s *accountingFakeServer) writeAccountingEvent(
	w http.ResponseWriter,
	operation string,
	organizationID string,
	idempotencyKey string,
	event accountingapi.AccountingEvent,
	reportEntry *accountingapi.GeneralLedgerEntry,
) {
	key := organizationID + "\x00" + operation + "\x00" + idempotencyKey
	s.mu.Lock()
	stored, found := s.events[key]
	if !found {
		s.events[key] = event
		stored = event
		if reportEntry != nil {
			if s.reportEntries[organizationID] == nil {
				s.reportEntries[organizationID] = make(
					map[uuid.UUID]accountingapi.GeneralLedgerEntry,
				)
			}
			s.reportEntries[organizationID][reportEntry.Id] = *reportEntry
		}
	}
	s.mu.Unlock()

	status := http.StatusCreated
	if found {
		if stored.SnapshotDigest != event.SnapshotDigest || stored.CommandId != event.CommandId {
			accountinghelpers.WriteProblem(
				w,
				http.StatusConflict,
				event.CorrelationId,
				"IDEMPOTENCY_KEY_REUSED",
				"idempotency key reused",
				"the key was already used with different command content",
			)
			return
		}
		status = http.StatusOK
		stored.Status = accountingapi.Duplicate
	}
	accountinghelpers.WriteJSON(w, status, stored)
}

func validateAccountingMetadata(
	w http.ResponseWriter,
	pathOrganizationID,
	headerIdempotencyKey,
	headerCorrelationID,
	bodyOrganizationID,
	bodyIdempotencyKey,
	bodyCorrelationID string,
) bool {
	if accountinghelpers.MetadataMatches(accountingmodels.Metadata{
		PathOrganizationID:   pathOrganizationID,
		HeaderIdempotencyKey: headerIdempotencyKey,
		HeaderCorrelationID:  headerCorrelationID,
		BodyOrganizationID:   bodyOrganizationID,
		BodyIdempotencyKey:   bodyIdempotencyKey,
		BodyCorrelationID:    bodyCorrelationID,
	}) {
		return true
	}
	accountinghelpers.WriteProblem(
		w,
		http.StatusBadRequest,
		headerCorrelationID,
		"VALIDATION_ERROR",
		"command metadata mismatch",
		"path, headers and body metadata must match",
	)
	return false
}
