package core

import "github.com/google/uuid"

// AuditEvent is a single immutable audit trail entry written by the
// application layer whenever a domain mutation happens.
type AuditEvent struct {
	OrganisationID uuid.UUID
	PropertyID     uuid.UUID
	ActorID        uuid.UUID
	Action         string
	EntityType     string
	EntityID       string
	RequestID      string
	Metadata       map[string]interface{}
}
