// Package api owns the REST handlers and JSON wire shapes.
package api

import (
	"encoding/json"
	"net/http"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// Stable error codes the SPA can switch on.
const (
	ErrCodeBadRequest = "bad_request"
	ErrCodeNotFound   = "not_found"
	ErrCodeConflict   = "conflict"
	ErrCodeInternal   = "internal"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: msg}})
}
