package handler

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestCandidateGitRepoDirs(t *testing.T) {
	cases := []struct {
		name string
		root string
		dirs []string
		want []string
	}{
		{"empty workspace", "/workspace", nil, []string{""}},
		{"root only has files", "/workspace", []string{}, []string{""}},
		{
			"first-level subdirs sorted, root first",
			"/workspace",
			[]string{"/workspace/lib", "/workspace/app"},
			[]string{"", "app", "lib"},
		},
		{
			"nested paths are dropped",
			"/workspace",
			[]string{"/workspace/app", "/workspace/app/nested"},
			[]string{"", "app"},
		},
		{
			"trailing slash on root is tolerated",
			"/workspace/",
			[]string{"/workspace/test-work"},
			[]string{"", "test-work"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := candidateGitRepoDirs(tc.root, tc.dirs)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("candidateGitRepoDirs(%q, %v) = %v, want %v", tc.root, tc.dirs, got, tc.want)
			}
		})
	}
}

func TestGitWorkdir(t *testing.T) {
	if got := gitWorkdir("/workspace", ""); got != "/workspace" {
		t.Fatalf("root repo: %q", got)
	}
	if got := gitWorkdir("/workspace", "test-work"); got != "/workspace/test-work" {
		t.Fatalf("subdir repo: %q", got)
	}
	if got := gitWorkdir("/workspace/", "test-work"); got != "/workspace/test-work" {
		t.Fatalf("trailing slash: %q", got)
	}
}

func TestGitRepoName(t *testing.T) {
	if got := gitRepoName("/workspace", "test-work"); got != "test-work" {
		t.Fatalf("subdir repo name: %q", got)
	}
	if got := gitRepoName("/workspace", ""); got != "workspace" {
		t.Fatalf("root repo name should be workspace dir name: %q", got)
	}
}

func TestSameFilesystemPath(t *testing.T) {
	if !sameFilesystemPath("/workspace", "/workspace/") {
		t.Fatal("trailing slash should still match")
	}
	if sameFilesystemPath("/workspace/backend", "/workspace") {
		t.Fatal("a subdirectory is not the same as its parent")
	}
	// Symlink resolution: /tmp on macOS is /private/tmp. Both forms must match
	// or every repo on such a host would be rejected as not-its-own-root. Skip
	// when this platform does not symlink /tmp.
	if resolved, err := filepath.EvalSymlinks("/tmp"); err == nil && resolved != "/tmp" {
		if !sameFilesystemPath("/tmp", resolved) {
			t.Fatalf("symlinked /tmp should match its resolved form %q", resolved)
		}
	}
}
