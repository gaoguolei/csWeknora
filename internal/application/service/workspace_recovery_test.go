package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type recoveryArtifacts struct {
	artifacts types.MessageArtifacts
	err       error
	calls     int
}

func (s *recoveryArtifacts) GetSessionArtifacts(context.Context, string) (types.MessageArtifacts, error) {
	s.calls++
	return s.artifacts, s.err
}

type recoveryFiles struct {
	contents map[string][]byte
	data     []byte
	urls     []string
	err      error
}

func (f *recoveryFiles) GetFile(_ context.Context, url string) (io.ReadCloser, error) {
	f.urls = append(f.urls, url)
	data := f.data
	if f.contents != nil {
		data = f.contents[url]
	}
	return io.NopCloser(bytes.NewReader(data)), f.err
}

type recoveryClient struct {
	recordingRemoteClient
	staged   []byte
	removed  []string
	writeErr error
}

func (c *recoveryClient) WriteFile(_ context.Context, _ sandbox.RemoteSandboxHandle, path string, data []byte) error {
	if strings.Contains(path, ".weknora-artifact-recovery-") {
		return nil
	}
	c.staged = data
	return c.writeErr
}
func (c *recoveryClient) Stat(context.Context, sandbox.RemoteSandboxHandle, string) (*sandbox.RemoteStatEntry, error) {
	return nil, sandbox.NewRemoteError(sandbox.SandboxTypeCube, "Stat", sandbox.RemoteErrorKindNotFound, "missing", nil)
}
func (c *recoveryClient) Remove(_ context.Context, _ sandbox.RemoteSandboxHandle, path string) error {
	c.removed = append(c.removed, path)
	return nil
}
func recoveryFixture() (*workspaceRecoveryBootstrapper, *recoveryArtifacts, *recoveryFiles, *recoveryClient) {
	store := &recoveryArtifacts{}
	files := &recoveryFiles{data: []byte("archive")}
	client := &recoveryClient{}
	fork := NewForkBootstrapper(newFakeSessionStore(&types.Session{ID: "fork-1", TenantID: 1}), nil, nil, nil)
	b := &workspaceRecoveryBootstrapper{fork: fork, store: store, files: files, client: client}
	return b, store, files, client
}
func TestWorkspaceRecoveryRestoresLatestBackup(t *testing.T) {
	b, store, files, client := recoveryFixture()
	hash := sha256.Sum256(files.data)
	store.artifacts = types.MessageArtifacts{
		{FileName: "workspace-source.zip", URL: "old"},
		{FileName: "workspace-source.zip", URL: "latest", ContentHash: hex.EncodeToString(hash[:])},
		{FileName: "other.zip", URL: "unrelated"},
	}
	require.NoError(t, b.AfterCreate(context.Background(), forkKey(), fakeHandle{id: "new"}))
	require.Equal(t, []string{"latest"}, files.urls)
	require.Equal(t, files.data, client.staged)
	require.Len(t, client.execs, 1)
	require.Contains(t, client.execs[0].Command, "python3 -c")
	require.Len(t, client.removed, 1)
}
func TestWorkspaceRecoveryNewConversationDoesNotTouchFiles(t *testing.T) {
	b, _, files, client := recoveryFixture()
	require.NoError(t, b.AfterCreate(context.Background(), forkKey(), fakeHandle{id: "new"}))
	require.Empty(t, files.urls)
	require.Empty(t, client.execs)
}
func TestWorkspaceRecoveryFailuresNeverFallBackToOlderSource(t *testing.T) {
	for _, kind := range []string{"list", "storage", "checksum", "write", "extract"} {
		t.Run(kind, func(t *testing.T) {
			b, store, files, client := recoveryFixture()
			store.artifacts = types.MessageArtifacts{{FileName: "workspace-source.zip", URL: "old"}, {FileName: "workspace-source.zip", URL: "new"}}
			switch kind {
			case "list":
				store.err = errors.New("database offline")
			case "storage":
				files.err = errors.New("storage offline")
			case "checksum":
				store.artifacts[1].ContentHash = "incorrect"
			case "write":
				client.writeErr = errors.New("write failed")
			case "extract":
				client.execResult = &sandbox.RemoteExecResult{ExitCode: 1, Stderr: "bad zip"}
			}
			require.Error(t, b.AfterCreate(context.Background(), forkKey(), fakeHandle{id: "new"}))
			require.NotContains(t, files.urls, "old")
		})
	}
}
func TestWorkspaceRecoveryKeepsForkSnapshotAuthoritative(t *testing.T) {
	b, store, _, client := recoveryFixture()
	fork := NewForkBootstrapper(newFakeSessionStore(pendingForkSession()), nil, nil, &fakeSnapshotDeleter{})
	b.fork = fork
	bound := b.WithClient(client)
	require.NoError(t, bound.AfterCreate(context.Background(), forkKey(), fakeHandle{id: "new"}))
	require.Zero(t, store.calls)
	require.Len(t, client.execs, 1)
	require.Contains(t, client.execs[0].Command, "reset --hard")
}

func zipRecoveryEntry(t *testing.T, name string, mode os.FileMode) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	file, err := writer.CreateHeader(header)
	require.NoError(t, err)
	_, err = file.Write([]byte("source content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}
func TestWorkspaceRecoveryScriptRestoresSourceAndRejectsUnsafeArchives(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for recovery script tests")
	}
	for _, tc := range []struct {
		name  string
		mode  os.FileMode
		valid bool
	}{
		{"src/app.sh", 0755, true}, {"../escape", 0644, false},
		{"/absolute", 0644, false}, {".git/config", 0644, false},
		{"link", os.ModeSymlink | 0777, false}, {"back\\slash", 0644, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			archive := filepath.Join(dir, "source.zip")
			root := filepath.Join(dir, "workspace")
			require.NoError(t, os.WriteFile(archive, zipRecoveryEntry(t, tc.name, tc.mode), 0600))
			output, err := exec.Command(python, "-c", workspaceRecoveryScript, archive, root).CombinedOutput()
			if !tc.valid {
				require.Error(t, err, string(output))
				return
			}
			require.NoError(t, err, string(output))
			data, err := os.ReadFile(filepath.Join(root, tc.name))
			require.NoError(t, err)
			require.Equal(t, "source content", string(data))
			info, err := os.Stat(filepath.Join(root, tc.name))
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0755), info.Mode().Perm())
		})
	}
}

func TestWorkspaceRecoveryBackupRoundTrip(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for recovery script tests")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	root := filepath.Join(dir, "original")
	output := filepath.Join(root, "output")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "app.ts"), []byte("export const answer = 42\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("LOCAL_SETTING=value\n"), 0600))
	args := fmt.Sprintf(`{"root":%q,"output":%q,"max_bytes":1048576,"turn":"turn-1"}`, root, output)
	result, err := exec.Command(python, "-c", workspaceBackupScript, args).CombinedOutput()
	require.NoError(t, err, string(result))
	restored := filepath.Join(dir, "replacement")
	result, err = exec.Command(python, "-c", workspaceRecoveryScript, filepath.Join(output, "workspace-source.zip"), restored).CombinedOutput()
	require.NoError(t, err, string(result))
	for _, name := range []string{"src/app.ts", ".env"} {
		before, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		after, err := os.ReadFile(filepath.Join(restored, name))
		require.NoError(t, err)
		require.Equal(t, before, after)
	}
}

func TestWorkspaceRecoveryRejectsDestinationSymlinks(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for recovery script tests")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	root := filepath.Join(dir, "workspace")
	outside := filepath.Join(dir, "outside")
	require.NoError(t, os.MkdirAll(root, 0755))
	require.NoError(t, os.MkdirAll(outside, 0755))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "src")))
	archive := filepath.Join(dir, "source.zip")
	require.NoError(t, os.WriteFile(archive, zipRecoveryEntry(t, "src/app.ts", 0644), 0600))
	result, err := exec.Command(python, "-c", workspaceRecoveryScript, archive, root).CombinedOutput()
	require.Error(t, err, string(result))
	_, err = os.Stat(filepath.Join(outside, "app.ts"))
	require.True(t, os.IsNotExist(err))
}

type recoveryOtherHandle struct{ fakeHandle }

func (recoveryOtherHandle) Provider() sandbox.RemoteProvider { return sandbox.SandboxTypeE2B }

func TestWorkspaceRecoveryDoesNotChangeOtherProviders(t *testing.T) {
	b, store, files, client := recoveryFixture()
	require.NoError(t, b.AfterCreate(context.Background(), forkKey(), recoveryOtherHandle{}))
	require.Zero(t, store.calls)
	require.Empty(t, files.urls)
	require.Empty(t, client.execs)
}

func TestWorkspaceRecoveryForwardsForkCreateFailures(t *testing.T) {
	sessions := newFakeSessionStore(pendingForkSession())
	snapshots := &fakeSnapshotDeleter{}
	b := &workspaceRecoveryBootstrapper{fork: NewForkBootstrapper(sessions, nil, nil, snapshots)}
	override, err := b.TemplateOverride(context.Background(), forkKey())
	require.NoError(t, err)
	require.Equal(t, "snap-1", override)
	b.OnCreateFailed(context.Background(), forkKey(), sandbox.NewRemoteError(sandbox.SandboxTypeCube, "Create", sandbox.RemoteErrorKindNotFound, "snapshot gone", nil))
	require.Nil(t, sessions.updatedBootstrap)
	require.Equal(t, []string{"snap-1"}, snapshots.deleted)
}

// Exercise the actual panel accessor, lifecycle, backup download and Python
// extraction together. The fake provider's disk is a temporary local directory.
type recoveryDiskClient struct {
	recordingRemoteClient
	root, guest, archive, remoteArchive string
	creates                             int
}

func (c *recoveryDiskClient) Capabilities() sandbox.RemoteSandboxCapabilities {
	return sandbox.RemoteSandboxCapabilities{SupportsReconnect: true, SupportsFilesystemEnumeration: true}
}
func (c *recoveryDiskClient) Create(context.Context, sandbox.RemoteCreateRequest) (sandbox.RemoteSandboxHandle, error) {
	c.creates++
	c.guest = fmt.Sprintf("cube-%d", c.creates)
	return fakeHandle{id: c.guest}, nil
}
func (c *recoveryDiskClient) Connect(_ context.Context, req sandbox.RemoteConnectRequest) (sandbox.RemoteSandboxHandle, error) {
	if req.SandboxID != c.guest || c.guest == "" {
		return nil, sandbox.NewRemoteError(sandbox.SandboxTypeCube, "Connect", sandbox.RemoteErrorKindNotFound, "guest deleted", nil)
	}
	return fakeHandle{id: c.guest}, nil
}
func (c *recoveryDiskClient) Get(_ context.Context, id string) (*sandbox.RemoteSandboxSummary, error) {
	if id != c.guest {
		return nil, sandbox.NewRemoteError(sandbox.SandboxTypeCube, "Get", sandbox.RemoteErrorKindNotFound, "guest deleted", nil)
	}
	return &sandbox.RemoteSandboxSummary{ID: id, State: sandbox.RemoteStateRunning}, nil
}
func (c *recoveryDiskClient) Delete(context.Context, string) error {
	c.guest = ""
	return os.RemoveAll(c.root)
}
func (c *recoveryDiskClient) diskPath(remote string) string {
	if strings.HasPrefix(remote, sandbox.SessionWorkspaceRoot+"/") {
		return filepath.Join(c.root, strings.TrimPrefix(remote, sandbox.SessionWorkspaceRoot+"/"))
	}
	return filepath.Join(filepath.Dir(c.root), "tmp-"+c.guest, filepath.Base(remote))
}
func (c *recoveryDiskClient) Stat(_ context.Context, _ sandbox.RemoteSandboxHandle, remote string) (*sandbox.RemoteStatEntry, error) {
	info, err := os.Stat(c.diskPath(remote))
	if os.IsNotExist(err) {
		return nil, sandbox.NewRemoteError(sandbox.SandboxTypeCube, "Stat", sandbox.RemoteErrorKindNotFound, "missing file", err)
	}
	if err != nil {
		return nil, err
	}
	return &sandbox.RemoteStatEntry{Path: remote, Type: sandbox.RemoteEntryFile, Size: info.Size(), ModTime: info.ModTime()}, nil
}
func (c *recoveryDiskClient) WriteFile(_ context.Context, _ sandbox.RemoteSandboxHandle, remote string, data []byte) error {
	c.remoteArchive = remote
	local := c.diskPath(remote)
	if err := os.MkdirAll(filepath.Dir(local), 0755); err != nil {
		return err
	}
	return os.WriteFile(local, data, 0600)
}
func (c *recoveryDiskClient) Remove(_ context.Context, _ sandbox.RemoteSandboxHandle, remote string) error {
	return os.Remove(c.diskPath(remote))
}
func (c *recoveryDiskClient) Exec(ctx context.Context, _ sandbox.RemoteSandboxHandle, req sandbox.RemoteExecRequest) (*sandbox.RemoteExecResult, error) {
	command := strings.ReplaceAll(req.Command, sandbox.ShellQuote(c.remoteArchive), sandbox.ShellQuote(c.diskPath(c.remoteArchive)))
	command = strings.ReplaceAll(command, sandbox.ShellQuote(sandbox.SessionWorkspaceRoot), sandbox.ShellQuote(c.root))
	output, err := exec.CommandContext(ctx, "/bin/sh", "-c", command).CombinedOutput()
	if err != nil {
		return &sandbox.RemoteExecResult{ExitCode: 1, Stderr: string(output)}, nil
	}
	return &sandbox.RemoteExecResult{}, nil
}
func (c *recoveryDiskClient) ReadFile(_ context.Context, _ sandbox.RemoteSandboxHandle, path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(c.root, strings.TrimPrefix(path, sandbox.SessionWorkspaceRoot+"/")))
}
func TestWorkspaceRecoveryPanelReadsSourceAfterCubeDeletion(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	client := &recoveryDiskClient{root: filepath.Join(dir, "guest"), archive: filepath.Join(dir, "restore.zip")}
	b, store, files, _ := recoveryFixture()
	store.artifacts = types.MessageArtifacts{{FileName: "workspace-source.zip", URL: "durable-backup"}}
	files.data = zipRecoveryEntry(t, "src/app.ts", 0644)
	cfg := sandbox.DefaultConfig()
	cfg.CubeTemplate = "template"
	mgr, err := sandbox.NewSessionBoundManager(sandbox.SessionBoundManagerConfig{
		Config: cfg, Client: client, Store: sandbox.NewMemorySessionSandboxBindingStore(),
		Checker: sandbox.PermissiveSessionExistenceChecker{}, Bootstrapper: b, SkipHealthProbe: true,
	})
	require.NoError(t, err)
	access := NewPinnedSessionSandbox(stubPinReader{configID: "cfg-1"}, &stubTenantSandboxResolver{mgr: mgr}, nil, nil)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	panelFiles, err := access.SessionFileStore(ctx, "fork-1")
	require.NoError(t, err)
	data, err := panelFiles.ReadSessionFile(ctx, "fork-1", "/workspace/src/app.ts")
	require.NoError(t, err)
	require.Equal(t, "source content", string(data))
	require.NoError(t, client.Delete(ctx, client.guest))
	_, err = panelFiles.ReadSessionFile(ctx, "fork-1", "/workspace/src/app.ts")
	require.Error(t, err, "non-provisioning reads alone cannot recover the guest")
	panelFiles, err = access.SessionFileStore(ctx, "fork-1")
	require.NoError(t, err)
	data, err = panelFiles.ReadSessionFile(ctx, "fork-1", "/workspace/src/app.ts")
	require.NoError(t, err)
	require.Equal(t, "source content", string(data))
	require.Equal(t, 2, client.creates)
}

func TestWorkspaceRecoveryRestoresOutputOmittedFromSourceArchive(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	client := &recoveryDiskClient{root: filepath.Join(dir, "guest"), guest: "existing-source-only"}
	require.NoError(t, os.MkdirAll(client.root, 0755))
	b, store, files, _ := recoveryFixture()
	sourceZip := zipRecoveryEntry(t, "tests/probe.js", 0644)
	files.contents = map[string][]byte{"source": sourceZip, "html": []byte("<html>recovered game</html>")}
	store.artifacts = types.MessageArtifacts{
		{FileName: "workspace-source.zip", URL: "source"},
		{FileName: "crow-plane.html", SourcePath: "/workspace/output/crow-plane.html", URL: "html"},
	}
	bound := b.WithClient(client).(*workspaceRecoveryBootstrapper)
	key := forkKey()
	ctx := context.Background()
	handle := fakeHandle{id: client.guest}
	// The guest already exists but the previous version did not restore its files.
	require.NoError(t, bound.RepairWorkspace(ctx, key, handle))
	_, err = os.Stat(filepath.Join(client.root, "tests", "probe.js"))
	require.NoError(t, err)
	target := filepath.Join(client.root, "output", "crow-plane.html")
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "<html>recovered game</html>", string(data))
	// Reopening and a new manifest must both preserve local changes.
	require.NoError(t, os.WriteFile(target, []byte("local edit"), 0644))
	require.NoError(t, bound.RepairWorkspace(ctx, key, handle))
	store.artifacts[1].URL = "new-html"
	files.contents["new-html"] = []byte("new artifact")
	require.NoError(t, bound.RepairWorkspace(ctx, key, handle))
	data, err = os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "local edit", string(data))
	// A replacement guest gets BOTH the source archive and output artifacts.
	require.NoError(t, client.Delete(ctx, client.guest))
	client.guest = "replacement"
	require.NoError(t, bound.AfterCreate(ctx, key, fakeHandle{id: client.guest}))
	data, err = os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "new artifact", string(data))
	_, err = os.Stat(filepath.Join(client.root, "tests", "probe.js"))
	require.NoError(t, err)
}

func TestWorkspaceRecoveryOutputOnlySessionIsRecoverable(t *testing.T) {
	b, store, _, _ := recoveryFixture()
	store.artifacts = types.MessageArtifacts{{FileName: "index.html", SourcePath: "/workspace/output/index.html", URL: "durable-html"}}
	available, err := b.CanRecoverWorkspace(context.Background(), forkKey())
	require.NoError(t, err)
	require.True(t, available)
	at := time.Now()
	store.artifacts = append(store.artifacts, types.MessageArtifact{FileName: "index.html", SourcePath: "/workspace/output/index.html", URL: "deleted", DeletedAt: &at})
	available, err = b.CanRecoverWorkspace(context.Background(), forkKey())
	require.NoError(t, err)
	require.False(t, available)
}
