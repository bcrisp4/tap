package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_VersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"tap", "--version"}, &stdout, &stderr)
	require.Equal(t, 0, code)
	require.True(t, strings.HasPrefix(stdout.String(), "tap "))
	require.Contains(t, stdout.String(), "0.0.0-dev")
}

func TestRun_NoArgs_PrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"tap"}, &stdout, &stderr)
	require.Equal(t, 0, code)
	// Help mentions the serve subcommand.
	require.Contains(t, stderr.String(), "serve")
}

func TestRun_UnknownSubcommand_Errors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"tap", "bogus"}, &stdout, &stderr)
	require.NotEqual(t, 0, code)
}
