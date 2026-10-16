package handler

import (
	"testing"
)

func TestParseGitStatusPorcelainZ(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []gitStatusEntry
	}{
		{"clean repo is empty", "", nil},
		{"one modified", " M a.go\x00", []gitStatusEntry{{Path: "a.go", Status: "modified"}}},
		{"staged add", "A  c.md\x00", []gitStatusEntry{{Path: "c.md", Status: "added"}}},
		{"deleted", " D b.go\x00", []gitStatusEntry{{Path: "b.go", Status: "deleted"}}},
		{"untracked", "?? a.txt\x00?? dir/file with space.go\x00",
			[]gitStatusEntry{{Path: "a.txt", Status: "untracked"}, {Path: "dir/file with space.go", Status: "untracked"}}},
		{"renamed carries the old path", "R  new.go\x00old.go\x00",
			[]gitStatusEntry{{Path: "new.go", OldPath: "old.go", Status: "renamed"}}},
		{"rename then delete consumes both paths", "RD new.go\x00old.go\x00 M next.go\x00",
			[]gitStatusEntry{{Path: "new.go", OldPath: "old.go", Status: "deleted"}, {Path: "next.go", Status: "modified"}}},
		{"copy then delete consumes both paths", "CD copy.go\x00old.go\x00 M next.go\x00",
			[]gitStatusEntry{{Path: "copy.go", OldPath: "old.go", Status: "deleted"}, {Path: "next.go", Status: "modified"}}},
		{"trailing separator is ignored", " M a.go\x00\x00", []gitStatusEntry{{Path: "a.go", Status: "modified"}}},
		{"unknown record is kept verbatim", "XY odd\x00", []gitStatusEntry{{Path: "odd", Status: "XY"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseGitStatusPorcelainZ(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d: %#v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Fatalf("entry %d = %#v, want %#v", i, got[i], w)
				}
			}
		})
	}
}

func TestParseGitNumstat(t *testing.T) {
	// Real `git diff --numstat -z` bytes: counts are tab-separated, each path
	// is NUL-terminated, and a rename carries an empty path followed by the
	// source and destination as their own records.
	raw := "5\t2\ta.go\x00-\t-\tb.bin\x00" +
		"1\t0\t\x00文件 with space.go\x00中文名.go\x00" +
		"7\t1\tc.go\x00"
	got := parseGitNumstat(raw)
	if len(got) != 5 {
		t.Fatalf("len = %d: %#v", len(got), got)
	}
	if got["a.go"] != (gitNumstatEntry{Added: 5, Removed: 2}) {
		t.Fatalf("a.go = %#v", got["a.go"])
	}
	if got["b.bin"] != (gitNumstatEntry{Added: 0, Removed: 0}) {
		t.Fatalf("binary must count as 0/0: %#v", got["b.bin"])
	}
	if got["c.go"] != (gitNumstatEntry{Added: 7, Removed: 1}) {
		t.Fatalf("entry after a rename must still parse: %#v", got["c.go"])
	}
	// Porcelain status reports the rename destination as the path, so both
	// sides must find the counts — and non-ASCII names must arrive unquoted.
	want := gitNumstatEntry{Added: 1, Removed: 0}
	if got["中文名.go"] != want || got["文件 with space.go"] != want {
		t.Fatalf("rename sides = %#v / %#v", got["中文名.go"], got["文件 with space.go"])
	}
}

func TestParseGitNumstatUntrackedNoIndex(t *testing.T) {
	// `git diff --no-index --numstat -z /dev/null f` reports the source as
	// /dev/null. It is not a repo path and must never become a key, or every
	// untracked file would overwrite the same entry.
	got := parseGitNumstat("4\t0\t\x00/dev/null\x00new.go\x00-\t-\t\x00/dev/null\x00img.png\x00")
	if _, ok := got["/dev/null"]; ok {
		t.Fatalf("/dev/null leaked into the map: %#v", got)
	}
	if got["new.go"] != (gitNumstatEntry{Added: 4}) {
		t.Fatalf("new.go = %#v", got["new.go"])
	}
	if _, ok := got["img.png"]; !ok {
		t.Fatalf("binary untracked file missing: %#v", got)
	}
}

func TestParseGitNumstatIgnoresGarbage(t *testing.T) {
	if got := parseGitNumstat(""); len(got) != 0 {
		t.Fatalf("empty input: %#v", got)
	}
	// A truncated rename (counts but no paths) must not panic or invent keys.
	if got := parseGitNumstat("1\t0\t\x00only-source.go"); len(got) != 0 {
		t.Fatalf("truncated rename: %#v", got)
	}
}

func TestBuildGitStatusReport(t *testing.T) {
	report := buildGitStatusReport(
		" M a.go\x00?? b.go\x00R  new.go\x00old.go\x00",
		"3\t1\ta.go\x007\t0\t\x00/dev/null\x00b.go\x00",
	)
	if !report.Available {
		t.Fatal("report must be available")
	}
	if len(report.Files) != 3 {
		t.Fatalf("files = %v", report.Files)
	}
	by := map[string]gitStatusEntry{}
	for _, f := range report.Files {
		by[f.Path] = f
	}
	if by["a.go"].Added != 3 || by["a.go"].Removed != 1 {
		t.Fatalf("numstat not joined: %#v", by["a.go"])
	}
	if by["b.go"].Status != "untracked" || by["b.go"].Added != 7 {
		t.Fatalf("untracked must still carry its line count: %#v", by["b.go"])
	}
	rename, ok := by["new.go"]
	if !ok {
		t.Fatalf("rename entry missing, files = %v", report.Files)
	}
	if rename.Status != "renamed" || rename.OldPath != "old.go" {
		t.Fatalf("rename: %#v", rename)
	}
}

func TestBuildGitStatusReportUntrackedFallback(t *testing.T) {
	// Missing numstat output must still leave the status entries in place.
	report := buildGitStatusReport("?? solo.go\x00", "")
	if len(report.Files) != 1 || report.Files[0].Status != "untracked" {
		t.Fatalf("files = %v", report.Files)
	}
	if report.Files[0].Path != "solo.go" {
		t.Fatalf("path lost: %#v", report.Files[0])
	}
}
