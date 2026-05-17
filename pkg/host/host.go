// Package host composes modules into a small HTTP-capable runtime host.
package host

// host.go owns the minimal app host contract for OSS PlatformKit runtimes.
//
// ADR: ADR-0017 (composition through dependency injection), ADR-0029 (file purpose declaration).
// Convention: C-14 (file purpose declaration).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/septagon-oss/pk-core/pkg/authz"
	"github.com/septagon-oss/pk-core/pkg/module"
	"github.com/septagon-oss/pk-runtime/pkg/health"
	"github.com/septagon-oss/pk-runtime/pkg/httpx"
)

// Config identifies a runtime host.
type Config struct {
	Name        string
	Version     string
	Environment string
}

// Normalize validates host identity and fills stable defaults.
func (c Config) Normalize() (Config, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Version = strings.TrimSpace(c.Version)
	c.Environment = strings.TrimSpace(c.Environment)
	if c.Name == "" {
		return Config{}, fmt.Errorf("host name is required")
	}
	if strings.ContainsAny(c.Name, " \t\n\r") {
		return Config{}, fmt.Errorf("host name %q must not contain whitespace", c.Name)
	}
	if c.Version == "" {
		c.Version = "0.0.0"
	}
	if c.Environment == "" {
		c.Environment = "development"
	}
	return c, nil
}

// Input is the complete host construction contract.
type Input struct {
	Config       Config
	Catalog      *module.Catalog
	Modules      []string
	HealthChecks []health.Check
	Routes       []httpx.Route
	Authz        authz.Evaluator
	HTTPOptions  []httpx.RouterOption
}

// Host is an immutable composed runtime.
type Host struct {
	config Config
	plan   *module.Plan
	health *health.Registry
	router *httpx.Router
}

// New validates, composes, and returns a runtime host.
func New(ctx context.Context, input Input) (*Host, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	config, err := input.Config.Normalize()
	if err != nil {
		return nil, err
	}
	if input.Catalog == nil {
		return nil, fmt.Errorf("host catalog is required")
	}

	plan, err := module.Compose(input.Catalog, input.Modules...)
	if err != nil {
		return nil, fmt.Errorf("compose modules: %w", err)
	}
	checks := append([]health.Check{compositionCheck(plan)}, input.HealthChecks...)
	healthRegistry, err := health.NewRegistry(checks...)
	if err != nil {
		return nil, fmt.Errorf("build health registry: %w", err)
	}

	if err := rejectReservedRoutes(input.Routes); err != nil {
		return nil, err
	}
	options := append([]httpx.RouterOption{httpx.WithEvaluator(input.Authz)}, input.HTTPOptions...)
	router, err := httpx.NewRouter(input.Routes, options...)
	if err != nil {
		return nil, fmt.Errorf("build router: %w", err)
	}
	return &Host{config: config, plan: plan, health: healthRegistry, router: router}, nil
}

// Config returns host identity.
func (h *Host) Config() Config {
	if h == nil {
		return Config{}
	}
	return h.config
}

// Plan returns the composed module plan.
func (h *Host) Plan() *module.Plan {
	if h == nil || h.plan == nil {
		return &module.Plan{}
	}
	return &module.Plan{
		Modules:     slices.Clone(h.plan.Modules),
		Providers:   slices.Clone(h.plan.Providers),
		Invocations: slices.Clone(h.plan.Invocations),
	}
}

// ModuleMetadata returns composed module metadata in runtime order.
func (h *Host) ModuleMetadata() []module.Metadata {
	if h == nil || h.plan == nil {
		return nil
	}
	out := make([]module.Metadata, 0, len(h.plan.Modules))
	for _, mod := range h.plan.Modules {
		out = append(out, mod.Metadata())
	}
	return out
}

// Health returns the host health registry.
func (h *Host) Health() *health.Registry {
	if h == nil {
		return nil
	}
	return h.health
}

// ServeHTTP serves health endpoints and registered runtime routes.
func (h *Host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	switch r.URL.Path {
	case "/live":
		w.WriteHeader(http.StatusNoContent)
	case "/ready":
		h.writeReady(w, r)
	default:
		h.router.ServeHTTP(w, r)
	}
}

func (h *Host) writeReady(w http.ResponseWriter, r *http.Request) {
	snapshot := h.health.Snapshot(r.Context())
	status := http.StatusOK
	if snapshot.Status != health.StatusOK {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(snapshot)
}

func compositionCheck(plan *module.Plan) health.Check {
	return health.Check{
		ID:       "runtime.modules",
		ModuleID: "runtime",
		Critical: true,
		Run: func(context.Context) (health.Result, error) {
			if plan == nil {
				return health.Down("module plan is nil"), nil
			}
			return health.Result{
				Status:  health.StatusOK,
				Message: "module plan composed",
				Details: map[string]string{
					"modules": fmt.Sprintf("%d", len(plan.Modules)),
				},
			}, nil
		},
	}
}

func rejectReservedRoutes(routes []httpx.Route) error {
	for _, route := range routes {
		pattern := strings.TrimSpace(route.Pattern)
		if pattern == "/live" || pattern == "/ready" {
			return fmt.Errorf("route %q uses reserved runtime path %q", strings.TrimSpace(route.ID), pattern)
		}
	}
	return nil
}
