package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/sandbox"
)

type sessionShellOptionsExecutor interface {
	ExecShellCommandWithOptions(
		context.Context, string, string, sandbox.ShellExecOptions,
	) (*sandbox.ExecuteResult, error)
}

type runningSessionSandboxAsserter interface {
	RequireRunningSessionSandbox(context.Context, string) error
}

func requireRunningSessionSandbox(
	ctx context.Context,
	mgr sandbox.Manager,
	sessionID string,
) error {
	if a, ok := mgr.(runningSessionSandboxAsserter); ok {
		return a.RequireRunningSessionSandbox(ctx, sessionID)
	}
	return nil
}
