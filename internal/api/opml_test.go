package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
)

func TestOPMLAPI_Export(t *testing.T) {
	t.Parallel()
	d := newTestAPIDB(t)
	userID := insertAPITestUser(t, d, "opmlexp")
	mux := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: userID, Username: "opmlexp", Role: "admin"}})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/opml", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/x-opml; charset=utf-8", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), `version="2.0"`)
}

func TestOPMLAPI_ImportTooLarge(t *testing.T) {
	t.Parallel()
	d := newTestAPIDB(t)
	userID := insertAPITestUser(t, d, "opmlbig")
	mux := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: userID, Username: "opmlbig", Role: "admin"}})

	// Create >10MiB body
	big := bytes.Repeat([]byte("x"), 10<<20+1)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/opml", bytes.NewReader(big)))
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestOPMLAPI_ValidImport(t *testing.T) {
	t.Parallel()
	d := newTestAPIDB(t)
	userID := insertAPITestUser(t, d, "opmlimport")
	mux := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: userID, Username: "opmlimport", Role: "admin"}})

	opmlBody := `<?xml version="1.0"?><opml version="2.0"><head><title>t</title></head><body>
		<outline type="rss" text="Feed1" xmlUrl="https://import1.com/feed"/>
	</body></opml>`

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/opml", strings.NewReader(opmlBody)))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"imported"`)
	require.Contains(t, w.Body.String(), `"skipped"`)
}

func TestOPMLAPI_Unauthenticated(t *testing.T) {
	t.Parallel()
	d := newTestAPIDB(t)
	mux := NewTestMux(d, TestMuxOpts{})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/opml", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
