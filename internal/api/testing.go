package api

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/bcrisp4/tap/internal/db"
)

// NewTestMux returns an UNAUTHENTICATED mux carrying the same /api/v1/*
// routes as NewMux, but skipping requireSession + requireCSRF. Intended for
// unit and integration tests that exercise the route plumbing without
// driving the auth flow.
//
// The TestUser (if non-zero ID) is injected into every request context so
// that handlers that call userFromContext get a valid user. Pass a zero User
// to test unauthenticated behavior (handlers will return 401).
//
// Production code path is NewMux, which wraps every authenticated route
// (everything except /healthz and POST /api/v1/sessions) with the
// session-and-CSRF middleware chain. End-to-end auth coverage lives in
// cmd/tap/main_test.go; this helper exists so that handler tests don't have
// to take a dependency on the full auth flow.
type TestMuxOpts struct {
	MuxOpts
	TestUser    db.User
	TestSession db.Session
}

func NewTestMux(d *sql.DB, opts TestMuxOpts) *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	if d == nil {
		return m
	}

	var inject func(http.Handler) http.Handler
	if opts.TestUser.ID != 0 {
		inject = func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), ctxKeyUser, opts.TestUser)
				ctx = context.WithValue(ctx, ctxKeySession, opts.TestSession)
				h.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	} else {
		inject = func(h http.Handler) http.Handler { return h }
	}

	subsMux := http.NewServeMux()
	registerSubscriptionRoutes(subsMux, d, opts.Poke)
	entriesMux := http.NewServeMux()
	registerEntryRoutes(entriesMux, d)
	catsMux := http.NewServeMux()
	registerCategoryRoutes(catsMux, d)
	searchMux := http.NewServeMux()
	registerSearchRoutes(searchMux, d)
	opmlMux := http.NewServeMux()
	registerOPMLRoutes(opmlMux, d)

	for _, p := range []struct {
		method, path string
		handler      http.Handler
	}{
		{"GET", "/api/v1/subscriptions", inject(subsMux)},
		{"GET", "/api/v1/subscriptions/{id}", inject(subsMux)},
		{"POST", "/api/v1/subscriptions", inject(subsMux)},
		{"PATCH", "/api/v1/subscriptions/{id}", inject(subsMux)},
		{"DELETE", "/api/v1/subscriptions/{id}", inject(subsMux)},
		{"GET", "/api/v1/entries", inject(entriesMux)},
		{"GET", "/api/v1/entries/{id}", inject(entriesMux)},
		{"PATCH", "/api/v1/entries/{id}", inject(entriesMux)},
		{"GET", "/api/v1/categories", inject(catsMux)},
		{"POST", "/api/v1/categories", inject(catsMux)},
		{"PATCH", "/api/v1/categories/{id}", inject(catsMux)},
		{"DELETE", "/api/v1/categories/{id}", inject(catsMux)},
		{"POST", "/api/v1/categories/{id}/mark-read", inject(catsMux)},
		{"GET", "/api/v1/search", inject(searchMux)},
		{"GET", "/api/v1/opml", inject(opmlMux)},
		{"POST", "/api/v1/opml", inject(opmlMux)},
	} {
		m.Handle(p.method+" "+p.path, p.handler)
	}

	if opts.ProxyHandler != nil {
		m.Handle("GET /api/v1/proxy/{token}", inject(opts.ProxyHandler))
	}
	if opts.DiscoverClient != nil {
		discoverMux := http.NewServeMux()
		registerDiscoverRoutes(discoverMux, opts.DiscoverClient)
		m.Handle("POST /api/v1/discover", inject(discoverMux))
	}
	return m
}
