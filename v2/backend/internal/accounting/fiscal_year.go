package accounting

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FiscalYearState string

const (
	FiscalYearOpen    FiscalYearState = "open"
	FiscalYearClosing FiscalYearState = "closing"
	FiscalYearClosed  FiscalYearState = "closed"
)

func (state FiscalYearState) Valid() bool {
	return state == FiscalYearOpen ||
		state == FiscalYearClosing ||
		state == FiscalYearClosed
}

type AnnualCloseStatus string

const (
	AnnualCloseNotReady    AnnualCloseStatus = "not_ready"
	AnnualCloseReady       AnnualCloseStatus = "ready"
	AnnualCloseDraft       AnnualCloseStatus = "draft"
	AnnualClosePosted      AnnualCloseStatus = "posted"
	AnnualCloseReversed    AnnualCloseStatus = "reversed"
	AnnualCloseNotRequired AnnualCloseStatus = "not_required"
)

func (status AnnualCloseStatus) Valid() bool {
	switch status {
	case AnnualCloseNotReady,
		AnnualCloseReady,
		AnnualCloseDraft,
		AnnualClosePosted,
		AnnualCloseReversed,
		AnnualCloseNotRequired:
		return true
	default:
		return false
	}
}

type FiscalYearPeriodCounts struct {
	Open       int `json:"open"`
	SoftClosed int `json:"soft_closed"`
	Locked     int `json:"locked"`
}

func (counts FiscalYearPeriodCounts) Total() int {
	return counts.Open + counts.SoftClosed + counts.Locked
}

type FiscalYearCapabilities struct {
	CanPrepareAnnualClose bool     `json:"can_prepare_annual_close"`
	CanReopen             bool     `json:"can_reopen"`
	BlockingReasons       []string `json:"blocking_reasons,omitempty"`
}

type PeriodCapabilities struct {
	CanSoftClose bool     `json:"can_soft_close"`
	CanLock      bool     `json:"can_lock"`
	CanReopen    bool     `json:"can_reopen"`
	Targets      []string `json:"targets,omitempty"`
	Blockers     []string `json:"blockers,omitempty"`
}

type FiscalYearSummary struct {
	ID                         uuid.UUID              `json:"id"`
	Code                       string                 `json:"code"`
	StartDate                  time.Time              `json:"start_date"`
	EndDate                    time.Time              `json:"end_date"`
	IsLegacy                   bool                   `json:"is_legacy,omitempty"`
	State                      FiscalYearState        `json:"state"`
	Version                    int64                  `json:"version"`
	PeriodCounts               FiscalYearPeriodCounts `json:"period_counts"`
	AnnualCloseStatus          AnnualCloseStatus      `json:"annual_close_status"`
	AnnualCloseDraftID         *uuid.UUID             `json:"annual_close_draft_id,omitempty"`
	AnnualCloseEntryID         *uuid.UUID             `json:"annual_close_entry_id,omitempty"`
	AnnualCloseReversalEntryID *uuid.UUID             `json:"-"`
	IdempotencyKey             string                 `json:"-"`
	Capabilities               FiscalYearCapabilities `json:"capabilities"`
	CreatedBy                  string                 `json:"created_by,omitempty"`
	CreatedAt                  time.Time              `json:"created_at,omitempty"`
	UpdatedAt                  time.Time              `json:"updated_at,omitempty"`
}

func (year FiscalYearSummary) Validate() error {
	if year.ID == uuid.Nil || strings.TrimSpace(year.Code) == "" ||
		year.StartDate.IsZero() || year.EndDate.IsZero() ||
		year.EndDate.Before(year.StartDate) || year.Version <= 0 ||
		!year.State.Valid() || !year.AnnualCloseStatus.Valid() {
		return fmt.Errorf("%w: invalid fiscal year", ErrInvalidArgument)
	}
	if !year.IsLegacy && year.PeriodCounts.Total() != 12 {
		return fmt.Errorf("%w: fiscal year must contain twelve periods", ErrInvalidArgument)
	}
	return nil
}

type FiscalYearDetail struct {
	FiscalYearSummary
	Periods      []Period          `json:"periods"`
	RecentEvents []FiscalYearEvent `json:"recent_events"`
}

type PeriodDetail struct {
	Period
	RecentEvents []PeriodAudit `json:"recent_events"`
}

type FiscalYearEvent struct {
	ID           uuid.UUID      `json:"id"`
	FiscalYearID uuid.UUID      `json:"fiscal_year_id"`
	EventType    string         `json:"event_type"`
	FromStatus   string         `json:"from_status,omitempty"`
	ToStatus     string         `json:"to_status,omitempty"`
	FromVersion  *int64         `json:"from_version,omitempty"`
	ToVersion    int64          `json:"to_version"`
	ActorID      string         `json:"actor_id"`
	Reason       string         `json:"reason,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

type FiscalYearFilter struct {
	Query string
	State FiscalYearState
	After string
	Limit int
}

func (filter FiscalYearFilter) Validate() error {
	if len(strings.TrimSpace(filter.Query)) > 160 {
		return fmt.Errorf("%w: fiscal year query is too long", ErrInvalidArgument)
	}
	if filter.State != "" && !filter.State.Valid() {
		return fmt.Errorf("%w: invalid fiscal year state", ErrInvalidArgument)
	}
	if filter.Limit < 0 || filter.Limit > 200 {
		return fmt.Errorf("%w: invalid fiscal year page limit", ErrInvalidArgument)
	}
	return nil
}

func (filter FiscalYearFilter) normalized() FiscalYearFilter {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.After = strings.TrimSpace(filter.After)
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	return filter
}

type FiscalYearPage struct {
	Items      []FiscalYearSummary `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
	Total      int                 `json:"total"`
}

type CreateFiscalYearCommand struct {
	StartYear      int
	StartMonth     time.Month
	IdempotencyKey string
}

type FiscalYearAnnualClosingCommand struct {
	FiscalYearID       uuid.UUID
	ExpectedVersion    int64
	FunctionalCurrency Currency
	IdempotencyKey     string
}

type FiscalYearAnnualCloseResult struct {
	FiscalYear FiscalYearSummary `json:"fiscal_year"`
	Draft      *Draft            `json:"draft,omitempty"`
}

type ReopenFiscalYearCommand struct {
	FiscalYearID    uuid.UUID
	ExpectedVersion int64
	Reason          string
	IdempotencyKey  string
}

type FiscalYearAnnualCloseUpdate struct {
	FiscalYearID    uuid.UUID
	ExpectedVersion int64
	Status          AnnualCloseStatus
	DraftID         *uuid.UUID
	EntryID         *uuid.UUID
	ReversalEntryID *uuid.UUID
	Reason          string
	IdempotencyKey  string
}

// GenerateFiscalYear returns one exact twelve-month fiscal year and its twelve
// contiguous calendar-month periods. It deliberately has no database behavior,
// which keeps leap-year and non-calendar-year rules independently testable.
func GenerateFiscalYear(
	startYear int,
	startMonth time.Month,
	fiscalYearID uuid.UUID,
	ids IDGenerator,
) (FiscalYearSummary, []Period, error) {
	if startYear < 1900 || startYear > 9998 ||
		startMonth < time.January || startMonth > time.December ||
		fiscalYearID == uuid.Nil || ids == nil {
		return FiscalYearSummary{}, nil, fmt.Errorf(
			"%w: invalid fiscal year start",
			ErrInvalidArgument,
		)
	}
	start := time.Date(startYear, startMonth, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0).AddDate(0, 0, -1)
	code := strconv.Itoa(startYear)
	if startMonth != time.January {
		code = fmt.Sprintf("%d/%d", startYear, startYear+1)
	}
	periods := make([]Period, 0, 12)
	for index := 0; index < 12; index++ {
		periodStart := start.AddDate(0, index, 0)
		periodEnd := periodStart.AddDate(0, 1, 0).AddDate(0, 0, -1)
		yearID := fiscalYearID
		periods = append(periods, Period{
			ID:           ids.NewID(),
			Name:         periodStart.Format("2006-01"),
			StartDate:    periodStart,
			EndDate:      periodEnd,
			Status:       PeriodOpen,
			Version:      1,
			FiscalYearID: &yearID,
			SequenceNo:   index + 1,
		})
	}
	year := FiscalYearSummary{
		ID:                fiscalYearID,
		Code:              code,
		StartDate:         start,
		EndDate:           end,
		State:             FiscalYearOpen,
		Version:           1,
		PeriodCounts:      FiscalYearPeriodCounts{Open: 12},
		AnnualCloseStatus: AnnualCloseNotReady,
	}
	return year, periods, nil
}

func DeriveFiscalYearState(
	counts FiscalYearPeriodCounts,
	annualStatus AnnualCloseStatus,
) FiscalYearState {
	if counts.Open > 0 {
		return FiscalYearOpen
	}
	if counts.Total() > 0 &&
		counts.Locked == counts.Total() &&
		(annualStatus == AnnualClosePosted ||
			annualStatus == AnnualCloseNotRequired) {
		return FiscalYearClosed
	}
	return FiscalYearClosing
}

func DeriveFiscalYearCapabilities(
	state FiscalYearState,
	counts FiscalYearPeriodCounts,
	annualStatus AnnualCloseStatus,
) FiscalYearCapabilities {
	capabilities := FiscalYearCapabilities{
		CanPrepareAnnualClose: counts.Total() > 0 &&
			counts.Locked == counts.Total()-1 &&
			counts.SoftClosed == 1 &&
			annualStatus != AnnualCloseDraft &&
			annualStatus != AnnualClosePosted,
		CanReopen: state == FiscalYearClosed,
	}
	if !capabilities.CanPrepareAnnualClose {
		capabilities.BlockingReasons = append(
			capabilities.BlockingReasons,
			"annual_close_sequence",
		)
	}
	return capabilities
}
