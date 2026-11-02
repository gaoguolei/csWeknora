package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
)

// Source archives intentionally exclude output/. Persisted message artifacts
// are its durable copy; choose the latest row per original path, respecting
// tombstones so an older deleted version cannot be resurrected.
func recoverableWorkspaceArtifacts(artifacts types.MessageArtifacts) []types.MessageArtifact {
	latest := map[string]types.MessageArtifact{}
	for _, a := range artifacts {
		if a.FileName == "workspace-source.zip" || !strings.HasPrefix(a.SourcePath, sandbox.SessionWorkspaceRoot+"/") {
			continue
		}
		relative := strings.TrimPrefix(a.SourcePath, sandbox.SessionWorkspaceRoot+"/")
		if path.Clean(relative) != relative || strings.Contains(relative, "\\") {
			continue
		}
		safe := true
		for _, part := range strings.Split(relative, "/") {
			if part == ".git" || part == ".." || part == "." || part == "" {
				safe = false
			}
		}
		if safe {
			latest[a.SourcePath] = a
		}
	}
	var out []types.MessageArtifact
	for _, a := range latest {
		if !a.Deleted() && a.URL != "" {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourcePath < out[j].SourcePath })
	return out
}

// RepairWorkspace also handles guests created by the old source-only recovery.
// The per-manifest marker avoids repeatedly scanning artifacts on panel polls.
// A successful repair never overwrites a file already present in the guest.
func (b *workspaceRecoveryBootstrapper) RepairWorkspace(ctx context.Context, key sandbox.SessionSandboxKey, handle sandbox.RemoteSandboxHandle) error {
	return b.repairWorkspace(ctx, key, handle, false)
}

func (b *workspaceRecoveryBootstrapper) repairWorkspace(ctx context.Context, key sandbox.SessionSandboxKey, handle sandbox.RemoteSandboxHandle, sourceRestored bool) error {
	if handle.Provider() != sandbox.SandboxTypeCube {
		return nil
	}
	artifacts, err := b.store.GetSessionArtifacts(ctx, key.SessionID)
	if err != nil {
		return fmt.Errorf("workspace recovery: list artifacts: %w", err)
	}
	var source *types.MessageArtifact
	for i := len(artifacts) - 1; i >= 0; i-- {
		if artifacts[i].FileName == "workspace-source.zip" && !artifacts[i].Deleted() {
			source = &artifacts[i]
			break
		}
	}
	files := recoverableWorkspaceArtifacts(artifacts)
	if len(files) == 0 && source == nil {
		return nil
	}
	if b.client == nil || b.files == nil {
		return fmt.Errorf("workspace recovery: artifact storage is unavailable")
	}
	hash := sha256.New()
	if source != nil {
		fmt.Fprintf(hash, "source:%s:%s\n", source.URL, source.ContentHash)
	}
	for _, a := range files {
		fmt.Fprintf(hash, "%s\x00%s\x00%s\n", a.SourcePath, a.URL, a.ContentHash)
	}
	marker := "/tmp/.weknora-artifact-recovery-" + hex.EncodeToString(hash.Sum(nil))
	if _, err := b.client.Stat(ctx, handle, marker); err == nil {
		return nil
	} else if !sandbox.IsRemoteNotFound(err) {
		return fmt.Errorf("workspace recovery: check marker: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if source != nil && !sourceRestored {
		if err := b.restoreSource(ctx, handle, *source, true); err != nil {
			return err
		}
	}
	for _, a := range files {
		if _, err := b.client.Stat(ctx, handle, a.SourcePath); err == nil {
			continue
		} else if !sandbox.IsRemoteNotFound(err) {
			return fmt.Errorf("workspace recovery: check artifact: %w", err)
		}
		if err := b.restoreArtifact(ctx, handle, a); err != nil {
			return err
		}
	}
	if err := b.client.WriteFile(ctx, handle, marker, []byte("complete\n")); err != nil {
		return fmt.Errorf("workspace recovery: record artifact repair: %w", err)
	}
	return nil
}

func (b *workspaceRecoveryBootstrapper) restoreArtifact(ctx context.Context, handle sandbox.RemoteSandboxHandle, a types.MessageArtifact) error {
	reader, err := b.files.GetFile(ctx, a.URL)
	if err != nil {
		return fmt.Errorf("workspace recovery: read artifact %s: %w", a.FileName, err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, defaultMaxArtifactFileBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > defaultMaxArtifactFileBytes {
		return fmt.Errorf("workspace recovery: artifact too large")
	}
	if a.ContentHash != "" {
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), a.ContentHash) {
			return fmt.Errorf("workspace recovery: artifact checksum mismatch")
		}
	}
	staged := "/tmp/weknora-artifact-recovery-" + uuid.NewString()
	if err := b.client.WriteFile(ctx, handle, staged, data); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = b.client.Remove(cleanup, handle, staged)
	}()
	command := "python3 -c " + sandbox.ShellQuote(workspaceRecoveryScript) + " --artifact " + sandbox.ShellQuote(staged) + " " + sandbox.ShellQuote(sandbox.SessionWorkspaceRoot) + " " + sandbox.ShellQuote(strings.TrimPrefix(a.SourcePath, sandbox.SessionWorkspaceRoot+"/"))
	result, err := b.client.Exec(ctx, handle, sandbox.RemoteExecRequest{Command: command, Shell: true, WorkDir: "/tmp", Timeout: 30 * time.Second, User: sandbox.DefaultSandboxExecUser})
	if err != nil {
		return err
	}
	if result == nil || result.ExitCode != 0 {
		return fmt.Errorf("workspace recovery: publish artifact %s failed", a.FileName)
	}
	return nil
}

var _ sandbox.SessionWorkspaceRepairer = (*workspaceRecoveryBootstrapper)(nil)
