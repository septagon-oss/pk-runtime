// Package httpx provides small HTTP runtime contracts.
package httpx

// Implements: REQ-005.
// Per: ADR-0028.
// Discipline: C-14.
// router.go owns route registration and fail-closed authorization guards for
// the standard library HTTP runtime.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (file purpose declaration).

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/septagon-oss/pk-core/pkg/authz"
	"github.com/septagon-oss/pk-runtime/pkg/request"
)

// AuthzSpec declares the authorization check for a route.
type AuthzSpec struct {
	Action       string
	ResourceType string
	ResourceID   string
	ModuleID     string
}

// Route is one HTTP route contribution.
type Route struct {
	ID       string
	ModuleID string
	Method   string
	Pattern  string
	Handler  http.Handler
	Authz    *AuthzSpec
}

// Normalize validates and returns a normalized route.
func (r Route) Normalize() (Route, error) {
	r.ID = strings.TrimSpace(r.ID)
	r.ModuleID = strings.TrimSpace(r.ModuleID)
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	r.Pattern = strings.TrimSpace(r.Pattern)
	if r.ID == "" {
		return Route{}, fmt.Errorf("route ID is required")
	}
	if strings.ContainsAny(r.ID, " \t\n\r") {
		return Route{}, fmt.Errorf("route ID %q must not contain whitespace", r.ID)
	}
	if r.ModuleID != "" && strings.ContainsAny(r.ModuleID, " \t\n\r") {
		return Route{}, fmt.Errorf("route %q module ID %q must not contain whitespace", r.ID, r.ModuleID)
	}
	if !validMethod(r.Method) {
		return Route{}, fmt.Errorf("route %q method %q is not supported", r.ID, r.Method)
	}
	if r.Pattern == "" || !strings.HasPrefix(r.Pattern, "/") {
		return Route{}, fmt.Errorf("route %q pattern must start with /", r.ID)
	}
	if r.Handler == nil {
		return Route{}, fmt.Errorf("route %q handler is required", r.ID)
	}
	if r.Authz != nil {
		spec := *r.Authz
		spec.Action = strings.TrimSpace(spec.Action)
		spec.ResourceType = strings.TrimSpace(spec.ResourceType)
		spec.ResourceID = strings.TrimSpace(spec.ResourceID)
		spec.ModuleID = strings.TrimSpace(spec.ModuleID)
		if spec.Action == "" {
			return Route{}, fmt.Errorf("route %q authz action is required", r.ID)
		}
		if spec.ResourceType == "" {
			return Route{}, fmt.Errorf("route %q authz resource type is required", r.ID)
		}
		r.Authz = &spec
	}
	return r, nil
}

// RequestMapper maps a route and HTTP request to an authorization request.
type RequestMapper func(*http.Request, Route, AuthzSpec) (authz.Request, error)

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithEvaluator configures the authorization evaluator for protected routes.
func WithEvaluator(evaluator authz.Evaluator) RouterOption {
	return func(r *Router) {
		r.evaluator = evaluator
	}
}

// WithRequestMapper overrides the default authorization request mapper.
func WithRequestMapper(mapper RequestMapper) RouterOption {
	return func(r *Router) {
		if mapper != nil {
			r.mapper = mapper
		}
	}
}

// Router is an immutable route registry backed by http.ServeMux.
type Router struct {
	mux       *http.ServeMux
	routes    []Route
	evaluator authz.Evaluator
	mapper    RequestMapper
}

// NewRouter validates routes and returns a standard library HTTP handler.
func NewRouter(routes []Route, opts ...RouterOption) (*Router, error) {
	router := &Router{
		mux:    http.NewServeMux(),
		mapper: DefaultRequestMapper,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(router)
		}
	}

	normalized := make([]Route, 0, len(routes))
	seen := map[string]struct{}{}
	for _, route := range routes {
		r, err := route.Normalize()
		if err != nil {
			return nil, err
		}
		key := r.Method + " " + r.Pattern
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate route %s", key)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, r)
	}
	slices.SortStableFunc(normalized, func(a, b Route) int {
		return cmp.Compare(a.Method+" "+a.Pattern, b.Method+" "+b.Pattern)
	})

	for _, route := range normalized {
		handler := route.Handler
		if route.Authz != nil {
			handler = router.guard(route, *route.Authz, handler)
		}
		router.mux.Handle(route.Method+" "+route.Pattern, handler)
	}
	router.routes = normalized
	return router, nil
}

// Routes returns the registered routes.
func (r *Router) Routes() []Route {
	if r == nil {
		return nil
	}
	return cloneRoutes(r.routes)
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r == nil || r.mux == nil {
		http.NotFound(w, req)
		return
	}
	r.mux.ServeHTTP(w, req)
}

func (r *Router) guard(route Route, spec AuthzSpec, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if r.evaluator == nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		authReq, err := r.mapper(req, route, spec)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		result, err := r.evaluator.Evaluate(req.Context(), authReq)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				status = http.StatusServiceUnavailable
			}
			http.Error(w, http.StatusText(status), status)
			return
		}
		if result.Decision != authz.DecisionAllow {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// DefaultRequestMapper maps principal and tenant values from request context.
func DefaultRequestMapper(req *http.Request, route Route, spec AuthzSpec) (authz.Request, error) {
	if req == nil {
		return authz.Request{}, fmt.Errorf("http request is required")
	}
	principal, ok := request.Principal(req.Context())
	if !ok {
		return authz.Request{}, fmt.Errorf("request principal is required")
	}
	tenantID, _ := request.TenantID(req.Context())
	if tenantID == "" {
		tenantID = principal.TenantID
	}
	moduleID := spec.ModuleID
	if moduleID == "" {
		moduleID = route.ModuleID
	}
	return authz.Request{
		Principal: principal,
		Action:    spec.Action,
		Resource: authz.Resource{
			Type:     spec.ResourceType,
			ID:       spec.ResourceID,
			TenantID: tenantID,
			ModuleID: moduleID,
		},
	}, nil
}

func validMethod(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodHead,
		http.MethodOptions:
		return true
	default:
		return false
	}
}

func cloneRoutes(routes []Route) []Route {
	if len(routes) == 0 {
		return nil
	}
	out := make([]Route, len(routes))
	for i, route := range routes {
		out[i] = route
		if route.Authz != nil {
			spec := *route.Authz
			out[i].Authz = &spec
		}
	}
	return out
}
