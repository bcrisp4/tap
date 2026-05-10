package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
)

func newCatTestSetup(t *testing.T) (*http.ServeMux, *db.User) {
	t.Helper()
	d := newTestDB(t)
	userID := insertAPITestUser(t, d, "catuser")
	user := db.User{ID: userID, Username: "catuser", Role: "admin"}
	mux := NewTestMux(d, TestMuxOpts{TestUser: user})
	return mux, &user
}

func TestCategoriesAPI_CRUD(t *testing.T) {
	t.Parallel()
	mux, _ := newCatTestSetup(t)

	// Create
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories",
		strings.NewReader(`{"name":"Tech"}`)))
	require.Equal(t, http.StatusCreated, w.Code)
	var cat categoryDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cat))
	require.Equal(t, "Tech", cat.Name)
	catID := cat.ID

	// List
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/categories", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var listResp struct{ Data []categoryDTO `json:"data"` }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 1)

	// Rename
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/categories/"+itoa(catID),
		strings.NewReader(`{"name":"Science"}`)))
	require.Equal(t, http.StatusOK, w.Code)

	// Delete
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/v1/categories/"+itoa(catID), nil))
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestCategoriesAPI_DuplicateName(t *testing.T) {
	t.Parallel()
	mux, _ := newCatTestSetup(t)

	body := `{"name":"Tech"}`
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories", strings.NewReader(body)))
	require.Equal(t, http.StatusCreated, w.Code)

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories", strings.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp ErrorEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, ErrCodeCategoryNameTaken, resp.Error.Code)
}

func TestCategoriesAPI_CrossUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertAPITestUser(t, d, "u1")
	u2 := insertAPITestUser(t, d, "u2")

	// u1 creates a category
	mux1 := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: u1, Username: "u1", Role: "admin"}})
	w := httptest.NewRecorder()
	mux1.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories", strings.NewReader(`{"name":"Tech"}`)))
	require.Equal(t, http.StatusCreated, w.Code)
	var cat categoryDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cat))

	// u2 cannot GET it
	mux2 := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: u2, Username: "u2", Role: "admin"}})
	w = httptest.NewRecorder()
	mux2.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/v1/categories/"+itoa(cat.ID), nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestCategoriesAPI_Unauthenticated(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	mux := NewTestMux(d, TestMuxOpts{}) // zero user = unauthenticated

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/categories", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCategoriesAPI_MarkRead(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertAPITestUser(t, d, "markread")
	user := db.User{ID: userID, Username: "markread", Role: "admin"}
	mux := NewTestMux(d, TestMuxOpts{TestUser: user})

	catID, err := db.InsertCategory(context.Background(), d, db.NewCategory{UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/categories/"+itoa(catID)+"/mark-read", nil))
	require.Equal(t, http.StatusNoContent, w.Code)
}
