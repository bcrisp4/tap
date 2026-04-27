package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_VersionFlag(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"tap", "--version"}, &out)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := out.String()
	if !strings.HasPrefix(got, "tap ") || !strings.HasSuffix(got, "\n") {
		t.Fatalf("stdout = %q, want %q", got, "tap <version>\n")
	}
	if !strings.Contains(got, "0.0.0-dev") {
		t.Fatalf("stdout = %q, want it to contain %q", got, "0.0.0-dev")
	}
}

func TestRun_ShortVersionFlag(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"tap", "-v"}, &out)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "0.0.0-dev") {
		t.Fatalf("stdout = %q, want it to contain %q", out.String(), "0.0.0-dev")
	}
}

func TestRun_NoArgs(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"tap"}, &out)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}
