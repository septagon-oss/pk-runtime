package host

// host_test.go validates module composition and readiness serving for the OSS
// runtime host.
//
// ADR: ADR-0017 (composition through dependency injection), ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/septagon-oss/pk-core/pkg/module"
	"github.com/septagon-oss/pk-runtime/pkg/health"
	"github.com/septagon-oss/pk-runtime/pkg/httpx"
)

func TestHostComposesModulesAndServesReady(t *testing.T) {
	t.Parallel()

	catalog := module.NewCatalog().Add(module.NewBundle("test", []module.Entry{
		{ID: "base", New: func() module.Composable {
			return module.Must(module.Metadata{ID: "base"})
		}},
	}, []string{"base"})).MustBuild()

	host, err := New(context.Background(), Input{
		Config:  Config{Name: "test"},
		Catalog: catalog,
		HealthChecks: []health.Check{{
			ID: "custom",
			Run: func(context.Context) (health.Result, error) {
				return health.OK("ok"), nil
			},
		}},
		Routes: []httpx.Route{{
			ID:      "hello",
			Method:  http.MethodGet,
			Pattern: "/hello",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
			}),
		}},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(host.ModuleMetadata()) != 1 {
		t.Fatalf("len(ModuleMetadata()) = %d, want 1", len(host.ModuleMetadata()))
	}

	ready := httptest.NewRecorder()
	host.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("/ready status = %d, want %d", ready.Code, http.StatusOK)
	}

	route := httptest.NewRecorder()
	host.ServeHTTP(route, httptest.NewRequest(http.MethodGet, "/hello", nil))
	if route.Code != http.StatusCreated {
		t.Fatalf("/hello status = %d, want %d", route.Code, http.StatusCreated)
	}
}

func TestHostRejectsReservedRuntimeRoute(t *testing.T) {
	t.Parallel()

	catalog := module.NewCatalog().MustBuild()
	_, err := New(context.Background(), Input{
		Config:  Config{Name: "test"},
		Catalog: catalog,
		Routes: []httpx.Route{{
			ID:      "ready",
			Method:  http.MethodGet,
			Pattern: "/ready",
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		}},
	})
	if err == nil {
		t.Fatal("expected reserved route error")
	}
}
