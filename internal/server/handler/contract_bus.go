package handler

// ADR-0006 contract bus — publishers, subscribers and observability routes.
//
// Producers emit domain events through the transactional outbox inside their
// own transaction (see emitDomainEvent), then Schedule delivery after commit.
// The subscribers below react without their producers knowing them:
//
//	payment.captured        → M-FIN posts Dr cash/bank, Cr Guest Ledger
//	reservation.checked_out → M-CRM accrues points, LTV and tier
//	reservation.checked_in  → M-CRM one-time welcome bonus
//	stock.movement          → M-INV drafts a replenishment PO (or alerts)
//
// Every subscriber is idempotent because delivery is at-least-once.

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"infineon/internal/auth"
	"infineon/internal/events"
	"infineon/internal/ledger"
)

// ── Publishing helpers ───────────────────────────────────────────────────

// emitDomainEvent records an outbox event through ex — pass the caller's
// transaction so the event row commits atomically with the state change that
// caused it (transactional-outbox). Returns the event id for ScheduleEvent
// after a successful commit.
func (h *Handler) emitDomainEvent(ctx context.Context, ex events.EventExecutor, propertyID, actor, etype, source, entityID string, payload interface{}) (string, error) {
	if h.Outbox == nil {
		return "", nil
	}
	if propertyID == "" {
		return "", fmt.Errorf("event %s: missing property", etype)
	}
	return h.Outbox.Emit(ctx, ex, events.EventInput{
		PropertyID: propertyID,
		Type:       etype,
		Source:     source,
		EntityID:   entityID,
		Payload:    payload,
		EmittedBy:  actor,
	})
}

// ScheduleEvent starts async delivery for a previously emitted id. Must be
// called only after the emitting transaction committed.
func (h *Handler) ScheduleEvent(id string) {
	if h.Outbox != nil && id != "" {
		h.Outbox.Schedule(id)
	}
}

// eventActor identifies the requesting user for events; system when anonymous.
func eventActor(r *http.Request) string {
	if c := auth.ClaimsFromContext(r.Context()); c != nil {
		return c.UserID.String()
	}
	return "system"
}

// ── Subscriber registration ──────────────────────────────────────────────

// RegisterSubscribers wires the module consumers onto the contract bus.
func (h *Handler) RegisterSubscribers(o *events.Outbox) {
	o.Subscribe(events.PaymentCaptured, h.onPaymentCaptured)
	o.Subscribe(events.ReservationCheckedIn, h.onGuestCheckedIn)
	o.Subscribe(events.ReservationCheckedOut, h.onStayCompleted)
	o.Subscribe(events.StockMovement, h.onStockMovement)
}

// ── payment.captured → GL posting (M-FIN) ───────────────────────────────

func (h *Handler) onPaymentCaptured(ctx context.Context, ev *events.Event) error {
	var p ledger.PaymentCapturedPayload
	if err := ev.UnmarshalPayload(&p); err != nil {
		return fmt.Errorf("payment.captured payload: %w", err)
	}
	if p.PropertyID == "" {
		p.PropertyID = ev.PropertyID
	}
	return ledger.OnPaymentCaptured(ctx, h.db, p)
}

// ── CRM loyalty (M-CRM) ──────────────────────────────────────────────────

type stayCompletedPayload struct {
	PropertyID    string `json:"propertyId"`
	GuestID       string `json:"guestId"`
	ReservationID string `json:"reservationId"`
	Nights        int    `json:"nights"`
	TotalKobo     int64  `json:"totalKobo"`
	FolioCode     string `json:"folioCode"`
}

type checkedInPayload struct {
	PropertyID    string `json:"propertyId"`
	GuestID       string `json:"guestId"`
	ReservationID string `json:"reservationId"`
}

// stayPoints accrues 1 point per ₦100 spent plus 25 bonus points per night
// (integer kobo math, round half up).
func stayPoints(totalKobo int64, nights int) int64 {
	if nights < 0 {
		nights = 0
	}
	return (totalKobo+5000)/10000 + int64(nights)*25
}

// loyaltyTierFor maps lifetime value (kobo) to a tier name — ₦200k / ₦500k /
// ₦1m thresholds.
func loyaltyTierFor(lifetimeValueKobo int64) string {
	switch {
	case lifetimeValueKobo >= 100_000_000:
		return "platinum"
	case lifetimeValueKobo >= 50_000_000:
		return "gold"
	case lifetimeValueKobo >= 20_000_000:
		return "silver"
	default:
		return "bronze"
	}
}

// loadOrCreateLoyaltyMember returns the member for (property, guest),
// enrolling them as bronze with zero balance when they do not exist yet.
func (h *Handler) loadOrCreateLoyaltyMember(ctx context.Context, propertyID, guestID string) (memberID string, lifetimeValueKobo int64, err error) {
	rows, err := h.db.Query(ctx, `
		SELECT id, COALESCE(lifetime_value_kobo, 0) FROM loyalty_members
		WHERE property_id = $1 AND guest_id = $2
	`, propertyID, guestID)
	if err != nil {
		return "", 0, fmt.Errorf("loyalty member lookup: %w", err)
	}
	if rows.Next() {
		if err := rows.Scan(&memberID, &lifetimeValueKobo); err != nil {
			rows.Close()
			return "", 0, err
		}
		rows.Close()
		return memberID, lifetimeValueKobo, nil
	}
	rows.Close()

	// Default tier: bronze when a matching tier row exists for the property.
	var tierID interface{}
	var bronzeID string
	err = h.db.QueryRow(ctx, `
		SELECT id FROM loyalty_tiers WHERE property_id = $1 AND LOWER(name) = 'bronze' LIMIT 1
	`, propertyID).Scan(&bronzeID)
	if err == nil {
		tierID = bronzeID
	}

	memberID = uuid.New().String()
	_, err = h.db.Exec(ctx, `
		INSERT INTO loyalty_members (id, property_id, guest_id, member_number,
			tier_id, total_points, lifetime_points, lifetime_value_kobo)
		VALUES ($1, $2, $3, $4, $5, 0, 0, 0)
	`, memberID, propertyID, guestID, "LM-"+memberID[:8], tierID)
	if err != nil {
		return "", 0, fmt.Errorf("loyalty member create: %w", err)
	}
	return memberID, 0, nil
}

// onStayCompleted: reservation.checked_out → accrue points, LTV and tier.
// Idempotent per reservation (one 'stay' movement), mirroring
// src/convex/crm.ts onStayCompleted.
func (h *Handler) onStayCompleted(ctx context.Context, ev *events.Event) error {
	var p stayCompletedPayload
	if err := ev.UnmarshalPayload(&p); err != nil {
		return fmt.Errorf("reservation.checked_out payload: %w", err)
	}
	if p.PropertyID == "" || p.GuestID == "" || p.ReservationID == "" {
		return nil
	}

	memberID, ltvKobo, err := h.loadOrCreateLoyaltyMember(ctx, p.PropertyID, p.GuestID)
	if err != nil {
		return err
	}

	var prior int
	if err := h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM loyalty_transactions
		WHERE member_id = $1 AND type = 'stay' AND reference_id = $2
	`, memberID, p.ReservationID).Scan(&prior); err != nil {
		return fmt.Errorf("loyalty idempotency check: %w", err)
	}
	if prior > 0 {
		return nil
	}

	points := stayPoints(p.TotalKobo, p.Nights)
	ltvKobo += p.TotalKobo
	tierName := loyaltyTierFor(ltvKobo)

	// Tier resolves by name against the property's tier table; COALESCE keeps
	// the current tier when no matching row is configured. Placeholders
	// ascend in textual order (SQLite binds positionally after $N→?).
	if _, err := h.db.Exec(ctx, `
		UPDATE loyalty_members
		SET total_points = total_points + $1,
		    lifetime_points = lifetime_points + $2,
		    lifetime_value_kobo = COALESCE(lifetime_value_kobo, 0) + $3,
		    last_activity = CURRENT_TIMESTAMP,
		    tier_id = COALESCE((
			    SELECT id FROM loyalty_tiers
			    WHERE property_id = $4 AND LOWER(name) = LOWER($5) LIMIT 1
		    ), tier_id)
		WHERE id = $6
	`, points, points, p.TotalKobo, p.PropertyID, tierName, memberID); err != nil {
		return fmt.Errorf("loyalty accrual: %w", err)
	}

	var balanceAfter int64
	if err := h.db.QueryRow(ctx, `SELECT total_points FROM loyalty_members WHERE id = $1`, memberID).
		Scan(&balanceAfter); err != nil {
		return fmt.Errorf("loyalty balance read: %w", err)
	}

	if _, err := h.db.Exec(ctx, `
		INSERT INTO loyalty_transactions (id, member_id, type, points, description, reference_id)
		VALUES ($1, $2, 'stay', $3, $4, $5)
	`, uuid.New().String(), memberID, points,
		fmt.Sprintf("Stay accrual — balance %d", balanceAfter), p.ReservationID); err != nil {
		return fmt.Errorf("loyalty movement: %w", err)
	}
	return nil
}

// onGuestCheckedIn: reservation.checked_in → one-time welcome bonus for a
// guest with no loyalty history anywhere.
func (h *Handler) onGuestCheckedIn(ctx context.Context, ev *events.Event) error {
	var p checkedInPayload
	if err := ev.UnmarshalPayload(&p); err != nil {
		return fmt.Errorf("reservation.checked_in payload: %w", err)
	}
	if p.PropertyID == "" || p.GuestID == "" || p.ReservationID == "" {
		return nil
	}

	// Guard BEFORE enrolling: any prior movement (any property) means the
	// guest already has history — no welcome bonus, no member created here.
	var prior int
	if err := h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM loyalty_transactions t
		JOIN loyalty_members m ON m.id = t.member_id
		WHERE m.guest_id = $1
	`, p.GuestID).Scan(&prior); err != nil {
		return fmt.Errorf("welcome idempotency check: %w", err)
	}
	if prior > 0 {
		return nil
	}

	memberID, _, err := h.loadOrCreateLoyaltyMember(ctx, p.PropertyID, p.GuestID)
	if err != nil {
		return err
	}

	if _, err := h.db.Exec(ctx, `
		UPDATE loyalty_members
		SET total_points = total_points + 100,
		    lifetime_points = lifetime_points + 100,
		    last_activity = CURRENT_TIMESTAMP
		WHERE id = $1
	`, memberID); err != nil {
		return fmt.Errorf("welcome accrual: %w", err)
	}

	var balanceAfter int64
	if err := h.db.QueryRow(ctx, `SELECT total_points FROM loyalty_members WHERE id = $1`, memberID).
		Scan(&balanceAfter); err != nil {
		return fmt.Errorf("loyalty balance read: %w", err)
	}

	if _, err := h.db.Exec(ctx, `
		INSERT INTO loyalty_transactions (id, member_id, type, points, description, reference_id)
		VALUES ($1, $2, 'welcome', 100, $3, $4)
	`, uuid.New().String(), memberID,
		fmt.Sprintf("Welcome bonus — balance %d", balanceAfter), p.ReservationID); err != nil {
		return fmt.Errorf("welcome movement: %w", err)
	}
	return nil
}

// ── stock.movement → replenishment (M-INV → M-PRC) ───────────────────────

type stockMovementPayload struct {
	PropertyID   string  `json:"propertyId"`
	ItemID       string  `json:"itemId"`
	SKU          string  `json:"sku"`
	Type         string  `json:"type"`
	Delta        float64 `json:"delta"`
	QtyOnHand    float64 `json:"qtyOnHand"`
	ReorderLevel float64 `json:"reorderLevel"`
}

// onStockMovement reorders when stock drops to the reorder level: it drafts
// a replenishment PO (2× reorder level less on hand) unless one is already
// open, and always raises stock.low for purchasing to action.
func (h *Handler) onStockMovement(ctx context.Context, ev *events.Event) error {
	var p stockMovementPayload
	if err := ev.UnmarshalPayload(&p); err != nil {
		return fmt.Errorf("stock.movement payload: %w", err)
	}
	if p.PropertyID == "" || p.ItemID == "" {
		return nil
	}
	if p.QtyOnHand > p.ReorderLevel {
		return nil // healthy
	}

	var name, supplierID string
	var costPrice, reorderLevel float64
	rows, err := h.db.Query(ctx, `
		SELECT name, COALESCE(supplier_id, ''), COALESCE(cost_price, 0), COALESCE(reorder_level, 0)
		FROM inventory_products
		WHERE id = $1 AND property_id = $2 AND deleted_at IS NULL
	`, p.ItemID, p.PropertyID)
	if err != nil {
		return fmt.Errorf("product lookup: %w", err)
	}
	if !rows.Next() {
		rows.Close()
		return nil
	}
	if err := rows.Scan(&name, &supplierID, &costPrice, &reorderLevel); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if reorderLevel == 0 {
		reorderLevel = p.ReorderLevel
	}

	// Already on an open PO? → do not duplicate (fail-safe: any status that
	// is not received/cancelled counts as open).
	var open int
	if err := h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM purchase_orders po
		JOIN purchase_order_lines l ON l.po_id = po.id
		WHERE po.property_id = $1 AND l.product_id = $2
		  AND po.status NOT IN ('received', 'cancelled', 'closed', 'completed')
	`, p.PropertyID, p.ItemID).Scan(&open); err != nil {
		return fmt.Errorf("open PO check: %w", err)
	}
	if open > 0 {
		return nil
	}

	// Draft a replenishment quantity: 2× reorder level less what's on hand.
	targetQty := math.Max(1, reorderLevel*2-p.QtyOnHand)

	stockLowPayload := map[string]interface{}{
		"itemId":        p.ItemID,
		"sku":           p.SKU,
		"qtyOnHand":     p.QtyOnHand,
		"reorderLevel":  reorderLevel,
		"suggestedQty":  targetQty,
		"autoPoCreated": supplierID != "",
	}
	stockLow, err := h.emitDomainEvent(ctx, h.db, p.PropertyID, "system:events",
		events.StockLow, "inventory", p.SKU, stockLowPayload)
	if err != nil {
		return err
	}
	h.ScheduleEvent(stockLow)

	if supplierID == "" {
		return nil // no supplier on file — the alert is already raised
	}

	poNumber, err := h.nextPONumber(ctx, p.PropertyID)
	if err != nil {
		return err
	}
	poID := uuid.New().String()
	amount := math.Round(targetQty*costPrice*100) / 100
	if _, err := h.db.Exec(ctx, `
		INSERT INTO purchase_orders (id, property_id, po_number, supplier_id,
			order_date, total, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, 'draft', 'auto-reorder')
	`, poID, p.PropertyID, poNumber, supplierID,
		time.Now().Format("2006-01-02"), amount); err != nil {
		return fmt.Errorf("draft PO: %w", err)
	}
	if _, err := h.db.Exec(ctx, `
		INSERT INTO purchase_order_lines (id, po_id, product_id, quantity, unit_price, amount)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New().String(), poID, p.ItemID, targetQty, costPrice, amount); err != nil {
		return fmt.Errorf("draft PO line: %w", err)
	}
	return nil
}

// nextPONumber returns the next sequential PO number for the property
// (PO-00001 style), skipping taken values.
func (h *Handler) nextPONumber(ctx context.Context, propertyID string) (string, error) {
	var count int
	if err := h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM purchase_orders WHERE property_id = $1
	`, propertyID).Scan(&count); err != nil {
		return "", fmt.Errorf("PO sequence: %w", err)
	}
	for i := 0; i < 500; i++ {
		candidate := fmt.Sprintf("PO-%05d", count+1+i)
		var taken int
		if err := h.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM purchase_orders
			WHERE property_id = $1 AND po_number = $2
		`, propertyID, candidate).Scan(&taken); err != nil {
			return "", fmt.Errorf("PO sequence: %w", err)
		}
		if taken == 0 {
			return candidate, nil
		}
	}
	return fmt.Sprintf("PO-%d", time.Now().UnixMilli()), nil
}

// ── Observability routes (sec.admin inbox) ───────────────────────────────

// ListDomainEvents handles GET /domain-events and GET /domain-events?type=.
func (h *Handler) ListDomainEvents(w http.ResponseWriter, r *http.Request) {
	if h.Outbox == nil {
		jsonSuccess(w, []events.Event{})
		return
	}
	pid := getPropertyID2(r)
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	var (
		list []events.Event
		err  error
	)
	if t := r.URL.Query().Get("type"); t != "" {
		list, err = h.Outbox.ByType(r.Context(), pid, t, limit)
	} else {
		list, err = h.Outbox.Recent(r.Context(), pid, limit)
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonSuccess(w, list)
}

// ListFailedDomainEvents handles GET /domain-events/failed.
func (h *Handler) ListFailedDomainEvents(w http.ResponseWriter, r *http.Request) {
	if h.Outbox == nil {
		jsonSuccess(w, []events.Event{})
		return
	}
	list, err := h.Outbox.Failed(r.Context(), getPropertyID2(r))
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonSuccess(w, list)
}

// RetryDomainEvent handles POST /domain-events/{id}/retry — manual
// re-delivery of a failed event, scoped to the caller's property.
func (h *Handler) RetryDomainEvent(w http.ResponseWriter, r *http.Request) {
	if h.Outbox == nil {
		jsonError(w, "contract bus not available", http.StatusNotFound)
		return
	}
	id := chi.URLParam(r, "id")
	ev, err := h.Outbox.Get(r.Context(), id)
	if err != nil || ev == nil {
		jsonError(w, "Event not found", http.StatusNotFound)
		return
	}
	if ev.PropertyID != getPropertyID2(r) {
		jsonError(w, "Event belongs to another property", http.StatusForbidden)
		return
	}
	if err := h.Outbox.Retry(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}
	auditLog(h, r, "settings", "event_retry", id, "manual re-delivery of failed domain event")
	jsonSuccess(w, map[string]interface{}{"ok": true, "id": id})
}
