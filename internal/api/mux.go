package api

import (
	"net/http"

	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/poller"
	"github.com/bcrisp4/tap/internal/storage"
)

// Dependencies wires the API handlers to the rest of the binary.
// Every field must be non-nil at runtime; the API package never
// constructs its own store / client / runstate.
type Dependencies struct {
	Store      *storage.Store
	HTTPClient *httpclient.Client
	RunState   *poller.RunState
}

// Mux returns the ServeMux carrying every /api/v1/* route except the
// proxy (mounted by the poller's media-proxy handler) and /healthz
// (mounted by internal/server). Routes use Go 1.22's method-aware
// pattern syntax; subsequent tasks register categories, entries,
// search, OPML, and system endpoints alongside these feed routes.
func Mux(deps Dependencies) *http.ServeMux {
	feeds := &feedHandlers{store: deps.Store, client: deps.HTTPClient}
	cats := &categoryHandlers{store: deps.Store}
	entries := &entryHandlers{store: deps.Store}
	search := &searchHandlers{store: deps.Store}
	opml := &opmlHandlers{store: deps.Store}

	mux := http.NewServeMux()

	// Feeds — discovery is a literal, register before the {id} routes.
	mux.HandleFunc("GET /api/v1/feeds", feeds.list)
	mux.HandleFunc("POST /api/v1/feeds", feeds.subscribe)
	mux.HandleFunc("POST /api/v1/feeds/discover", feeds.discover)
	mux.HandleFunc("GET /api/v1/feeds/{id}", feeds.get)
	mux.HandleFunc("PUT /api/v1/feeds/{id}", feeds.update)
	mux.HandleFunc("DELETE /api/v1/feeds/{id}", feeds.delete)
	mux.HandleFunc("POST /api/v1/feeds/{id}/refresh", feeds.refresh)

	mux.HandleFunc("GET /api/v1/categories", cats.list)
	mux.HandleFunc("POST /api/v1/categories", cats.create)
	mux.HandleFunc("PUT /api/v1/categories/{id}", cats.rename)
	mux.HandleFunc("DELETE /api/v1/categories/{id}", cats.delete)

	// Entries — `PUT /entries/read` is more specific than `PUT
	// /entries/{id}`, so ServeMux dispatches it to bulkRead.
	mux.HandleFunc("GET /api/v1/entries", entries.list)
	mux.HandleFunc("PUT /api/v1/entries/read", entries.bulkRead)
	mux.HandleFunc("GET /api/v1/entries/{id}", entries.get)
	mux.HandleFunc("PUT /api/v1/entries/{id}", entries.put)

	mux.HandleFunc("GET /api/v1/search", search.handle)

	mux.HandleFunc("POST /api/v1/opml/import", opml.importHandler)
	mux.HandleFunc("GET /api/v1/opml/export", opml.exportHandler)

	return mux
}
