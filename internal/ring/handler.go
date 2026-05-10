package ring

import (
	"context"
	"log/slog"
)

type handler struct {
	next slog.Handler
	buf  *Buffer
}

// NewHandler returns a slog.Handler that forwards every record to next and
// additionally adds warn/error records to buf.
func NewHandler(next slog.Handler, buf *Buffer) slog.Handler {
	return &handler{next: next, buf: buf}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	// Always enable warn/error so the ring buffer captures them even when
	// the wrapped next handler has a higher minimum level (e.g. DiscardHandler).
	if level >= slog.LevelWarn {
		return true
	}
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		e := Event{
			Time:  r.Time,
			Level: r.Level.String(),
			Attrs: make(map[string]any),
		}
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "event" {
				e.Event = a.Value.String()
			} else {
				e.Attrs[a.Key] = a.Value.Any()
			}
			return true
		})
		if e.Event == "" {
			e.Event = r.Message
		}
		h.buf.Add(e)
	}
	return h.next.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{next: h.next.WithAttrs(attrs), buf: h.buf}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{next: h.next.WithGroup(name), buf: h.buf}
}
