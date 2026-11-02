package approval

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestLivePolicyReleasesManualWaitAndEmitsResolution(t *testing.T) {
	gate := NewGate(nil, nil, nil)
	bus := event.NewEventBus()
	policy := types.NewToolApprovalPolicy(types.ToolApprovalManual)
	waiting := make(chan struct{}, 1)
	resolved := make(chan bool, 1)
	bus.On(event.EventToolApprovalRequired, func(context.Context, event.Event) error { waiting <- struct{}{}; return nil })
	bus.On(event.EventToolApprovalResolved, func(_ context.Context, e event.Event) error {
		resolved <- e.Data.(event.ToolApprovalResolvedData).Approved
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan Decision, 1)
	go func() {
		d, _ := gate.RequestAndWait(ctx, PendingRequest{TenantID: 1, EventBus: bus, Policy: policy, CanSkipApproval: func(mode types.ToolApprovalMode) bool { return mode == types.ToolApprovalAuto }})
		result <- d
	}()
	<-waiting
	policy.Set(types.ToolApprovalAuto)
	require.True(t, (<-result).Approved)
	require.True(t, <-resolved)
}

func TestLivePolicyDoesNotBypassServiceApprovalWhenSwitchingToAuto(t *testing.T) {
	gate := NewGate(nil, &stubChecker{required: true}, nil)
	bus := event.NewEventBus()
	policy := types.NewToolApprovalPolicy(types.ToolApprovalManual)
	waiting := make(chan struct{}, 1)
	bus.On(event.EventToolApprovalRequired, func(context.Context, event.Event) error { waiting <- struct{}{}; return nil })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan Decision, 1)
	go func() {
		d, _ := gate.RequestAndWait(ctx, PendingRequest{TenantID: 1, EventBus: bus, Policy: policy, CanSkipApproval: func(mode types.ToolApprovalMode) bool {
			return mode == types.ToolApprovalBypass || (mode == types.ToolApprovalAuto && !gate.NeedsApproval(ctx, 1, "svc", "dangerous"))
		}})
		result <- d
	}()
	<-waiting
	policy.Set(types.ToolApprovalAuto)
	select {
	case <-result:
		t.Fatal("automatic mode must retain service approval")
	case <-time.After(30 * time.Millisecond):
	}
	policy.Set(types.ToolApprovalBypass)
	require.True(t, (<-result).Approved)
}

func TestLivePolicyRejectsStaleAutomaticDecisionAfterManualUpdate(t *testing.T) {
	gate := NewGate(nil, nil, nil)
	bus := event.NewEventBus()
	policy := types.NewToolApprovalPolicy(types.ToolApprovalManual)
	waiting := make(chan struct{}, 1)
	checking := make(chan struct{})
	release := make(chan struct{})
	bus.On(event.EventToolApprovalRequired, func(context.Context, event.Event) error { waiting <- struct{}{}; return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan Decision, 1)
	go func() {
		d, _ := gate.RequestAndWait(ctx, PendingRequest{TenantID: 1, EventBus: bus, Policy: policy, CanSkipApproval: func(mode types.ToolApprovalMode) bool {
			if mode == types.ToolApprovalAuto {
				close(checking)
				<-release
				return true
			}
			return mode == types.ToolApprovalBypass
		}})
		result <- d
	}()
	<-waiting
	policy.Set(types.ToolApprovalAuto)
	<-checking
	policy.Set(types.ToolApprovalManual)
	close(release)
	select {
	case <-result:
		t.Fatal("stale auto check approved a now-manual operation")
	case <-time.After(30 * time.Millisecond):
	}
	policy.Set(types.ToolApprovalBypass)
	require.True(t, (<-result).Approved)
}
