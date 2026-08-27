package public_test

import (
	"context"
	"testing"

	"github.com/soma-genesis/scenario-3-order-stability/internal/dependencies"
	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

func TestDependencySimulator_SucceedsWhenConfiguredHealthy(t *testing.T) {
	sim := dependencies.NewSimulator(domain.DependencyPayment)
	sim.Configure("tenant-a", dependencies.Behaviour{Mode: dependencies.FailureNone})
	if err := sim.Call(context.Background(), "tenant-a"); err != nil {
		t.Fatalf("expected a healthy simulator to succeed, got %v", err)
	}
}

func TestBreaker_OpensAfterThresholdAndResetsOnSuccess(t *testing.T) {
	b := dependencies.NewBreaker(2)
	scope := dependencies.TenantScope("tenant-a", domain.DependencyPayment)
	if !b.Allow(scope) {
		t.Fatal("expected a fresh breaker to allow calls")
	}
	b.RecordFailure(scope)
	b.RecordFailure(scope)
	if b.Allow(scope) {
		t.Fatal("expected the breaker to open after reaching its threshold")
	}
	b.RecordSuccess(scope)
	if !b.Allow(scope) {
		t.Fatal("expected the breaker to close again after a recorded success")
	}
}
