package api

import (
	"context"
	"net/http"

	"github.com/bcrisp4/tap/internal/db"
)

// ctxKey is unexported so external packages cannot read or override the
// per-request user/session injected by requireSession.
type ctxKey int

const (
	ctxKeyUser ctxKey = iota
	ctxKeySession
)

// userFromContext returns the user injected by requireSession, if any.
func userFromContext(ctx context.Context) (db.User, bool) {
	u, ok := ctx.Value(ctxKeyUser).(db.User)
	return u, ok
}

// sessionFromContext returns the session injected by requireSession, if any.
func sessionFromContext(ctx context.Context) (db.Session, bool) {
	s, ok := ctx.Value(ctxKeySession).(db.Session)
	return s, ok
}

// chain composes middleware. Outermost is first.
func chain(mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}
