package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type recoverableBootstrap struct {
	recordingBootstrapper
	available bool
	checkErr  error
}

func (b *recoverableBootstrap) CanRecoverWorkspace(context.Context, SessionSandboxKey) (bool, error) {
	return b.available, b.checkErr
}

func TestPanelRecoveryRecreatesDeletedCubeBeforeReading(t *testing.T) {
	mgr, client := newSessionManagerExecTestHarness(t)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(10000))
	old, err := mgr.resolveSession(ctx, "s1")
	require.NoError(t, err)
	require.NoError(t, client.Delete(ctx, old.ID()))
	boot := &recoverableBootstrap{available: true}
	mgr.lifecycle.bootstrapper = boot
	// Ordinary reads remain non-provisioning; the explicit panel access recovers.
	_, err = mgr.ReadSessionFile(ctx, "s1", "/workspace/app.ts")
	require.Error(t, err)
	require.Equal(t, 1, client.createCount)
	require.NoError(t, mgr.RecoverSessionWorkspace(ctx, "s1"))
	require.Equal(t, 2, client.createCount)
	require.Equal(t, 1, boot.afterCalls)
	id, bound := mgr.BoundSandboxID(ctx, "s1")
	require.True(t, bound)
	require.NotEqual(t, old.ID(), id)
	_, err = mgr.ReadSessionFile(ctx, "s1", "/workspace/app.ts")
	require.NoError(t, err)
	require.NoError(t, mgr.RecoverSessionWorkspace(ctx, "s1"))
	require.Equal(t, 2, client.createCount, "opening the panel twice must not recreate a live sandbox")
}
func TestPanelRecoveryNeverCreatesWithoutBackupOrOnConnectionFailure(t *testing.T) {
	for _, name := range []string{"no-backup", "backup-query-failed", "provider-offline", "restore-failed"} {
		t.Run(name, func(t *testing.T) {
			mgr, client := newSessionManagerExecTestHarness(t)
			ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(10000))
			old, err := mgr.resolveSession(ctx, "s1")
			require.NoError(t, err)
			boot := &recoverableBootstrap{available: true}
			mgr.lifecycle.bootstrapper = boot
			if name == "provider-offline" {
				client.connectErrs[old.ID()] = NewRemoteError(SandboxTypeCube, "Connect", RemoteErrorKindUnavailable, "offline", nil)
			} else {
				require.NoError(t, client.Delete(ctx, old.ID()))
			}
			switch name {
			case "no-backup":
				boot.available = false
			case "backup-query-failed":
				boot.checkErr = errors.New("storage offline")
			case "restore-failed":
				boot.afterErr = errors.New("corrupt archive")
			}
			err = mgr.RecoverSessionWorkspace(ctx, "s1")
			if name == "no-backup" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if name == "restore-failed" {
				require.Equal(t, 2, client.createCount)
				_, bound := mgr.BoundSandboxID(ctx, "s1")
				require.False(t, bound, "partially restored sandbox must not be exposed")
				boot.afterErr = nil
				require.NoError(t, mgr.RecoverSessionWorkspace(ctx, "s1"), "retry must recover even after failed bootstrap removed the binding")
			} else {
				require.Equal(t, 1, client.createCount)
			}
		})
	}
}

type blockingRecoveryBootstrap struct {
	recoverableBootstrap
	entered, release chan struct{}
}

func (b *blockingRecoveryBootstrap) AfterCreate(context.Context, SessionSandboxKey, RemoteSandboxHandle) error {
	close(b.entered)
	<-b.release
	return nil
}
func TestPanelRecoveryWaitsForConcurrentRestore(t *testing.T) {
	mgr, client := newSessionManagerExecTestHarness(t)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(10000))
	boot := &blockingRecoveryBootstrap{recoverableBootstrap: recoverableBootstrap{available: true}, entered: make(chan struct{}), release: make(chan struct{})}
	mgr.lifecycle.bootstrapper = boot
	first := make(chan error, 1)
	go func() { first <- mgr.RecoverSessionWorkspace(ctx, "s1") }()
	<-boot.entered // A binding exists, but files have not been restored yet.
	second := make(chan error, 1)
	go func() { second <- mgr.RecoverSessionWorkspace(ctx, "s1") }()
	select {
	case err := <-second:
		close(boot.release)
		<-first
		t.Fatalf("panel access returned before restore completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(boot.release)
	require.NoError(t, <-first)
	require.NoError(t, <-second)
	require.Equal(t, 1, client.createCount)
}
