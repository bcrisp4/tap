package poller_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/poller"
	"github.com/bcrisp4/tap/internal/storage"
)

func TestPoller_PicksUpDueFeed(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(dbPath)
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(context.Background(), d))
	store := storage.New(d)

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, minimalAtom(hits))
	}))
	defer srv.Close()

	now := time.Now().Unix() - 60
	_, err = store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "T", FeedURL: srv.URL,
		NextPollAt: &now, PollInterval: 3600,
	})
	require.NoError(t, err)

	p := poller.New(poller.Config{
		Store:    store,
		Client:   httpclient.NewClient(httpclient.Config{Timeout: 2 * time.Second, MaxBodyBytes: 1 << 20, AllowPrivate: true}),
		Interval: 50 * time.Millisecond,
		Workers:  2,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Start(ctx)

	require.Eventually(t, func() bool {
		entries, _ := store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
		return len(entries) >= 2
	}, 2*time.Second, 50*time.Millisecond, "poller never inserted entries")
}
