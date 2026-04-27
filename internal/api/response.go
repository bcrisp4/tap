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
	if isNilSlice(data) {
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

// isNilSlice reports whether data is nil or a typed nil slice. JSON-
// encoding a nil slice emits null instead of [], which the SPA treats
// as a parse error.
func isNilSlice(data any) bool {
	if data == nil {
		return true
	}
	v := reflect.ValueOf(data)
	return v.Kind() == reflect.Slice && v.IsNil()
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

// maxJSONBodyBytes caps inbound JSON payloads. JSON request bodies on
// /api/v1 are tiny (<1KB even for a fully-populated feed update); 1MiB
// is generous and keeps a runaway client from exhausting memory.
const maxJSONBodyBytes int64 = 1 << 20

// decodeJSON pulls a JSON request body into dst, bounded by
// maxJSONBodyBytes. On failure it writes a 400 bad_json envelope and
// returns false, so the caller can simply:
//
//	if !decodeJSON(w, r, &req) { return }
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	body := http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_json", err.Error())
		return false
	}
	return true
}
