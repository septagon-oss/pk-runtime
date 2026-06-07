// Package health provides deterministic runtime health snapshots.
package health

// health.go owns provider-neutral health checks used by OSS and downstream
// runtimes.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (file purpose declaration).

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Status is the normalized runtime health status.
type Status string

const (
	// StatusOK reports a fully healthy component.
	StatusOK Status = "ok"
	// StatusDegraded reports a component that is serving but impaired.
	StatusDegraded Status = "degraded"
	// StatusDown reports a component that is not serving.
	StatusDown Status = "down"
	// StatusUnknown reports a component whose health could not be determined.
	StatusUnknown Status = "unknown"
)

// Result is returned by a health check.
type Result struct {
	Status  Status            `json:"status"`
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// OK returns a successful health result.
func OK(message string) Result {
	return Result{Status: StatusOK, Message: strings.TrimSpace(message)}
}

// Degraded returns a degraded health result.
func Degraded(message string) Result {
	return Result{Status: StatusDegraded, Message: strings.TrimSpace(message)}
}

// Down returns a failed health result.
func Down(message string) Result {
	return Result{Status: StatusDown, Message: strings.TrimSpace(message)}
}

// CheckFunc executes a health check.
type CheckFunc func(context.Context) (Result, error)

// Check is one health contribution.
type Check struct {
	ID       string
	ModuleID string
	Critical bool
	Run      CheckFunc
}

// Normalize validates and returns a deterministic copy of a check.
func (c Check) Normalize() (Check, error) {
	c.ID = strings.TrimSpace(c.ID)
	c.ModuleID = strings.TrimSpace(c.ModuleID)
	if c.ID == "" {
		return Check{}, fmt.Errorf("health check ID is required")
	}
	if strings.ContainsAny(c.ID, " \t\n\r") {
		return Check{}, fmt.Errorf("health check ID %q must not contain whitespace", c.ID)
	}
	if c.ModuleID != "" && strings.ContainsAny(c.ModuleID, " \t\n\r") {
		return Check{}, fmt.Errorf("health check %q module ID %q must not contain whitespace", c.ID, c.ModuleID)
	}
	if c.Run == nil {
		return Check{}, fmt.Errorf("health check %q run function is required", c.ID)
	}
	return c, nil
}

// Registry is an immutable health check registry.
type Registry struct {
	checks []Check
}

// NewRegistry validates checks and stores them in deterministic order.
func NewRegistry(checks ...Check) (*Registry, error) {
	normalized := make([]Check, 0, len(checks))
	seen := map[string]struct{}{}
	for _, check := range checks {
		c, err := check.Normalize()
		if err != nil {
			return nil, err
		}
		if _, exists := seen[c.ID]; exists {
			return nil, fmt.Errorf("duplicate health check ID %q", c.ID)
		}
		seen[c.ID] = struct{}{}
		normalized = append(normalized, c)
	}
	slices.SortStableFunc(normalized, func(a, b Check) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return &Registry{checks: normalized}, nil
}

// Checks returns a copy of registered check metadata.
func (r *Registry) Checks() []Check {
	if r == nil {
		return nil
	}
	return slices.Clone(r.checks)
}

// CheckResult is the runtime result of one check.
type CheckResult struct {
	ID       string            `json:"id"`
	ModuleID string            `json:"module_id,omitempty"`
	Critical bool              `json:"critical"`
	Status   Status            `json:"status"`
	Message  string            `json:"message,omitempty"`
	Error    string            `json:"error,omitempty"`
	Duration time.Duration     `json:"duration"`
	Details  map[string]string `json:"details,omitempty"`
}

// Snapshot is an aggregate health response.
type Snapshot struct {
	Status    Status        `json:"status"`
	CheckedAt time.Time     `json:"checked_at"`
	Results   []CheckResult `json:"results"`
}

// Snapshot evaluates all checks in deterministic order.
func (r *Registry) Snapshot(ctx context.Context) Snapshot {
	if ctx == nil {
		ctx = context.Background()
	}
	snapshot := Snapshot{Status: StatusUnknown, CheckedAt: time.Now().UTC()}
	if r == nil || len(r.checks) == 0 {
		return snapshot
	}

	results := make([]CheckResult, 0, len(r.checks))
	for _, check := range r.checks {
		start := time.Now()
		result := CheckResult{
			ID:       check.ID,
			ModuleID: check.ModuleID,
			Critical: check.Critical,
			Status:   StatusUnknown,
		}
		if err := ctx.Err(); err != nil {
			result.Status = StatusDown
			result.Error = err.Error()
			result.Duration = time.Since(start)
			results = append(results, result)
			continue
		}

		out, err := check.Run(ctx)
		if err != nil {
			result.Status = StatusDown
			result.Error = err.Error()
			result.Duration = time.Since(start)
			results = append(results, result)
			continue
		}
		status, ok := normalizeStatus(out.Status)
		if !ok {
			status = StatusUnknown
			result.Error = fmt.Sprintf("invalid health status %q", out.Status)
		}
		result.Status = status
		result.Message = strings.TrimSpace(out.Message)
		result.Details = cloneMap(out.Details)
		result.Duration = time.Since(start)
		results = append(results, result)
	}

	snapshot.Results = results
	snapshot.Status = aggregate(results)
	return snapshot
}

func normalizeStatus(status Status) (Status, bool) {
	switch Status(strings.TrimSpace(string(status))) {
	case StatusOK:
		return StatusOK, true
	case StatusDegraded:
		return StatusDegraded, true
	case StatusDown:
		return StatusDown, true
	case StatusUnknown:
		return StatusUnknown, true
	default:
		return "", false
	}
}

func aggregate(results []CheckResult) Status {
	if len(results) == 0 {
		return StatusUnknown
	}
	status := StatusOK
	for _, result := range results {
		switch result.Status {
		case StatusDown:
			if result.Critical {
				return StatusDown
			}
			status = StatusDegraded
		case StatusDegraded, StatusUnknown:
			if status == StatusOK {
				status = StatusDegraded
			}
		}
	}
	return status
}

func cloneMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	return out
}
