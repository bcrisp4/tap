package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func TestWriteOK_BareObject(t *testing.T) {
	w := httptest.NewRecorder()
	WriteOK(w, http.StatusOK, map[string]any{"id": 1, "name": "Tap"})

	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, float64(1), got["id"])
	require.Equal(t, "Tap", got["name"])
}

func TestWriteList_Envelope(t *testing.T) {
	w := httptest.NewRecorder()
	WriteList(w, []map[string]int{{"a": 1}, {"a": 2}}, 50, 0, 2)

	var got struct {
		Data       []map[string]int `json:"data"`
		Pagination struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
			Total  int `json:"total"`
		} `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Data, 2)
	require.Equal(t, 50, got.Pagination.Limit)
	require.Equal(t, 0, got.Pagination.Offset)
	require.Equal(t, 2, got.Pagination.Total)
}

func TestWriteList_EmptyDataIsArrayNotNull(t *testing.T) {
	// SPA expects data to always be a JSON array, never null, even
	// when the underlying slice is nil.
	w := httptest.NewRecorder()
	var nilSlice []int
	WriteList(w, nilSlice, 50, 0, 0)
	require.Contains(t, w.Body.String(), `"data":[]`)
}

func TestWriteError_Envelope(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusNotFound, "not_found", "feed 123 not found")

	require.Equal(t, http.StatusNotFound, w.Code)
	var got struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "not_found", got.Error.Code)
	require.Equal(t, "feed 123 not found", got.Error.Message)
}

func TestWriteErr_MapsNotFoundTo404(t *testing.T) {
	w := httptest.NewRecorder()
	writeErr(w, storage.ErrNotFound)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), `"code":"not_found"`)
}

func TestWriteErr_MapsGenericTo500(t *testing.T) {
	w := httptest.NewRecorder()
	writeErr(w, errors.New("boom"))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), `"code":"internal"`)
}
