package poll

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func TestSchedulerOpts_DefaultsApplied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	s := NewScheduler(ctx, d, http.DefaultClient, SchedulerOpts{})
	require.NotZero(t, s.opts.Floor, "Floor should default to 15m")
	require.NotZero(t, s.opts.Ceiling, "Ceiling should default to 24h")
	require.NotZero(t, s.opts.ErrorBase, "ErrorBase should default to 5m")
	require.NotNil(t, s.opts.Now, "Now should default to time.Now")
	require.NotNil(t, s.opts.Rand, "Rand should default to a fresh rand.Rand")
}

func TestScheduler_TickDispatchesDueFeeds(t *testing.T) {
	t.Parallel()

	var hits sync.Map
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Store(r.URL.Path, true)
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	d := newDB(t)
	id1, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{Title: "a", FeedURL: srv.URL + "/a", NextPoll: 0, Created: 0})
	id2, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{Title: "b", FeedURL: srv.URL + "/b", NextPoll: 0, Created: 0})

	sch := NewScheduler(context.Background(), d, http.DefaultClient, SchedulerOpts{Workers: 2, Cadence: 30 * time.Minute})
	t.Cleanup(sch.Stop)
	sch.Tick(context.Background())
	require.NoError(t, sch.Wait(5*time.Second))

	_, ok1 := hits.Load("/a")
	_, ok2 := hits.Load("/b")
	require.True(t, ok1, "feed a should have been polled")
	require.True(t, ok2, "feed b should have been polled")

	for _, id := range []int64{id1, id2} {
		s, _ := db.GetSubscription(context.Background(), d, id)
		require.True(t, s.LastPollAt.Valid)
	}
}

func TestScheduler_SkipsInflight(t *testing.T) {
	t.Parallel()

	d := newDB(t)
	id, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{Title: "a", FeedURL: "http://invalid.invalid", NextPoll: 0, Created: 0})

	sch := NewScheduler(context.Background(), d, http.DefaultClient, SchedulerOpts{Workers: 1, Cadence: 30 * time.Minute})
	t.Cleanup(sch.Stop)

	// Manually mark in-flight; Tick must skip it.
	require.True(t, sch.inflight.TryAcquire(id))

	dispatched := sch.Tick(context.Background())
	require.Equal(t, 0, dispatched)
}
