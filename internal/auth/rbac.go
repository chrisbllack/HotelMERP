package auth

import "context"

func HasPermission(ctx context.Context, permission string) bool {
	claims := ClaimsFromContext(ctx)
	if claims == nil { return false }
	for _, p := range claims.Permissions { if p == permission || p == "*" { return true } }
	return false
}

func HasRole(ctx context.Context, role string) bool {
	claims := ClaimsFromContext(ctx)
	if claims == nil { return false }
	for _, r := range claims.Roles {
		if r == role || r == "platform_admin" || r == "system_admin" { return true }
	}
	return false
}

func RequirePermission(ctx context.Context, permission string) error {
	if !HasPermission(ctx, permission) { return ErrForbidden }
	return nil
}
