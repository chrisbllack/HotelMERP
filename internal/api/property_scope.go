package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"infineon/internal/auth"
)

func RequirePropertyHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimSpace(r.Header.Get("X-Property-ID"))
		if raw == "" {
			WriteError(
				w,
				r,
				http.StatusBadRequest,
				"property_context_required",
				"X-Property-ID is required for this request",
				nil,
			)
			return
		}

		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			WriteError(
				w,
				r,
				http.StatusUnauthorized,
				"unauthorized",
				"Authentication is required",
				nil,
			)
			return
		}

		if propertyUUID, err := uuid.Parse(raw); err == nil {
			if err := auth.ClaimsFromRequestProperty(claims, propertyUUID); err != nil {
				WriteError(
					w,
					r,
					http.StatusForbidden,
					"forbidden",
					"User is not authorised for this property",
					nil,
				)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithPropertyID(r.Context(), propertyUUID)))
			return
		}

		if claims.PropertyID != uuid.Nil {
			if _, err := strconv.Atoi(raw); err != nil {
				WriteError(
					w,
					r,
					http.StatusBadRequest,
					"invalid_property_context",
					"X-Property-ID must be a valid property identifier",
					nil,
				)
				return
			}

			WriteError(
				w,
				r,
				http.StatusForbidden,
				"forbidden",
				"User is not authorised for this property",
				nil,
			)
			return
		}

		WriteError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_property_context",
			"X-Property-ID must be a valid UUID",
			nil,
		)
	})
}
