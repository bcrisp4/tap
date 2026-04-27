// Package api implements Tap's /api/v1 REST surface.
//
// Response shape (design.md §6):
//   - List: {"data": [...], "pagination": {"limit": N, "offset": N, "total": N}}
//   - Single: bare object
//   - Error: {"error": {"code": "...", "message": "..."}} with HTTP status
//
// Timestamps are unix epoch seconds.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	"github.com/bcrisp4/tap/internal/storage"
)

// WriteOK writes a JSON body with the given status code.
func WriteOK(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteList writes a list response with the {data, pagination} envelope
// per design.md §6. A nil slice is normalised to `[]` so the SPA never
// sees `"data":null`.
func WriteList(w http.ResponseWriter, data any, limit, offset, total int) {
	if data == nil || (reflect.ValueOf(data).Kind() == reflect.Slice && reflect.ValueOf(data).IsNil()) {
		data = []struct{}{}
	}
	WriteOK(w, http.StatusOK, struct {
		Data       any            `json:"data"`
		Pagination paginationMeta `json:"pagination"`
	}{
		Data:       data,
		Pagination: paginationMeta{Limit: limit, Offset: offset, Total: total},
	})
}

type paginationMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// WriteError writes an error envelope with the given HTTP status.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteOK(w, status, struct {
		Error errorBody `json:"error"`
	}{Error: errorBody{Code: code, Message: message}})
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeErr translates internal errors to HTTP responses uniformly.
func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", err.Error())
	default:
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
	}
}
