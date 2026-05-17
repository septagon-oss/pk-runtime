package httpx

// router_test.go validates route registration, authz guards, and request
// context propagation for the OSS HTTP runtime.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/septagon-oss/pk-core/pkg/authz"
	"github.com/septagon-oss/pk-runtime/pkg/request"
)

func TestRouterServesRoute(t *testing.T) {
	t.Parallel()

	router, err := NewRouter([]Route{{
		ID:      "hello",
		Method:  http.MethodGet,
		Pattern: "/hello",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	}})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRouterProtectedRouteFailsClosedWithoutEvaluator(t *testing.T) {
	t.Parallel()

	router, err := NewRouter([]Route{protectedRoute()})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRouterProtectedRouteAllowsPolicyMatch(t *testing.T) {
	t.Parallel()

	evaluator, err := authz.NewPolicyEvaluator(authz.Policy{
		ID:        "allow-admin",
		ModuleID:  "admin",
		Effect:    authz.EffectAllow,
		Actions:   []string{"read"},
		Resources: []string{"page"},
		Roles:     []string{"admin"},
	})
	if err != nil {
		t.Fatalf("NewPolicyEvaluator() error = %v", err)
	}
	router, err := NewRouter([]Route{protectedRoute()}, WithEvaluator(evaluator))
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := request.WithPrincipal(context.Background(), authz.Principal{ID: "u1", TenantID: "t1", Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRouterRoutesReturnsDefensiveAuthzCopies(t *testing.T) {
	t.Parallel()

	router, err := NewRouter([]Route{protectedRoute()})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	routes := router.Routes()
	routes[0].Authz.Action = "write"

	fresh := router.Routes()
	if fresh[0].Authz.Action != "read" {
		t.Fatalf("Routes() exposed mutable authz spec, action = %q", fresh[0].Authz.Action)
	}
}

func TestDefaultRequestMapperRejectsNilRequest(t *testing.T) {
	t.Parallel()

	_, err := DefaultRequestMapper(nil, Route{}, AuthzSpec{Action: "read", ResourceType: "page"})
	if err == nil {
		t.Fatal("expected nil request error")
	}
}

func protectedRoute() Route {
	return Route{
		ID:       "admin",
		ModuleID: "admin",
		Method:   http.MethodGet,
		Pattern:  "/admin",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		Authz: &AuthzSpec{Action: "read", ResourceType: "page"},
	}
}
