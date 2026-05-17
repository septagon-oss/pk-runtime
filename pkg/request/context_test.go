package request

// context_test.go validates typed request context helpers.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"context"
	"testing"

	"github.com/septagon-oss/pk-core/pkg/authz"
)

func TestTenantIDRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := WithTenantID(context.Background(), " tenant-1 ")
	got, ok := TenantID(ctx)
	if !ok || got != "tenant-1" {
		t.Fatalf("TenantID() = %q, %v", got, ok)
	}
}

func TestPrincipalIsNormalized(t *testing.T) {
	t.Parallel()

	ctx := WithPrincipal(context.Background(), authz.Principal{
		ID:       " user-1 ",
		Kind:     " user ",
		TenantID: " tenant-1 ",
		Roles:    []string{" admin ", "admin"},
	})
	principal, ok := Principal(ctx)
	if !ok {
		t.Fatal("expected principal")
	}
	if principal.ID != "user-1" || principal.Kind != "user" || principal.TenantID != "tenant-1" {
		t.Fatalf("unexpected principal: %#v", principal)
	}
	if len(principal.Roles) != 1 || principal.Roles[0] != "admin" {
		t.Fatalf("roles were not normalized: %#v", principal.Roles)
	}
}
