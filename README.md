# pk-runtime

> Part of [PlatformKit](https://github.com/septagon-oss/platformkit) — the open-source Go backend for multi-tenant SaaS.

**Depends on.** `pk-core` only. Nothing else in PlatformKit.

[![Go Reference](https://pkg.go.dev/badge/github.com/septagon-oss/pk-runtime.svg)](https://pkg.go.dev/github.com/septagon-oss/pk-runtime)
[![CI](https://github.com/septagon-oss/pk-runtime/actions/workflows/go.yml/badge.svg)](https://github.com/septagon-oss/pk-runtime/actions/workflows/go.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Small OSS runtime host contracts for the PlatformKit family. `pk-core` defines module contracts; `pk-runtime` hosts them. The host layer stays inspectable: route registration, readiness, request context, and fail-closed authorization gates are public contracts, while databases, queues, cloud SDKs, dependency-injection containers, browser automation, and product workflows stay outside the runtime core.

## Install

```bash
go get github.com/septagon-oss/pk-runtime@v0.1.0
```

## Usage

```go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/septagon-oss/pk-core/pkg/module"
	"github.com/septagon-oss/pk-runtime/pkg/host"
	"github.com/septagon-oss/pk-runtime/pkg/httpx"
)

func main() {
	catalog := module.NewCatalog().Add(module.NewBundle("app", []module.Entry{
		{ID: "base", New: func() module.Composable {
			return module.Must(module.Metadata{ID: "base"})
		}},
	}, []string{"base"})).MustBuild()

	runtime, err := host.New(context.Background(), host.Input{
		Config:  host.Config{Name: "app"},
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
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServe(":8080", runtime))
}
```

## Current Surface

- `pkg/host`: compose modules into an immutable runtime, expose `/live` and `/ready`, and serve registered routes.
- `pkg/health`: deterministic health check registry and aggregate readiness snapshots.
- `pkg/httpx`: standard-library HTTP route registry with fail-closed authorization guards.
- `pkg/request`: typed request context helpers for tenant and principal data.

## Verify

```bash
make verify   # go test + go vet + staticcheck + race
```

## License

Apache-2.0. See [LICENSE](LICENSE).
