package log_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/log"
)

func TestNew_JSONHandler(t *testing.T) {
	var buf bytes.Buffer
	logger, err := log.New("info", "json", &buf)
	require.NoError(t, err)
	logger.Info("hello", "k", "v")

	out := buf.String()
	require.Contains(t, out, `"msg":"hello"`)
	require.Contains(t, out, `"k":"v"`)
	require.Contains(t, out, `"level":"INFO"`)
}

func TestNew_TextHandler(t *testing.T) {
	var buf bytes.Buffer
	logger, err := log.New("info", "text", &buf)
	require.NoError(t, err)
	logger.Info("hello")

	out := buf.String()
	require.Contains(t, out, "msg=hello")
	require.NotContains(t, out, `"msg"`, "text handler must not produce JSON")
}

func TestNew_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger, err := log.New("warn", "json", &buf)
	require.NoError(t, err)
	logger.Debug("debug-msg")
	logger.Info("info-msg")
	logger.Warn("warn-msg")

	out := buf.String()
	require.NotContains(t, out, "debug-msg")
	require.NotContains(t, out, "info-msg")
	require.Contains(t, out, "warn-msg")
}

func TestNew_UnknownLevelErrors(t *testing.T) {
	var buf bytes.Buffer
	_, err := log.New("loud", "json", &buf)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "loud"))
}

func TestNew_UnknownFormatErrors(t *testing.T) {
	var buf bytes.Buffer
	_, err := log.New("info", "binary", &buf)
	require.Error(t, err)
}
