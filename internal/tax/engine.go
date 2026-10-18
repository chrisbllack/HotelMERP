package tax

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// ═══════════════════════════════════════════════════════════════
// NIGERIAN TAX ENGINE
// FIRS VAT (7.5%), State Hotel Occupancy Tax, WHT, Tourism Levies
// ═══════════════════════════════════════════════════════════════

// TaxRule defines a configurable tax rule
type TaxRule struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Code        string  `json:"code"` // VAT, HOT, WHT, Tourism Levy
	Type        string  `json:"type"` // percentage, fixed
	Rate        float64 `json:"rate"`
	AppliesTo   string  `json:"applies_to"` // all, room, food, beverage, service, total
	State       string  `json:"state"`       // lagos, abuja, rivers, oyo, etc.
	Country     string  `json:"country"`
	MinAmount   float64 `json:"min_amount"`
	MaxAmount   float64 `json:"max_amount"`
	ExemptGuestTypes string `json:"exempt_guest_types"` // corporate, diplomatic, government
	Enabled     bool    `json:"enabled"`
	RemittanceAcct string `json:"remittance_account"`
}

// TaxCalculation represents a computed tax amount
type TaxCalculation struct {
	RuleID    string  `json:"rule_id"`
	Name      string  `json:"name"`
	Code      string  `json:"code"`
	BaseAmount float64 `json:"base_amount"`
	Rate      float64 `json:"rate"`
	Amount    float64 `json:"amount"`
	State     string  `json:"state"`
}

// TaxEngine handles all tax calculations
type TaxEngine struct {
	db *sql.DB
}

// NewTaxEngine creates the tax engine
func NewTaxEngine(db *sql.DB) *TaxEngine {
	te := &TaxEngine{db: db}
	te.createTables()
	te.seedTaxRules()
	return te
}

func (te *TaxEngine) createTables() {
	ctx := context.Background()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS tax_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			type TEXT NOT NULL,
			rate REAL NOT NULL,
			applies_to TEXT DEFAULT 'all',
			state TEXT DEFAULT 'all',
			country TEXT DEFAULT 'NG',
			min_amount REAL DEFAULT 0,
			max_amount REAL DEFAULT 0,
			exempt_guest_types TEXT DEFAULT '',
			enabled INTEGER DEFAULT 1,
			remittance_account TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tax_calculations (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL,
			reservation_id TEXT,
			folio_id TEXT,
			total_amount REAL NOT NULL,
			taxes JSON NOT NULL,
			total_tax REAL NOT NULL,
			grand_total REAL NOT NULL,
			state TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tax_remittances (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL,
			tax_code TEXT NOT NULL,
			period_start DATE NOT NULL,
			period_end DATE NOT NULL,
			total_collected REAL NOT NULL,
			amount_remitted REAL DEFAULT 0,
			remittance_date DATE,
			status TEXT DEFAULT 'pending',
			reference TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, t := range tables {
		_, _ = te.db.ExecContext(ctx, t)
	}
}

// seedTaxRules creates Nigerian tax rules for different states
func (te *TaxEngine) seedTaxRules() {
	ctx := context.Background()

	rules := []TaxRule{
		// FIRS VAT - Federal (7.5%)
		{ID: "tax-vat-firs", Name: "FIRS VAT", Code: "VAT", Type: "percentage", Rate: 0.075, AppliesTo: "all", State: "all", Country: "NG", Enabled: true, RemittanceAcct: "FIRS VAT Pool"},

		// Lagos State Hotel Occupancy Tax
		{ID: "tax-hot-lagos", Name: "Lagos Hotel Occupancy Tax", Code: "HOT", Type: "percentage", Rate: 0.05, AppliesTo: "room", State: "lagos", Country: "NG", Enabled: true, RemittanceAcct: "Lagos IRS"},
		{ID: "tax-tourism-lagos", Name: "Lagos Tourism Levy", Code: "TOURISM", Type: "percentage", Rate: 0.01, AppliesTo: "all", State: "lagos", Country: "NG", Enabled: true, RemittanceAcct: "Lagos Tourism Board"},
		{ID: "tax-service-lagos", Name: "Lagos Service Charge", Code: "SVC", Type: "percentage", Rate: 0.10, AppliesTo: "food", State: "lagos", Country: "NG", Enabled: true},

		// Abuja/FCT
		{ID: "tax-hot-abuja", Name: "FCT Hotel Occupancy Tax", Code: "HOT", Type: "percentage", Rate: 0.05, AppliesTo: "room", State: "abuja", Country: "NG", Enabled: true, RemittanceAcct: "FCT IRS"},
		{ID: "tax-tourism-abuja", Name: "FCT Tourism Levy", Code: "TOURISM", Type: "percentage", Rate: 0.01, AppliesTo: "all", State: "abuja", Country: "NG", Enabled: true, RemittanceAcct: "FCT Tourism Board"},

		// Rivers State
		{ID: "tax-hot-rivers", Name: "Rivers Hotel Occupancy Tax", Code: "HOT", Type: "percentage", Rate: 0.05, AppliesTo: "room", State: "rivers", Country: "NG", Enabled: true, RemittanceAcct: "Rivers IRS"},
		{ID: "tax-tourism-rivers", Name: "Rivers Tourism Levy", Code: "TOURISM", Type: "percentage", Rate: 0.02, AppliesTo: "all", State: "rivers", Country: "NG", Enabled: true, RemittanceAcct: "Rivers Tourism Board"},

		// Oyo State
		{ID: "tax-hot-oyo", Name: "Oyo Hotel Occupancy Tax", Code: "HOT", Type: "percentage", Rate: 0.03, AppliesTo: "room", State: "oyo", Country: "NG", Enabled: true, RemittanceAcct: "Oyo IRS"},

		// Edo State
		{ID: "tax-hot-edo", Name: "Edo Hotel Occupancy Tax", Code: "HOT", Type: "percentage", Rate: 0.05, AppliesTo: "room", State: "edo", Country: "NG", Enabled: true, RemittanceAcct: "Edo IRS"},

		// WHT on contractor payments
		{ID: "tax-wht-contractor", Name: "WHT - Contractors", Code: "WHT", Type: "percentage", Rate: 0.05, AppliesTo: "service", State: "all", Country: "NG", Enabled: true, RemittanceAcct: "FIRS WHT Pool"},
		{ID: "tax-wht-professional", Name: "WHT - Professional Fees", Code: "WHT", Type: "percentage", Rate: 0.10, AppliesTo: "service", State: "all", Country: "NG", Enabled: true, RemittanceAcct: "FIRS WHT Pool"},

		// NHIS (National Health Insurance)
		{ID: "tax-nhis-employer", Name: "NHIS Employer Contribution", Code: "NHIS", Type: "percentage", Rate: 0.0325, AppliesTo: "salary", State: "all", Country: "NG", Enabled: true, RemittanceAcct: "NHIS Pool"},

		// Nigeria Police Trust Fund
		{ID: "tax-nptf", Name: "Nigeria Police Trust Fund", Code: "NPTF", Type: "percentage", Rate: 0.005, AppliesTo: "salary", State: "all", Country: "NG", Enabled: true, RemittanceAcct: "NPTF Pool"},
	}

	for _, r := range rules {
		_, _ = te.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO tax_rules (id, name, code, type, rate, applies_to, state, country, exempt_guest_types, enabled, remittance_account) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, r.Name, r.Code, r.Type, r.Rate, r.AppliesTo, r.State, r.Country, r.ExemptGuestTypes, r.Enabled, r.RemittanceAcct)
	}

	log.Printf("[TAX] Seeded %d tax rules", len(rules))
}

// CalculateTaxes calculates all applicable taxes for a given amount in a state
func (te *TaxEngine) CalculateTaxes(ctx context.Context, state string, charges []ChargeItem) ([]TaxCalculation, float64) {
	rules := te.getApplicableRules(ctx, state)
	var calculations []TaxCalculation
	totalTax := 0.0

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Sum applicable charges
		baseAmount := 0.0
		for _, charge := range charges {
			if rule.AppliesTo == "all" || rule.AppliesTo == charge.Category {
				baseAmount += charge.Amount
			}
		}

		if baseAmount <= 0 {
			continue
		}

		// Apply minimum/maximum
		if rule.MinAmount > 0 && baseAmount < rule.MinAmount {
			continue
		}

		var taxAmount float64
		if rule.Type == "percentage" {
			taxAmount = baseAmount * rule.Rate
		} else {
			taxAmount = rule.Rate
		}

		if rule.MaxAmount > 0 && taxAmount > rule.MaxAmount {
			taxAmount = rule.MaxAmount
		}

		if taxAmount > 0 {
			calculations = append(calculations, TaxCalculation{
				RuleID:     rule.ID,
				Name:       rule.Name,
				Code:       rule.Code,
				BaseAmount: baseAmount,
				Rate:       rule.Rate,
				Amount:     taxAmount,
				State:      state,
			})
			totalTax += taxAmount
		}
	}

	return calculations, totalTax
}

// CalculatePAYE computes Nigerian PAYE income tax using graduated bands
func (te *TaxEngine) CalculatePAYE(annualGross float64) PAYECalculation {
	// Nigerian PAYE Tax Bands 2024/2025
	bands := []PAYEBand{
		{Min: 0, Max: 300000, Rate: 0.07},
		{Min: 300000, Max: 660000, Rate: 0.11},
		{Min: 660000, Max: 1116000, Rate: 0.15},
		{Min: 1116000, Max: 1668000, Rate: 0.19},
		{Min: 1668000, Max: 3276000, Rate: 0.21},
		{Min: 3276000, Max: 0, Rate: 0.24}, // 24% of everything above 3,276,000
	}

	// Consolidated relief allowance: ₦200,000 + 20% of gross, max ₦200,000
	consolidatedRelief := 200000.0
	relief20 := annualGross * 0.20
	if relief20 < consolidatedRelief {
		consolidatedRelief = relief20
	}

	taxableIncome := annualGross - consolidatedRelief
	if taxableIncome < 0 {
		taxableIncome = 0
	}

	totalTax := 0.0
	for _, band := range bands {
		if taxableIncome <= 0 {
			break
		}
		if band.Max == 0 {
			// Final band: everything remaining
			totalTax += taxableIncome * band.Rate
			break
		}
		bandWidth := band.Max - band.Min
		if taxableIncome > bandWidth {
			totalTax += bandWidth * band.Rate
			taxableIncome -= bandWidth
		} else {
			totalTax += taxableIncome * band.Rate
			taxableIncome = 0
		}
	}

	monthlyPAYE := totalTax / 12

	return PAYECalculation{
		AnnualGross:        annualGross,
		ConsolidatedRelief: consolidatedRelief,
		TaxableIncome:      annualGross - consolidatedRelief,
		AnnualTax:          totalTax,
		MonthlyPAYE:        monthlyPAYE,
		EffectiveRate:      totalTax / annualGross * 100,
	}
}

type PAYEBand struct {
	Min, Max float64
	Rate     float64
}

type PAYECalculation struct {
	AnnualGross        float64 `json:"annual_gross"`
	ConsolidatedRelief float64 `json:"consolidated_relief"`
	TaxableIncome      float64 `json:"taxable_income"`
	AnnualTax          float64 `json:"annual_tax"`
	MonthlyPAYE        float64 `json:"monthly_paye"`
	EffectiveRate      float64 `json:"effective_rate"`
}

// ChargeItem represents a charge line item for tax calculation
type ChargeItem struct {
	Category string  `json:"category"` // room, food, beverage, service, spa, laundry
	Amount   float64 `json:"amount"`
}

func (te *TaxEngine) getApplicableRules(ctx context.Context, state string) []TaxRule {
	rows, err := te.db.QueryContext(ctx,
		`SELECT id, name, code, type, rate, applies_to, state, country, enabled, remittance_account
		FROM tax_rules WHERE enabled=1 AND (state=? OR state='all')`, state)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var rules []TaxRule
	for rows.Next() {
		var r TaxRule
		rows.Scan(&r.ID, &r.Name, &r.Code, &r.Type, &r.Rate, &r.AppliesTo, &r.State, &r.Country, &r.Enabled, &r.RemittanceAcct)
		rules = append(rules, r)
	}
	return rules
}

// GetTaxRemittanceSummary returns tax collected vs remitted for a property
func (te *TaxEngine) GetTaxRemittanceSummary(ctx context.Context, propertyID, state string, periodStart, periodEnd string) map[string]interface{} {
	rules := te.getApplicableRules(ctx, state)
	summary := map[string]interface{}{
		"state":   state,
		"period":  fmt.Sprintf("%s to %s", periodStart, periodEnd),
		"taxes":   []map[string]interface{}{},
	}

	var taxes []map[string]interface{}
	for _, rule := range rules {
		var collected float64
		te.db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(amount), 0) FROM tax_calculations tc
			WHERE tc.property_id=? AND tc.created_at BETWEEN ? AND ?
			AND JSON_EXTRACT(tc.taxes, '$[*].code') LIKE ?`,
			propertyID, periodStart, periodEnd, "%"+rule.Code+"%").Scan(&collected)

		taxes = append(taxes, map[string]interface{}{
			"name":      rule.Name,
			"code":      rule.Code,
			"rate":      rule.Rate,
			"collected": collected,
			"remitted":  0,
			"outstanding": collected,
		})
	}

	summary["taxes"] = taxes
	return summary
}
