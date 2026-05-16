package service

import "context"

type rbacActorSuperAdminContextKey struct{}

// ContextWithRBACActorSuperAdmin records the authenticated admin actor's
// super-admin status for RBAC service boundary checks.
func ContextWithRBACActorSuperAdmin(ctx context.Context, isSuperAdmin bool) context.Context {
	return context.WithValue(ctx, rbacActorSuperAdminContextKey{}, isSuperAdmin)
}

func rbacActorIsSuperAdmin(ctx context.Context) bool {
	isSuperAdmin, ok := ctx.Value(rbacActorSuperAdminContextKey{}).(bool)
	return ok && isSuperAdmin
}
