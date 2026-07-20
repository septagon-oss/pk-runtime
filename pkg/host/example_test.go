package host_test

// Validates: REQ-016.
// Per: ADR-0034.
// Discipline: C-14.
// example_test.go provides runnable godoc examples for composing and serving an
// OSS runtime host.
//
// ADR: ADR-0017 (composition through dependency injection), ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/septagon-oss/pk-core/pkg/module"
	"github.com/septagon-oss/pk-runtime/pkg/host"
	"github.com/septagon-oss/pk-runtime/pkg/httpx"
)

// ExampleNew composes a single module into a runtime host, registers a route,
// and serves the built-in readiness endpoint.
func ExampleNew() {
	catalog := module.NewCatalog().Add(module.NewBundle("example", []module.Entry{
		{ID: "base", New: func() module.Composable {
			return module.Must(module.Metadata{ID: "base"})
		}},
	}, []string{"base"})).MustBuild()

	runtime, err := host.New(context.Background(), host.Input{
		Config:  host.Config{Name: "example"},
		Catalog: catalog,
		Routes: []httpx.Route{{
			ID:      "hello",
			Method:  http.MethodGet,
			Pattern: "/hello",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		}},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	ready := httptest.NewRecorder()
	runtime.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))

	fmt.Println("modules:", len(runtime.ModuleMetadata()))
	fmt.Println("ready:", ready.Code)
	// Output:
	// modules: 1
	// ready: 200
}
