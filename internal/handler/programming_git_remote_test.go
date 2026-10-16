package handler

import "testing"

func TestParseGitAheadBehind(t *testing.T) {
	cases := []struct {
		name          string
		in            string
		ahead, behind int
	}{
		{"both sides", "2\t3\n", 2, 3},
		{"only behind", "0\t5\n", 0, 5},
		{"clean", "0\t0\n", 0, 0},
		{"no trailing newline", "1\t4", 1, 4},
		{"malformed line is zero", "garbage", 0, 0},
		{"empty is zero", "", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ahead, behind := parseGitAheadBehind(tc.in)
			if ahead != tc.ahead || behind != tc.behind {
				t.Fatalf("parseGitAheadBehind(%q) = %d/%d, want %d/%d", tc.in, ahead, behind, tc.ahead, tc.behind)
			}
		})
	}
}

func TestParseGitIncoming(t *testing.T) {
	got := parseGitIncoming("a1b2c3d\tfix: subject with spaces\n9f8e7d6\tfeat: 支持远程显示\n")
	if len(got) != 2 {
		t.Fatalf("len = %d: %#v", len(got), got)
	}
	if got[0].SHA != "a1b2c3d" || got[0].Subject != "fix: subject with spaces" {
		t.Fatalf("first entry: %#v", got[0])
	}
	if got[1].Subject != "feat: 支持远程显示" {
		t.Fatalf("multi-byte subject: %#v", got[1])
	}
	if empty := parseGitIncoming(""); len(empty) != 0 {
		t.Fatalf("empty input must yield no entries: %#v", empty)
	}
}
