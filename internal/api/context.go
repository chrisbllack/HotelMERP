package api

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

type propertyIDKey struct{}

// PropertyScope is the property identifier carried in the request context.
type PropertyScope uuid.UUID

// ID returns a stable numeric form of the property id for subsystems that
// still work with integers.
func (p PropertyScope) ID() uint64 {
	return binary.BigEndian.Uint64(p[:8])
}

// UUID returns the canonical uuid form of the scope.
func (p PropertyScope) UUID() uuid.UUID {
	return uuid.UUID(p)
}

// WithPropertyID attaches the property scope to the context.
func WithPropertyID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, propertyIDKey{}, id)
}

// PropertyID returns the property scope attached to the context, if any.
func PropertyID(ctx context.Context) (PropertyScope, bool) {
	raw, ok := ctx.Value(propertyIDKey{}).(uuid.UUID)
	if !ok {
		return PropertyScope{}, false
	}
	return PropertyScope(raw), true
}

// nowRFC3339 renders the current UTC time in the API timestamp format.
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
