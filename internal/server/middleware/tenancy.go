package middleware

import (
	"context"
	"net/http"

	"infineon/internal/api"
	"infineon/internal/auth"

	"github.com/google/uuid"
)

// scopeContext pins a request to a property scope. Besides the typed
// api.PropertyID scope it also exposes the legacy string context keys
// ("property_id", "user_id") that a number of handlers still read directly.
// Without them those handlers silently saw a nil tenant and either scanned
// no rows or panicked on the unguarded type assertion.
func scopeContext(ctx context.Context, propertyID, userID uuid.UUID) context.Context {
	if propertyID != uuid.Nil {
		ctx = api.WithPropertyID(ctx, propertyID)
		ctx = context.WithValue(ctx, "property_id", propertyID.String())
	}
	if userID != uuid.Nil {
		ctx = context.WithValue(ctx, "user_id", userID.String())
	}
	return ctx
}

// PropertyScope pins request tenancy to the JWT. The X-Property-ID header is
// verified against the caller's claims before any handler runs, and injected
// from the token when absent — handlers keep reading the header, but it can
// no longer be spoofed by an authenticated caller.
func PropertyScope() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.ClaimsFromContext(r.Context())
			if claims == nil {
				// Auth middleware runs first; be defensive anyway.
				next.ServeHTTP(w, r)
				return
			}

			raw := r.Header.Get("X-Property-ID")

			// Scoped token: either verify the supplied header or inject the
			// caller's own property.
			if claims.PropertyID != uuid.Nil {
				if raw == "" {
					r.Header.Set("X-Property-ID", claims.PropertyID.String())
					ctx := scopeContext(r.Context(), claims.PropertyID, claims.UserID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				propertyUUID, err := uuid.Parse(raw)
				if err != nil {
					http.Error(w,
						`{"success":false,"message":"X-Property-ID must be a valid property identifier"}`,
						http.StatusBadRequest)
					return
				}
				if err := auth.ClaimsFromRequestProperty(claims, propertyUUID); err != nil {
					http.Error(w,
						`{"success":false,"message":"not authorised for this property"}`,
						http.StatusForbidden)
					return
				}
				ctx := scopeContext(r.Context(), propertyUUID, claims.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Unscoped (super-admin) token: propagate when parseable so the
			// context scope exists for downstream subsystems.
			if propertyUUID, err := uuid.Parse(raw); err == nil {
				ctx := scopeContext(r.Context(), propertyUUID, claims.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			next.ServeHTTP(w, r.WithContext(scopeContext(r.Context(), uuid.Nil, claims.UserID)))
		})
	}
}
