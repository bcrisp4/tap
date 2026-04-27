// Package log builds a *slog.Logger from a level + format pair.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// New returns a *slog.Logger with the given minimum level and format.
// level: "debug" | "info" | "warn" | "error".
// format: "json" | "text".
func New(level, format string, w io.Writer) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	switch strings.ToLower(format) {
	case "json":
		h = slog.NewJSONHandler(w, opts)
	case "text":
		h = slog.NewTextHandler(w, opts)
	default:
		return nil, fmt.Errorf("log: unknown format %q (want json|text)", format)
	}
	return slog.New(h), nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log: unknown level %q (want debug|info|warn|error)", s)
	}
}
