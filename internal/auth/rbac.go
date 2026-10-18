package auth

import (
	"context"

	"github.com/google/uuid"
)

func HasPermission(ctx context.Context, permission string) bool {
	claims := ClaimsFromContext(ctx)
	if claims == nil {
		return false
	}
	for _, p := range claims.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

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

func RequirePermission(ctx context.Context, permission string) error {
	if !HasPermission(ctx, permission) {
		return ErrForbidden
	}
	return nil
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
