package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// HasPermission reports whether the caller holds the requested permission,
// honouring wildcard grants such as "*" and "finance.*".
func HasPermission(ctx context.Context, permission string) bool {
	claims := ClaimsFromContext(ctx)
	if claims == nil {
		return false
	}
	return HasPermissionIn(claims.Permissions, permission)
}

// HasPermissionIn matches a permission against a grant list.
func HasPermissionIn(grants []string, permission string) bool {
	for _, g := range grants {
		if g == "*" || g == permission {
			return true
		}
		if strings.HasSuffix(g, ".*") {
			base := strings.TrimSuffix(g, "*") // "finance." from "finance.*"
			if strings.HasPrefix(permission, base) {
				return true
			}
		}
	}
	return false
}

// HasModuleGrant reports whether any grant lives inside the module namespace
// (e.g. "rooms.view", "front_desk.*" → module "rooms"/"front_desk").
func HasModuleGrant(ctx context.Context, module string) bool {
	claims := ClaimsFromContext(ctx)
	if claims == nil {
		return false
	}
	return ModuleGrantIn(claims.Permissions, module)
}

// ModuleGrantIn reports namespace membership for a grant list.
func ModuleGrantIn(grants []string, module string) bool {
	for _, g := range grants {
		if g == "*" || g == module || strings.HasPrefix(g, module+".") {
			return true
		}
	}
	return false
}

// WriteGrantIn reports whether a grant list authorises write operations on a
// module: a wildcard, or a module-scoped action grant that is more than a
// bare read (e.g. "reservations.create", "rooms.update_status",
// "folios.post_charge", "front_desk.*"). Holding only "rooms.view" does not
// authorise writes.
func WriteGrantIn(grants []string, module string) bool {
	readOnly := []string{
		module + ".view",
		module + ".list",
		module + ".search",
		module + ".show",
		module + ".read",
	}
	for _, g := range grants {
		if g == "*" {
			return true
		}
		if g == module || strings.HasPrefix(g, module+".") {
			bare := true
			for _, ro := range readOnly {
				if g == ro {
					bare = false
					break
				}
			}
			if bare {
				return true
			}
		}
	}
	return false
}

// RequirePermission enforces a single permission on the caller's context.
func RequirePermission(ctx context.Context, permission string) error {
	if !HasPermission(ctx, permission) {
		return ErrForbidden
	}
	return nil
}

// HasRole reports whether the caller holds the role; platform/system admins
// implicitly hold every role.
func HasRole(ctx context.Context, role string) bool {
	claims := ClaimsFromContext(ctx)
	if claims == nil {
		return false
	}
	for _, r := range claims.Roles {
		if r == role || r == "platform_admin" || r == "system_admin" {
			return true
		}
	}
	return false
}

// ClaimsFromRequestProperty authorises the caller for the requested property.
// Unscoped tokens and super-admin roles may cross properties.
func ClaimsFromRequestProperty(claims *Claims, property uuid.UUID) error {
	if claims == nil {
		return ErrForbidden
	}
	if claims.PropertyID == uuid.Nil || claims.PropertyID == property {
		return nil
	}
	for _, r := range claims.Roles {
		if r == "system_admin" || r == "platform_admin" {
			return nil
		}
	}
	return ErrForbidden
}
