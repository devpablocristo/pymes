package accounting

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type StatementFormat string

const (
	StatementCSV  StatementFormat = "csv"
	StatementXLSX StatementFormat = "xlsx"
	StatementOFX  StatementFormat = "ofx"
)

type StatementImport struct {
	ID                 uuid.UUID           `json:"id"`
	FinancialAccountID uuid.UUID           `json:"financial_account_id"`
	FileName           string              `json:"file_name"`
	Format             StatementFormat     `json:"format"`
	SHA256             string              `json:"sha256"`
	Currency           Currency            `json:"currency"`
	ImportedBy         string              `json:"imported_by"`
	ImportedAt         time.Time           `json:"imported_at"`
	Movements          []StatementMovement `json:"movements,omitempty"`
}

type StatementMovement struct {
	ID          uuid.UUID `json:"id"`
	ImportID    uuid.UUID `json:"import_id"`
	BookedAt    time.Time `json:"booked_at"`
	ValueAt     time.Time `json:"value_at"`
	Description string    `json:"description"`
	Reference   string    `json:"reference"`
	Amount      Decimal   `json:"amount"`
	Currency    Currency  `json:"currency"`
	Fingerprint string    `json:"fingerprint"`
}

type ReconciliationStatus string

const (
	ReconciliationOpen   ReconciliationStatus = "open"
	ReconciliationClosed ReconciliationStatus = "closed"
)

type Reconciliation struct {
	ID                 uuid.UUID             `json:"id"`
	FinancialAccountID uuid.UUID             `json:"financial_account_id"`
	PeriodStart        time.Time             `json:"period_start"`
	PeriodEnd          time.Time             `json:"period_end"`
	StatementOpening   Decimal               `json:"statement_opening"`
	StatementClosing   Decimal               `json:"statement_closing"`
	LedgerOpening      Decimal               `json:"ledger_opening"`
	LedgerClosing      Decimal               `json:"ledger_closing"`
	Status             ReconciliationStatus  `json:"status"`
	Version            int64                 `json:"version"`
	Matches            []ReconciliationMatch `json:"matches"`
	ClosedAt           *time.Time            `json:"closed_at,omitempty"`
	ClosedBy           string                `json:"closed_by,omitempty"`
	ReopenedAt         *time.Time            `json:"reopened_at,omitempty"`
	ReopenedBy         string                `json:"reopened_by,omitempty"`
	ReopenedReason     string                `json:"reopened_reason,omitempty"`
}

type ReconciliationMatch struct {
	ID                  uuid.UUID `json:"id"`
	StatementMovementID uuid.UUID `json:"statement_movement_id"`
	JournalLineID       uuid.UUID `json:"journal_line_id"`
	StatementAmount     Decimal   `json:"statement_amount"`
	LedgerAmount        Decimal   `json:"ledger_amount"`
	CreatedBy           string    `json:"created_by"`
	CreatedAt           time.Time `json:"created_at"`
}

type MatchSuggestion struct {
	StatementMovementID uuid.UUID `json:"statement_movement_id"`
	JournalLineID       uuid.UUID `json:"journal_line_id"`
	Amount              Decimal   `json:"amount"`
	Score               int       `json:"score"`
	Reasons             []string  `json:"reasons"`
}

type ReconciliationLedgerCandidate struct {
	JournalLineID uuid.UUID `json:"journal_line_id"`
	Date          time.Time `json:"date"`
	Amount        Decimal   `json:"amount"`
	Reference     string    `json:"reference"`
	Description   string    `json:"description"`
}

func NewStatementImport(
	accountID uuid.UUID,
	fileName string,
	format StatementFormat,
	content []byte,
	currency Currency,
	actor string,
	now time.Time,
) (StatementImport, error) {
	if accountID == uuid.Nil || strings.TrimSpace(fileName) == "" || len(content) == 0 ||
		strings.TrimSpace(actor) == "" {
		return StatementImport{}, fmt.Errorf("%w: incomplete statement import", ErrInvalidArgument)
	}
	hash := sha256.Sum256(content)
	statement := StatementImport{
		ID:                 uuid.New(),
		FinancialAccountID: accountID,
		FileName:           fileName,
		Format:             format,
		SHA256:             hex.EncodeToString(hash[:]),
		Currency:           currency,
		ImportedBy:         actor,
		ImportedAt:         now,
	}
	var (
		movements []StatementMovement
		err       error
	)
	switch format {
	case StatementCSV:
		movements, err = ParseStatementCSV(bytes.NewReader(content), currency)
	case StatementOFX:
		movements, err = ParseStatementOFX(content, currency)
	case StatementXLSX:
		movements, err = ParseStatementXLSX(content, currency)
	default:
		err = fmt.Errorf("%w: unsupported statement format %q", ErrInvalidArgument, format)
	}
	if err != nil {
		return StatementImport{}, err
	}
	for index := range movements {
		movements[index].ID = uuid.New()
		movements[index].ImportID = statement.ID
	}
	statement.Movements = movements
	return statement, nil
}

// ParseStatementCSV expects a header with date, description and amount.
// Optional headers are value_date, reference and currency.
func ParseStatementCSV(reader io.Reader, defaultCurrency Currency) ([]StatementMovement, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: read statement CSV: %v", ErrInvalidArgument, err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%w: statement CSV is empty", ErrInvalidArgument)
	}
	headers := make(map[string]int, len(records[0]))
	for index, header := range records[0] {
		headers[normalizeHeader(header)] = index
	}
	dateColumn, hasDate := firstHeader(headers, "date", "fecha", "booked_at")
	descriptionColumn, hasDescription := firstHeader(headers, "description", "descripcion", "concepto")
	amountColumn, hasAmount := firstHeader(headers, "amount", "importe", "monto")
	if !hasDate || !hasDescription || !hasAmount {
		return nil, fmt.Errorf("%w: CSV requires date, description and amount columns", ErrInvalidArgument)
	}
	valueDateColumn, hasValueDate := firstHeader(headers, "value_date", "fecha_valor")
	referenceColumn, hasReference := firstHeader(headers, "reference", "referencia", "id")
	currencyColumn, hasCurrency := firstHeader(headers, "currency", "moneda")

	movements := make([]StatementMovement, 0, len(records)-1)
	seen := make(map[string]struct{}, len(records)-1)
	for rowIndex, record := range records[1:] {
		if isBlankCSVRecord(record) {
			continue
		}
		requiredMax := max(dateColumn, descriptionColumn, amountColumn)
		if len(record) <= requiredMax {
			return nil, fmt.Errorf("%w: CSV row %d is incomplete", ErrInvalidArgument, rowIndex+2)
		}
		bookedAt, parseErr := parseStatementDate(record[dateColumn])
		if parseErr != nil {
			return nil, fmt.Errorf("CSV row %d: %w", rowIndex+2, parseErr)
		}
		valueAt := bookedAt
		if hasValueDate && valueDateColumn < len(record) && strings.TrimSpace(record[valueDateColumn]) != "" {
			valueAt, parseErr = parseStatementDate(record[valueDateColumn])
			if parseErr != nil {
				return nil, fmt.Errorf("CSV row %d: %w", rowIndex+2, parseErr)
			}
		}
		amount, parseErr := parseStatementAmount(record[amountColumn])
		if parseErr != nil {
			return nil, fmt.Errorf("CSV row %d: %w", rowIndex+2, parseErr)
		}
		currency := defaultCurrency
		if hasCurrency && currencyColumn < len(record) && strings.TrimSpace(record[currencyColumn]) != "" {
			currency, parseErr = NewCurrency(record[currencyColumn])
			if parseErr != nil {
				return nil, fmt.Errorf("CSV row %d: %w", rowIndex+2, parseErr)
			}
		}
		reference := ""
		if hasReference && referenceColumn < len(record) {
			reference = strings.TrimSpace(record[referenceColumn])
		}
		movement := StatementMovement{
			BookedAt:    bookedAt,
			ValueAt:     valueAt,
			Description: strings.TrimSpace(record[descriptionColumn]),
			Reference:   reference,
			Amount:      amount,
			Currency:    currency,
		}
		movement.Fingerprint = statementMovementFingerprint(movement)
		if _, duplicate := seen[movement.Fingerprint]; duplicate {
			return nil, fmt.Errorf("%w: duplicate movement at CSV row %d", ErrDuplicate, rowIndex+2)
		}
		seen[movement.Fingerprint] = struct{}{}
		movements = append(movements, movement)
	}
	return movements, nil
}

// ParseStatementOFX accepts the common SGML OFX 1.x transaction subset.
func ParseStatementOFX(content []byte, defaultCurrency Currency) ([]StatementMovement, error) {
	text := strings.ReplaceAll(string(content), "\r", "")
	upper := strings.ToUpper(text)
	currency := defaultCurrency
	if value := ofxTag(upper, text, "CURDEF"); value != "" {
		parsed, err := NewCurrency(value)
		if err != nil {
			return nil, err
		}
		currency = parsed
	}
	var movements []StatementMovement
	offset := 0
	for {
		startRelative := strings.Index(upper[offset:], "<STMTTRN>")
		if startRelative < 0 {
			break
		}
		start := offset + startRelative + len("<STMTTRN>")
		endRelative := strings.Index(upper[start:], "</STMTTRN>")
		if endRelative < 0 {
			return nil, fmt.Errorf("%w: unterminated OFX transaction", ErrInvalidArgument)
		}
		end := start + endRelative
		block := text[start:end]
		blockUpper := upper[start:end]
		date, err := parseOFXDate(ofxTag(blockUpper, block, "DTPOSTED"))
		if err != nil {
			return nil, err
		}
		amount, err := ParseAmount(normalizeAmountText(ofxTag(blockUpper, block, "TRNAMT")))
		if err != nil {
			return nil, err
		}
		movement := StatementMovement{
			BookedAt:    date,
			ValueAt:     date,
			Description: firstNonEmpty(ofxTag(blockUpper, block, "MEMO"), ofxTag(blockUpper, block, "NAME")),
			Reference:   ofxTag(blockUpper, block, "FITID"),
			Amount:      amount,
			Currency:    currency,
		}
		movement.Fingerprint = statementMovementFingerprint(movement)
		movements = append(movements, movement)
		offset = end + len("</STMTTRN>")
	}
	if len(movements) == 0 {
		return nil, fmt.Errorf("%w: OFX contains no transactions", ErrInvalidArgument)
	}
	return movements, nil
}

func SuggestReconciliationMatches(
	movements []StatementMovement,
	candidates []ReconciliationLedgerCandidate,
	maxDays int,
) []MatchSuggestion {
	if maxDays <= 0 {
		maxDays = 3
	}
	suggestions := make([]MatchSuggestion, 0)
	for _, movement := range movements {
		for _, candidate := range candidates {
			if !movement.Amount.Abs().Equal(candidate.Amount.Abs()) {
				continue
			}
			days := absoluteDays(movement.BookedAt, candidate.Date)
			if days > maxDays {
				continue
			}
			score := 70 - days*5
			reasons := []string{"same_amount"}
			if days == 0 {
				score += 15
				reasons = append(reasons, "same_date")
			}
			if referencesMatch(movement.Reference, candidate.Reference) {
				score += 15
				reasons = append(reasons, "same_reference")
			}
			suggestions = append(suggestions, MatchSuggestion{
				StatementMovementID: movement.ID,
				JournalLineID:       candidate.JournalLineID,
				Amount:              movement.Amount.Abs(),
				Score:               min(score, 100),
				Reasons:             reasons,
			})
		}
	}
	sort.SliceStable(suggestions, func(i, j int) bool {
		if suggestions[i].Score == suggestions[j].Score {
			return suggestions[i].StatementMovementID.String() < suggestions[j].StatementMovementID.String()
		}
		return suggestions[i].Score > suggestions[j].Score
	})
	return suggestions
}

func (r Reconciliation) Validate(
	movements map[uuid.UUID]StatementMovement,
	candidates map[uuid.UUID]ReconciliationLedgerCandidate,
) error {
	if r.ID == uuid.Nil || r.FinancialAccountID == uuid.Nil ||
		r.PeriodStart.IsZero() || r.PeriodEnd.Before(r.PeriodStart) {
		return fmt.Errorf("%w: invalid reconciliation", ErrInvalidArgument)
	}
	statementAllocated := make(map[uuid.UUID]Decimal)
	ledgerAllocated := make(map[uuid.UUID]Decimal)
	for index, match := range r.Matches {
		movement, ok := movements[match.StatementMovementID]
		if !ok {
			return fmt.Errorf("match %d: %w: statement movement", index+1, ErrNotFound)
		}
		candidate, ok := candidates[match.JournalLineID]
		if !ok {
			return fmt.Errorf("match %d: %w: journal line", index+1, ErrNotFound)
		}
		if match.StatementAmount.Sign() <= 0 || match.LedgerAmount.Sign() <= 0 ||
			!match.StatementAmount.Equal(match.LedgerAmount) {
			return fmt.Errorf("match %d: %w: allocations must be equal and positive", index+1, ErrInvalidArgument)
		}
		statementAllocated[match.StatementMovementID] =
			statementAllocated[match.StatementMovementID].Add(match.StatementAmount)
		ledgerAllocated[match.JournalLineID] =
			ledgerAllocated[match.JournalLineID].Add(match.LedgerAmount)
		if statementAllocated[match.StatementMovementID].Cmp(movement.Amount.Abs()) > 0 {
			return fmt.Errorf("match %d: %w: statement movement is over-allocated", index+1, ErrConflict)
		}
		if ledgerAllocated[match.JournalLineID].Cmp(candidate.Amount.Abs()) > 0 {
			return fmt.Errorf("match %d: %w: journal line is over-allocated", index+1, ErrConflict)
		}
	}
	return nil
}

func (r Reconciliation) Difference() Decimal {
	return r.StatementClosing.Sub(r.LedgerClosing)
}

func statementMovementFingerprint(movement StatementMovement) string {
	payload := strings.Join([]string{
		movement.BookedAt.UTC().Format("2006-01-02"),
		movement.ValueAt.UTC().Format("2006-01-02"),
		movement.Amount.String(),
		movement.Currency.Code(),
		strings.ToLower(strings.TrimSpace(movement.Reference)),
		strings.ToLower(strings.Join(strings.Fields(movement.Description), " ")),
	}, "\x1f")
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", " ", "_", "-", "_").Replace(value)
	return value
}

func firstHeader(headers map[string]int, names ...string) (int, bool) {
	for _, name := range names {
		if index, ok := headers[name]; ok {
			return index, true
		}
	}
	return 0, false
}

func isBlankCSVRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func parseStatementDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", "02/01/2006", "02-01-2006", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: invalid statement date %q", ErrInvalidArgument, value)
}

func parseStatementAmount(value string) (Decimal, error) {
	normalized := normalizeAmountText(value)
	amount, err := ParseAmount(normalized)
	if err != nil {
		return Decimal{}, fmt.Errorf("%w: invalid statement amount", err)
	}
	if amount.IsZero() {
		return Decimal{}, fmt.Errorf("%w: statement amount cannot be zero", ErrInvalidArgument)
	}
	return amount, nil
}

func normalizeAmountText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\u00a0", "")
	value = strings.ReplaceAll(value, " ", "")
	if strings.Contains(value, ",") {
		if strings.Contains(value, ".") {
			if strings.LastIndex(value, ",") > strings.LastIndex(value, ".") {
				value = strings.ReplaceAll(value, ".", "")
				value = strings.ReplaceAll(value, ",", ".")
			} else {
				value = strings.ReplaceAll(value, ",", "")
			}
		} else {
			value = strings.ReplaceAll(value, ",", ".")
		}
	}
	return value
}

func ofxTag(upper, original, name string) string {
	open := "<" + name + ">"
	index := strings.Index(upper, open)
	if index < 0 {
		return ""
	}
	start := index + len(open)
	end := strings.IndexAny(original[start:], "\r\n<")
	if end < 0 {
		return strings.TrimSpace(original[start:])
	}
	return strings.TrimSpace(original[start : start+end])
}

func parseOFXDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if len(value) < 8 {
		return time.Time{}, fmt.Errorf("%w: invalid OFX date", ErrInvalidArgument)
	}
	parsed, err := time.Parse("20060102", value[:8])
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid OFX date", ErrInvalidArgument)
	}
	return parsed.UTC(), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func absoluteDays(left, right time.Time) int {
	days := int(left.Sub(right) / (24 * time.Hour))
	if days < 0 {
		return -days
	}
	return days
}

func referencesMatch(left, right string) bool {
	left = strings.ToLower(strings.Join(strings.Fields(left), ""))
	right = strings.ToLower(strings.Join(strings.Fields(right), ""))
	return left != "" && right != "" && (left == right || strings.Contains(left, right) || strings.Contains(right, left))
}
