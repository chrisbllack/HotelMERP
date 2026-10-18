package handler

import (
	context "context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════
// RBAC, SHIFT/CASH, FOLIO, RATE MGMT, PROFILES, POS ENHANCEMENTS
// ═══════════════════════════════════════════════════════════════

// ─── 1. RBAC: ROLE-BASED ACCESS CONTROL ──────────────────────

// Permission defines a granular system permission
type Permission struct {
	ID       string `json:"id"`
	Module   string `json:"module"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
}

// RolePermission defines what a role can do
type RolePermission struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// CheckPermission verifies if a user has a specific permission
func (h *Handler) CheckPermission(ctx context.Context, userID, permission string) bool {
	var perms string
	err := h.db.QueryRow(ctx,
		`SELECT permissions FROM user_property_roles WHERE user_id=? LIMIT 1`, userID).Scan(&perms)
	if err != nil {
		return false
	}
	if perms == `["*"]` {
		return true // Admin wildcard
	}
	return strings.Contains(perms, permission)
}

// ListPermissions returns all defined permissions in the system
func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	permissions := []Permission{
		// Front Desk
		{ID: "reservations.create", Module: "reservations", Action: "create", Resource: "reservation"},
		{ID: "reservations.view", Module: "reservations", Action: "view", Resource: "reservation"},
		{ID: "reservations.modify", Module: "reservations", Action: "modify", Resource: "reservation"},
		{ID: "reservations.cancel", Module: "reservations", Action: "cancel", Resource: "reservation"},
		{ID: "front_desk.check_in", Module: "front_desk", Action: "check_in", Resource: "reservation"},
		{ID: "front_desk.check_out", Module: "front_desk", Action: "check_out", Resource: "reservation"},
		{ID: "front_desk.room_assign", Module: "front_desk", Action: "assign", Resource: "room"},
		{ID: "front_desk.folio_view", Module: "front_desk", Action: "view", Resource: "folio"},
		{ID: "front_desk.folio_post", Module: "front_desk", Action: "post_charge", Resource: "folio"},
		{ID: "front_desk.folio_transfer", Module: "front_desk", Action: "transfer", Resource: "folio"},
		{ID: "front_desk.folio_split", Module: "front_desk", Action: "split", Resource: "folio"},
		{ID: "front_desk.rate_override", Module: "front_desk", Action: "rate_override", Resource: "reservation"},
		{ID: "front_desk.discount", Module: "front_desk", Action: "discount", Resource: "folio"},
		{ID: "front_desk.void", Module: "front_desk", Action: "void", Resource: "folio"},
		{ID: "front_desk.refund", Module: "front_desk", Action: "refund", Resource: "folio"},
		// Rooms
		{ID: "rooms.view", Module: "rooms", Action: "view", Resource: "room"},
		{ID: "rooms.update_status", Module: "rooms", Action: "update_status", Resource: "room"},
		// POS
		{ID: "pos.create_order", Module: "pos", Action: "create_order", Resource: "pos_order"},
		{ID: "pos.close_order", Module: "pos", Action: "close_order", Resource: "pos_order"},
		{ID: "pos.void_order", Module: "pos", Action: "void_order", Resource: "pos_order"},
		{ID: "pos.discount", Module: "pos", Action: "discount", Resource: "pos_order"},
		{ID: "pos.refund", Module: "pos", Action: "refund", Resource: "pos_order"},
		{ID: "pos.view_menu", Module: "pos", Action: "view_menu", Resource: "menu_item"},
		// Finance
		{ID: "finance.view", Module: "finance", Action: "view", Resource: "report"},
		{ID: "finance.post_journal", Module: "finance", Action: "post_journal", Resource: "journal"},
		{ID: "finance.bank_reconciliation", Module: "finance", Action: "reconcile", Resource: "bank_account"},
		{ID: "finance.accounts_receivable", Module: "finance", Action: "view", Resource: "receivable"},
		{ID: "finance.accounts_payable", Module: "finance", Action: "view", Resource: "payable"},
		// HR
		{ID: "hr.employees", Module: "hr", Action: "view", Resource: "employee"},
		{ID: "hr.attendance", Module: "hr", Action: "view", Resource: "attendance"},
		{ID: "hr.leave", Module: "hr", Action: "manage", Resource: "leave_request"},
		{ID: "hr.payroll", Module: "hr", Action: "process", Resource: "payroll"},
		// Settings
		{ID: "settings.manage", Module: "settings", Action: "manage", Resource: "system"},
		{ID: "settings.users", Module: "settings", Action: "manage", Resource: "user"},
		{ID: "settings.rates", Module: "settings", Action: "manage", Resource: "rate"},
		// Reports
		{ID: "reports.night_audit", Module: "reports", Action: "run", Resource: "night_audit"},
		{ID: "reports.executive", Module: "reports", Action: "view", Resource: "executive"},
		{ID: "reports.financial", Module: "reports", Action: "view", Resource: "financial"},
		// Approval
		{ID: "approval.approve", Module: "approval", Action: "approve", Resource: "request"},
		{ID: "approval.void", Module: "approval", Action: "approve_void", Resource: "folio_charge"},
		{ID: "approval.discount", Module: "approval", Action: "approve_discount", Resource: "folio_charge"},
		{ID: "approval.refund", Module: "approval", Action: "approve_refund", Resource: "payment"},
	}

	respondJSON(w, 200, map[string]interface{}{
		"permissions": permissions,
		"total":       len(permissions),
		"modules":     []string{"reservations", "front_desk", "rooms", "pos", "finance", "hr", "settings", "reports", "approval"},
	})
}

// ListRoles returns role definitions with their permission sets
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles := []RolePermission{
		{Role: "admin", Permissions: []string{"*"}},
		{Role: "general_manager", Permissions: []string{"reservations.*", "front_desk.*", "rooms.*", "pos.*", "finance.*", "hr.*", "settings.*", "reports.*", "approval.*"}},
		{Role: "front_desk_manager", Permissions: []string{"reservations.*", "front_desk.*", "rooms.view", "rooms.update_status", "pos.view_menu", "reports.night_audit"}},
		{Role: "receptionist", Permissions: []string{"reservations.create", "reservations.view", "front_desk.check_in", "front_desk.check_out", "front_desk.room_assign", "front_desk.folio_view", "front_desk.folio_post", "rooms.view"}},
		{Role: "cashier", Permissions: []string{"pos.*", "front_desk.folio_view", "front_desk.folio_post", "finance.accounts_receivable"}},
		{Role: "housekeeping_supervisor", Permissions: []string{"rooms.view", "rooms.update_status"}},
		{Role: "restaurant_manager", Permissions: []string{"pos.*", "rooms.view"}},
		{Role: "waiter", Permissions: []string{"pos.create_order", "pos.close_order", "pos.view_menu", "rooms.view"}},
		{Role: "chef", Permissions: []string{"pos.view_menu"}},
		{Role: "accountant", Permissions: []string{"finance.*", "hr.payroll", "reports.financial"}},
		{Role: "hr_manager", Permissions: []string{"hr.*"}},
		{Role: "owner", Permissions: []string{"reports.executive", "reports.financial", "reports.*"}},
	}

	respondJSON(w, 200, map[string]interface{}{
		"roles": roles,
		"total": len(roles),
	})
}

// ─── 2. SHIFT & CASH MANAGEMENT ──────────────────────────────

// OpenShift starts a new cash shift
func (h *Handler) OpenShift(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	var req struct {
		CashierID   string  `json:"cashier_id"`
		OpeningCash float64 `json:"opening_cash"`
		POSOutlet   string  `json:"pos_outlet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	// Check if there's already an open shift
	var existingShift string
	err := h.db.QueryRow(ctx,
		`SELECT id FROM cash_shifts WHERE property_id=? AND cashier_id=? AND status='open'`,
		propertyID, req.CashierID).Scan(&existingShift)
	if err == nil {
		respondJSON(w, 400, map[string]interface{}{"error": "You already have an open shift", "shift_id": existingShift})
		return
	}

	id := uuid.New().String()
	_, err = h.db.Exec(ctx, `
		INSERT INTO cash_shifts (id, property_id, cashier_id, opening_cash, pos_outlet, status, opened_at)
		VALUES (?, ?, ?, ?, ?, 'open', ?)`,
		id, propertyID, req.CashierID, req.OpeningCash, req.POSOutlet, time.Now().UTC())
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to open shift"})
		return
	}

	log.Printf("[SHIFT] Shift %s opened by %s with ₦%.2f", id[:8], req.CashierID, req.OpeningCash)
	respondJSON(w, 201, map[string]interface{}{
		"id": id, "opening_cash": req.OpeningCash, "status": "open",
	})
}

// CloseShift closes the current cash shift with reconciliation
func (h *Handler) CloseShift(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	var req struct {
		ShiftID       string  `json:"shift_id"`
		ClosingCash   float64 `json:"closing_cash"`
		CardTotal     float64 `json:"card_total"`
		TransferTotal float64 `json:"transfer_total"`
		Notes         string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	// Calculate expected cash
	var openingCash, cashSales, cashRefunds float64
	var shiftCashierID string
	_ = h.db.QueryRow(ctx,
		`SELECT cashier_id, opening_cash FROM cash_shifts WHERE id=? AND property_id=?`, req.ShiftID, propertyID).Scan(&shiftCashierID, &openingCash)

	_ = h.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM payment_transactions WHERE property_id=? AND status='successful' AND payment_method='cash' AND processed_by=? AND created_at >= (SELECT opened_at FROM cash_shifts WHERE id=?)`,
		propertyID, shiftCashierID, req.ShiftID).Scan(&cashSales)

	expectedCash := openingCash + cashSales - cashRefunds
	variance := req.ClosingCash - expectedCash

	// Calculate totals
	var totalSales float64
	_ = h.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM payment_transactions WHERE property_id=? AND status='successful' AND processed_by=? AND created_at >= (SELECT opened_at FROM cash_shifts WHERE id=?)`,
		propertyID, shiftCashierID, req.ShiftID).Scan(&totalSales)

	_, err := h.db.Exec(ctx, `
		UPDATE cash_shifts SET status='closed', closing_cash=?, expected_cash=?, variance=?,
			card_total=?, transfer_total=?, total_sales=?, notes=?, closed_at=?
		WHERE id=? AND property_id=?`,
		req.ClosingCash, expectedCash, variance, req.CardTotal, req.TransferTotal,
		totalSales, req.Notes, time.Now().UTC(), req.ShiftID, propertyID)
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to close shift"})
		return
	}

	log.Printf("[SHIFT] Shift %s closed. Variance: ₦%.2f", req.ShiftID[:8], variance)
	respondJSON(w, 200, map[string]interface{}{
		"shift_id":       req.ShiftID,
		"opening_cash":   openingCash,
		"cash_sales":     cashSales,
		"expected_cash":  expectedCash,
		"closing_cash":   req.ClosingCash,
		"variance":       variance,
		"card_total":     req.CardTotal,
		"transfer_total": req.TransferTotal,
		"total_sales":    totalSales,
		"status":         "closed",
	})
}

// ListShifts returns shift history
func (h *Handler) ListCashShifts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	rows, err := h.db.Query(ctx, `
		SELECT cs.id, cs.cashier_id, cs.opening_cash, cs.closing_cash, cs.expected_cash,
			cs.variance, cs.card_total, cs.transfer_total, cs.total_sales, cs.status, cs.opened_at, cs.closed_at
		FROM cash_shifts cs WHERE cs.property_id=?
		ORDER BY cs.opened_at DESC LIMIT 50`, propertyID)
	if err != nil {
		respondJSON(w, 200, map[string]interface{}{"shifts": []interface{}{}})
		return
	}
	defer rows.Close()

	shifts := []map[string]interface{}{}
	for rows.Next() {
		var id, cashierID, status, openedAt string
		var opening, closing, expected, variance, card, transfer, total float64
		var closedAt *string
		rows.Scan(&id, &cashierID, &opening, &closing, &expected, &variance, &card, &transfer, &total, &status, &openedAt, &closedAt)
		shifts = append(shifts, map[string]interface{}{
			"id": id, "cashier_id": cashierID, "opening_cash": opening,
			"closing_cash": closing, "expected_cash": expected, "variance": variance,
			"card_total": card, "transfer_total": transfer, "total_sales": total,
			"status": status, "opened_at": openedAt, "closed_at": closedAt,
		})
	}

	respondJSON(w, 200, map[string]interface{}{"shifts": shifts})
}

// ─── 3. FOLIO ENHANCEMENTS ──────────────────────────────────

// SplitFolio splits a folio into two
func (h *Handler) SplitFolio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		FolioID   string   `json:"folio_id"`
		ChargeIDs []string `json:"charge_ids"` // charges to move to new folio
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	// Create new folio
	newFolioID := uuid.New().String()
	var reservationID, propertyID string
	_ = h.db.QueryRow(ctx,
		`SELECT reservation_id, property_id FROM folios WHERE id=?`, req.FolioID).Scan(&reservationID, &propertyID)

	_, err := h.db.Exec(ctx, `
		INSERT INTO folios (id, reservation_id, property_id, status, created_at)
		VALUES (?, ?, ?, 'open', ?)`,
		newFolioID, reservationID, propertyID, time.Now().UTC())
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to create split folio"})
		return
	}

	// Move charges to new folio
	for _, chargeID := range req.ChargeIDs {
		_, _ = h.db.Exec(ctx, `UPDATE folio_charges SET folio_id=? WHERE id=?`, newFolioID, chargeID)
	}

	log.Printf("[FOLIO] Split %s → %s (%d charges moved)", req.FolioID[:8], newFolioID[:8], len(req.ChargeIDs))
	respondJSON(w, 200, map[string]interface{}{
		"original_folio": req.FolioID,
		"new_folio":      newFolioID,
		"charges_moved":  len(req.ChargeIDs),
	})
}

// TransferFolio transfers charges between folios
func (h *Handler) TransferFolio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := r.Header.Get("X-Property-ID")

	var req struct {
		ChargeIDs   []string `json:"charge_ids"`
		TargetFolio string   `json:"target_folio_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}
	if req.TargetFolio == "" || len(req.ChargeIDs) == 0 {
		respondJSON(w, 400, map[string]interface{}{"error": "charge_ids and target_folio_id are required"})
		return
	}

	// Target folio must belong to this property — transfers never cross
	// the tenancy boundary.
	var target string
	if err := h.db.QueryRow(ctx, `
		SELECT id::text FROM folios WHERE id = $1 AND property_id = $2
	`, req.TargetFolio, propertyID).Scan(&target); err != nil {
		respondJSON(w, 404, map[string]interface{}{"error": "Target folio not found"})
		return
	}

	// Move the lines, remembering every folio touched so both sides of the
	// transfer get their balances recomputed from the source of truth.
	touched := map[string]bool{target: true}
	transferred := 0
	for _, chargeID := range req.ChargeIDs {
		var src string
		if err := h.db.QueryRow(ctx, `
			SELECT folio_id::text FROM folio_lines WHERE id = $1 AND property_id = $2
		`, chargeID, propertyID).Scan(&src); err != nil {
			continue // unknown or foreign charge — skip
		}
		if _, err := h.db.Exec(ctx, `
			UPDATE folio_lines SET folio_id = $1 WHERE id = $2 AND property_id = $3
		`, target, chargeID, propertyID); err == nil {
			transferred++
			touched[src] = true
		}
	}
	for fid := range touched {
		if err := refreshFolioBalance(ctx, h.db, fid); err != nil {
			log.Printf("[FOLIO] Balance refresh failed for %s: %v", fid, err)
		}
	}

	log.Printf("[FOLIO] Transferred %d charges to %s", transferred, target[:8])
	respondJSON(w, 200, map[string]interface{}{
		"transferred": transferred, "target_folio": target,
	})
}

// ─── 4. RATE MANAGEMENT ENHANCEMENTS ────────────────────────

// CreateSeasonalRate creates a seasonal rate override
func (h *Handler) CreateSeasonalRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	var req struct {
		RoomTypeID   string  `json:"room_type_id"`
		SeasonName   string  `json:"season_name"`
		StartDate    string  `json:"start_date"`
		EndDate      string  `json:"end_date"`
		BasePrice    float64 `json:"base_price"`
		MinStay      int     `json:"min_stay_nights"`
		TaxInclusive bool    `json:"tax_inclusive"`
		PlanCode     string  `json:"plan_code"` // BAR, CORP, OTA
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	id := uuid.New().String()
	taxInclusive := 0
	if req.TaxInclusive {
		taxInclusive = 1
	}

	_, err := h.db.Exec(ctx, `
		INSERT INTO seasonal_rates (id, property_id, room_type_id, season_name, start_date, end_date,
			base_price, min_stay, tax_inclusive, plan_code, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		id, propertyID, req.RoomTypeID, req.SeasonName, req.StartDate, req.EndDate,
		req.BasePrice, req.MinStay, taxInclusive, req.PlanCode, time.Now().UTC())
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to create seasonal rate"})
		return
	}

	log.Printf("[RATE] Seasonal rate created: %s (%s to %s) ₦%.0f/night", req.SeasonName, req.StartDate, req.EndDate, req.BasePrice)
	respondJSON(w, 201, map[string]interface{}{"id": id, "message": "Seasonal rate created"})
}

// CreateBusinessSourceRate creates special rates for business sources/travel agents
func (h *Handler) CreateBusinessSourceRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	var req struct {
		SourceID     string  `json:"source_id"`   // company or travel agent ID
		SourceType   string  `json:"source_type"` // company, travel_agent, ota
		RoomTypeID   string  `json:"room_type_id"`
		RatePerNight float64 `json:"rate_per_night"`
		TaxInclusive bool    `json:"tax_inclusive"`
		ValidFrom    string  `json:"valid_from"`
		ValidTo      string  `json:"valid_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	id := uuid.New().String()
	taxInclusive := 0
	if req.TaxInclusive {
		taxInclusive = 1
	}

	_, err := h.db.Exec(ctx, `
		INSERT INTO business_rates (id, property_id, source_id, source_type, room_type_id,
			rate_per_night, tax_inclusive, valid_from, valid_to, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		id, propertyID, req.SourceID, req.SourceType, req.RoomTypeID,
		req.RatePerNight, taxInclusive, req.ValidFrom, req.ValidTo, time.Now().UTC())
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to create business rate"})
		return
	}

	log.Printf("[RATE] Business rate created: %s %s ₦%.0f/night", req.SourceType, req.SourceID, req.RatePerNight)
	respondJSON(w, 201, map[string]interface{}{"id": id, "message": "Business source rate created"})
}

// GetBestRate returns the best available rate for a room type and date range
func (h *Handler) GetBestRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	roomTypeID := r.URL.Query().Get("room_type_id")
	checkIn := r.URL.Query().Get("check_in")
	checkOut := r.URL.Query().Get("check_out")
	sourceID := r.URL.Query().Get("source_id")

	// Get base rate
	var baseRate float64
	_ = h.db.QueryRow(ctx,
		`SELECT base_price FROM room_types WHERE id=? AND property_id=?`, roomTypeID, propertyID).Scan(&baseRate)

	// Check seasonal rate
	var seasonalRate float64
	err := h.db.QueryRow(ctx,
		`SELECT base_price FROM seasonal_rates WHERE room_type_id=? AND property_id=? AND enabled=1
		AND start_date <= ? AND end_date >= ? LIMIT 1`,
		roomTypeID, propertyID, checkIn, checkOut).Scan(&seasonalRate)
	if err == nil && seasonalRate > 0 {
		baseRate = seasonalRate
	}

	// Check business source rate
	if sourceID != "" {
		var bizRate float64
		err := h.db.QueryRow(ctx,
			`SELECT rate_per_night FROM business_rates WHERE source_id=? AND room_type_id=? AND property_id=? AND enabled=1
			AND valid_from <= ? AND valid_to >= ? LIMIT 1`,
			sourceID, roomTypeID, propertyID, checkIn, checkOut).Scan(&bizRate)
		if err == nil && bizRate > 0 && bizRate < baseRate {
			baseRate = bizRate
		}
	}

	// Get all rate plans and their prices
	rows, err := h.db.Query(ctx,
		`SELECT code, name, base_price FROM rate_plans WHERE property_id=? AND status='active' ORDER BY base_price ASC`,
		propertyID)
	if err == nil {
		defer rows.Close()
		rates := []map[string]interface{}{}
		for rows.Next() {
			var code, name string
			var price float64
			rows.Scan(&code, &name, &price)
			rates = append(rates, map[string]interface{}{
				"code": code, "name": name, "rate": price,
				"is_best": price <= baseRate,
			})
		}
		respondJSON(w, 200, map[string]interface{}{
			"room_type_id": roomTypeID,
			"check_in":     checkIn,
			"check_out":    checkOut,
			"base_rate":    baseRate,
			"all_rates":    rates,
			"best_rate":    baseRate,
		})
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"room_type_id": roomTypeID,
		"base_rate":    baseRate,
		"best_rate":    baseRate,
	})
}

// ─── 5. PROFILE MANAGEMENT ──────────────────────────────────

// CreateCompanyProfile creates a company profile for corporate bookings
func (h *Handler) CreateCompanyProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	var req struct {
		Name        string  `json:"name"`
		ContactName string  `json:"contact_name"`
		Email       string  `json:"email"`
		Phone       string  `json:"phone"`
		Address     string  `json:"address"`
		TaxID       string  `json:"tax_id"`
		PaymentTerm string  `json:"payment_term"` // net30, net60, prepaid
		CreditLimit float64 `json:"credit_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	id := uuid.New().String()
	_, err := h.db.Exec(ctx, `
		INSERT INTO company_profiles (id, property_id, name, contact_name, email, phone, address,
			tax_id, payment_term, credit_limit, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)`,
		id, propertyID, req.Name, req.ContactName, req.Email, req.Phone, req.Address,
		req.TaxID, req.PaymentTerm, req.CreditLimit, time.Now().UTC())
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to create company profile"})
		return
	}

	respondJSON(w, 201, map[string]interface{}{"id": id, "message": "Company profile created"})
}

// CreateTravelAgentProfile creates a travel agent profile
func (h *Handler) CreateTravelAgentProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	var req struct {
		Name        string  `json:"name"`
		ContactName string  `json:"contact_name"`
		Email       string  `json:"email"`
		Phone       string  `json:"phone"`
		Address     string  `json:"address"`
		Commission  float64 `json:"commission_pct"` // e.g. 15%
		PaymentTerm string  `json:"payment_term"`
		AgreementNo string  `json:"agreement_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	id := uuid.New().String()
	_, err := h.db.Exec(ctx, `
		INSERT INTO travel_agent_profiles (id, property_id, name, contact_name, email, phone, address,
			commission_pct, payment_term, agreement_number, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)`,
		id, propertyID, req.Name, req.ContactName, req.Email, req.Phone, req.Address,
		req.Commission, req.PaymentTerm, req.AgreementNo, time.Now().UTC())
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to create travel agent profile"})
		return
	}

	respondJSON(w, 201, map[string]interface{}{"id": id, "message": "Travel agent profile created"})
}

// ListCompanyProfiles lists all company profiles
func (h *Handler) ListCompanyProfiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	rows, err := h.db.Query(ctx,
		`SELECT id, name, contact_name, email, phone, payment_term, credit_limit, status
		FROM company_profiles WHERE property_id=? AND status='active' ORDER BY name`, propertyID)
	if err != nil {
		respondJSON(w, 200, map[string]interface{}{"companies": []interface{}{}})
		return
	}
	defer rows.Close()

	companies := []map[string]interface{}{}
	for rows.Next() {
		var id, name, contact, email, phone, term, status string
		var credit float64
		rows.Scan(&id, &name, &contact, &email, &phone, &term, &credit, &status)
		companies = append(companies, map[string]interface{}{
			"id": id, "name": name, "contact_name": contact, "email": email,
			"phone": phone, "payment_term": term, "credit_limit": credit, "status": status,
		})
	}

	respondJSON(w, 200, map[string]interface{}{"companies": companies})
}

// ─── 6. RESERVATION ENHANCEMENTS ─────────────────────────────

// SearchReservations searches reservations by guest name, date, room, status
func (h *Handler) SearchReservations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	query := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	checkIn := r.URL.Query().Get("check_in")

	sqlQuery := `
		SELECT r.id, r.guest_name, r.room_id, r.room_type_id, r.check_in_date, r.check_out_date,
			r.status, r.total_amount, r.source, r.created_at
		FROM reservations r
		WHERE r.property_id=?`

	args := []interface{}{propertyID}

	if query != "" {
		sqlQuery += ` AND (r.guest_name LIKE ? OR r.id LIKE ?)`
		args = append(args, "%"+query+"%", "%"+query+"%")
	}
	if status != "" {
		sqlQuery += ` AND r.status=?`
		args = append(args, status)
	}
	if checkIn != "" {
		sqlQuery += ` AND r.check_in_date=?`
		args = append(args, checkIn)
	}

	sqlQuery += ` ORDER BY r.created_at DESC LIMIT 50`

	rows, err := h.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		respondJSON(w, 200, map[string]interface{}{"reservations": []interface{}{}})
		return
	}
	defer rows.Close()

	reservations := []map[string]interface{}{}
	for rows.Next() {
		var id, guestName, roomID, roomTypeID, checkInD, checkOutD, status, source, created string
		var total float64
		rows.Scan(&id, &guestName, &roomID, &roomTypeID, &checkInD, &checkOutD, &status, &total, &source, &created)
		reservations = append(reservations, map[string]interface{}{
			"id": id, "guest_name": guestName, "room_id": roomID,
			"room_type_id": roomTypeID, "check_in": checkInD, "check_out": checkOutD,
			"status": status, "total": total, "source": source, "created_at": created,
		})
	}

	respondJSON(w, 200, map[string]interface{}{"reservations": reservations})
}

// GroupCheckOut performs check-out for all rooms in a group booking
func (h *Handler) GroupCheckOut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		GroupID string `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(ctx,
		`UPDATE reservations SET status='checked_out' WHERE group_id=? AND status='checked_in'`, req.GroupID)
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to process group checkout"})
		return
	}

	rows := result.RowsAffected()
	log.Printf("[CHECKOUT] Group %s: %d rooms checked out", req.GroupID, rows)
	respondJSON(w, 200, map[string]interface{}{
		"group_id": req.GroupID, "rooms_checked_out": rows,
	})
}

// ─── 7. CHECK-IN/CHECK-OUT ENHANCEMENTS ─────────────────────

// ProcessEarlyCheckIn handles early check-in with fee calculation
func (h *Handler) ProcessEarlyCheckIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ReservationID string  `json:"reservation_id"`
		EarlyHour     string  `json:"early_hour"` // e.g. "10:00"
		Fee           float64 `json:"fee"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	// Post early check-in fee to folio
	var folioID string
	err := h.db.QueryRow(ctx,
		`SELECT id FROM folios WHERE reservation_id=? LIMIT 1`, req.ReservationID).Scan(&folioID)
	if err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "No folio found for this reservation"})
		return
	}

	_, _ = h.db.Exec(ctx, `
		INSERT INTO folio_charges (id, folio_id, description, amount, type, date, posted)
		VALUES (?, ?, ?, ?, 'fee', date('now'), 1)`,
		uuid.New().String(), folioID, fmt.Sprintf("Early Check-In Fee (%s)", req.EarlyHour), req.Fee)

	respondJSON(w, 200, map[string]interface{}{
		"reservation_id": req.ReservationID,
		"fee_charged":    req.Fee,
		"message":        fmt.Sprintf("Early check-in at %s. Fee ₦%.2f posted to folio.", req.EarlyHour, req.Fee),
	})
}

// ProcessLateCheckOut handles late check-out with fee
func (h *Handler) ProcessLateCheckOut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ReservationID string  `json:"reservation_id"`
		LateHour      string  `json:"late_hour"`
		Fee           float64 `json:"fee"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	var folioID string
	err := h.db.QueryRow(ctx,
		`SELECT id FROM folios WHERE reservation_id=? LIMIT 1`, req.ReservationID).Scan(&folioID)
	if err == nil {
		_, _ = h.db.Exec(ctx, `
			INSERT INTO folio_charges (id, folio_id, description, amount, type, date, posted)
			VALUES (?, ?, ?, ?, 'fee', date('now'), 1)`,
			uuid.New().String(), folioID, fmt.Sprintf("Late Check-Out Fee (%s)", req.LateHour), req.Fee)
	}

	respondJSON(w, 200, map[string]interface{}{
		"reservation_id": req.ReservationID,
		"fee_charged":    req.Fee,
		"message":        fmt.Sprintf("Late check-out at %s. Fee ₦%.2f posted.", req.LateHour, req.Fee),
	})
}

// ProcessDayUse handles day-use bookings
func (h *Handler) ProcessDayUse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	var req struct {
		RoomTypeID string  `json:"room_type_id"`
		GuestName  string  `json:"guest_name"`
		StartTime  string  `json:"start_time"`
		EndTime    string  `json:"end_time"`
		Rate       float64 `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	id := uuid.New().String()
	_, err := h.db.Exec(ctx, `
		INSERT INTO reservations (id, property_id, guest_name, room_type_id, check_in_date, check_out_date,
			status, source, total_amount, booking_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'confirmed', 'walk_in', ?, 'day_use', ?)`,
		id, propertyID, req.GuestName, req.RoomTypeID, req.StartTime, req.EndTime,
		req.Rate, time.Now().UTC())
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to create day-use booking"})
		return
	}

	respondJSON(w, 201, map[string]interface{}{
		"id": id, "type": "day_use", "rate": req.Rate, "message": "Day-use booking created",
	})
}

// GenerateCardKey generates an electronic card key (simulated)
func (h *Handler) GenerateCardKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ReservationID string `json:"reservation_id"`
		RoomNumber    string `json:"room_number"`
		ValidFrom     string `json:"valid_from"`
		ValidTo       string `json:"valid_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	keyCode := fmt.Sprintf("KEY-%s-%d", req.RoomNumber, time.Now().UnixNano()%1000000)
	_, _ = h.db.Exec(ctx, `
		INSERT INTO card_keys (id, reservation_id, room_number, key_code, valid_from, valid_to, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'active', ?)`,
		uuid.New().String(), req.ReservationID, req.RoomNumber, keyCode,
		req.ValidFrom, req.ValidTo, time.Now().UTC())

	respondJSON(w, 201, map[string]interface{}{
		"key_code":   keyCode,
		"room":       req.RoomNumber,
		"valid_from": req.ValidFrom,
		"valid_to":   req.ValidTo,
		"message":    "Electronic card key generated",
	})
}

// VoidReservation cancels a reservation with blacklisting check
func (h *Handler) VoidReservation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ReservationID        string  `json:"reservation_id"`
		Reason               string  `json:"reason"`
		ApplyCancellationFee bool    `json:"apply_cancellation_fee"`
		CancellationFee      float64 `json:"cancellation_fee"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	// Check if guest is blacklisted
	var guestName, guestPhone string
	_ = h.db.QueryRow(ctx,
		`SELECT guest_name, guest_phone FROM reservations WHERE id=?`, req.ReservationID).Scan(&guestName, &guestPhone)

	var isBlacklisted bool
	_ = h.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM blacklisted_guests WHERE phone=? OR name=?)`, guestPhone, guestName).Scan(&isBlacklisted)

	if isBlacklisted {
		respondJSON(w, 403, map[string]interface{}{
			"error":      "GUEST BLACKLISTED",
			"guest_name": guestName,
			"message":    "This guest is on the blacklist. Alert security.",
		})
		return
	}

	// Process cancellation
	_, _ = h.db.Exec(ctx,
		`UPDATE reservations SET status='cancelled' WHERE id=?`, req.ReservationID)

	if req.ApplyCancellationFee && req.CancellationFee > 0 {
		var folioID string
		err := h.db.QueryRow(ctx,
			`SELECT id FROM folios WHERE reservation_id=? LIMIT 1`, req.ReservationID).Scan(&folioID)
		if err == nil {
			_, _ = h.db.Exec(ctx, `
				INSERT INTO folio_charges (id, folio_id, description, amount, type, date, posted)
				VALUES (?, ?, ?, ?, 'fee', date('now'), 1)`,
				uuid.New().String(), folioID, "Cancellation Fee", req.CancellationFee)
		}
	}

	log.Printf("[RESERVATION] Cancelled %s. Reason: %s", req.ReservationID[:8], req.Reason)
	respondJSON(w, 200, map[string]interface{}{
		"reservation_id": req.ReservationID, "status": "cancelled",
		"cancellation_fee": req.CancellationFee,
	})
}

// ─── 8. RETAIL POS ENHANCEMENTS ─────────────────────────────

// CreateRetailSale creates a retail POS transaction with 5 price levels
func (h *Handler) CreateRetailSale(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	var req struct {
		CustomerID string `json:"customer_id"`
		PriceLevel int    `json:"price_level"` // 1-5
		Items      []struct {
			ProductID string  `json:"product_id"`
			Quantity  int     `json:"quantity"`
			Discount  float64 `json:"discount_pct"`
			SerialNo  string  `json:"serial_number,omitempty"`
		} `json:"items"`
		PaymentMethod string  `json:"payment_method"` // cash, card, transfer, mobile
		DepositAmount float64 `json:"deposit_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	saleID := uuid.New().String()
	totalAmount := 0.0

	for _, item := range req.Items {
		// Get price based on price level
		var price float64
		_ = h.db.QueryRow(ctx,
			`SELECT price FROM inventory_products WHERE id=? AND property_id=?`, item.ProductID, propertyID).Scan(&price)

		// Apply price level (1=full, 2=95%, 3=90%, 4=85%, 5=80%)
		levelMultipliers := []float64{1.0, 0.95, 0.90, 0.85, 0.80}
		if req.PriceLevel >= 1 && req.PriceLevel <= 5 {
			price *= levelMultipliers[req.PriceLevel-1]
		}

		lineTotal := price * float64(item.Quantity) * (1 - item.Discount/100)
		totalAmount += lineTotal

		_, _ = h.db.Exec(ctx, `
			INSERT INTO pos_order_items (id, order_id, menu_item_id, quantity, unit_price, total)
			VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), saleID, item.ProductID, item.Quantity, price, lineTotal)
	}

	// Record the sale
	_, _ = h.db.Exec(ctx, `
		INSERT INTO pos_orders (id, property_id, customer_id, total_amount, payment_method, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'closed', ?)`,
		saleID, propertyID, req.CustomerID, totalAmount, req.PaymentMethod, time.Now().UTC())

	// Track deposit if any
	if req.DepositAmount > 0 {
		_, _ = h.db.Exec(ctx, `
			INSERT INTO deposits (id, property_id, customer_id, sale_id, amount, status, created_at)
			VALUES (?, ?, ?, ?, ?, 'active', ?)`,
			uuid.New().String(), propertyID, req.CustomerID, saleID, req.DepositAmount, time.Now().UTC())
	}

	// Record serial numbers
	for _, item := range req.Items {
		if item.SerialNo != "" {
			_, _ = h.db.Exec(ctx, `
				INSERT INTO serial_numbers (id, product_id, serial_number, sale_id, property_id, created_at)
				VALUES (?, ?, ?, ?, ?, ?)`,
				uuid.New().String(), item.ProductID, item.SerialNo, saleID, propertyID, time.Now().UTC())
		}
	}

	respondJSON(w, 201, map[string]interface{}{
		"sale_id": saleID,
		"total":   math.Round(totalAmount*100) / 100,
		"deposit": req.DepositAmount,
		"balance": math.Round((totalAmount-req.DepositAmount)*100) / 100,
	})
}

// VoidSale voids a retail POS sale
func (h *Handler) VoidSale(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		SaleID string `json:"sale_id"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	_, _ = h.db.Exec(ctx, `UPDATE pos_orders SET status='voided', void_reason=? WHERE id=?`, req.Reason, req.SaleID)
	log.Printf("[POS] Sale %s voided. Reason: %s", req.SaleID[:8], req.Reason)
	respondJSON(w, 200, map[string]interface{}{"sale_id": req.SaleID, "status": "voided"})
}

// GetCustomerBalance returns customer outstanding balance
func (h *Handler) GetCustomerBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	propertyID := getPropertyID(r)

	customerID := r.URL.Query().Get("customer_id")

	var totalCharges, totalPayments float64
	_ = h.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_amount), 0) FROM pos_orders WHERE customer_id=? AND property_id=? AND status='closed'`,
		customerID, propertyID).Scan(&totalCharges)
	_ = h.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM payment_transactions WHERE guest_id=? AND property_id=? AND status='successful'`,
		customerID, propertyID).Scan(&totalPayments)

	balance := totalCharges - totalPayments
	respondJSON(w, 200, map[string]interface{}{
		"customer_id":    customerID,
		"total_charges":  totalCharges,
		"total_payments": totalPayments,
		"balance":        math.Round(balance*100) / 100,
	})
}

// ReprintReceipt reprints a receipt (returns the receipt data)
func (h *Handler) ReprintReceipt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	saleID := r.URL.Query().Get("sale_id")

	var total float64
	var paymentMethod, status, createdAt string
	err := h.db.QueryRow(ctx,
		`SELECT total_amount, payment_method, status, created_at FROM pos_orders WHERE id=?`, saleID).Scan(&total, &paymentMethod, &status, &createdAt)
	if err != nil {
		respondJSON(w, 404, map[string]interface{}{"error": "Sale not found"})
		return
	}

	rows, _ := h.db.Query(ctx,
		`SELECT poi.quantity, poi.unit_price, poi.total, mi.name FROM pos_order_items poi
		JOIN menu_items mi ON poi.menu_item_id = mi.id WHERE poi.order_id=?`, saleID)
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var qty int
		var price, lineTotal float64
		var name string
		rows.Scan(&qty, &price, &lineTotal, &name)
		items = append(items, map[string]interface{}{
			"name": name, "quantity": qty, "unit_price": price, "total": lineTotal,
		})
	}

	respondJSON(w, 200, map[string]interface{}{
		"sale_id":        saleID,
		"items":          items,
		"total":          total,
		"payment_method": paymentMethod,
		"status":         status,
		"date":           createdAt,
	})
}
