// Package ledger is M-FIN's double-entry general ledger (port of
// src/convex/ledger.ts, ADR-0006: "money always posts through M-FIN").
//
// Every module posts money here — folio charges, payment captures, stock
// receipts, payroll runs. Journals are validated for balance, chart
// membership and period locks before a single line is written, and all
// amounts are integer kobo (the platform money unit); floats never touch
// the stored ledger. gl_entries/gl_accounts are engine-neutral so SQLite
// and Postgres share one contract.
package ledger

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"infineon/internal/database"
)

// DBTX is satisfied by database.Pool and database.Tx alike, so posting can
// either own a transaction or enlist in the caller's (e.g. a folio charge and
// its GL line commit together).
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (database.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (database.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) database.Row
}

type beginner interface {
	Begin(ctx context.Context) (database.Tx, error)
}

// ── Standard hotel chart of accounts ─────────────────────────────────────

// Account is one standard general-ledger account.
type Account struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Type    string `json:"type"`    // asset | liability | equity | revenue | expense
	Subtype string `json:"subtype"` // cash | receivable | inventory | ...
}

// StandardCOA is the built-in chart every property gets (28 accounts).
var StandardCOA = []Account{
	{"1000", "Cash on Hand", "asset", "cash"},
	{"1010", "Bank Account — Current", "asset", "cash"},
	{"1100", "Guest Ledger (Receivable)", "asset", "receivable"},
	{"1110", "City Ledger (Receivable)", "asset", "receivable"},
	{"1200", "F&B Inventory", "asset", "inventory"},
	{"1210", "Supplies Inventory", "asset", "inventory"},
	{"1500", "Furniture, Fixtures & Equipment", "asset", "fixed"},
	{"2000", "Accounts Payable", "liability", "payable"},
	{"2100", "VAT Payable", "liability", "tax"},
	{"2110", "Service Charge Pool", "liability", "accrued"},
	{"2120", "Payroll Payable", "liability", "accrued"},
	{"2130", "Statutory Deductions Payable", "liability", "tax"},
	{"2200", "Guest Deposits Held", "liability", "deferred"},
	{"3000", "Owner's Equity", "equity", "capital"},
	{"4000", "Room Revenue", "revenue", "rooms"},
	{"4010", "Food & Beverage Revenue", "revenue", "fnb"},
	{"4020", "Events & Banqueting Revenue", "revenue", "events"},
	{"4030", "Other Operating Revenue", "revenue", "other"},
	{"4040", "Discounts & Allowances", "revenue", "contra"},
	{"5000", "Salaries & Wages", "expense", "payroll"},
	{"5010", "Employee Benefits", "expense", "payroll"},
	{"5100", "Utilities", "expense", "operating"},
	{"5110", "Repairs & Maintenance", "expense", "operating"},
	{"5120", "Housekeeping & Operating Supplies", "expense", "operating"},
	{"5200", "Cost of Food Sold", "expense", "cost"},
	{"5300", "Sales & Marketing", "expense", "operating"},
	{"5310", "OTA Commission Expense", "expense", "distribution"},
	{"5500", "Administrative & General", "expense", "operating"},
}

var coaByCode = func() map[string]Account {
	m := make(map[string]Account, len(StandardCOA))
	for _, a := range StandardCOA {
		m[a.Code] = a
	}
	return m
}()

// Standard account codes.
const (
	Cash            = "1000"
	Bank            = "1010"
	GuestLedger     = "1100"
	CityLedger      = "1110"
	AccountsPayable = "2000"
	VATPayable      = "2100"
	ServiceCharge   = "2110"
	PayrollPayable  = "2120"
	GuestDeposits   = "2200"
	RoomRevenue     = "4000"
	FBRevenue       = "4010"
	EventsRevenue   = "4020"
	OtherRevenue    = "4030"
	Discounts       = "4040"
	CostOfFood      = "5200"
)

// PaymentAccount returns the debit-side account for a settlement method.
func PaymentAccount(method string) string {
	if method == "cash" {
		return Cash
	}
	return Bank
}

// ── Money helpers (integer kobo) ─────────────────────────────────────────

// NairaToKobo converts a float naira amount to integer kobo (round-half-up).
func NairaToKobo(v float64) int64 {
	return int64(math.Round(v * 100))
}

// KoboToNaira converts integer kobo back to naira for display.
func KoboToNaira(k int64) float64 {
	return float64(k) / 100
}

// ── Journal input ────────────────────────────────────────────────────────

// JournalLine is one debit or credit leg. Exactly one side must be set.
type JournalLine struct {
	AccountCode string `json:"accountCode"`
	DebitKobo   int64  `json:"debitKobo"`
	CreditKobo  int64  `json:"creditKobo"`
	Description string `json:"description"`
	Ref         string `json:"ref,omitempty"`
}

// ErrPeriodLocked wraps every posting rejected because the accounting period
// is closed (HTTP layer maps it to 409 Conflict).
var ErrPeriodLocked = errors.New("ledger period locked")

// JournalInput is a balanced journal to post.
type JournalInput struct {
	PropertyID        string        `json:"property_id"`
	Date              string        `json:"date"` // business date YYYY-MM-DD
	Lines             []JournalLine `json:"lines"`
	Source            string        `json:"source"` // folio | pos | night_audit | inventory | payroll | payment | manual
	Ref               string        `json:"ref,omitempty"`
	By                string        `json:"by,omitempty"`
	AllowClosedPeriod bool          `json:"allow_closed_period,omitempty"`
}

// Validate runs every pure check before anything touches the database:
// non-empty, non-negative, no single line debiting and crediting, known
// account codes, balanced, positive total.
func Validate(in JournalInput) error {
	if len(in.Lines) == 0 {
		return fmt.Errorf("journal has no lines")
	}
	if in.PropertyID == "" {
		return fmt.Errorf("journal requires a property")
	}
	var debit, credit int64
	for _, line := range in.Lines {
		if line.DebitKobo < 0 || line.CreditKobo < 0 {
			return fmt.Errorf("journal lines must be non-negative")
		}
		if line.DebitKobo > 0 && line.CreditKobo > 0 {
			return fmt.Errorf("a journal line cannot debit and credit simultaneously")
		}
		if _, ok := coaByCode[line.AccountCode]; !ok {
			return fmt.Errorf("unknown account code %q", line.AccountCode)
		}
		debit += line.DebitKobo
		credit += line.CreditKobo
	}
	if debit != credit {
		return fmt.Errorf("unbalanced journal — debits %d ≠ credits %d", debit, credit)
	}
	if debit <= 0 {
		return fmt.Errorf("journal must move a positive amount")
	}
	return nil
}

// assertPeriodOpen rejects postings on or before the locked-through date
// unless the caller explicitly allows it (night audit's own accruals).
func assertPeriodOpen(ctx context.Context, d DBTX, propertyID, date string, allowClosed bool) error {
	if allowClosed || date == "" {
		return nil
	}
	var lock string
	rows, err := d.Query(ctx, `
		SELECT value FROM system_settings
		WHERE property_id = $1 AND key = 'gl.locked_through'
		ORDER BY updated_at DESC LIMIT 1
	`, propertyID)
	if err != nil {
		return fmt.Errorf("period lock check: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&lock); err != nil {
			return fmt.Errorf("period lock scan: %w", err)
		}
	}
	if lock != "" && date <= lock {
		return fmt.Errorf("%w: closed through %s — %s is locked", ErrPeriodLocked, lock, date)
	}
	return nil
}

// EnsureChartOfAccounts creates any missing standard accounts for the
// property. Idempotent.
func EnsureChartOfAccounts(ctx context.Context, d DBTX, propertyID string) error {
	for _, a := range StandardCOA {
		_, err := d.Exec(ctx, `
			INSERT INTO gl_accounts (id, property_id, code, name, type, subtype)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (property_id, code) DO NOTHING
		`, uuid.New().String(), propertyID, a.Code, a.Name, a.Type, a.Subtype)
		if err != nil {
			return fmt.Errorf("ensure chart of accounts: %w", err)
		}
	}
	return nil
}

// PostJournal validates then writes every line in one transaction. When d is
// a pool it owns the transaction; when d is already a Tx the lines enlist in
// the caller's transaction (atomic with their business event).
func PostJournal(ctx context.Context, d DBTX, in JournalInput) (int, error) {
	if err := Validate(in); err != nil {
		return 0, err
	}
	if err := assertPeriodOpen(ctx, d, in.PropertyID, in.Date, in.AllowClosedPeriod); err != nil {
		return 0, err
	}

	run := func(e DBTX) error {
		if err := EnsureChartOfAccounts(ctx, e, in.PropertyID); err != nil {
			return err
		}
		now := time.Now().UnixMilli()
		for _, line := range in.Lines {
			ref := line.Ref
			if ref == "" {
				ref = in.Ref
			}
			_, err := e.Exec(ctx, `
				INSERT INTO gl_entries (id, property_id, business_date, account_code,
					debit_kobo, credit_kobo, description, ref, source, emitted_by, at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`, uuid.New().String(), in.PropertyID, in.Date, line.AccountCode,
				line.DebitKobo, line.CreditKobo, line.Description, ref, in.Source, in.By, now)
			if err != nil {
				return fmt.Errorf("insert gl entry %s: %w", line.AccountCode, err)
			}
		}
		return nil
	}

	if b, ok := d.(beginner); ok {
		tx, err := b.Begin(ctx)
		if err != nil {
			return 0, err
		}
		defer tx.Rollback(context.Background()) //nolint:errcheck // no-op once committed
		if err := run(tx); err != nil {
			return 0, err
		}
		if err := tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit journal: %w", err)
		}
		return len(in.Lines), nil
	}
	if err := run(d); err != nil {
		return 0, err
	}
	return len(in.Lines), nil
}

// PostSimple is the two-sided convenience wrapper (Dr one account, Cr
// another) — the published contract every module settles money through.
func PostSimple(ctx context.Context, d DBTX, in JournalInput, debit, credit string, amountKobo int64, description string) (int, error) {
	in.Lines = []JournalLine{
		{AccountCode: debit, DebitKobo: amountKobo, Description: description},
		{AccountCode: credit, CreditKobo: amountKobo, Description: description},
	}
	return PostJournal(ctx, d, in)
}

// ── Revenue recognition for folio charges ────────────────────────────────

// RevenueAccount maps a folio/POS charge code to its revenue account.
func RevenueAccount(chargeCode string) string {
	switch strings.ToUpper(chargeCode) {
	case "ROOM", "ROOMS", "ACC", "ACCOMMODATION":
		return RoomRevenue
	case "F&B", "FOOD", "BEV", "BEVERAGE", "MINIBAR", "RESTAURANT", "ROOM_SERVICE", "POS", "BAR":
		return FBRevenue
	case "EVENT", "EVENTS", "BANQUET", "CONFERENCE", "VENUE":
		return EventsRevenue
	case "LAUNDRY", "SPA", "PARKING", "MISC", "OTHER":
		return OtherRevenue
	default:
		return OtherRevenue
	}
}

// PostFolioCharge recognizes revenue for a folio charge inside the caller's
// transaction (the published contract folio/POS handlers settle through):
//
//	Dr Guest Ledger        amount + tax
//	Cr revenue (by code)   amount
//	Cr VAT Payable         tax
func PostFolioCharge(ctx context.Context, d DBTX, propertyID, chargeCode string,
	amountNaira, taxNaira float64, description, ref, by string) (int, error) {
	amountKobo := NairaToKobo(amountNaira)
	taxKobo := NairaToKobo(taxNaira)
	total := amountKobo + taxKobo
	if total <= 0 {
		return 0, nil // comp nights / zero-rate lines move no money
	}
	lines := []JournalLine{
		{AccountCode: GuestLedger, DebitKobo: total, Description: description},
	}
	if amountKobo > 0 {
		lines = append(lines, JournalLine{
			AccountCode: RevenueAccount(chargeCode), CreditKobo: amountKobo, Description: description,
		})
	}
	if taxKobo > 0 {
		lines = append(lines, JournalLine{
			AccountCode: VATPayable, CreditKobo: taxKobo, Description: description + " (VAT)",
		})
	}
	return PostJournal(ctx, d, JournalInput{
		PropertyID: propertyID,
		Date:       time.Now().Format("2006-01-02"),
		Source:     "folio",
		Ref:        ref,
		By:         by,
		Lines:      lines,
	})
}

// ── payment.captured subscriber: cash/bank Dr, Guest Ledger Cr ───────────

// PaymentCapturedPayload is the payment.captured contract payload.
type PaymentCapturedPayload struct {
	PropertyID  string `json:"propertyId"`
	Date        string `json:"date"`
	Method      string `json:"method"`
	AmountKobo  int64  `json:"amountKobo"`
	Reference   string `json:"reference"`
	Description string `json:"description"`
}

// OnPaymentCaptured posts Dr cash/bank, Cr Guest Ledger for a settled
// payment. Idempotent per (property, reference): a redelivered event or a
// webhook replayed after a successful verify does not double-post.
func OnPaymentCaptured(ctx context.Context, d DBTX, p PaymentCapturedPayload) error {
	if p.PropertyID == "" || p.Date == "" || p.AmountKobo <= 0 {
		return nil // malformed → spec says drop silently
	}
	var count int
	if err := d.QueryRow(ctx, `
		SELECT COUNT(*) FROM gl_entries
		WHERE property_id = $1 AND ref = $2 AND source = 'payment'
	`, p.PropertyID, p.Reference).Scan(&count); err != nil {
		return fmt.Errorf("payment idempotency check: %w", err)
	}
	if count > 0 {
		return nil
	}
	description := p.Description
	if description == "" {
		description = fmt.Sprintf("Payment received — %s", p.Reference)
	}
	_, err := PostJournal(ctx, d, JournalInput{
		PropertyID: p.PropertyID,
		Date:       p.Date,
		Source:     "payment",
		Ref:        p.Reference,
		By:         "system:events",
		Lines: []JournalLine{
			{AccountCode: PaymentAccount(p.Method), DebitKobo: p.AmountKobo, Description: description},
			{AccountCode: GuestLedger, CreditKobo: p.AmountKobo, Description: description},
		},
	})
	return err
}

// ── Reporting ────────────────────────────────────────────────────────────

// TrialBalanceRow is one account's aggregate in the trial balance.
type TrialBalanceRow struct {
	AccountCode string `json:"accountCode"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	DebitKobo   int64  `json:"debitKobo"`
	CreditKobo  int64  `json:"creditKobo"`
	BalanceKobo int64  `json:"balanceKobo"`
}

// TrialBalanceResult wraps rows plus grand totals (always equal when books
// are balanced).
type TrialBalanceResult struct {
	Rows            []TrialBalanceRow `json:"rows"`
	TotalDebitKobo  int64             `json:"totalDebitKobo"`
	TotalCreditKobo int64             `json:"totalCreditKobo"`
}

type agg struct {
	debit  int64
	credit int64
}

func collectAgg(ctx context.Context, d DBTX, propertyID, from, to string) (map[string]agg, error) {
	sql := `SELECT account_code, SUM(debit_kobo), SUM(credit_kobo) FROM gl_entries WHERE property_id = $1`
	args := []any{propertyID}
	n := 1
	if from != "" {
		n++
		sql += fmt.Sprintf(` AND business_date >= $%d`, n)
		args = append(args, from)
	}
	if to != "" {
		n++
		sql += fmt.Sprintf(` AND business_date <= $%d`, n)
		args = append(args, to)
	}
	sql += ` GROUP BY account_code`

	rows, err := d.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]agg{}
	for rows.Next() {
		var code string
		var debit, credit int64
		if err := rows.Scan(&code, &debit, &credit); err != nil {
			return nil, err
		}
		out[code] = agg{debit: debit, credit: credit}
	}
	return out, rows.Err()
}

// listAccounts returns the property's GL accounts (ensuring the chart first
// so reports are never empty-headed).
func listAccounts(ctx context.Context, d DBTX, propertyID string) ([]Account, error) {
	if err := EnsureChartOfAccounts(ctx, d, propertyID); err != nil {
		return nil, err
	}
	rows, err := d.Query(ctx, `
		SELECT code, name, type, subtype FROM gl_accounts
		WHERE property_id = $1 ORDER BY code
	`, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.Code, &a.Name, &a.Type, &a.Subtype); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Accounts returns the property's GL accounts, ensuring the standard chart
// first so reports are never empty-headed.
func Accounts(ctx context.Context, d DBTX, propertyID string) ([]Account, error) {
	return listAccounts(ctx, d, propertyID)
}

// TrialBalance aggregates every account for the period.
func TrialBalance(ctx context.Context, d DBTX, propertyID, from, to string) (*TrialBalanceResult, error) {
	accounts, err := listAccounts(ctx, d, propertyID)
	if err != nil {
		return nil, err
	}
	byCode, err := collectAgg(ctx, d, propertyID, from, to)
	if err != nil {
		return nil, err
	}
	res := &TrialBalanceResult{Rows: []TrialBalanceRow{}}
	for _, a := range accounts {
		g := byCode[a.Code]
		debitFirst := a.Type == "asset" || a.Type == "expense"
		bal := g.debit - g.credit
		if !debitFirst {
			bal = g.credit - g.debit
		}
		res.Rows = append(res.Rows, TrialBalanceRow{
			AccountCode: a.Code, Name: a.Name, Type: a.Type,
			DebitKobo: g.debit, CreditKobo: g.credit, BalanceKobo: bal,
		})
		res.TotalDebitKobo += g.debit
		res.TotalCreditKobo += g.credit
	}
	sort.Slice(res.Rows, func(i, j int) bool { return res.Rows[i].AccountCode < res.Rows[j].AccountCode })
	return res, nil
}

// PLRow is one revenue or expense line in the P&L.
type PLRow struct {
	AccountCode string `json:"accountCode"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	AmountKobo  int64  `json:"amountKobo"`
}

// ProfitAndLossResult wraps the P&L with totals.
type ProfitAndLossResult struct {
	Rows        []PLRow `json:"rows"`
	RevenueKobo int64   `json:"revenueKobo"`
	ExpenseKobo int64   `json:"expenseKobo"`
	NetKobo     int64   `json:"netKobo"`
}

// ProfitAndLoss derives revenue and expense for the period (never stored).
func ProfitAndLoss(ctx context.Context, d DBTX, propertyID, from, to string) (*ProfitAndLossResult, error) {
	accounts, err := listAccounts(ctx, d, propertyID)
	if err != nil {
		return nil, err
	}
	byCode, err := collectAgg(ctx, d, propertyID, from, to)
	if err != nil {
		return nil, err
	}
	res := &ProfitAndLossResult{Rows: []PLRow{}}
	for _, a := range accounts {
		if a.Type != "revenue" && a.Type != "expense" {
			continue
		}
		g := byCode[a.Code]
		var amount int64
		if a.Type == "revenue" {
			amount = g.credit - g.debit
		} else {
			amount = g.debit - g.credit
		}
		if amount == 0 {
			continue
		}
		res.Rows = append(res.Rows, PLRow{AccountCode: a.Code, Name: a.Name, Type: a.Type, AmountKobo: amount})
		if a.Type == "revenue" {
			res.RevenueKobo += amount
		} else {
			res.ExpenseKobo += amount
		}
	}
	sort.Slice(res.Rows, func(i, j int) bool { return res.Rows[i].AccountCode < res.Rows[j].AccountCode })
	res.NetKobo = res.RevenueKobo - res.ExpenseKobo
	return res, nil
}

// Entry is one gl_entries line in an account's history.
type Entry struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	AccountCode string `json:"accountCode"`
	DebitKobo   int64  `json:"debitKobo"`
	CreditKobo  int64  `json:"creditKobo"`
	Description string `json:"description"`
	Ref         string `json:"ref"`
	Source      string `json:"source"`
}

// AccountLedger returns the posting history of one account.
func AccountLedger(ctx context.Context, d DBTX, propertyID, accountCode, from, to string) ([]Entry, error) {
	sql := `SELECT id, business_date, account_code, debit_kobo, credit_kobo,
		       COALESCE(description, ''), COALESCE(ref, ''), source
		FROM gl_entries WHERE property_id = $1 AND account_code = $2`
	args := []any{propertyID, accountCode}
	n := 2
	if from != "" {
		n++
		sql += fmt.Sprintf(` AND business_date >= $%d`, n)
		args = append(args, from)
	}
	if to != "" {
		n++
		sql += fmt.Sprintf(` AND business_date <= $%d`, n)
		args = append(args, to)
	}
	sql += ` ORDER BY business_date, created_at`

	rows, err := d.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Date, &e.AccountCode, &e.DebitKobo, &e.CreditKobo,
			&e.Description, &e.Ref, &e.Source); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
