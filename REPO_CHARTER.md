# pk-runtime Charter

## Purpose

Runtime host, request context, health check, and HTTP routing contracts for PlatformKit OSS. Provider-neutral primitives for bootstrapping and running a PlatformKit application.

## In Scope

- Host composition (`pkg/host`): application lifecycle and component boot
- Health checks (`pkg/health`): registry, aggregated checks, alert derivation
- HTTP routing (`pkg/httpx`): route registration and middleware contracts
- Request context (`pkg/request`): tenant, identity, and request-scoped state

## Out of Scope

- HTTP server implementations
- Database, queue, or cloud provider adapters
- Module wiring or dependency injection (handled by pk-core)
- CLI or developer tooling (handled by pk-tools)

## Dependencies

- `github.com/septagon-oss/pk-core` — module, observability, and infrastructure contracts
