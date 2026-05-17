# pk-runtime

Small OSS runtime host contracts for PlatformKit.

`pk-core` defines module contracts. `pk-runtime` hosts them. This repository
keeps the host layer inspectable: route registration, readiness, request
context, and authorization gates are public contracts, while databases, queues,
cloud SDKs, dependency-injection containers, browser automation, and product
workflows stay outside the runtime core.

## Current Surface

- `pkg/host`: compose modules, expose `/live` and `/ready`, and serve runtime
  routes.
- `pkg/health`: deterministic health check registry and readiness snapshots.
- `pkg/httpx`: standard-library HTTP route registry with fail-closed
  authorization guards.
- `pkg/request`: typed request context helpers for tenant and principal data.

## Verify

```bash
make verify
make staticcheck
```
