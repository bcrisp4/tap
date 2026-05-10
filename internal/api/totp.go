package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
)

type totpBeginResponse struct {
	SecretURI string `json:"secret_uri"`
	Secret    string `json:"secret"`
}

type totpConfirmRequest struct {
	Code string `json:"code"`
}

type totpConfirmResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

type totpDisableRequest struct {
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

type totpRegenerateRequest struct {
	Code string `json:"code"`
}

type totpRegenerateResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

func beginTOTPEnrolmentHandler(d *sql.DB, hashParams auth.Params) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		hasTOTP, confirmed, err := db.GetUserTOTPStatus(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if hasTOTP && confirmed {
			writeError(w, http.StatusConflict, ErrCodeTOTPAlreadyEnrolled, "TOTP already enrolled")
			return
		}

		secret, err := auth.GenerateTOTPSecret()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		encKey, err := getTOTPEncryptionKey(r.Context(), d)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		encrypted, err := auth.EncryptTOTPSecret(encKey, secret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.InsertTOTPSecret(r.Context(), d, u.ID, encrypted); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		secretURI := auth.TOTPSecretURI(secret, "Tap", u.Username)
		writeJSON(w, http.StatusOK, totpBeginResponse{SecretURI: secretURI, Secret: secret})
	})
}

func confirmTOTPEnrolmentHandler(d *sql.DB, hashParams auth.Params) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body totpConfirmRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		totpSecret, err := db.GetTOTPSecret(r.Context(), d, u.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeTOTPNotEnrolled, "no TOTP secret pending")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if totpSecret.Confirmed {
			writeError(w, http.StatusConflict, ErrCodeTOTPAlreadyEnrolled, "TOTP already confirmed")
			return
		}

		encKey, err := getTOTPEncryptionKey(r.Context(), d)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		plainSecret, err := auth.DecryptTOTPSecret(encKey, totpSecret.SecretEncrypted)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if !auth.VerifyTOTP(plainSecret, body.Code) {
			writeError(w, http.StatusUnauthorized, ErrCodeTOTPInvalid, "invalid TOTP code")
			return
		}

		// Generate + hash codes before opening the transaction (pure CPU work).
		codes, err := auth.GenerateRecoveryCodes()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		hashes := make([]string, len(codes))
		for i, c := range codes {
			h, err := auth.Hash(c, hashParams)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			hashes[i] = h
		}
		// Atomically confirm TOTP and insert recovery codes so a crash between
		// the two writes cannot leave TOTP active with no recovery codes.
		if err := db.ConfirmTOTPWithRecoveryCodes(r.Context(), d, u.ID, hashes); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, totpConfirmResponse{RecoveryCodes: codes})
	})
}

func deleteTOTPHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body totpDisableRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		hasTOTP, confirmed, err := db.GetUserTOTPStatus(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if !hasTOTP || !confirmed {
			writeError(w, http.StatusConflict, ErrCodeTOTPNotEnrolled, "TOTP not enrolled")
			return
		}

		if body.RecoveryCode != "" {
			if !validateRecoveryCode(r.Context(), d, u.ID, body.RecoveryCode) {
				writeError(w, http.StatusUnauthorized, ErrCodeRecoveryCodeInvalid, "invalid recovery code")
				return
			}
		} else if body.Code != "" {
			totpSecret, err := db.GetTOTPSecret(r.Context(), d, u.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			encKey, err := getTOTPEncryptionKey(r.Context(), d)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			plainSecret, err := auth.DecryptTOTPSecret(encKey, totpSecret.SecretEncrypted)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			if !auth.VerifyTOTP(plainSecret, body.Code) {
				writeError(w, http.StatusUnauthorized, ErrCodeTOTPInvalid, "invalid TOTP code")
				return
			}
		} else {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "code or recovery_code required")
			return
		}

		if err := db.DeleteTOTPSecret(r.Context(), d, u.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.DeleteRecoveryCodes(r.Context(), d, u.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func regenerateRecoveryCodesHandler(d *sql.DB, hashParams auth.Params) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body totpRegenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		totpSecret, err := db.GetTOTPSecret(r.Context(), d, u.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusConflict, ErrCodeTOTPNotEnrolled, "TOTP not enrolled")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if !totpSecret.Confirmed {
			writeError(w, http.StatusConflict, ErrCodeTOTPNotEnrolled, "TOTP not confirmed")
			return
		}

		encKey, err := getTOTPEncryptionKey(r.Context(), d)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		plainSecret, err := auth.DecryptTOTPSecret(encKey, totpSecret.SecretEncrypted)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if !auth.VerifyTOTP(plainSecret, body.Code) {
			writeError(w, http.StatusUnauthorized, ErrCodeTOTPInvalid, "invalid TOTP code")
			return
		}

		if err := db.DeleteRecoveryCodes(r.Context(), d, u.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		codes, err := auth.GenerateRecoveryCodes()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		hashes := make([]string, len(codes))
		for i, c := range codes {
			h, err := auth.Hash(c, hashParams)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			hashes[i] = h
		}
		if err := db.InsertRecoveryCodes(r.Context(), d, u.ID, hashes); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, totpRegenerateResponse{RecoveryCodes: codes})
	})
}
