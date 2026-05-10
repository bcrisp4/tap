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
	require.Equal(t, 15*time.Minute, s.opts.Floor)
	require.Equal(t, 24*time.Hour, s.opts.Ceiling)
	require.Equal(t, 5*time.Minute, s.opts.ErrorBase)
	require.NotNil(t, s.opts.Now)
}

func TestScheduler_TickDispatchesDueFeeds(t *testing.T) {
	t.Parallel()

	var hits sync.Map
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Store(r.URL.Path, true)
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	id1, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{UserID: uid, Title: "a", FeedURL: srv.URL + "/a", NextPoll: 0, Created: 0})
	id2, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{UserID: uid, Title: "b", FeedURL: srv.URL + "/b", NextPoll: 0, Created: 0})

	sch := NewScheduler(context.Background(), d, http.DefaultClient, SchedulerOpts{Workers: 2})
	t.Cleanup(sch.Stop)
	sch.Tick(context.Background())
	require.NoError(t, sch.Wait(5*time.Second))

	_, ok1 := hits.Load("/a")
	_, ok2 := hits.Load("/b")
	require.True(t, ok1, "feed a should have been polled")
	require.True(t, ok2, "feed b should have been polled")

	for _, id := range []int64{id1, id2} {
		s, _ := db.GetSubscription(context.Background(), d, id, uid)
		require.True(t, s.LastPollAt.Valid)
	}
}

func TestScheduler_SkipsInflight(t *testing.T) {
	t.Parallel()

	d, uid := newDBUser(t)
	id, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{UserID: uid, Title: "a", FeedURL: "http://invalid.invalid", NextPoll: 0, Created: 0})

	sch := NewScheduler(context.Background(), d, http.DefaultClient, SchedulerOpts{Workers: 1})
	t.Cleanup(sch.Stop)

	// Manually mark in-flight; Tick must skip it.
	require.True(t, sch.inflight.TryAcquire(id))

	dispatched := sch.Tick(context.Background())
	require.Equal(t, 0, dispatched)
}
