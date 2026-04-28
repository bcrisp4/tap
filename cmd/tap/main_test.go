package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

// Stub the package-level healthcheckProbe so the subcommand's
// exit-code routing can be exercised without a real HTTP server.
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
		return errors.New("synthetic")
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"tap", "healthcheck"}, &stdout, &stderr)
	require.NotEqual(t, 0, code)
	require.Contains(t, stderr.String(), "healthcheck")
}

func TestDefaultHealthcheckProbe_2xx_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	require.NoError(t, defaultHealthcheckProbe(context.Background(), srv.URL))
}

func TestDefaultHealthcheckProbe_Non2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	err := defaultHealthcheckProbe(context.Background(), srv.URL)
	require.Error(t, err)
	require.Contains(t, err.Error(), "503")
}

func TestDefaultHealthcheckProbe_BadURL_ReturnsError(t *testing.T) {
	// URL with a control character forces http.NewRequestWithContext
	// to fail before any network call happens.
	require.Error(t, defaultHealthcheckProbe(context.Background(), "http://\x7f/"))
}

func TestDefaultHealthcheckProbe_Unreachable_ReturnsError(t *testing.T) {
	// 127.0.0.1:1 has no listener; Dial will refuse / time out.
	require.Error(t, defaultHealthcheckProbe(context.Background(), "http://127.0.0.1:1/healthz"))
}
