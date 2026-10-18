package middleware

import (
	"net/http"

	"infineon/internal/api"
	"infineon/internal/auth"

	"github.com/google/uuid"
)

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
					ctx := api.WithPropertyID(r.Context(), claims.PropertyID)
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
				ctx := api.WithPropertyID(r.Context(), propertyUUID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Unscoped (super-admin) token: propagate when parseable so the
			// context scope exists for downstream subsystems.
			if propertyUUID, err := uuid.Parse(raw); err == nil {
				ctx := api.WithPropertyID(r.Context(), propertyUUID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
