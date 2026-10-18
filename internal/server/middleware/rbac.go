package middleware

import (
	"net/http"
	"strings"

	"infineon/internal/auth"
)

// moduleSegments maps the first API path segment after /api/v1 to the
// permission module that guards it. Segments mapped to "" require only an
// authenticated, property-scoped session (self-service or public catalog).
var moduleSegments = map[string]string{
	// Stay core
	"reservations":         "reservations",
	"reservation":          "reservations",
	"check-in":             "reservations",
	"check-out":            "reservations",
	"day-use":              "reservations",
	"card-key":             "reservations",
	"group-bookings":       "reservations",
	"folios":               "folios",
	"folio":                "folios",
	"rooms":                "rooms",
	"room-types":           "rooms",
	"availability":         "",
	"front-desk":           "front_desk",
	"shift-handovers":      "front_desk",
	"guests":               "guests",
	"crm":                  "guests",
	"service-requests":     "guests",
	"company-profile":      "guests",
	"company-profiles":     "guests",
	"travel-agent-profile": "guests",
	"loyalty":              "loyalty",

	// Food & beverage / POS
	"pos":              "pos",
	"kds":              "kds",
	"restaurant":       "pos",
	"bar":              "pos",
	"receipt":          "pos",
	"retail-sale":      "pos",
	"retail-void":      "pos",
	"customer-balance": "pos",

	// Inventory & procurement
	"inventory":   "inventory",
	"procurement": "inventory",
	"wastage":     "inventory",
	"recipes":     "recipes",

	// Finance
	"finance":  "finance",
	"gl":       "finance",
	"invoices": "finance",
	"expenses": "finance",
	"bank-rec": "finance",
	"assets":   "finance",
	"payments": "payments",

	// People
	"hr":  "hr",
	"ess": "", // self-service: authorisation lives in the handler (own profile)
	"mss": "", // manager self-service: team scoping from claims

	// Events & catering
	"events":         "events",
	"event-venues":   "events",
	"event-bookings": "events",

	// Revenue & distribution
	"revenue":       "revenue",
	"seasonal-rate": "revenue",
	"business-rate": "revenue",
	"best-rate":     "revenue",
	"channels":      "channels",

	// Reporting & operations
	"reports":     "reports",
	"executive":   "reports",
	"export":      "reports",
	"night-audit": "night_audit",
	"cashier":     "cashier",
	"shift":       "cashier",
	"shifts":      "cashier",

	// Platform administration
	"users":         "users",
	"roles":         "settings",
	"permissions":   "settings",
	"properties":    "settings",
	"settings":      "settings",
	"config":        "settings",
	"departments":   "settings",
	"hardware":      "settings",
	"workflow":      "settings",
	"documents":     "settings",
	"backups":       "settings",
	"audit-logs":    "settings",
	"domain-events": "settings",

	// Per-user surfaces: authenticated + property-scoped only
	"notifications": "",
	"self-service":  "",
}

// ModuleRBAC enforces permission boundaries at the route-module level.
//
//	Reads  (GET/HEAD/OPTIONS) require any grant inside the module namespace.
//	Writes (POST/PUT/PATCH/DELETE) require a module-scoped ACTION grant —
//	a bare "*.view" grant does not authorise mutations, while wildcards
//	("finance.*", "*") and action grants ("reservations.create",
//	"rooms.update_status", "folios.post_charge") do.
//
// Segments without a module mapping fall back to authentication only.
func ModuleRBAC() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			module := resolveModule(r.URL.Path)
			if module == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims := auth.ClaimsFromContext(r.Context())
			if claims == nil {
				http.Error(w, `{"success":false,"message":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			var allowed bool
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				allowed = auth.ModuleGrantIn(claims.Permissions, module)
			default:
				allowed = auth.WriteGrantIn(claims.Permissions, module)
			}

			if !allowed {
				http.Error(w,
					`{"success":false,"message":"insufficient permissions for module `+module+`"}`,
					http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveModule extracts the permission module for a request path
// (/api/v1/{segment}/... → moduleSegments[segment]).
func resolveModule(path string) string {
	const prefix = "/api/v1/"
	trimmed := strings.TrimPrefix(path, prefix)
	if trimmed == path {
		return "" // not an API path — nothing to gate
	}
	segment := trimmed
	if i := strings.IndexAny(trimmed, "/?"); i >= 0 {
		segment = trimmed[:i]
	}
	return moduleSegments[segment]
}
