package handler

import (
	"context"
	"log"
	"net/http"

	"infineon/internal/auth"
	"infineon/internal/database"
	"infineon/internal/events"

	"github.com/google/uuid"
)

// AuditLogger writes immutable audit trail records.
type AuditLogger struct {
	db database.Pool
}

// Log writes an audit trail entry.
func (a *AuditLogger) Log(ctx context.Context, userID *uuid.UUID, module, action string,
	entityID interface{}, oldValue, newValue interface{}, r *http.Request) {

	if a == nil || a.db == nil {
		return
	}
	remoteAddr := ""
	if r != nil {
		remoteAddr = r.RemoteAddr
	}
	var uid interface{}
	if userID != nil {
		uid = *userID
	}
	_, err := a.db.Exec(ctx, `
		INSERT INTO audit_logs (id, user_id, module, action, entity_id, old_value, new_value, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
	`, uuid.New(), uid, module, action, entityID, oldValue, newValue, remoteAddr)
	if err != nil {
		log.Printf("[audit] failed to write audit log: %v", err)
	}
}

type Handler struct {
	db    database.Pool
	jwt   *auth.JWTService
	audit *AuditLogger
	PayV2 *PaymentIntegration
	// Outbox is the ADR-0006 contract bus; nil disables publishing (unit
	// tests) and subscribers degrade to no-ops.
	Outbox *events.Outbox
}

func New(db database.Pool, jwt *auth.JWTService) *Handler {
	return &Handler{
		db:    db,
		jwt:   jwt,
		audit: &AuditLogger{db: db},
	}
}
