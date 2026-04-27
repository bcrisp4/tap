package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/poller"
	"github.com/bcrisp4/tap/internal/storage"
)

// apiFixture spins up an in-memory store + mux + run-state and lets
// tests round-trip through the full handler pipeline. Reused by every
// table test in this package.
type apiFixture struct {
	store *storage.Store
	mux   http.Handler
	state *poller.RunState
}

func newAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "tap.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	store := storage.New(d)
	state := poller.NewRunState(8)
	cli := httpclient.NewClient(httpclient.Config{
		Timeout:      2 * time.Second,
		MaxBodyBytes: 1 << 20,
		AllowPrivate: true,
	})
	mux := api.Mux(api.Dependencies{Store: store, RunState: state, HTTPClient: cli})
	return &apiFixture{store: store, mux: mux, state: state}
}

func (f *apiFixture) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	f.mux.ServeHTTP(w, r)
	return w
}

// doRaw lets a few tests (OPML import) post a non-JSON body.
func (f *apiFixture) doRaw(t *testing.T, method, path, contentType string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, body)
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	f.mux.ServeHTTP(w, r)
	return w
}

// startDiscoverServer hosts the given HTML for the discovery test.
func startDiscoverServer(t *testing.T, html string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
	t.Cleanup(s.Close)
	return s
}
