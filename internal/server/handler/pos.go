package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"infineon/internal/auth"
	"infineon/internal/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── Outlets ────────────────────────────────────────────────

func (h *Handler) ListOutlets(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	if propertyID == "" {
		response.RespondError(w, "Property ID required", http.StatusBadRequest)
		return
	}

	rows, err := h.db.Query(context.Background(), `
		SELECT id, name, code, type, status FROM outlets 
		WHERE property_id = $1 AND deleted_at IS NULL ORDER BY name
	`, propertyID)
	if err != nil {
		response.RespondError(w, "Failed to query outlets", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var outlets []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, code, outletType, status string
		rows.Scan(&id, &name, &code, &outletType, &status)
		outlets = append(outlets, map[string]interface{}{
			"id": id, "name": name, "code": code, "type": outletType, "status": status,
		})
	}
	if outlets == nil {
		outlets = []map[string]interface{}{}
	}
	response.RespondSuccess(w, outlets, "Outlets retrieved successfully")
}

func (h *Handler) CreateOutlet(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	var req struct {
		Name string `json:"name"`
		Code string `json:"code"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	id := uuid.New()
	_, err := h.db.Exec(context.Background(), `
		INSERT INTO outlets (id, property_id, name, code, type, status) VALUES ($1, $2, $3, $4, $5, 'active')
	`, id, propertyID, req.Name, req.Code, req.Type)
	if err != nil {
		response.RespondError(w, "Failed to create outlet", http.StatusInternalServerError)
		return
	}
	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Outlet created successfully", http.StatusCreated)
}

// ── Tables ────────────────────────────────────────────────

func (h *Handler) ListTables(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	outletID := r.URL.Query().Get("outlet_id")

	query := `SELECT id, outlet_id, number, capacity, status, section FROM pos_tables 
		WHERE property_id = $1 AND deleted_at IS NULL ORDER BY number`
	args := []interface{}{propertyID}

	if outletID != "" {
		query = `SELECT id, outlet_id, number, capacity, status, section FROM pos_tables 
			WHERE property_id = $1 AND outlet_id = $2 AND deleted_at IS NULL ORDER BY number`
		args = append(args, outletID)
	}

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		response.RespondError(w, "Failed to query tables", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []map[string]interface{}
	for rows.Next() {
		var id, tableOutletID uuid.UUID
		var number string
		var capacity int
		var status, section string
		rows.Scan(&id, &tableOutletID, &number, &capacity, &status, &section)
		tables = append(tables, map[string]interface{}{
			"id": id, "outlet_id": tableOutletID, "number": number,
			"capacity": capacity, "status": status, "section": section,
		})
	}
	if tables == nil {
		tables = []map[string]interface{}{}
	}
	response.RespondSuccess(w, tables, "Tables retrieved successfully")
}

// ── Menu ───────────────────────────────────────────────────

func (h *Handler) ListMenuCategories(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	outletID := r.URL.Query().Get("outlet_id")

	rows, err := h.db.Query(context.Background(), `
		SELECT id, name, code, display_order FROM menu_categories 
		WHERE property_id = $1 AND outlet_id = $2 AND is_active = TRUE ORDER BY display_order
	`, propertyID, outletID)
	if err != nil {
		response.RespondError(w, "Failed to query menu categories", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var categories []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, code string
		var displayOrder int
		rows.Scan(&id, &name, &code, &displayOrder)
		categories = append(categories, map[string]interface{}{
			"id": id, "name": name, "code": code, "display_order": displayOrder,
		})
	}
	if categories == nil {
		categories = []map[string]interface{}{}
	}
	response.RespondSuccess(w, categories, "Menu categories retrieved successfully")
}

func (h *Handler) ListMenuItems(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	categoryID := r.URL.Query().Get("category_id")

	query := `SELECT id, category_id, name, code, price, tax_inclusive, is_available, preparation_time 
		FROM menu_items WHERE property_id = $1 AND deleted_at IS NULL`
	args := []interface{}{propertyID}
	argIdx := 2

	if categoryID != "" {
		query += ` AND category_id = $` + strconv.Itoa(argIdx)
		args = append(args, categoryID)
		argIdx++
	}
	query += ` ORDER BY name`

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		response.RespondError(w, "Failed to query menu items", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var id, catID uuid.UUID
		var name, code string
		var price float64
		var taxInclusive, isAvailable bool
		var prepTime *int
		rows.Scan(&id, &catID, &name, &code, &price, &taxInclusive, &isAvailable, &prepTime)
		item := map[string]interface{}{
			"id": id, "category_id": catID, "name": name, "code": code,
			"price": price, "tax_inclusive": taxInclusive, "is_available": isAvailable,
		}
		if prepTime != nil {
			item["preparation_time"] = *prepTime
		}
		items = append(items, item)
	}
	if items == nil {
		items = []map[string]interface{}{}
	}
	response.RespondSuccess(w, items, "Menu items retrieved successfully")
}

// ── POS Orders ────────────────────────────────────────────

type POSOrderRequest struct {
	OutletID    string         `json:"outlet_id"`
	TableID     string         `json:"table_id"`
	RoomID      string         `json:"room_id"` // for room charge
	GuestStayID string         `json:"guest_stay_id"`
	Items       []POSOrderItem `json:"items"`
	Notes       string         `json:"notes"`
}

type POSOrderItem struct {
	MenuItemID string  `json:"menu_item_id"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	Discount   float64 `json:"discount"`
	Notes      string  `json:"notes"`
}

func (h *Handler) CreatePOSOrder(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	var req POSOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.OutletID == "" || len(req.Items) == 0 {
		response.RespondError(w, "outlet_id and items are required", http.StatusBadRequest)
		return
	}

	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	orderID := uuid.New()
	orderNumber := fmt.Sprintf("ORD-%s", generateConfirmationNumber()[:8])
	businessDate := time.Now().Format("2006-01-02")

	// Calculate totals
	var subtotal, taxTotal float64
	for _, item := range req.Items {
		lineTotal := item.Quantity*item.UnitPrice - item.Discount
		subtotal += lineTotal
	}

	// Start transaction
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		response.RespondError(w, "Transaction failed", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	// Create order
	_, err = tx.Exec(r.Context(), `
		INSERT INTO pos_orders (id, property_id, outlet_id, table_id, order_number, status, 
			subtotal, tax_amount, total, notes, guest_stay_id, created_by, business_date)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, 'open', $6, $7, $8, $9, NULLIF($10, '')::uuid, $11, $12)
	`, orderID, propertyID, req.OutletID, req.TableID, orderNumber, subtotal, taxTotal, subtotal+taxTotal,
		req.Notes, req.GuestStayID, createdBy, businessDate)
	if err != nil {
		response.RespondError(w, "Failed to create order", http.StatusInternalServerError)
		return
	}

	// Insert order items
	for _, item := range req.Items {
		lineTotal := item.Quantity*item.UnitPrice - item.Discount
		tx.Exec(r.Context(), `
			INSERT INTO pos_order_lines (id, order_id, menu_item_id, quantity, unit_price, discount, amount, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, uuid.New(), orderID, item.MenuItemID, item.Quantity, item.UnitPrice, item.Discount, lineTotal, item.Notes)
	}

	// Update table status if assigned
	if req.TableID != "" {
		tx.Exec(r.Context(), `
			UPDATE pos_tables SET status = 'occupied' WHERE id = $1
		`, req.TableID)
	}

	tx.Commit(r.Context())

	response.RespondSuccess(w, map[string]interface{}{
		"id": orderID, "order_number": orderNumber, "subtotal": subtotal,
	}, "POS order created successfully", http.StatusCreated)
}

func (h *Handler) GetPOSOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")
	propertyID := r.Header.Get("X-Property-ID")

	var oNumber, status, notes string
	var subtotal, taxAmount, total float64
	var outletID, tableID *uuid.UUID
	var createdAt interface{}

	err := h.db.QueryRow(context.Background(), `
		SELECT order_number, status, subtotal, tax_amount, total, outlet_id, table_id, notes, created_at
		FROM pos_orders WHERE id = $1 AND property_id = $2
	`, orderID, propertyID).Scan(&oNumber, &status, &subtotal, &taxAmount, &total, &outletID, &tableID, &notes, &createdAt)
	if err != nil {
		response.RespondError(w, "Order not found", http.StatusNotFound)
		return
	}

	// Get order lines
	rows, _ := h.db.Query(context.Background(), `
		SELECT pol.id, pol.menu_item_id, mi.name, pol.quantity, pol.unit_price, pol.discount, pol.amount, pol.notes
		FROM pos_order_lines pol
		JOIN menu_items mi ON pol.menu_item_id = mi.id
		WHERE pol.order_id = $1
	`, orderID)
	defer rows.Close()

	var lines []map[string]interface{}
	for rows.Next() {
		var lineID, menuItemID uuid.UUID
		var itemName string
		var qty, unitPrice, discount, amount float64
		var itemNotes *string
		rows.Scan(&lineID, &menuItemID, &itemName, &qty, &unitPrice, &discount, &amount, &itemNotes)
		line := map[string]interface{}{
			"id": lineID, "menu_item_id": menuItemID, "name": itemName,
			"quantity": qty, "unit_price": unitPrice, "discount": discount, "amount": amount,
		}
		if itemNotes != nil {
			line["notes"] = *itemNotes
		}
		lines = append(lines, line)
	}

	response.RespondSuccess(w, map[string]interface{}{
		"id": orderID, "order_number": oNumber, "status": status,
		"subtotal": subtotal, "tax_amount": taxAmount, "total": total,
		"outlet_id": outletID, "table_id": tableID, "notes": notes,
		"lines": lines,
	}, "POS order retrieved successfully")
}

func (h *Handler) ClosePOSOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")
	propertyID := r.Header.Get("X-Property-ID")

	var req struct {
		PaymentMethod string  `json:"payment_method"`
		Amount        float64 `json:"amount"`
		RoomCharge    bool    `json:"room_charge"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Get order
	var tableID *uuid.UUID
	var guestStayID *uuid.UUID
	var total float64
	h.db.QueryRow(context.Background(), `
		SELECT table_id, guest_stay_id, total FROM pos_orders WHERE id = $1 AND property_id = $2
	`, orderID, propertyID).Scan(&tableID, &guestStayID, &total)

	// If room charge, post to guest folio
	if req.RoomCharge && guestStayID != nil {
		var folioID uuid.UUID
		h.db.QueryRow(context.Background(), `
			SELECT id FROM folios WHERE guest_stay_id = $1 AND status = 'open'
		`, *guestStayID).Scan(&folioID)

		if folioID != uuid.Nil {
			var createdBy *uuid.UUID
			if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
				createdBy = &claims.UserID
			}
			_, err := h.db.Exec(context.Background(), `
				INSERT INTO folio_lines (id, folio_id, property_id, type, charge_code, description, amount, business_date, posted_by)
				VALUES ($1, $2, $3, 'debit', 'RESTAURANT', 'Restaurant/Bar charge', $4, CURRENT_DATE, $5)
			`, uuid.New(), folioID, propertyID, total, createdBy)
			if err == nil {
				if err := refreshFolioBalance(context.Background(), h.db, folioID.String()); err != nil {
					log.Printf("[POS] Folio balance refresh failed for %s: %v", folioID.String(), err)
				}
			} else {
				log.Printf("[POS] Folio charge post failed for %s: %v", folioID.String(), err)
			}
		}
	}

	// Record payment
	if req.Amount > 0 {
		h.db.Exec(context.Background(), `
			INSERT INTO pos_payments (id, order_id, payment_method, amount, created_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, uuid.New(), orderID, req.PaymentMethod, req.Amount)
	}

	// Close order
	h.db.Exec(context.Background(), `
		UPDATE pos_orders SET status = 'closed', closed_at = NOW() WHERE id = $1
	`, orderID)

	// Free table
	if tableID != nil {
		h.db.Exec(context.Background(), `
			UPDATE pos_tables SET status = 'available' WHERE id = $1
		`, *tableID)
	}

	response.RespondSuccess(w, map[string]interface{}{
		"order_id": orderID, "total": total, "status": "closed",
	}, "POS order closed successfully")
}

// Night Audit and GetNightAudit are defined in night_audit.go

// ── Cashier Shift ─────────────────────────────────────────

func (h *Handler) OpenCashierShift(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	var req struct {
		OpeningFloat float64 `json:"opening_float"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Check for existing open shift
	var openShifts int
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM cashier_shifts WHERE user_id = $1 AND status = 'open'
	`, createdBy).Scan(&openShifts)

	if openShifts > 0 {
		response.RespondError(w, "You already have an open shift", http.StatusConflict)
		return
	}

	shiftID := uuid.New()
	shiftNumber := fmt.Sprintf("SHIFT-%s", generateConfirmationNumber()[:6])

	_, err := h.db.Exec(r.Context(), `
		INSERT INTO cashier_shifts (id, property_id, shift_number, user_id, opening_float, status, opened_at)
		VALUES ($1, $2, $3, $4, $5, 'open', NOW())
	`, shiftID, propertyID, shiftNumber, createdBy, req.OpeningFloat)
	if err != nil {
		response.RespondError(w, "Failed to open shift", http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, map[string]interface{}{
		"shift_id": shiftID, "shift_number": shiftNumber,
	}, "Cashier shift opened successfully", http.StatusCreated)
}

func (h *Handler) CloseCashierShift(w http.ResponseWriter, r *http.Request) {
	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	var req struct {
		ClosingBalance float64 `json:"closing_balance"`
		Notes          string  `json:"notes"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var shiftID uuid.UUID
	var openingFloat float64
	err := h.db.QueryRow(r.Context(), `
		SELECT id, opening_float FROM cashier_shifts WHERE user_id = $1 AND status = 'open'
	`, createdBy).Scan(&shiftID, &openingFloat)

	if err != nil {
		response.RespondError(w, "No open shift found", http.StatusNotFound)
		return
	}

	// Calculate expected balance from payments during this shift
	var totalPayments float64
	h.db.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(amount), 0) FROM pos_payments 
		WHERE created_at >= (SELECT opened_at FROM cashier_shifts WHERE id = $1)
		AND payment_method = 'cash'
	`, shiftID).Scan(&totalPayments)

	expectedBalance := openingFloat + totalPayments
	variance := req.ClosingBalance - expectedBalance

	h.db.Exec(r.Context(), `
		UPDATE cashier_shifts SET 
			closing_balance = $2, expected_balance = $3, variance = $4, 
			notes = $5, status = 'closed', closed_at = NOW()
		WHERE id = $1
	`, shiftID, req.ClosingBalance, expectedBalance, variance, req.Notes)

	response.RespondSuccess(w, map[string]interface{}{
		"shift_id":         shiftID,
		"opening_float":    openingFloat,
		"expected_balance": expectedBalance,
		"closing_balance":  req.ClosingBalance,
		"variance":         variance,
	}, "Cashier shift closed successfully")
}
