package store

import "context"

type auditTokenKey struct{}

// WithAuditToken marks ctx as acting through a personal API token, so audit
// rows written with it record the token. The HTTP layer sets it; the store
// never reads request state.
func WithAuditToken(ctx context.Context, tokenID string) context.Context {
	return context.WithValue(ctx, auditTokenKey{}, tokenID)
}

// auditToken is the token ID for an admin_audit.token_id column, or nil.
func auditToken(ctx context.Context) any {
	if id, _ := ctx.Value(auditTokenKey{}).(string); id != "" {
		return id
	}
	return nil
}
