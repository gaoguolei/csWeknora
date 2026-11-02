package sandbox

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	cubesandbox "github.com/tencentcloud/CubeSandbox/sdk/go"
)

func TestCubeCapacityWaitRecoversFromExplicitRejection(t *testing.T) {
	for _, rejection := range []*cubesandbox.APIError{
		{StatusCode: http.StatusTooManyRequests},
		{StatusCode: http.StatusInternalServerError, Message: "no more resource"},
		{StatusCode: http.StatusInternalServerError, Message: "CubeMaster 130597: no more resource"},
	} {
		calls := 0
		result, err := waitForCubeCapacity(context.Background(), "Create", time.Second, time.Millisecond, func(context.Context) (string, error) {
			calls++
			if calls == 1 {
				return "", rejection
			}
			return "existing-request-result", nil
		})
		require.NoError(t, err)
		require.Equal(t, "existing-request-result", result)
		require.Equal(t, 2, calls)
	}
}

func TestCubeCapacityWaitDoesNotRetryAmbiguousCreation(t *testing.T) {
	for _, failure := range []error{context.DeadlineExceeded, errors.New("connection reset"), &cubesandbox.APIError{StatusCode: 500, Message: "internal failure"}, &cubesandbox.APIError{StatusCode: 403, Message: "no more resource"}} {
		calls := 0
		_, err := waitForCubeCapacity(context.Background(), "Create", time.Second, time.Millisecond, func(context.Context) (int, error) { calls++; return 0, failure })
		require.Error(t, err)
		require.Equal(t, 1, calls, "ambiguous failures can already have allocated a sandbox")
	}
}

func TestCubeCapacityWaitCancellationAndBudget(t *testing.T) {
	rejection := &cubesandbox.APIError{StatusCode: 429}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := waitForCubeCapacity(ctx, "Connect", time.Second, time.Second, func(context.Context) (int, error) { calls++; cancel(); return 0, rejection })
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
	calls = 0
	_, err = waitForCubeCapacity(context.Background(), "Connect", 10*time.Millisecond, time.Second, func(context.Context) (int, error) { calls++; return 0, rejection })
	require.Equal(t, RemoteErrorKindCapacity, remoteKind(err))
	require.Equal(t, 1, calls)
	require.False(t, CanReplaceRemoteBinding(err), "capacity exhaustion must preserve the original workspace binding")
}

func TestCubeCapacityWaitConfiguration(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    time.Duration
		invalid bool
	}{
		{"", time.Minute, false}, {"0", 0, false}, {"120", 2 * time.Minute, false}, {"-1", 0, true}, {"601", 0, true}, {"x", 0, true},
	} {
		t.Setenv("WEKNORA_SANDBOX_CUBE_CAPACITY_WAIT_SECONDS", tc.raw)
		wait, err := cubeCapacityWaitFromEnv()
		if tc.invalid {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.want, wait)
		}
	}
}

func TestCubeExecRenewsTTLUntilCommandCompletes(t *testing.T) {
	mock := newCubeMockServer(t)
	started := make(chan struct{})
	finish := make(chan struct{})
	defer close(finish)
	mock.SetExecutor(func(string, string, []string) (string, string, int) { close(started); <-finish; return "done", "", 0 })
	client := newTestCubeRemoteClient(t, mock)
	handle, err := client.Create(context.Background(), RemoteCreateRequest{TemplateID: "template-a"})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := client.Exec(ctx, handle, RemoteExecRequest{Command: "long-build"}); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("command did not start")
	}
	require.Eventually(t, func() bool { return mock.timeoutPOSTs.Load() > 0 }, time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("execution did not honor cancellation")
	}
}
