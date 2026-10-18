package events

// ADR-0006 cross-module contract bus — on-premise outbox implementation.
//
// Producers Emit() the event row inside their own transaction (pass the tx as
// the executor) and Schedule() delivery after commit, so modules interconnect
// without importing each other's mutations: a check-out fans out to CRM
// loyalty, a stock movement fans out to procurement, and neither producer
// knows its consumers. Delivery is at-least-once — every subscriber must be
// idempotent. Failed events stay parked for manual Retry.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"infineon/internal/database"
)

// Canonical event type names — the published contract vocabulary.
const (
	ReservationCreated    = "reservation.created"
	ReservationCheckedIn  = "reservation.checked_in"
	ReservationCheckedOut = "reservation.checked_out"
	ReservationCancelled  = "reservation.cancelled"
	ReservationNoShow     = "reservation.no_show"
	RoomStatusChanged     = "room.status_changed"
	FolioOpened           = "folio.opened"
	FolioChargePosted     = "folio.charge_posted"
	FolioSettled          = "folio.settled"
	FolioClosed           = "folio.closed"
	PaymentCaptured       = "payment.captured"
	JournalPosted         = "journal.posted"
	OrderClosed           = "order.closed"
	StockMovement         = "stock.movement"
	StockLow              = "stock.low"
	PORceived             = "po.received"
	PayrollApproved       = "payroll.approved"
	NightAuditClosed      = "nightaudit.closed"
)

// Status values for a domain event row.
const (
	StatusPending   = "pending"
	StatusDelivered = "delivered"
	StatusFailed    = "failed"
)

// EventExecutor is satisfied by both database.Pool and database.Tx, letting
// producers enlist the outbox insert in the same transaction as their state
// change (the transactional-outbox pattern).
type EventExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (database.CommandTag, error)
}

// EventInput is what producers publish.
type EventInput struct {
	PropertyID string
	Type       string
	Source     string
	EntityID   string
	Payload    interface{} // marshalled to JSON; camelCase keys per the contract
	EmittedBy  string
}

// Event is the stored outbox row handed to subscribers.
type Event struct {
	ID           string          `json:"id"`
	PropertyID   string          `json:"propertyId"`
	Type         string          `json:"type"`
	Source       string          `json:"source"`
	EntityID     string          `json:"entityId"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	Attempts     int             `json:"attempts"`
	Error        string          `json:"error"`
	EmittedBy    string          `json:"emittedBy"`
	At           int64           `json:"at"` // unix millis
	DispatchedAt int64           `json:"dispatchedAt"`
}

// Subscriber handles one event. Returning an error parks the event as failed
// for manual retry — handlers must be idempotent because delivery is
// at-least-once.
type Subscriber func(ctx context.Context, ev *Event) error

// UnmarshalPayload decodes the camelCase contract payload into dest.
func (e *Event) UnmarshalPayload(dest interface{}) error {
	return json.Unmarshal(e.Payload, dest)
}

// Outbox is the persisted contract bus.
type Outbox struct {
	db   database.Pool
	mu   sync.RWMutex
	subs map[string][]Subscriber
}

// NewOutbox creates the bus backed by the given pool.
func NewOutbox(db database.Pool) *Outbox {
	return &Outbox{db: db, subs: make(map[string][]Subscriber)}
}

// Subscribe registers a handler for an event type.
func (o *Outbox) Subscribe(eventType string, fn Subscriber) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.subs[eventType] = append(o.subs[eventType], fn)
}

// Emit records an event. Pass the caller's transaction so the outbox row
// commits atomically with the state change that caused it.
func (o *Outbox) Emit(ctx context.Context, ex EventExecutor, in EventInput) (string, error) {
	if in.PropertyID == "" {
		return "", fmt.Errorf("events: property_id is required")
	}
	if in.Type == "" {
		return "", fmt.Errorf("events: type is required")
	}
	payload := in.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("events: marshal payload: %w", err)
	}
	id := uuid.New().String()
	_, err = ex.Exec(ctx, `
		INSERT INTO domain_events (id, property_id, type, source, entity_id, payload, status, attempts, error, emitted_by, at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', 0, '', $7, $8)
	`, id, in.PropertyID, in.Type, in.Source, in.EntityID, string(raw), in.EmittedBy, time.Now().UnixMilli())
	if err != nil {
		return "", fmt.Errorf("events: emit %s: %w", in.Type, err)
	}
	return id, nil
}

// Schedule dispatches a previously emitted event asynchronously. Safe to call
// with an id whose row rolled back — dispatch simply finds nothing.
func (o *Outbox) Schedule(id string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[events] dispatch panic for %s: %v", id, r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := o.Dispatch(ctx, id); err != nil {
			log.Printf("[events] dispatch %s: %v", id, err)
		}
	}()
}

// Dispatch delivers one event to its subscribers in registration order.
// On subscriber error the event is parked as failed and later subscribers are
// skipped, mirroring the ADR contract (manual retry re-runs the chain).
func (o *Outbox) Dispatch(ctx context.Context, id string) error {
	ev, err := o.load(ctx, id)
	if err != nil {
		return err
	}
	if ev == nil || ev.Status == StatusDelivered {
		return nil
	}

	o.mu.RLock()
	handlers := append([]Subscriber(nil), o.subs[ev.Type]...)
	o.mu.RUnlock()

	for _, fn := range handlers {
		if err := fn(ctx, ev); err != nil {
			o.mark(ctx, id, StatusFailed, err)
			return fmt.Errorf("subscriber for %s failed: %w", ev.Type, err)
		}
	}
	o.mark(ctx, id, StatusDelivered, nil)
	return nil
}

// Get loads one event by id (property-scope checks are the caller's job).
func (o *Outbox) Get(ctx context.Context, id string) (*Event, error) {
	return o.load(ctx, id)
}

// Retry resets a failed event to pending and re-dispatches it (sec.admin
// manual re-delivery).
func (o *Outbox) Retry(ctx context.Context, id string) error {
	res, err := o.db.Exec(ctx, `
		UPDATE domain_events SET status = 'pending', error = '' WHERE id = $1 AND status <> 'delivered'
	`, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("event not found or already delivered")
	}
	return o.Dispatch(ctx, id)
}

// RequeuePending re-dispatches events left pending by an interrupted process
// (crash between commit and delivery). Call once at boot.
func (o *Outbox) RequeuePending(ctx context.Context) int {
	rows, err := o.db.Query(ctx, `SELECT id FROM domain_events WHERE status = 'pending' ORDER BY at LIMIT 500`)
	if err != nil {
		log.Printf("[events] requeue scan: %v", err)
		return 0
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		o.Schedule(id)
	}
	return len(ids)
}

// ── Observability reads ──────────────────────────────────────────────────

// Recent returns the newest events for a property.
func (o *Outbox) Recent(ctx context.Context, propertyID string, limit int) ([]Event, error) {
	return o.queryEvents(ctx, `
		SELECT id, property_id, type, source, entity_id, payload, status, attempts,
		       COALESCE(error, ''), COALESCE(emitted_by, ''), at, COALESCE(dispatched_at, 0)
		FROM domain_events WHERE property_id = $1 ORDER BY at DESC LIMIT $2
	`, propertyID, clampLimit(limit))
}

// ByType returns the newest events of one type for a property.
func (o *Outbox) ByType(ctx context.Context, propertyID, eventType string, limit int) ([]Event, error) {
	return o.queryEvents(ctx, `
		SELECT id, property_id, type, source, entity_id, payload, status, attempts,
		       COALESCE(error, ''), COALESCE(emitted_by, ''), at, COALESCE(dispatched_at, 0)
		FROM domain_events WHERE property_id = $1 AND type = $2 ORDER BY at DESC LIMIT $3
	`, propertyID, eventType, clampLimit(limit))
}

// Failed returns undelivered events for a property (admin inbox).
func (o *Outbox) Failed(ctx context.Context, propertyID string) ([]Event, error) {
	return o.queryEvents(ctx, `
		SELECT id, property_id, type, source, entity_id, payload, status, attempts,
		       COALESCE(error, ''), COALESCE(emitted_by, ''), at, COALESCE(dispatched_at, 0)
		FROM domain_events WHERE property_id = $1 AND status = 'failed' ORDER BY at DESC LIMIT 100
	`, propertyID)
}

// ── internals ────────────────────────────────────────────────────────────

func (o *Outbox) load(ctx context.Context, id string) (*Event, error) {
	evs, err := o.queryEvents(ctx, `
		SELECT id, property_id, type, source, entity_id, payload, status, attempts,
		       COALESCE(error, ''), COALESCE(emitted_by, ''), at, COALESCE(dispatched_at, 0)
		FROM domain_events WHERE id = $1
	`, id)
	if err != nil || len(evs) == 0 {
		return nil, err
	}
	return &evs[0], nil
}

func (o *Outbox) mark(ctx context.Context, id, status string, markErr error) {
	msg := ""
	if markErr != nil {
		msg = markErr.Error()
	}
	// Placeholders must ascend in TEXTUAL order: SQLite's $N→? conversion
	// binds positionally, so the args array order is the SQL text order.
	_, err := o.db.Exec(ctx, `
		UPDATE domain_events
		SET status = $1, attempts = attempts + 1, error = $2, dispatched_at = $3
		WHERE id = $4
	`, status, msg, time.Now().UnixMilli(), id)
	if err != nil {
		log.Printf("[events] mark %s as %s: %v", id, status, err)
	}
}

func (o *Outbox) queryEvents(ctx context.Context, query string, args ...any) ([]Event, error) {
	rows, err := o.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var ev Event
		var payload string
		if err := rows.Scan(&ev.ID, &ev.PropertyID, &ev.Type, &ev.Source, &ev.EntityID,
			&payload, &ev.Status, &ev.Attempts, &ev.Error, &ev.EmittedBy,
			&ev.At, &ev.DispatchedAt); err != nil {
			return nil, err
		}
		ev.Payload = json.RawMessage(payload)
		out = append(out, ev)
	}
	return out, rows.Err()
}

func clampLimit(limit int) int {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return limit
}
