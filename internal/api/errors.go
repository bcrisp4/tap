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
	ErrCodeBadRequest             = "bad_request"
	ErrCodeNotFound               = "not_found"
	ErrCodeConflict               = "conflict"
	ErrCodeInternal               = "internal"
	ErrCodeExtractSelectorInvalid = "extract_selector_invalid"
	ErrCodeInvalidCredentials     = "invalid_credentials"
	ErrCodeInvalidSession         = "invalid_session"
	ErrCodeCSRFInvalid            = "csrf_invalid"
	ErrCodePasswordTooShort       = "password_too_short"

	// M7 error codes.
	ErrCodeTOTPRequired               = "totp_required"
	ErrCodeTOTPInvalid                = "totp_invalid"
	ErrCodeRecoveryCodeInvalid        = "recovery_code_invalid"
	ErrCodeTOTPNotEnrolled            = "totp_not_enrolled"
	ErrCodeTOTPAlreadyEnrolled        = "totp_already_enrolled"
	ErrCodePasskeyNotFound            = "passkey_not_found"
	ErrCodeCannotRevokeCurrentSession = "cannot_revoke_current_session"
	ErrCodeAdminRequired              = "admin_required"
	ErrCodeUserAlreadyExists          = "user_already_exists"
	ErrCodeCannotDeleteSelf           = "cannot_delete_self"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: msg}})
}
