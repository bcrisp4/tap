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
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("log: %w", err)
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
