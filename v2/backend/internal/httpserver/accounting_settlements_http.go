package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
)

func (h *IAMAPI) listAccountingOpenItems(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListAccountingOpenItemsParams,
) {
	cursor, err := decodeKeysetCursor((*string)(params.Cursor))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	var (
		cursorDate *time.Time
		cursorID   *uuid.UUID
	)
	if cursor.Sort != "" {
		parsedDate, parseErr := time.Parse("2006-01-02", cursor.Sort)
		if parseErr != nil {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid open-item cursor")
			return
		}
		parsedID, parseErr := uuid.Parse(cursor.ID)
		if parseErr != nil {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid open-item cursor")
			return
		}
		cursorDate = &parsedDate
		cursorID = &parsedID
	}
	openOnly := true
	if params.OpenOnly != nil {
		openOnly = *params.OpenOnly
	}
	limit := accountingAPILimit(params.Limit)
	var (
		items []accounting.OpenItem
		total int
	)
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			_ *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			var err error
			items, total, err = queryAccountingOpenItems(
				ctx,
				tx,
				scope.OrganizationID,
				openItemQuery{
					Kind:       stringValue((*string)(params.ItemType)),
					PartyID:    params.PartyId,
					Currency:   stringValue((*string)(params.Currency)),
					AsOf:       apiDateValue(params.AsOf),
					OpenOnly:   openOnly,
					CursorDate: cursorDate,
					CursorID:   cursorID,
					Limit:      limit + 1,
				},
			)
			return err
		},
	) {
		return
	}
	var next *string
	if len(items) > limit {
		last := items[limit-1]
		next = encodeKeysetCursor(
			last.IssueDate.Format("2006-01-02"),
			last.ID.String(),
		)
		items = items[:limit]
	}
	responseItems := make([]api.AccountingOpenItem, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, apiAccountingOpenItem(item))
	}
	writeJSON(w, http.StatusOK, api.AccountingOpenItemList{
		Items: responseItems,
		Page:  api.PageInfo{NextCursor: next, Total: total},
	})
}

func (h *IAMAPI) createAccountingSettlement(
	w http.ResponseWriter,
	r *http.Request,
	rawKey api.IdempotencyKey,
	supplier bool,
) {
	idempotencyKey, valid := validateIdempotencyKey(w, rawKey)
	if !valid {
		return
	}
	var input api.AccountingSettlementInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	amount, err := accounting.ParseAmount(string(input.Amount))
	if err != nil || amount.Sign() <= 0 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Settlement amount must be positive")
		return
	}
	exchangeRate, err := accounting.ParseExchangeRate(string(input.ExchangeRate))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Exchange rate is invalid")
		return
	}
	exchangeSource := strings.TrimSpace(input.ExchangeRateSource)
	if exchangeSource == "" ||
		input.ExchangeRateDate.Time.After(input.AccountingDate.Time) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Exchange rate date and source are invalid")
		return
	}
	paymentMethod := strings.TrimSpace(string(input.PaymentMethod))
	if !validAccountingPaymentMethod(paymentMethod) {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Payment method is invalid")
		return
	}
	sourceType := "receipt"
	expectedKind := accounting.Receivable
	if supplier {
		sourceType = "supplier_payment"
		expectedKind = accounting.Payable
	}
	fingerprint, err := accountingSettlementFingerprint(
		input,
		amount,
		exchangeRate,
		exchangeSource,
	)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		return
	}
	sourceEvent := sourceType + ".created:" + fingerprint
	var response api.JournalEntry
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			sourceID := uuid.NewSHA1(
				uuid.NameSpaceURL,
				[]byte(
					"https://pymes.local/accounting/"+
						scope.OrganizationID.String()+"/"+
						sourceType+"/"+idempotencyKey,
				),
			)
			existing, found, replayErr := loadAccountingSettlementReplay(
				ctx,
				tx,
				scope.OrganizationID,
				idempotencyKey,
				sourceType,
				sourceID,
				sourceEvent,
			)
			if replayErr != nil {
				return replayErr
			}
			if found {
				response = apiEntry(existing)
				return nil
			}
			item, err := loadAccountingOpenItemForSettlement(
				ctx,
				tx,
				scope.OrganizationID,
				input.OpenItemId,
			)
			if err != nil {
				return err
			}
			if item.Kind != expectedKind {
				return fmt.Errorf("%w: open item type does not match the operation", accounting.ErrInvalidArgument)
			}
			if input.AccountingDate.Time.Before(item.IssueDate) {
				return fmt.Errorf("%w: settlement date precedes the open item", accounting.ErrInvalidArgument)
			}
			if item.OpenAmount.Sign() <= 0 ||
				amount.Cmp(item.OpenAmount) > 0 {
				return fmt.Errorf("%w: settlement exceeds the open balance", accounting.ErrInvalidArgument)
			}
			functionalCurrency, err := loadFunctionalCurrency(ctx, tx)
			if err != nil {
				return err
			}
			if item.Currency == functionalCurrency && !exchangeRate.Equal(accounting.One) {
				return fmt.Errorf("%w: functional-currency settlement rate must be one", accounting.ErrInvalidArgument)
			}
			bookFunctional, err := settlementBookFunctionalAmount(
				item,
				amount,
				functionalCurrency,
			)
			if err != nil {
				return err
			}
			mappingList, err := service.ListAccountMappings(ctx, scope)
			if err != nil {
				return err
			}
			mappings := make(map[string]accounting.AccountMapping, len(mappingList))
			for _, mapping := range mappingList {
				mappings[mapping.Role] = mapping
			}
			event := accounting.ReceiptEvent{
				ID:                   sourceID,
				OpenItemID:           item.ID,
				PartyID:              &item.PartyID,
				Date:                 input.AccountingDate.Time,
				Currency:             item.Currency,
				FunctionalCurrency:   functionalCurrency,
				ExchangeRate:         exchangeRate,
				ExchangeRateDate:     input.ExchangeRateDate.Time,
				ExchangeRateSource:   exchangeSource,
				PaymentMethod:        paymentMethod,
				Amount:               amount,
				BookFunctionalAmount: bookFunctional,
				Actor:                scope.ActorID,
				IdempotencyKey:       idempotencyKey,
			}
			engine := accounting.NewPostingEngine(mappings)
			var plan accounting.PostingPlan
			if supplier {
				plan, err = engine.BuildSupplierPayment(
					accounting.SupplierPaymentEvent(event),
				)
			} else {
				plan, err = engine.BuildReceipt(event)
			}
			if err != nil {
				return err
			}
			plan.Entry.Source.Event = sourceEvent
			posted, err := service.PostPlan(ctx, scope, plan)
			if err != nil {
				return err
			}
			response = apiEntry(posted.Entry)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

type openItemQuery struct {
	Kind       string
	PartyID    *uuid.UUID
	Currency   string
	AsOf       *time.Time
	OpenOnly   bool
	CursorDate *time.Time
	CursorID   *uuid.UUID
	Limit      int
}

func queryAccountingOpenItems(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	filter openItemQuery,
) ([]accounting.OpenItem, int, error) {
	rows, err := tx.Query(ctx, `
		WITH filtered AS (
			SELECT
				item.id,
				item.item_type,
				item.party_id,
				item.account_id,
				item.origin_journal_entry_id,
				item.origin_journal_line_id,
				item.document_type,
				item.document_id,
				item.issued_at,
				item.due_date,
				item.currency_code,
				item.original_currency_amount::text,
				item.original_functional_amount::text,
				balance.remaining_currency_amount::text,
				balance.remaining_functional_amount::text
			  FROM accounting.open_items AS item
			  JOIN accounting.open_item_balances_as_of(
					coalesce($5::date, 'infinity'::date)
			  ) AS balance
			    ON balance.org_id = item.org_id
			   AND balance.open_item_id = item.id
			 WHERE item.org_id = $1
			   AND ($2 = '' OR item.item_type = $2)
			   AND ($3 = '' OR item.party_id = $3)
			   AND ($4 = '' OR item.currency_code = $4)
			   AND ($5::date IS NULL OR item.issued_at <= $5)
			   AND (
					NOT $6::boolean
					OR balance.remaining_currency_amount > 0
			   )
		)
		SELECT filtered.*, (SELECT count(*) FROM filtered)
		  FROM filtered
		 WHERE (
				$7::date IS NULL
				OR (filtered.issued_at, filtered.id) < ($7::date, $8::uuid)
		 )
		 ORDER BY filtered.issued_at DESC, filtered.id DESC
		 LIMIT $9
	`,
		organizationID,
		filter.Kind,
		uuidString(filter.PartyID),
		filter.Currency,
		nullableDate(filter.AsOf),
		filter.OpenOnly,
		nullableDate(filter.CursorDate),
		nullableUUID(filter.CursorID),
		filter.Limit,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]accounting.OpenItem, 0)
	total := 0
	for rows.Next() {
		item, scanErr := scanAccountingOpenItem(rows, &total)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func loadAccountingOpenItemForSettlement(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	openItemID uuid.UUID,
) (accounting.OpenItem, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			item.id,
			item.item_type,
			item.party_id,
			item.account_id,
			item.origin_journal_entry_id,
			item.origin_journal_line_id,
			item.document_type,
			item.document_id,
			item.issued_at,
			item.due_date,
			item.currency_code,
			item.original_currency_amount::text,
			item.original_functional_amount::text,
			balance.remaining_currency_amount::text,
			balance.remaining_functional_amount::text
		  FROM accounting.open_items AS item
		  JOIN accounting.open_item_balances_as_of('infinity'::date) AS balance
		    ON balance.org_id = item.org_id
		   AND balance.open_item_id = item.id
		 WHERE item.org_id = $1
		   AND item.id = $2
		 FOR UPDATE OF item
	`, organizationID, openItemID)
	item, err := scanAccountingOpenItem(row, nil)
	if errors.Is(err, pgx.ErrNoRows) {
		return accounting.OpenItem{}, accounting.ErrNotFound
	}
	return item, err
}

func scanAccountingOpenItem(
	row interface{ Scan(...any) error },
	total *int,
) (accounting.OpenItem, error) {
	var (
		item                     accounting.OpenItem
		partyID                  string
		sourceID                 string
		currencyCode             string
		originalAmount           string
		originalFunctionalAmount string
		openAmount               string
		openFunctionalAmount     string
		dueDate                  *time.Time
		scanTargets              = []any{
			&item.ID,
			&item.Kind,
			&partyID,
			&item.AccountID,
			&item.EntryID,
			&item.OriginLineID,
			&item.SourceType,
			&sourceID,
			&item.IssueDate,
			&dueDate,
			&currencyCode,
			&originalAmount,
			&originalFunctionalAmount,
			&openAmount,
			&openFunctionalAmount,
		}
	)
	if total != nil {
		scanTargets = append(scanTargets, total)
	}
	if err := row.Scan(scanTargets...); err != nil {
		return accounting.OpenItem{}, err
	}
	var err error
	item.PartyID, err = uuid.Parse(partyID)
	if err != nil {
		return accounting.OpenItem{}, fmt.Errorf("parse open-item party: %w", err)
	}
	item.SourceID, err = uuid.Parse(sourceID)
	if err != nil {
		return accounting.OpenItem{}, fmt.Errorf("parse open-item source: %w", err)
	}
	item.Currency, err = accounting.NewCurrency(currencyCode)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.OriginalAmount, err = accounting.ParseAmount(originalAmount)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.FunctionalAmount, err = accounting.ParseAmount(originalFunctionalAmount)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.OpenAmount, err = accounting.ParseAmount(openAmount)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.OpenFunctional, err = accounting.ParseAmount(openFunctionalAmount)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	if dueDate != nil {
		item.DueDate = *dueDate
	}
	return item, nil
}

func loadAccountingSettlementReplay(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	idempotencyKey string,
	sourceType string,
	sourceID uuid.UUID,
	sourceEvent string,
) (accounting.JournalEntry, bool, error) {
	var (
		entryID       uuid.UUID
		storedType    string
		storedID      string
		storedEvent   string
		storedPosting string
	)
	err := tx.QueryRow(ctx, `
		SELECT id, source_type, source_id, source_event, posting_kind
		  FROM accounting.journal_entries
		 WHERE org_id = $1
		   AND idempotency_key = $2
	`, organizationID, idempotencyKey).Scan(
		&entryID,
		&storedType,
		&storedID,
		&storedEvent,
		&storedPosting,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return accounting.JournalEntry{}, false, nil
	}
	if err != nil {
		return accounting.JournalEntry{}, false, err
	}
	if storedType != sourceType ||
		storedID != sourceID.String() ||
		storedEvent != sourceEvent ||
		storedPosting != "primary" {
		return accounting.JournalEntry{}, false, accounting.ErrIdempotencyConflict
	}
	entry, err := loadAccountingEntry(ctx, tx, entryID)
	return entry, err == nil, err
}

func accountingSettlementFingerprint(
	input api.AccountingSettlementInput,
	amount accounting.Decimal,
	exchangeRate accounting.Decimal,
	exchangeSource string,
) (string, error) {
	canonical := struct {
		OpenItemID         string `json:"open_item_id"`
		AccountingDate     string `json:"accounting_date"`
		PaymentMethod      string `json:"payment_method"`
		Amount             string `json:"amount"`
		ExchangeRate       string `json:"exchange_rate"`
		ExchangeRateDate   string `json:"exchange_rate_date"`
		ExchangeRateSource string `json:"exchange_rate_source"`
	}{
		OpenItemID:         input.OpenItemId.String(),
		AccountingDate:     input.AccountingDate.Time.Format("2006-01-02"),
		PaymentMethod:      string(input.PaymentMethod),
		Amount:             amount.String(),
		ExchangeRate:       exchangeRate.String(),
		ExchangeRateDate:   input.ExchangeRateDate.Time.Format("2006-01-02"),
		ExchangeRateSource: exchangeSource,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("canonicalize settlement: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", sum[:]), nil
}

func settlementBookFunctionalAmount(
	item accounting.OpenItem,
	amount accounting.Decimal,
	functionalCurrency accounting.Currency,
) (accounting.Decimal, error) {
	if amount.Equal(item.OpenAmount) {
		return item.OpenFunctional, nil
	}
	bookAmount, err := item.OpenFunctional.Mul(amount).Quo(
		item.OpenAmount,
		functionalCurrency.MinorUnits()+4,
	)
	if err != nil {
		return accounting.Decimal{}, err
	}
	bookAmount = bookAmount.Round(functionalCurrency.MinorUnits())
	if bookAmount.Sign() <= 0 || bookAmount.Cmp(item.OpenFunctional) > 0 {
		return accounting.Decimal{}, fmt.Errorf("%w: invalid functional settlement amount", accounting.ErrInvalidArgument)
	}
	return bookAmount, nil
}

func apiAccountingOpenItem(item accounting.OpenItem) api.AccountingOpenItem {
	var dueDate *openapi_types.Date
	if !item.DueDate.IsZero() {
		value := openapi_types.Date{Time: item.DueDate}
		dueDate = &value
	}
	return api.AccountingOpenItem{
		AccountId:                item.AccountID,
		Currency:                 item.Currency.Code(),
		DueDate:                  dueDate,
		Id:                       item.ID,
		IssuedAt:                 openapi_types.Date{Time: item.IssueDate},
		ItemType:                 api.AccountingOpenItemType(item.Kind),
		OpenAmount:               item.OpenAmount.String(),
		OpenFunctionalAmount:     item.OpenFunctional.String(),
		OriginEntryId:            item.EntryID,
		OriginalAmount:           item.OriginalAmount.String(),
		OriginalFunctionalAmount: item.FunctionalAmount.String(),
		PartyId:                  item.PartyID,
		SourceId:                 item.SourceID,
		SourceType:               item.SourceType,
	}
}

func validAccountingPaymentMethod(value string) bool {
	switch value {
	case "cash", "bank_transfer", "card", "wallet", "check":
		return true
	default:
		return false
	}
}

func apiDateValue(value *openapi_types.Date) *time.Time {
	if value == nil {
		return nil
	}
	date := value.Time
	return &date
}

func uuidString(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func nullableUUID(value *uuid.UUID) any {
	if value == nil {
		return nil
	}
	return *value
}
