// architecture:adapter external
package fakeservice

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	accountingapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/models"
	accountinghelpers "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/accounting/helpers"
	accountingmodels "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/accounting/models"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ accountingapi.ServerInterface = (*accountingFakeServer)(nil)

type accountingFakeServer struct {
	mu          sync.Mutex
	provisioned map[string]struct{}
	events      map[string]accountingapi.AccountingEvent
	periods     map[string]map[uuid.UUID]accountingapi.Period
	periodByKey map[string]uuid.UUID
}

func newAccountingFakeServer() *accountingFakeServer {
	return &accountingFakeServer{
		provisioned: make(map[string]struct{}),
		events:      make(map[string]accountingapi.AccountingEvent),
		periods:     make(map[string]map[uuid.UUID]accountingapi.Period),
		periodByKey: make(map[string]uuid.UUID),
	}
}

func (s *accountingFakeServer) AccountingHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, accountingapi.HealthStatus{Status: accountingapi.Ok})
}

func (s *accountingFakeServer) AccountingReadiness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, accountingapi.ReadinessStatus{Status: accountingapi.ReadinessStatusStatusReady})
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
	if !decodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}
	if body.OrganizationId != organizationID {
		writeProblem(w, http.StatusBadRequest, params.XCorrelationID, "VALIDATION_ERROR", "organization mismatch", "path and body organization_id must match")
		return
	}

	s.mu.Lock()
	_, found := s.provisioned[organizationID]
	s.provisioned[organizationID] = struct{}{}
	s.mu.Unlock()

	status := http.StatusCreated
	if found {
		status = http.StatusOK
	}
	writeJSON(w, status, accountingapi.ProvisioningResult{
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
	writeJSON(w, http.StatusOK, []accountingapi.Account{})
}

func (s *accountingFakeServer) SubmitPostingCommand(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.SubmitPostingCommandParams,
) {
	var body accountingapi.PostingCommand
	if !decodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateAccountingMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body.OrganizationId, body.IdempotencyKey, body.CorrelationId) {
		return
	}

	journalEntryID := stableUUID("journal-entry", organizationID, body.CommandId.String())
	openItemIDs := make([]uuid.UUID, 0)
	for index, line := range body.Lines {
		if line.OpenItem != nil && *line.OpenItem {
			openItemIDs = append(openItemIDs, stableUUID("open-item", organizationID, body.CommandId.String(), fmt.Sprint(index)))
		}
	}
	event := accountingapi.AccountingEvent{
		CommandId:      body.CommandId,
		CorrelationId:  body.CorrelationId,
		EventId:        stableUUID("event", organizationID, body.CommandId.String()),
		IdempotencyKey: body.IdempotencyKey,
		JournalEntryId: &journalEntryID,
		OccurredAt:     time.Now().UTC(),
		OpenItemIds:    &openItemIDs,
		OrganizationId: organizationID,
		SnapshotDigest: body.SnapshotDigest,
		Source:         &body.Source,
		SourceVersion:  body.SourceVersion,
		Status:         accountingapi.Posted,
	}
	s.writeAccountingEvent(w, "posting", organizationID, body.IdempotencyKey, event)
}

func (s *accountingFakeServer) ReverseJournalEntry(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.ReverseJournalEntryParams,
) {
	var body accountingapi.ReversalCommand
	if !decodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateAccountingMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body.OrganizationId, body.IdempotencyKey, body.CorrelationId) {
		return
	}

	journalEntryID := stableUUID("reversal", organizationID, body.CommandId.String())
	event := accountingapi.AccountingEvent{
		CommandId:      body.CommandId,
		CorrelationId:  body.CorrelationId,
		EventId:        stableUUID("event", organizationID, body.CommandId.String()),
		IdempotencyKey: body.IdempotencyKey,
		JournalEntryId: &journalEntryID,
		OccurredAt:     time.Now().UTC(),
		OrganizationId: organizationID,
		SnapshotDigest: body.SnapshotDigest,
		Source:         body.SourceDocumentRef,
		SourceVersion:  body.SourceVersion,
		Status:         accountingapi.Reversed,
	}
	s.writeAccountingEvent(w, "reversal", organizationID, body.IdempotencyKey, event)
}

func (s *accountingFakeServer) ApplyOpenItem(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.ApplyOpenItemParams,
) {
	var body accountingapi.OpenItemApplicationCommand
	if !decodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateAccountingMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body.OrganizationId, body.IdempotencyKey, body.CorrelationId) {
		return
	}

	applicationID := stableUUID("application", organizationID, body.CommandId.String())
	event := accountingapi.AccountingEvent{
		ApplicationId:  &applicationID,
		CommandId:      body.CommandId,
		CorrelationId:  body.CorrelationId,
		EventId:        stableUUID("event", organizationID, body.CommandId.String()),
		IdempotencyKey: body.IdempotencyKey,
		OccurredAt:     time.Now().UTC(),
		OrganizationId: organizationID,
		SnapshotDigest: body.SnapshotDigest,
		Source:         body.SourceDocumentRef,
		SourceVersion:  body.SourceVersion,
		Status:         accountingapi.Applied,
	}
	s.writeAccountingEvent(w, "application", organizationID, body.IdempotencyKey, event)
}

func (s *accountingFakeServer) ReverseOpenItemApplication(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.ReverseOpenItemApplicationParams,
) {
	var body accountingapi.OpenItemApplicationReversalCommand
	if !decodeJSON(w, r, &body, params.XCorrelationID) ||
		!validateAccountingMetadata(w, organizationID, params.IdempotencyKey, params.XCorrelationID, body.OrganizationId, body.IdempotencyKey, body.CorrelationId) {
		return
	}

	event := accountingapi.AccountingEvent{
		ApplicationId:  &body.ApplicationId,
		CommandId:      body.CommandId,
		CorrelationId:  body.CorrelationId,
		EventId:        stableUUID("event", organizationID, body.CommandId.String()),
		IdempotencyKey: body.IdempotencyKey,
		OccurredAt:     time.Now().UTC(),
		OrganizationId: organizationID,
		SnapshotDigest: body.SnapshotDigest,
		SourceVersion:  body.SourceVersion,
		Status:         accountingapi.Reversed,
	}
	s.writeAccountingEvent(w, "application-reversal", organizationID, body.IdempotencyKey, event)
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
	writeJSON(w, http.StatusOK, periods)
}

func (s *accountingFakeServer) CreatePeriod(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	params accountingapi.CreatePeriodParams,
) {
	var body accountingapi.PeriodInput
	if !decodeJSON(w, r, &body, params.XCorrelationID) {
		return
	}
	key := organizationID + "\x00period\x00" + params.IdempotencyKey

	s.mu.Lock()
	id, found := s.periodByKey[key]
	if !found {
		id = stableUUID("period", organizationID, params.IdempotencyKey)
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
	writeJSON(w, status, period)
}

func (s *accountingFakeServer) TransitionPeriod(
	w http.ResponseWriter,
	r *http.Request,
	organizationID accountingapi.OrganizationId,
	periodID openapi_types.UUID,
	params accountingapi.TransitionPeriodParams,
) {
	var body accountingapi.TransitionPeriodJSONRequestBody
	if !decodeJSON(w, r, &body, params.XCorrelationID) {
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
		writeProblem(w, http.StatusBadRequest, params.XCorrelationID, "VALIDATION_ERROR", "period not found", periodID.String())
		return
	}
	writeJSON(w, http.StatusOK, period)
}

func (s *accountingFakeServer) GetReport(
	w http.ResponseWriter,
	_ *http.Request,
	organizationID accountingapi.OrganizationId,
	report accountingapi.GetReportParamsReport,
	params accountingapi.GetReportParams,
) {
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":           params.AsOf,
		"organization_id": organizationID,
		"report":          report,
		"rows":            []any{},
	})
}

func (s *accountingFakeServer) writeAccountingEvent(
	w http.ResponseWriter,
	operation string,
	organizationID string,
	idempotencyKey string,
	event accountingapi.AccountingEvent,
) {
	key := organizationID + "\x00" + operation + "\x00" + idempotencyKey
	s.mu.Lock()
	stored, found := s.events[key]
	if !found {
		s.events[key] = event
		stored = event
	}
	s.mu.Unlock()

	status := http.StatusCreated
	if found {
		if stored.SnapshotDigest != event.SnapshotDigest || stored.CommandId != event.CommandId {
			writeProblem(
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
	writeJSON(w, status, stored)
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
	writeProblem(
		w,
		http.StatusBadRequest,
		headerCorrelationID,
		"VALIDATION_ERROR",
		"command metadata mismatch",
		"path, headers and body metadata must match",
	)
	return false
}
