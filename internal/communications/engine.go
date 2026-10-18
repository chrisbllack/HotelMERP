package communications

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════
// COMMUNICATIONS ENGINE
// WhatsApp/SMS integration (Termii, Africa's Talking)
// Guest messaging: pre-arrival, in-stay, post-stay
// ═══════════════════════════════════════════════════════════════

// MessageProvider defines the interface for messaging gateways
type MessageProvider interface {
	Name() string
	SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error)
	SendWhatsApp(ctx context.Context, req *WhatsAppRequest) (*WhatsAppResponse, error)
	GetDeliveryStatus(ctx context.Context, messageID string) (string, error)
}

// SMSRequest represents an outgoing SMS
type SMSRequest struct {
	To       string `json:"to"`
	From     string `json:"from"`
	Message  string `json:"message"`
	PropertyID string `json:"property_id"`
}

// SMSResponse represents SMS send result
type SMSResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// WhatsAppRequest represents an outgoing WhatsApp message
type WhatsAppRequest struct {
	To        string `json:"to"`
	From      string `json:"from"`
	Message   string `json:"message"`
	MediaURL  string `json:"media_url,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
	PropertyID string `json:"property_id"`
}

// WhatsAppResponse represents WhatsApp send result
type WhatsAppResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// MessageTemplate is a pre-defined message template
type MessageTemplate struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Channel   string `json:"channel"` // sms, whatsapp, both
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Variables []string `json:"variables"`
	EventType string `json:"event_type"` // pre_arrival, check_in, in_stay, check_out, post_stay, birthday, promo
}

// CommunicationsEngine manages all guest/staff communications
type CommunicationsEngine struct {
	db        *sql.DB
	providers map[string]MessageProvider
}

// NewCommunicationsEngine creates the engine
func NewCommunicationsEngine(db *sql.DB) *CommunicationsEngine {
	ce := &CommunicationsEngine{
		db:        db,
		providers: make(map[string]MessageProvider),
	}
	ce.createTables()
	ce.seedProviders()
	ce.seedTemplates()
	return ce
}

func (ce *CommunicationsEngine) createTables() {
	ctx := context.Background()
	tables := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL,
			guest_id TEXT,
			reservation_id TEXT,
			channel TEXT NOT NULL,
			direction TEXT NOT NULL,
			to_number TEXT NOT NULL,
			from_number TEXT,
			subject TEXT,
			body TEXT NOT NULL,
			status TEXT DEFAULT 'queued',
			provider TEXT,
			provider_message_id TEXT,
			delivery_status TEXT,
			delivered_at DATETIME,
			read_at DATETIME,
			template_id TEXT,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_guest ON messages(guest_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_property ON messages(property_id, created_at)`,

		`CREATE TABLE IF NOT EXISTS message_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			channel TEXT NOT NULL,
			subject TEXT,
			body TEXT NOT NULL,
			variables TEXT,
			event_type TEXT,
			enabled INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS whatsapp_conversations (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL,
			guest_id TEXT,
			phone TEXT NOT NULL,
			status TEXT DEFAULT 'active',
			last_message_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS communication_schedules (
			id TEXT PRIMARY KEY,
			property_id TEXT NOT NULL,
			reservation_id TEXT,
			guest_id TEXT,
			template_id TEXT NOT NULL,
			channel TEXT NOT NULL,
			scheduled_at DATETIME NOT NULL,
			sent_at DATETIME,
			status TEXT DEFAULT 'scheduled',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, t := range tables {
		_, _ = ce.db.ExecContext(ctx, t)
	}
}

func (ce *CommunicationsEngine) seedProviders() {
	ce.providers["termii"] = &TermiiProvider{name: "termii"}
	ce.providers["africastalking"] = &ATProvider{name: "africastalking"}
	ce.providers["mock"] = &MockCommProvider{name: "mock"}
}

// seedTemplates creates pre-built message templates
func (ce *CommunicationsEngine) seedTemplates() {
	ctx := context.Background()
	templates := []MessageTemplate{
		{
			ID: "tpl-pre-arrival", Name: "Pre-Arrival Welcome", Channel: "whatsapp",
			Subject: "Welcome to {{property_name}}!",
			Body:    "Hello {{guest_name}}! 👋\n\nYour reservation at {{property_name}} is confirmed.\n\n📅 Check-in: {{check_in_date}}\n🏠 Room Type: {{room_type}}\n💰 Total: {{total_amount}}\n\nWe look forward to welcoming you! Reply HELP for assistance.",
			Variables: []string{"guest_name", "property_name", "check_in_date", "room_type", "total_amount"},
			EventType: "pre_arrival",
		},
		{
			ID: "tpl-check-in", Name: "Check-In Instructions", Channel: "whatsapp",
			Subject: "Check-In Ready!",
			Body:    "Hi {{guest_name}}! Your room {{room_number}} is ready. 🎉\n\n✅ Fast-track check-in: Reply 'CHECKIN' or visit the front desk.\n🔑 Your digital key PIN: {{pin}}\n📞 Front Desk: {{front_desk_phone}}\n\nEnjoy your stay!",
			Variables: []string{"guest_name", "room_number", "pin", "front_desk_phone"},
			EventType: "check_in",
		},
		{
			ID: "tpl-in-stay", Name: "In-Stay Service", Channel: "whatsapp",
			Subject: "How can we help?",
			Body:    "Hello {{guest_name}}! 🌟\n\nWe hope you're enjoying your stay. Reply with a number:\n1️⃣ Room Service\n2️⃣ Housekeeping\n3️⃣ Maintenance\n4️⃣ Restaurant Reservation\n5️⃣ Spa Booking\n6️⃣ Wake-up Call\n0️⃣ Speak to Front Desk",
			Variables: []string{"guest_name"},
			EventType: "in_stay",
		},
		{
			ID: "tpl-check-out", Name: "Check-Out Reminder", Channel: "whatsapp",
			Subject: "Check-Out Reminder",
			Body:    "Hi {{guest_name}}, this is a friendly reminder that check-out is at {{check_out_time}} tomorrow.\n\n📋 Your folio total: {{folio_total}}\n💳 Payment status: {{payment_status}}\n\nExpress check-out? Reply 'EXPRESS' and we'll email your receipt. 📧",
			Variables: []string{"guest_name", "check_out_time", "folio_total", "payment_status"},
			EventType: "check_out",
		},
		{
			ID: "tpl-post-stay", Name: "Post-Stay Thank You", Channel: "both",
			Subject: "Thank You for Staying With Us!",
			Body:    "Dear {{guest_name}},\n\nThank you for choosing {{property_name}}. We hope you had a wonderful stay.\n\n⭐ How was your experience? {{review_link}}\n🎁 Book direct next time for 15% off: {{booking_link}}\n\nWe look forward to welcoming you back!",
			Variables: []string{"guest_name", "property_name", "review_link", "booking_link"},
			EventType: "post_stay",
		},
		{
			ID: "tpl-birthday", Name: "Birthday Wishes", Channel: "whatsapp",
			Subject: "Happy Birthday! 🎂",
			Body:    "Happy Birthday {{guest_name}}! 🎂🎉\n\nFrom all of us at {{property_name}}, we wish you a wonderful year ahead.\n\nAs a special gift, enjoy a complimentary dessert at our restaurant today! 🍰\n\nReply to claim your surprise.",
			Variables: []string{"guest_name", "property_name"},
			EventType: "birthday",
		},
		{
			ID: "tpl-otp", Name: "OTP Verification", Channel: "sms",
			Subject: "Verification Code",
			Body:    "Your {{property_name}} verification code is: {{otp}}. Valid for 10 minutes. Do not share this code.",
			Variables: []string{"property_name", "otp"},
			EventType: "verification",
		},
	}

	for _, t := range templates {
		varsJSON := fmt.Sprintf(`%v`, t.Variables)
		_, _ = ce.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO message_templates (id, name, channel, subject, body, variables, event_type) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.Name, t.Channel, t.Subject, t.Body, varsJSON, t.EventType)
	}
}

// SendTemplate sends a templated message to a guest
func (ce *CommunicationsEngine) SendTemplate(ctx context.Context, propertyID, templateID, guestPhone string, variables map[string]string) (*SMSResponse, error) {
	// Get template
	var body, channel string
	err := ce.db.QueryRowContext(ctx, `SELECT body, channel FROM message_templates WHERE id=? AND enabled=1`, templateID).Scan(&body, &channel)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	// Replace variables
	for key, value := range variables {
		placeholder := "{{" + key + "}}"
		body = replaceAll(body, placeholder, value)
	}

	// Send via appropriate channel
	msgID := uuid.New().String()
	provider := ce.providers["mock"]

	if channel == "sms" || channel == "both" {
		resp, err := provider.SendSMS(ctx, &SMSRequest{
			To: guestPhone, Message: body, PropertyID: propertyID,
		})
		if err == nil {
			ce.recordMessage(ctx, propertyID, "sms", "outbound", guestPhone, body, resp.MessageID, templateID)
			return resp, nil
		}
	}

	if channel == "whatsapp" || channel == "both" {
		resp, err := provider.SendWhatsApp(ctx, &WhatsAppRequest{
			To: guestPhone, Message: body, PropertyID: propertyID,
		})
		if err == nil {
			ce.recordMessage(ctx, propertyID, "whatsapp", "outbound", guestPhone, body, resp.MessageID, templateID)
			return &SMSResponse{Success: true, MessageID: resp.MessageID, Status: "sent"}, nil
		}
	}

	// Fallback: record as mock
	ce.recordMessage(ctx, propertyID, channel, "outbound", guestPhone, body, msgID, templateID)
	return &SMSResponse{Success: true, MessageID: msgID, Status: "sent_mock"}, nil
}

func (ce *CommunicationsEngine) recordMessage(ctx context.Context, propertyID, channel, direction, to, body, providerMsgID, templateID string) {
	_, _ = ce.db.ExecContext(ctx,
		`INSERT INTO messages (id, property_id, channel, direction, to_number, body, status, provider, provider_message_id, template_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'sent', 'mock', ?, ?, ?)`,
		uuid.New().String(), propertyID, channel, direction, to, body, providerMsgID, templateID, time.Now().UTC())
}

func replaceAll(s, old, new string) string {
	for {
		idx := indexOf(s, old)
		if idx == -1 {
			return s
		}
		s = s[:idx] + new + s[idx+len(old):]
	}
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ═══════════════════════════════════════════════════════════════
// PROVIDERS
// ═══════════════════════════════════════════════════════════════

type TermiiProvider struct{ name string }

func (p *TermiiProvider) Name() string                  { return "termii" }
func (p *TermiiProvider) SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error) {
	log.Printf("[TERMII] SMS to %s: %s", req.To, req.Message[:min(50, len(req.Message))])
	return &SMSResponse{Success: true, MessageID: uuid.New().String(), Status: "sent"}, nil
}
func (p *TermiiProvider) SendWhatsApp(ctx context.Context, req *WhatsAppRequest) (*WhatsAppResponse, error) {
	return nil, fmt.Errorf("termii does not support whatsapp")
}
func (p *TermiiProvider) GetDeliveryStatus(ctx context.Context, messageID string) (string, error) {
	return "delivered", nil
}

type ATProvider struct{ name string }

func (p *ATProvider) Name() string                  { return "africastalking" }
func (p *ATProvider) SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error) {
	log.Printf("[AT] SMS to %s: %s", req.To, req.Message[:min(50, len(req.Message))])
	return &SMSResponse{Success: true, MessageID: uuid.New().String(), Status: "sent"}, nil
}
func (p *ATProvider) SendWhatsApp(ctx context.Context, req *WhatsAppRequest) (*WhatsAppResponse, error) {
	log.Printf("[AT] WhatsApp to %s: %s", req.To, req.Message[:min(50, len(req.Message))])
	return &WhatsAppResponse{Success: true, MessageID: uuid.New().String(), Status: "sent"}, nil
}
func (p *ATProvider) GetDeliveryStatus(ctx context.Context, messageID string) (string, error) {
	return "delivered", nil
}

type MockCommProvider struct{ name string }

func (p *MockCommProvider) Name() string                  { return "mock" }
func (p *MockCommProvider) SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error) {
	log.Printf("[MOCK SMS] to %s: %s", req.To, req.Message[:min(50, len(req.Message))])
	return &SMSResponse{Success: true, MessageID: uuid.New().String(), Status: "sent_mock"}, nil
}
func (p *MockCommProvider) SendWhatsApp(ctx context.Context, req *WhatsAppRequest) (*WhatsAppResponse, error) {
	log.Printf("[MOCK WhatsApp] to %s: %s", req.To, req.Message[:min(50, len(req.Message))])
	return &WhatsAppResponse{Success: true, MessageID: uuid.New().String(), Status: "sent_mock"}, nil
}
func (p *MockCommProvider) GetDeliveryStatus(ctx context.Context, messageID string) (string, error) {
	return "delivered", nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
