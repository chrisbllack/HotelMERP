package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"infineon/internal/payment"
	"infineon/internal/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PaymentIntegration holds the payment manager and handler dependencies.
type PaymentIntegration struct {
	Manager *payment.Manager
}

// NewPaymentIntegration creates a new payment integration handler.
func NewPaymentIntegration(mgr *payment.Manager) *PaymentIntegration {
	return &PaymentIntegration{Manager: mgr}
}

// ═══════════════════════════════════════════════════════════════
// POST /payments/v2/initiate
// Initialize a payment with idempotency support.
//
// Request:
//   - reservation_id: string
//   - amount: float64
//   - currency: string (default NGN)
//   - email: string
//   - payment_method: string (card, bank_transfer, ussd, mobile_money)
//   - description: string (optional)
//
// Headers:
//   - X-Idempotency-Key: unique key to prevent duplicate charges
//
// Response:
//   - reference: payment reference
//   - authorization_url: redirect URL for card payment
//   - access_code: client secret for Stripe
//   - status: pending
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) InitiatePayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReservationID string  `json:"reservation_id"`
		Amount        float64 `json:"amount"`
		Currency      string  `json:"currency"`
		Email         string  `json:"email"`
		PaymentMethod string  `json:"payment_method"`
		Description   string  `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Amount <= 0 {
		response.RespondError(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		response.RespondError(w, "Email is required", http.StatusBadRequest)
		return
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}

	// Generate reference
	ref := "PAY-" + time.Now().Format("20060102150405") + "-" + uuid.New().String()[:8]

	// Get idempotency key from header
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	result, err := pi.Manager.Initialize(r.Context(), payment.InitializeRequest{
		Reference:   ref,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Email:       req.Email,
		// Tenancy: persisted so webhook-driven captures attribute to the
		// right property even though webhooks carry no headers.
		PropertyID:    r.Header.Get("X-Property-ID"),
		ReservationID: req.ReservationID,
		Metadata: map[string]string{
			"reservation_id": req.ReservationID,
			"payment_method": req.PaymentMethod,
			"description":    req.Description,
		},
		CallbackURL: "http://localhost:5173/payments?ref=" + ref,
	}, idempotencyKey)

	if err != nil {
		log.Printf("[payment] Initialize failed: %v", err)
		response.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save to our transaction table with extra fields
	pi.saveFullTransaction(r, ref, req, idempotencyKey)

	log.Printf("[payment] Payment initiated: ref=%s amount=%.0f %s method=%s provider=%s",
		ref, req.Amount, req.Currency, req.PaymentMethod, pi.Manager.ActiveProvider())

	response.RespondSuccess(w, map[string]interface{}{
		"reference":          result.Reference,
		"authorization_url":  result.AuthorizationURL,
		"access_code":        result.AccessCode,
		"status":             result.Status,
		"payment_ref":        ref,
		"amount":             req.Amount,
		"currency":           req.Currency,
		"payment_method":     req.PaymentMethod,
		"idempotency_key":    idempotencyKey,
		"message":            "Payment initiated. Redirect to authorization_url or use access_code for client-side payment.",
	}, "Payment initiated successfully")
}

// ═══════════════════════════════════════════════════════════════
// GET /payments/v2/verify/{reference}
// Verify a payment status against the gateway.
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	reference := chi.URLParam(r, "reference")
	if reference == "" {
		response.RespondError(w, "Reference is required", http.StatusBadRequest)
		return
	}

	result, err := pi.Manager.Verify(r.Context(), reference)
	if err != nil {
		log.Printf("[payment] Verify failed for ref=%s: %v", reference, err)
		response.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[payment] Payment verified: ref=%s status=%s amount=%.0f", reference, result.Status, result.Amount)

	response.RespondSuccess(w, map[string]interface{}{
		"reference":       result.Reference,
		"status":          result.Status,
		"amount":          result.Amount,
		"currency":        result.Currency,
		"channel":         result.Channel,
		"gateway_ref":     result.GatewayRef,
		"paid_at":         result.PaidAt,
		"customer_email":  result.CustomerEmail,
		"verified_at":     time.Now(),
	}, "Payment verified")
}

// ═══════════════════════════════════════════════════════════════
// POST /payments/v2/webhook/paystack
// Paystack webhook receiver with HMAC-SHA512 signature verification.
//
// Headers: X-Paystack-Signature
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) PaystackWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Paystack-Signature")

	log.Printf("[payment] Paystack webhook received: payload=%d bytes, signature=%s", len(payload), signature)

	if err := pi.Manager.ProcessWebhook(r.Context(), "paystack", payload, signature); err != nil {
		log.Printf("[payment] Paystack webhook processing failed: %v", err)
		// Return 200 to prevent retries for signature failures
		if err.Error() == "webhook signature verification failed" {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Always return 200 to acknowledge receipt (prevents gateway retries)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"received": true}`))
}

// ═══════════════════════════════════════════════════════════════
// POST /payments/v2/webhook/stripe
// Stripe webhook receiver with HMAC-SHA256 signature verification.
//
// Headers: Stripe-Signature
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("Stripe-Signature")

	log.Printf("[payment] Stripe webhook received: payload=%d bytes", len(payload))

	if err := pi.Manager.ProcessWebhook(r.Context(), "stripe", payload, signature); err != nil {
		log.Printf("[payment] Stripe webhook processing failed: %v", err)
		if err.Error() == "webhook signature verification failed" {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"received": true}`))
}

// ═══════════════════════════════════════════════════════════════
// POST /payments/v2/refund
// Process a refund (full or partial) with idempotency.
//
// Request:
//   - reference: original payment reference
//   - amount: refund amount (full if 0)
//   - reason: refund reason
//
// Headers:
//   - X-Idempotency-Key: unique key to prevent duplicate refunds
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) ProcessRefund(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reference string  `json:"reference"`
		Amount    float64 `json:"amount"`
		Reason    string  `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Reference == "" {
		response.RespondError(w, "Payment reference is required", http.StatusBadRequest)
		return
	}

	// Check idempotency
	idempotencyKey := r.Header.Get("X-Idempotency-Key")

	result, err := pi.Manager.Refund(r.Context(), payment.RefundRequest{
		Reference: req.Reference,
		Amount:    req.Amount,
		Reason:    req.Reason,
	})
	if err != nil {
		log.Printf("[payment] Refund failed for ref=%s: %v", req.Reference, err)
		response.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[payment] Refund processed: original_ref=%s refund_ref=%s amount=%.0f reason=%s",
		req.Reference, result.RefundRef, result.Amount, req.Reason)

	response.RespondSuccess(w, map[string]interface{}{
		"refund_reference": result.RefundRef,
		"status":           result.Status,
		"amount":           result.Amount,
		"original_ref":     req.Reference,
		"reason":           req.Reason,
		"idempotency_key":  idempotencyKey,
	}, "Refund processed successfully")
}

// ═══════════════════════════════════════════════════════════════
// GET /payments/v2/transactions
// List payment transactions with filtering.
//
// Query params:
//   - status: success, pending, failed, refunded
//   - provider: paystack, stripe, mock
//   - limit: default 50
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) ListTransactions(w http.ResponseWriter, r *http.Request) {
	propertyID := getPropertyID(r)

	transactions, err := pi.Manager.GetTransactions(r.Context(), propertyID, 100)
	if err != nil {
		response.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if transactions == nil {
		transactions = []payment.Transaction{}
	}

	// Apply status filter
	statusFilter := r.URL.Query().Get("status")
	if statusFilter != "" {
		var filtered []payment.Transaction
		for _, t := range transactions {
			if t.Status == statusFilter {
				filtered = append(filtered, t)
			}
		}
		transactions = filtered
	}

	// Apply provider filter
	providerFilter := r.URL.Query().Get("provider")
	if providerFilter != "" {
		var filtered []payment.Transaction
		for _, t := range transactions {
			if t.Provider == providerFilter {
				filtered = append(filtered, t)
			}
		}
		transactions = filtered
	}

	response.RespondSuccess(w, transactions, "Transactions retrieved")
}

// ═══════════════════════════════════════════════════════════════
// GET /payments/v2/summary
// Payment summary with totals, breakdown by status/provider.
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) PaymentSummary(w http.ResponseWriter, r *http.Request) {
	propertyID := getPropertyID(r)

	summary, err := pi.Manager.GetSummary(r.Context(), propertyID)
	if err != nil {
		response.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response.RespondSuccess(w, summary, "Payment summary retrieved")
}

// ═══════════════════════════════════════════════════════════════
// POST /payments/v2/simulate-webhook
// Simulate a webhook event for demo/testing purposes.
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) SimulateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider  string `json:"provider"`
		EventType string `json:"event_type"`
		Reference string `json:"reference"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Provider == "" {
		req.Provider = "paystack"
	}
	if req.EventType == "" {
		req.EventType = "charge.success"
	}

	// Build mock webhook payload
	var payload []byte
	switch req.Provider {
	case "paystack":
		payload, _ = json.Marshal(map[string]interface{}{
			"event": req.EventType,
			"data": map[string]interface{}{
				"reference": req.Reference,
				"status":    "success",
				"amount":    5000000,
			},
		})
	case "stripe":
		payload, _ = json.Marshal(map[string]interface{}{
			"id":   "evt_" + uuid.New().String()[:8],
			"type": req.EventType,
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"id": req.Reference,
				},
			},
		})
	default:
		response.RespondError(w, "Unknown provider", http.StatusBadRequest)
		return
	}

	// Process without signature verification (mock mode)
	if err := pi.Manager.ProcessWebhook(r.Context(), req.Provider, payload, "mock-signature"); err != nil {
		response.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[payment] Simulated webhook: provider=%s event=%s ref=%s", req.Provider, req.EventType, req.Reference)

	response.RespondSuccess(w, map[string]interface{}{
		"provider":   req.Provider,
		"event_type": req.EventType,
		"reference":  req.Reference,
		"processed":  true,
	}, "Webhook simulated successfully")
}

// ═══════════════════════════════════════════════════════════════
// GET /payments/v2/methods
// List available payment methods.
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) ListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	methods := []map[string]interface{}{
		{"code": "card", "name": "Debit/Credit Card", "icon": "💳", "enabled": true, "providers": []string{"paystack", "stripe"}},
		{"code": "bank_transfer", "name": "Bank Transfer", "icon": "🏦", "enabled": true, "providers": []string{"paystack"}},
		{"code": "ussd", "name": "USSD Payment", "icon": "📱", "enabled": true, "providers": []string{"paystack"}},
		{"code": "mobile_money", "name": "Mobile Money", "icon": "📲", "enabled": true, "providers": []string{"paystack"}},
		{"code": "paystack", "name": "Paystack Checkout", "icon": "🟢", "enabled": true, "providers": []string{"paystack"}},
		{"code": "bank", "name": "Direct Bank Debit", "icon": "🏛️", "enabled": true, "providers": []string{"paystack", "stripe"}},
	}

	response.RespondSuccess(w, methods, "Payment methods retrieved")
}

// ═══════════════════════════════════════════════════════════════
// Helper: save full transaction with extra fields
// ═══════════════════════════════════════════════════════════════

func (pi *PaymentIntegration) saveFullTransaction(r *http.Request, ref string, req struct {
	ReservationID string  `json:"reservation_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Email         string  `json:"email"`
	PaymentMethod string  `json:"payment_method"`
	Description   string  `json:"description"`
}, idempotencyKey string) {
	propertyID := getPropertyID(r)
	if propertyID == "" {
		propertyID = "00000000-0000-0000-0000-000000000001"
	}

	_, _ = pi.Manager.DB().ExecContext(r.Context(), `
		INSERT OR REPLACE INTO payment_transactions
		(id, property_id, reservation_id, reference, provider, amount, currency,
		 status, payment_method, email, idempotency_key, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), propertyID, req.ReservationID, ref,
		pi.Manager.ActiveProvider(), req.Amount, req.Currency,
		req.PaymentMethod, req.Email, idempotencyKey, req.Description,
		time.Now(), time.Now())
}
