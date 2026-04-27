package api_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCategories_CRUD(t *testing.T) {
	f := newAPIFixture(t)

	// Create
	w := f.do(t, "POST", "/api/v1/categories", `{"name":"Tech"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	require.Contains(t, w.Body.String(), `"name":"Tech"`)

	// List
	w = f.do(t, "GET", "/api/v1/categories", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Tech")
	require.Contains(t, w.Body.String(), `"data"`)

	// Find the id.
	cats, err := f.store.ListCategories(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, cats, 1)
	id := strconv.FormatInt(cats[0].ID, 10)

	// Rename
	w = f.do(t, "PUT", "/api/v1/categories/"+id, `{"name":"Engineering"}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"name":"Engineering"`)

	// Delete
	w = f.do(t, "DELETE", "/api/v1/categories/"+id, "")
	require.Equal(t, http.StatusNoContent, w.Code)

	cats, err = f.store.ListCategories(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, cats, 0)
}

func TestCategories_Create_RequiresName(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/categories", `{}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code":"bad_json"`)
}

func TestCategories_Rename_NotFound(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "PUT", "/api/v1/categories/9999", `{"name":"x"}`)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestCategories_Delete_NotFound(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "DELETE", "/api/v1/categories/9999", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestCategories_Rename_BadID(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "PUT", "/api/v1/categories/abc", `{"name":"x"}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
