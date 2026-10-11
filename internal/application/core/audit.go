package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	domain "infineon/internal/domain/core"
)

type PGSQLauditWriter struct {
	db interface {
		Exec(
			ctx context.Context,
			sql string,
			args ...any,
		) (pgconn.CommandTag, error)
	}
}

func NewPGSQLAuditWriter(db interface {
	Exec(
		ctx context.Context,
		sql string,
		args ...any,
	) (pgconn.CommandTag, error)
}) *PGSQLauditWriter {
	return &PGSQLauditWriter{db: db}
}

func (w *PGSQLauditWriter) Write(
	ctx context.Context,
	event domain.AuditEvent,
) error {
	if w == nil || w.db == nil {
		return fmt.Errorf("audit database is not configured")
	}

	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	_, err = w.db.Exec(
		ctx,
		`
		INSERT INTO audit_logs (
			id,
			organisation_id,
			property_id,
			actor_id,
			action,
			entity_type,
			entity_id,
			request_id,
			metadata,
			created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
		`,
		uuid.New(),
		event.OrganisationID,
		event.PropertyID,
		event.ActorID,
		event.Action,
		event.EntityType,
		event.EntityID,
		event.RequestID,
		metadata,
	)
	if err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}

	return nil
}
