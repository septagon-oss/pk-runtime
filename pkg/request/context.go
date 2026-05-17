// Package request provides typed request context helpers.
package request

// context.go owns request-scoped tenant and principal context values without
// coupling runtime to an HTTP framework.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (file purpose declaration).

import (
	"context"
	"strings"

	"github.com/septagon-oss/pk-core/pkg/authz"
)

type contextKey string

const (
	tenantKey    contextKey = "platformkit.tenant"
	principalKey contextKey = "platformkit.principal"
)

// WithTenantID returns a context carrying a normalized tenant ID. Empty tenant
// IDs are ignored so callers do not accidentally overwrite an existing value
// with whitespace.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ctx
	}
	return context.WithValue(ctx, tenantKey, tenantID)
}

// TenantID returns the request tenant ID when one has been set.
func TenantID(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	tenantID, ok := ctx.Value(tenantKey).(string)
	if !ok || strings.TrimSpace(tenantID) == "" {
		return "", false
	}
	return tenantID, true
}

// WithPrincipal returns a context carrying a normalized authorization
// principal.
func WithPrincipal(ctx context.Context, principal authz.Principal) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized := authz.Request{Principal: principal}.Normalize().Principal
	if normalized.ID == "" && normalized.Kind == "" && normalized.TenantID == "" && len(normalized.Roles) == 0 && len(normalized.Claims) == 0 {
		return ctx
	}
	return context.WithValue(ctx, principalKey, normalized)
}

// Principal returns the request principal when one has been set.
func Principal(ctx context.Context) (authz.Principal, bool) {
	if ctx == nil {
		return authz.Principal{}, false
	}
	principal, ok := ctx.Value(principalKey).(authz.Principal)
	if !ok {
		return authz.Principal{}, false
	}
	return principal, true
}
