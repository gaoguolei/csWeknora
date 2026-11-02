package service

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

//go:embed workspace_recovery.py
var workspaceRecoveryScript string

type workspaceRecoveryStore interface {
	GetSessionArtifacts(context.Context, string) (types.MessageArtifacts, error)
}
type workspaceRecoveryFiles interface {
	GetFile(context.Context, string) (io.ReadCloser, error)
}

// Recovery only runs on a NEW Cube sandbox, under the lifecycle lock. A paused
// guest keeps its disk and never enters this path. Fork snapshots take priority.
// A failed recovery must fail bootstrap so an empty guest cannot overwrite the
// last durable backup on the next turn.
type workspaceRecoveryBootstrapper struct {
	fork   sandbox.SessionBootstrapper
	store  workspaceRecoveryStore
	files  workspaceRecoveryFiles
	client sandbox.RemoteSandboxClient
}

// NewWorkspaceRecoveryBootstrapper composes fork bootstrap with durable Cube source recovery.
func NewWorkspaceRecoveryBootstrapper(fork *ForkBootstrapper, messages interfaces.MessageRepository, files interfaces.FileService) sandbox.SessionBootstrapper {
	return &workspaceRecoveryBootstrapper{fork: fork, store: messages, files: files}
}

func (b *workspaceRecoveryBootstrapper) WithClient(client sandbox.RemoteSandboxClient) sandbox.SessionBootstrapper {
	cp := *b
	cp.client = client
	if fork, ok := b.fork.(sandbox.SessionBootstrapperWithClient); ok {
		cp.fork = fork.WithClient(client)
	}
	return &cp
}
func (b *workspaceRecoveryBootstrapper) TemplateOverride(ctx context.Context, key sandbox.SessionSandboxKey) (string, error) {
	return b.fork.TemplateOverride(ctx, key)
}
func (b *workspaceRecoveryBootstrapper) OnCreateFailed(ctx context.Context, key sandbox.SessionSandboxKey, err error) {
	if handler, ok := b.fork.(sandbox.SessionCreateFailureHandler); ok {
		handler.OnCreateFailed(ctx, key, err)
	}
}
func (b *workspaceRecoveryBootstrapper) AfterCreate(ctx context.Context, key sandbox.SessionSandboxKey, handle sandbox.RemoteSandboxHandle) error {
	override, err := b.fork.TemplateOverride(ctx, key)
	if err != nil {
		return err
	}
	if err := b.fork.AfterCreate(ctx, key, handle); err != nil {
		return err
	}
	if override != "" || handle.Provider() != sandbox.SandboxTypeCube {
		return nil
	}
	if b.store == nil || b.files == nil || b.client == nil {
		return fmt.Errorf("workspace recovery: storage or sandbox client is unavailable")
	}
	artifact, err := b.latestBackup(ctx, key)
	if err != nil {
		return err
	}
	if artifact != nil {
		if err := b.restore(ctx, handle, *artifact); err != nil {
			return err
		}
		logger.Infof(ctx, "[WorkspaceRecovery] restored source backup for session=%s sandbox=%s", key.SessionID, handle.ID())
	}
	return b.repairWorkspace(ctx, key, handle, true)
}

// CanRecoverWorkspace is checked before panel access is allowed to allocate a
// replacement guest. Fork data and source backups are both durable baselines.
func (b *workspaceRecoveryBootstrapper) CanRecoverWorkspace(ctx context.Context, key sandbox.SessionSandboxKey) (bool, error) {
	override, err := b.fork.TemplateOverride(ctx, key)
	if err != nil {
		return false, err
	}
	if override != "" {
		return true, nil
	}
	artifact, err := b.latestBackup(ctx, key)
	if err != nil || artifact != nil {
		return artifact != nil, err
	}
	artifacts, err := b.store.GetSessionArtifacts(ctx, key.SessionID)
	return len(recoverableWorkspaceArtifacts(artifacts)) > 0, err
}

func (b *workspaceRecoveryBootstrapper) latestBackup(ctx context.Context, key sandbox.SessionSandboxKey) (*types.MessageArtifact, error) {
	if b.store == nil {
		return nil, fmt.Errorf("workspace recovery: backup store is unavailable")
	}
	artifacts, err := b.store.GetSessionArtifacts(ctx, key.SessionID)
	if err != nil {
		return nil, fmt.Errorf("workspace recovery: list backups: %w", err)
	}
	// Repository order is message chronology, oldest first. Never use an older
	// recovery point just because the newest live backup fails to restore.
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if !artifact.Deleted() && artifact.FileName == "workspace-source.zip" {
			return &artifact, nil
		}
	}
	return nil, nil
}

func (b *workspaceRecoveryBootstrapper) restore(ctx context.Context, handle sandbox.RemoteSandboxHandle, artifact types.MessageArtifact) error {
	return b.restoreSource(ctx, handle, artifact, false)
}

func (b *workspaceRecoveryBootstrapper) restoreSource(ctx context.Context, handle sandbox.RemoteSandboxHandle, artifact types.MessageArtifact, missingOnly bool) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	reader, err := b.files.GetFile(ctx, artifact.URL)
	if err != nil {
		return fmt.Errorf("workspace recovery: read latest backup: %w", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, defaultMaxArtifactFileBytes+1))
	if err != nil {
		return fmt.Errorf("workspace recovery: read backup: %w", err)
	}
	if int64(len(data)) > defaultMaxArtifactFileBytes {
		return fmt.Errorf("workspace recovery: backup exceeds size limit")
	}
	if artifact.ContentHash != "" {
		hash := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(hash[:]), artifact.ContentHash) {
			return fmt.Errorf("workspace recovery: backup checksum mismatch")
		}
	}
	path := "/tmp/weknora-workspace-recovery-" + uuid.NewString() + ".zip"
	if err := b.client.WriteFile(ctx, handle, path, data); err != nil {
		return fmt.Errorf("workspace recovery: stage backup: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = b.client.Remove(cleanupCtx, handle, path)
	}()
	mode := " "
	if missingOnly {
		mode = " --missing "
	}
	result, err := b.client.Exec(ctx, handle, sandbox.RemoteExecRequest{
		Command: "python3 -c " + sandbox.ShellQuote(workspaceRecoveryScript) + mode + sandbox.ShellQuote(path) + " " + sandbox.ShellQuote(sandbox.SessionWorkspaceRoot),
		Shell:   true, WorkDir: "/tmp", Timeout: 90 * time.Second, User: sandbox.DefaultSandboxExecUser,
	})
	if err != nil {
		return fmt.Errorf("workspace recovery: extract backup: %w", err)
	}
	if result == nil || result.ExitCode != 0 {
		detail := "no result"
		if result != nil {
			detail = truncateForLog(result.Stderr)
		}
		return fmt.Errorf("workspace recovery: extract backup failed: %s", detail)
	}
	return nil
}

var _ sandbox.SessionBootstrapperWithClient = (*workspaceRecoveryBootstrapper)(nil)
var _ sandbox.SessionCreateFailureHandler = (*workspaceRecoveryBootstrapper)(nil)

var _ sandbox.SessionWorkspaceRecoveryChecker = (*workspaceRecoveryBootstrapper)(nil)
