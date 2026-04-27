package main

import (
	"bytes"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantOutput string
	}{
		{"version long", []string{"tap", "--version"}, "tap 0.0.0-dev\n"},
		{"version short", []string{"tap", "-v"}, "tap 0.0.0-dev\n"},
		{"no args", []string{"tap"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if code := run(tt.args, &out); code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if got := out.String(); got != tt.wantOutput {
				t.Fatalf("stdout = %q, want %q", got, tt.wantOutput)
			}
		})
	}
}
