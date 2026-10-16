package handler

import (
	"strings"
	"testing"
)

func TestShellQuotePOSIX(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain message", "fix typo", `'fix typo'`},
		{"empty string", "", `''`},
		{"multi-line", "subject\n\nbody line", "'subject\n\nbody line'"},
		{"single quote closes and reopens", "it's done", `'it'\''s done'`},
		{"dollar and backtick stay literal", "run $HOME and `id`", "'run $HOME and `id`'"},
		{"double quotes need no escape", `say "hi"`, `'say "hi"'`},
		{"semicolon stays inside the argument", "a; rm -rf /", `'a; rm -rf /'`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellQuotePOSIX(tc.in); got != tc.want {
				t.Fatalf("shellQuotePOSIX(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBuildGitCommitCommands(t *testing.T) {
	add, commit := buildGitCommitCommands("feat: 支持手动提交\n\n详细说明")
	if add != "git add -A" {
		t.Fatalf("add = %q", add)
	}
	if !strings.HasPrefix(commit, "git commit -m '") || !strings.HasSuffix(commit, "'") {
		t.Fatalf("commit must be one quoted -m argument: %q", commit)
	}
	if !strings.Contains(commit, "feat: 支持手动提交\n\n详细说明") {
		t.Fatalf("multi-line message lost: %q", commit)
	}
}
