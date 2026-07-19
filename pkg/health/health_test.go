package health

// health_test.go validates deterministic health snapshot aggregation.
//
// Validates: REQ-009.
// Per: ADR-0029 (file purpose declaration).
// Discipline: C-14.

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestRegistrySnapshotOrdersChecksAndAggregates(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(
		Check{ID: "zeta", Run: func(context.Context) (Result, error) { return OK("ready"), nil }},
		Check{ID: "alpha", Run: func(context.Context) (Result, error) { return Degraded("slow"), nil }},
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	snapshot := registry.Snapshot(context.Background())
	if snapshot.Status != StatusDegraded {
		t.Fatalf("snapshot.Status = %q, want %q", snapshot.Status, StatusDegraded)
	}
	got := []string{snapshot.Results[0].ID, snapshot.Results[1].ID}
	if !slices.Equal(got, []string{"alpha", "zeta"}) {
		t.Fatalf("check order = %v", got)
	}
}

func TestRegistryCriticalFailureIsDown(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(
		Check{ID: "db", Critical: true, Run: func(context.Context) (Result, error) {
			return Result{}, errors.New("unreachable")
		}},
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	snapshot := registry.Snapshot(context.Background())
	if snapshot.Status != StatusDown {
		t.Fatalf("snapshot.Status = %q, want %q", snapshot.Status, StatusDown)
	}
	if snapshot.Results[0].Error == "" {
		t.Fatal("expected check error to be captured")
	}
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry(
		Check{ID: "same", Run: func(context.Context) (Result, error) { return OK(""), nil }},
		Check{ID: "same", Run: func(context.Context) (Result, error) { return OK(""), nil }},
	)
	if err == nil {
		t.Fatal("expected duplicate health check error")
	}
}

func TestRegistrySnapshotAcceptsNilContext(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(Check{ID: "ready", Run: func(context.Context) (Result, error) {
		return OK("ready"), nil
	}})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	var ctx context.Context
	snapshot := registry.Snapshot(ctx)
	if snapshot.Status != StatusOK {
		t.Fatalf("snapshot.Status = %q, want %q", snapshot.Status, StatusOK)
	}
}

func TestRegistrySnapshotDoesNotRunChecksAfterCancellation(t *testing.T) {
	t.Parallel()

	called := false
	registry, err := NewRegistry(Check{
		ID:       "database",
		Critical: true,
		Run: func(context.Context) (Result, error) {
			called = true
			return OK("unexpected"), nil
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot := registry.Snapshot(ctx)

	if called {
		t.Fatal("Snapshot invoked a health check after context cancellation")
	}
	if snapshot.Status != StatusDown {
		t.Fatalf("snapshot.Status = %q, want %q", snapshot.Status, StatusDown)
	}
	if len(snapshot.Results) != 1 || snapshot.Results[0].Error != context.Canceled.Error() {
		t.Fatalf("snapshot.Results = %#v; want context cancellation", snapshot.Results)
	}
}
