package payment

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// ═══════════════════════════════════════════════════════════════
// Payment Manager
//
// Orchestrates payment operations across providers:
// - Routes to active provider (Paystack/Stripe/Mock)
// - Enforces idempotency
// - Persists transactions to DB
// - Processes webhooks
// - Handles refunds
// ═══════════════════════════════════════════════════════════════

type Manager struct {
	db          *sql.DB
	providers   map[string]Provider
	active      string
	idempotency *IdempotencyStore
	// onCaptured fires exactly once per reference when a payment transitions
	// to success (verify or webhook) — the ADR-0006 payment.captured origin
	// for the GL, wired by the server.
	onCaptured func(context.Context, CaptureInfo)
}

// CaptureInfo describes a settled payment handed to OnCaptured subscribers.
type CaptureInfo struct {
	ID            string
	PropertyID    string
	ReservationID string
	Reference     string
	Provider      string
	PaymentMethod string
	Currency      string
	AmountKobo    int64
	Channel       string
	PaidAt        time.Time
}

// OnCaptured registers the capture callback (set once at startup).
func (m *Manager) OnCaptured(fn func(context.Context, CaptureInfo)) {
	m.onCaptured = fn
}

// NewManager creates a new payment manager with configured providers.
func NewManager(db *sql.DB, config Config) *Manager {
	m := &Manager{
		db:          db,
		providers:   make(map[string]Provider),
		active:      config.ActiveProvider,
		idempotency: NewIdempotencyStore(),
	}

	// Register providers
	if config.Paystack.SecretKey != "" {
		m.providers["paystack"] = NewPaystackProvider(config.Paystack)
		log.Println("[payment] Paystack provider registered")
	}

	if config.Stripe.SecretKey != "" {
		m.providers["stripe"] = NewStripeProvider(config.Stripe)
		log.Println("[payment] Stripe provider registered")
	}

	// Always register mock provider for demo mode
	m.providers["mock"] = NewMockProvider()

	if m.active == "" {
		m.active = "mock"
	}

	// Ensure tables exist
	m.createTables()

	log.Printf("[payment] Manager initialized with active provider: %s, idempotency: enabled", m.active)
	return m
}

// ── Initialize Payment ──

func (m *Manager) Initialize(ctx context.Context, req InitializeRequest, idempotencyKey string) (*InitializeResponse, error) {
	// Check idempotency
	if idempotencyKey != "" {
		if entry := m.idempotency.Get(idempotencyKey); entry != nil {
			if entry.Status == StatusCompleted {
				log.Printf("[payment] Idempotency hit: key=%s, returning cached response", idempotencyKey)
				return entry.Body.(*InitializeResponse), nil
			}
			return nil, fmt.Errorf("payment already in progress for key: %s", idempotencyKey)
		}

		if !m.idempotency.SetInFlight(idempotencyKey) {
			return nil, fmt.Errorf("payment in progress for key: %s", idempotencyKey)
		}
	}

	provider := m.getProvider()

	resp, err := provider.Initialize(ctx, req)
	if err != nil {
		if idempotencyKey != "" {
			m.idempotency.Remove(idempotencyKey)
		}
		return nil, err
	}

	// Persist transaction
	m.saveTransaction(ctx, &Transaction{
		Reference:      req.Reference,
		Provider:       provider.Name(),
		PropertyID:     req.PropertyID,
		ReservationID:  req.ReservationID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         "pending",
		Email:          req.Email,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	// Store idempotency response
	if idempotencyKey != "" {
		m.idempotency.SetCompleted(idempotencyKey, 200, resp)
	}

	return resp, nil
}

// ── Verify Payment ──

func (m *Manager) Verify(ctx context.Context, reference string) (*VerificationResult, error) {
	provider := m.getProvider()

	result, err := provider.Verify(ctx, reference)
	if err != nil {
		return nil, err
	}

	// Update transaction status
	m.updateTransactionStatus(ctx, reference, result.Status, result.GatewayRef, result.PaidAt)

	return result, nil
}

// ── Process Refund ──

func (m *Manager) Refund(ctx context.Context, req RefundRequest) (*RefundResult, error) {
	provider := m.getProvider()

	result, err := provider.Refund(ctx, req)
	if err != nil {
		return nil, err
	}

	// Record refund in DB
	m.recordRefund(ctx, req.Reference, result.RefundRef, req.Amount, req.Reason)

	return result, nil
}

// ── Process Webhook ──

func (m *Manager) ProcessWebhook(ctx context.Context, providerName string, payload []byte, signature string) error {
	provider, ok := m.providers[providerName]
	if !ok {
		// Fall back to mock for testing/simulation
		provider, ok = m.providers["mock"]
		if !ok {
			return fmt.Errorf("unknown provider: %s", providerName)
		}
	}

	// Verify signature
	if !provider.VerifyWebhookSignature(payload, signature) {
		log.Printf("[payment] Webhook signature verification FAILED for provider=%s", providerName)
		return fmt.Errorf("webhook signature verification failed")
	}

	log.Printf("[payment] Webhook signature verified for provider=%s", providerName)

	// Parse event
	var event *WebhookEvent
	var err error

	switch providerName {
	case "paystack":
		event, err = ParsePaystackWebhook(payload)
	case "stripe":
		event, err = ParseStripeWebhook(payload)
	default:
		return fmt.Errorf("webhook parsing not supported for provider: %s", providerName)
	}

	if err != nil {
		return fmt.Errorf("parse webhook: %w", err)
	}

	// Process based on event type
	switch {
	case IsSuccessEvent(event.EventType):
		m.updateTransactionStatus(ctx, event.Reference, "success", "", time.Now())
		log.Printf("[payment] Webhook: payment success ref=%s", event.Reference)

	case IsFailureEvent(event.EventType):
		m.updateTransactionStatus(ctx, event.Reference, "failed", "", time.Time{})
		log.Printf("[payment] Webhook: payment failed ref=%s", event.Reference)

	case IsRefundEvent(event.EventType):
		log.Printf("[payment] Webhook: refund event ref=%s type=%s", event.Reference, event.EventType)
	}

	// Log webhook event
	m.logWebhookEvent(ctx, event, providerName)

	return nil
}

// ── Get Transaction History ──

func (m *Manager) GetTransactions(ctx context.Context, propertyID string, limit int) ([]Transaction, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := m.db.QueryContext(ctx, `
		SELECT id, property_id, reservation_id, reference, provider, amount, currency,
		       status, payment_method, channel, email, gateway_ref, description,
		       failure_reason, refunded_amount, created_at, updated_at
		FROM payment_transactions
		WHERE property_id = ? OR property_id = ''
		ORDER BY created_at DESC
		LIMIT ?
	`, propertyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var paidAt sql.NullTime
		err := rows.Scan(
			&t.ID, &t.PropertyID, &t.ReservationID, &t.Reference, &t.Provider,
			&t.Amount, &t.Currency, &t.Status, &t.PaymentMethod, &t.Channel,
			&t.Email, &t.GatewayRef, &t.Description, &t.FailureReason,
			&t.RefundedAmount, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			continue
		}
		if paidAt.Valid {
			t.PaidAt = paidAt.Time
		}
		transactions = append(transactions, t)
	}

	return transactions, nil
}

// ── Get Payment Summary ──

func (m *Manager) GetSummary(ctx context.Context, propertyID string) (map[string]interface{}, error) {
	summary := map[string]interface{}{}

	// Total transactions and amount
	var totalTxns int
	var totalAmount, totalRefunded float64
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0), COALESCE(SUM(refunded_amount), 0)
		FROM payment_transactions WHERE (property_id = ? OR property_id = '')
	`, propertyID).Scan(&totalTxns, &totalAmount, &totalRefunded)

	// By status
	rows, _ := m.db.QueryContext(ctx, `
		SELECT status, COUNT(*), COALESCE(SUM(amount), 0)
		FROM payment_transactions WHERE (property_id = ? OR property_id = '')
		GROUP BY status
	`, propertyID)
	defer rows.Close()

	statusBreakdown := map[string]map[string]interface{}{}
	for rows.Next() {
		var status string
		var count int
		var amount float64
		rows.Scan(&status, &count, &amount)
		statusBreakdown[status] = map[string]interface{}{
			"count":  count,
			"amount": amount,
		}
	}

	// By provider
	pRows, _ := m.db.QueryContext(ctx, `
		SELECT provider, COUNT(*), COALESCE(SUM(amount), 0)
		FROM payment_transactions WHERE (property_id = ? OR property_id = '')
		GROUP BY provider
	`, propertyID)
	defer pRows.Close()

	providerBreakdown := map[string]map[string]interface{}{}
	for pRows.Next() {
		var provider string
		var count int
		var amount float64
		pRows.Scan(&provider, &count, &amount)
		providerBreakdown[provider] = map[string]interface{}{
			"count":  count,
			"amount": amount,
		}
	}

	summary["total_transactions"] = totalTxns
	summary["total_amount"] = totalAmount
	summary["total_refunded"] = totalRefunded
	summary["net_amount"] = totalAmount - totalRefunded
	summary["by_status"] = statusBreakdown
	summary["by_provider"] = providerBreakdown
	summary["idempotency_stats"] = m.idempotency.Stats()

	return summary, nil
}

// ── Accessors ──

// DB returns the underlying database connection.
func (m *Manager) DB() *sql.DB { return m.db }

// ActiveProvider returns the name of the active payment provider.
func (m *Manager) ActiveProvider() string { return m.active }

// ── Internal Helpers ──

func (m *Manager) getProvider() Provider {
	p, ok := m.providers[m.active]
	if !ok {
		return m.providers["mock"]
	}
	return p
}

func (m *Manager) saveTransaction(ctx context.Context, t *Transaction) {
	if t.ID == "" {
		t.ID = fmt.Sprintf("txn-%d", time.Now().UnixNano())
	}
	if t.PropertyID == "" {
		t.PropertyID = "00000000-0000-0000-0000-000000000001"
	}

	_, err := m.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO payment_transactions
		(id, property_id, reservation_id, reference, provider, amount, currency,
		 status, payment_method, channel, email, gateway_ref, idempotency_key,
		 description, failure_reason, refunded_amount, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.PropertyID, t.ReservationID, t.Reference, t.Provider,
		t.Amount, t.Currency, t.Status, t.PaymentMethod, t.Channel,
		t.Email, t.GatewayRef, t.IdempotencyKey, t.Description,
		t.FailureReason, t.RefundedAmount, t.CreatedAt, t.UpdatedAt)

	if err != nil {
		log.Printf("[payment] Failed to save transaction: %v", err)
	}
}

func (m *Manager) updateTransactionStatus(ctx context.Context, reference, status, gatewayRef string, paidAt time.Time) {
	// Idempotent transition: a webhook replayed after a successful verify (or
	// vice versa) must not re-fire capture events.
	res, err := m.db.ExecContext(ctx, `
		UPDATE payment_transactions SET status = ?, gateway_ref = ?, updated_at = ?
		WHERE reference = ? AND status <> ?
	`, status, gatewayRef, time.Now(), reference, status)
	if err != nil {
		log.Printf("[payment] Failed to update transaction status: %v", err)
		return
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return
	}
	if status == "success" {
		m.fireCaptured(ctx, reference, paidAt)
	}
}

// fireCaptured loads the settled transaction and notifies the capture
// callback exactly once per status transition.
func (m *Manager) fireCaptured(ctx context.Context, reference string, paidAt time.Time) {
	var (
		id, propertyID, reservationID, provider, method, currency, channel string
		amount                                                             float64
	)
	err := m.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(property_id, ''), COALESCE(reservation_id, ''), provider,
		       COALESCE(payment_method, ''), COALESCE(currency, 'NGN'), COALESCE(channel, ''), amount
		FROM payment_transactions WHERE reference = ?
	`, reference).Scan(&id, &propertyID, &reservationID, &provider, &method, &currency, &channel, &amount)
	if err != nil {
		log.Printf("[payment] Captured but unable to load transaction %s: %v", reference, err)
		return
	}
	if method == "" {
		method = channel
	}
	if paidAt.IsZero() {
		paidAt = time.Now()
	}
	info := CaptureInfo{
		ID:            id,
		PropertyID:    propertyID,
		ReservationID: reservationID,
		Reference:     reference,
		Provider:      provider,
		PaymentMethod: method,
		Currency:      currency,
		AmountKobo:    int64(amount*100 + 0.5),
		Channel:       channel,
		PaidAt:        paidAt,
	}
	if m.onCaptured != nil {
		m.onCaptured(ctx, info)
	}
}

func (m *Manager) recordRefund(ctx context.Context, originalRef, refundRef string, amount float64, reason string) {
	// Update original transaction's refunded amount
	_, err := m.db.ExecContext(ctx, `
		UPDATE payment_transactions
		SET refunded_amount = refunded_amount + ?, updated_at = ?
		WHERE reference = ?
	`, amount, time.Now(), originalRef)
	if err != nil {
		log.Printf("[payment] Failed to record refund: %v", err)
	}

	// Create refund record
	refundID := fmt.Sprintf("ref-%d", time.Now().UnixNano())
	m.saveTransaction(ctx, &Transaction{
		ID:          refundID,
		Reference:   refundRef,
		Provider:    m.active,
		Amount:      -amount,
		Currency:    "NGN",
		Status:      "success",
		Description: reason,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
}

func (m *Manager) logWebhookEvent(ctx context.Context, event *WebhookEvent, providerName string) {
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO webhook_events (id, provider, event_type, reference, raw_payload, processed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, event.ID, providerName, event.EventType, event.Reference,
		string(event.RawPayload), time.Now())
	if err != nil {
		log.Printf("[payment] Failed to log webhook event: %v", err)
	}
}

// ── Database Schema ──

func (m *Manager) createTables() {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS payment_transactions (
			id TEXT PRIMARY KEY,
			property_id TEXT DEFAULT '',
			reservation_id TEXT DEFAULT '',
			reference TEXT UNIQUE NOT NULL,
			provider TEXT NOT NULL DEFAULT 'paystack',
			amount REAL NOT NULL DEFAULT 0,
			currency TEXT NOT NULL DEFAULT 'NGN',
			status TEXT NOT NULL DEFAULT 'pending',
			payment_method TEXT DEFAULT '',
			channel TEXT DEFAULT '',
			email TEXT DEFAULT '',
			gateway_ref TEXT DEFAULT '',
			idempotency_key TEXT DEFAULT '',
			description TEXT DEFAULT '',
			failure_reason TEXT DEFAULT '',
			refunded_amount REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_reference ON payment_transactions(reference)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_status ON payment_transactions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_property ON payment_transactions(property_id)`,

		`CREATE TABLE IF NOT EXISTS webhook_events (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			event_type TEXT NOT NULL,
			reference TEXT DEFAULT '',
			raw_payload TEXT DEFAULT '',
			processed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_provider ON webhook_events(provider)`,
	}

	for _, table := range tables {
		_, _ = m.db.Exec(table)
	}
}
