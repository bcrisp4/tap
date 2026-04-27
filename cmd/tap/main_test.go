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

// healthcheckProbe is set in main_test.go to short-circuit the HTTP
// call so we can test the healthcheck subcommand's exit code routing
// without standing up a real server.
func TestRun_Healthcheck_ServerHealthy_ExitsZero(t *testing.T) {
	t.Cleanup(func() { healthcheckProbe = defaultHealthcheckProbe })
	healthcheckProbe = func(_ context.Context, _ string) error { return nil }

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"tap", "healthcheck"}, &stdout, &stderr)
	require.Equal(t, 0, code)
}

func TestRun_Healthcheck_ServerUnreachable_ExitsNonZero(t *testing.T) {
	t.Cleanup(func() { healthcheckProbe = defaultHealthcheckProbe })
	healthcheckProbe = func(_ context.Context, _ string) error {
		return errSyntheticHealthcheckFailure
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"tap", "healthcheck"}, &stdout, &stderr)
	require.NotEqual(t, 0, code)
	require.Contains(t, stderr.String(), "healthcheck")
}

var errSyntheticHealthcheckFailure = errSentinel("synthetic")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
