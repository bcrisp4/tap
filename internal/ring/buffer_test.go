package ring_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/ring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuffer_RingSemantics(t *testing.T) {
	buf := ring.NewBuffer(3)
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "a"})
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "b"})
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "c"})
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "d"}) // overwrites "a"

	got := buf.Recent(10)
	require.Len(t, got, 3)
	// newest first
	assert.Equal(t, "d", got[0].Event)
	assert.Equal(t, "c", got[1].Event)
	assert.Equal(t, "b", got[2].Event)
}

func TestBuffer_RecentCapped(t *testing.T) {
	buf := ring.NewBuffer(10)
	buf.Add(ring.Event{Time: time.Now(), Level: "error", Event: "x"})
	buf.Add(ring.Event{Time: time.Now(), Level: "error", Event: "y"})

	got := buf.Recent(1)
	require.Len(t, got, 1)
	assert.Equal(t, "y", got[0].Event)
}

func TestBuffer_RecentZero(t *testing.T) {
	buf := ring.NewBuffer(10)
	got := buf.Recent(0)
	assert.Empty(t, got)
}

func TestBuffer_Concurrent(t *testing.T) {
	buf := ring.NewBuffer(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "concurrent"})
			_ = buf.Recent(5)
		}()
	}
	wg.Wait()
}

func TestHandler_WarnAndErrorFeedBuffer(t *testing.T) {
	buf := ring.NewBuffer(10)
	h := ring.NewHandler(slog.DiscardHandler, buf)
	logger := slog.New(h)

	logger.Info("this is info", "event", "info.event")
	logger.Warn("this is warn", "event", "warn.event")
	logger.Error("this is error", "event", "error.event")
	logger.Debug("this is debug", "event", "debug.event")

	got := buf.Recent(10)
	require.Len(t, got, 2) // only warn + error
	assert.Equal(t, "error.event", got[0].Event) // newest first
	assert.Equal(t, "warn.event", got[1].Event)
}

func TestHandler_EventKeyExtracted(t *testing.T) {
	buf := ring.NewBuffer(10)
	h := ring.NewHandler(slog.DiscardHandler, buf)
	slog.New(h).Warn("poll failed", "event", "poll.failure", "feed_id", 42)

	got := buf.Recent(1)
	require.Len(t, got, 1)
	assert.Equal(t, "poll.failure", got[0].Event)
	assert.Equal(t, int64(42), got[0].Attrs["feed_id"])
}

func TestHandler_NextHandlerReceivesAllRecords(t *testing.T) {
	buf := ring.NewBuffer(10)
	var count int
	counting := &countingHandler{&count}
	h := ring.NewHandler(counting, buf)
	logger := slog.New(h)

	logger.Debug("d")
	logger.Info("i")
	logger.Warn("w")
	logger.Error("e")

	assert.Equal(t, 4, count) // all records forwarded
	assert.Len(t, buf.Recent(10), 2) // only warn+error buffered
}

type countingHandler struct{ n *int }

func (c *countingHandler) Enabled(_ context.Context, _ slog.Level) bool  { return true }
func (c *countingHandler) Handle(_ context.Context, _ slog.Record) error  { *c.n++; return nil }
func (c *countingHandler) WithAttrs(_ []slog.Attr) slog.Handler           { return c }
func (c *countingHandler) WithGroup(_ string) slog.Handler                { return c }
