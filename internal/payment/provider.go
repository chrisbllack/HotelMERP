package payment

import (
	"context"
	"time"
)

// ═══════════════════════════════════════════════════════════════
// Payment Provider Interface
// ═══════════════════════════════════════════════════════════════

// Provider defines the interface for payment gateway integrations.
// Each provider (Paystack, Stripe, Mock) implements this interface.
type Provider interface {
	Name() string
	Initialize(ctx context.Context, req InitializeRequest) (*InitializeResponse, error)
	Verify(ctx context.Context, reference string) (*VerificationResult, error)
	Refund(ctx context.Context, req RefundRequest) (*RefundResult, error)
	VerifyWebhookSignature(payload []byte, signature string) bool
}

// InitializeRequest is sent when initiating a payment.
type InitializeRequest struct {
	Reference   string            `json:"reference"`
	Amount      float64           `json:"amount"`
	Currency    string            `json:"currency"`
	Email       string            `json:"email"`
	Metadata    map[string]string `json:"metadata"`
	CallbackURL string            `json:"callback_url"`
	// Tenancy: persisted with the transaction so captures can be attributed
	// to the right property when webhooks arrive without request headers.
	PropertyID    string `json:"property_id"`
	ReservationID string `json:"reservation_id"`
}

// InitializeResponse contains gateway-specific response data.
type InitializeResponse struct {
	Reference        string `json:"reference"`
	AuthorizationURL string `json:"authorization_url"`
	AccessCode       string `json:"access_code"`
	Status           string `json:"status"`
}

// VerificationResult is the verified payment status.
type VerificationResult struct {
	Reference     string            `json:"reference"`
	Status        string            `json:"status"` // success, failed, abandoned
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	Channel       string            `json:"channel"` // card, bank, ussd, mobile_money
	GatewayRef    string            `json:"gateway_ref"`
	PaidAt        time.Time         `json:"paid_at"`
	CustomerEmail string            `json:"customer_email"`
	Metadata      map[string]string `json:"metadata"`
}

// RefundRequest is sent when initiating a refund.
type RefundRequest struct {
	Reference string  `json:"reference"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason"`
}

// RefundResult contains the refund response.
type RefundResult struct {
	RefundRef string  `json:"refund_reference"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
}

// ═══════════════════════════════════════════════════════════════
// Payment Transaction Record
// ═══════════════════════════════════════════════════════════════

type Transaction struct {
	ID             string    `json:"id"`
	PropertyID     string    `json:"property_id"`
	ReservationID  string    `json:"reservation_id,omitempty"`
	Reference      string    `json:"reference"`
	Provider       string    `json:"provider"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"` // pending, success, failed, refunded, partially_refunded
	PaymentMethod  string    `json:"payment_method"`
	Channel        string    `json:"channel"`
	Email          string    `json:"email"`
	GatewayRef     string    `json:"gateway_ref"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Metadata       string    `json:"metadata,omitempty"`
	Description    string    `json:"description,omitempty"`
	FailureReason  string    `json:"failure_reason,omitempty"`
	RefundedAmount float64   `json:"refunded_amount"`
	PaidAt         time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════
// Webhook Event
// ═══════════════════════════════════════════════════════════════

type WebhookEvent struct {
	ID          string            `json:"id"`
	Provider    string            `json:"provider"`
	EventType   string            `json:"event_type"`
	Reference   string            `json:"reference"`
	RawPayload  []byte            `json:"-"`
	ProcessedAt time.Time         `json:"processed_at"`
	Metadata    map[string]string `json:"metadata"`
}

// ═══════════════════════════════════════════════════════════════
// Provider Configuration
// ═══════════════════════════════════════════════════════════════

type Config struct {
	ActiveProvider string // paystack, stripe, mock
	Paystack       PaystackConfig
	Stripe         StripeConfig
	WebhookSecret  string
	BaseURL        string
}

type PaystackConfig struct {
	SecretKey     string
	PublicKey     string
	WebhookSecret string
	BaseURL       string // default: https://api.paystack.co
}

type StripeConfig struct {
	SecretKey      string
	PublishableKey string
	WebhookSecret  string
	BaseURL        string // default: https://api.stripe.com
}
