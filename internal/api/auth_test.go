package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

// testParams keeps the auth tests fast — production uses auth.DefaultParams.
var testHashParams = auth.Params{Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 8, KeyLen: 16}

// seedUser inserts a user with the given password hashed under testHashParams.
func seedUser(t *testing.T, d *sql.DB, username, password, role string) int64 {
	t.Helper()
	hash, err := auth.Hash(password, testHashParams)
	require.NoError(t, err)
	id, err := db.InsertUser(context.Background(), d, db.NewUser{
		Username: username, PasswordHash: hash, Role: role, CreatedAt: 0,
	})
	require.NoError(t, err)
	return id
}

func TestLoginHappyPath(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := seedUser(t, d, "ben", "correct horse battery staple", "admin")

	deps := authDeps{
		d:                  d,
		sessionIdleTTL:     time.Hour,
		sessionAbsoluteTTL: 24 * time.Hour,
		cookieSecure:       false,
	}

	body, _ := json.Marshal(loginRequest{Username: "ben", Password: "correct horse battery staple"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	loginHandler(deps).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var resp loginResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, uid, resp.User.ID)
	require.Equal(t, "ben", resp.User.Username)
	require.Equal(t, "admin", resp.User.Role)
	require.NotEmpty(t, resp.CSRFToken)

	cookies := rr.Result().Cookies()
	require.NotEmpty(t, cookies)
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "tap_session" {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie)
	require.True(t, sessionCookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite)
	require.NotEmpty(t, sessionCookie.Value)
	require.True(t, strings.HasPrefix(sessionCookie.Path, "/"))
}
