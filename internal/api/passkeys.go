package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
)

// webAuthnUser wraps db.User + their passkeys to satisfy webauthn.User interface.
type webAuthnUser struct {
	user     db.User
	passkeys []db.Passkey
}

func (u *webAuthnUser) WebAuthnID() []byte {
	id := u.user.ID
	return []byte{
		byte(id >> 56), byte(id >> 48), byte(id >> 40), byte(id >> 32),
		byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id),
	}
}

func (u *webAuthnUser) WebAuthnName() string        { return u.user.Username }
func (u *webAuthnUser) WebAuthnDisplayName() string { return u.user.Username }

func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.passkeys))
	for i, p := range u.passkeys {
		creds[i] = webauthn.Credential{
			ID:              p.CredentialID,
			PublicKey:       p.PublicKey,
			AttestationType: "none",
			Authenticator: webauthn.Authenticator{
				AAGUID:    p.AAGUID,
				SignCount: uint32(p.SignCounter),
			},
		}
	}
	return creds
}

type passkeyDTO struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	CreatedAt int64  `json:"created_at"`
}

func beginPasskeyRegistrationHandler(d *sql.DB, wa *webauthn.WebAuthn) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		s, ok2 := sessionFromContext(r.Context())
		if !ok || !ok2 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		passkeys, err := db.GetPasskeysByUserID(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		waUser := &webAuthnUser{user: u, passkeys: passkeys}
		creation, session, err := wa.BeginRegistration(waUser)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		sessionJSON, err := json.Marshal(session)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.SetWebAuthnChallenge(r.Context(), d, s.ID, sessionJSON); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, creation)
	})
}

func finishPasskeyRegistrationHandler(d *sql.DB, wa *webauthn.WebAuthn) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		s, ok2 := sessionFromContext(r.Context())
		if !ok || !ok2 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		if s.WebAuthnChallenge == nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "no registration session active")
			return
		}

		var waSession webauthn.SessionData
		if err := json.Unmarshal(s.WebAuthnChallenge, &waSession); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid session data")
			return
		}

		label := r.URL.Query().Get("label")
		if label == "" {
			label = "Passkey"
		}

		passkeys, err := db.GetPasskeysByUserID(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		waUser := &webAuthnUser{user: u, passkeys: passkeys}
		credential, err := wa.FinishRegistration(waUser, waSession, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "registration failed: "+err.Error())
			return
		}

		if err := db.ClearWebAuthnChallenge(r.Context(), d, s.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		now := time.Now().Unix()
		id, err := db.InsertPasskey(r.Context(), d, db.Passkey{
			UserID:       u.ID,
			CredentialID: credential.ID,
			PublicKey:    credential.PublicKey,
			SignCounter:  int64(credential.Authenticator.SignCount),
			AAGUID:       credential.Authenticator.AAGUID,
			Label:        label,
			CreatedAt:    now,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, passkeyDTO{ID: id, Label: label, CreatedAt: now})
	})
}

func listPasskeysHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		passkeys, err := db.GetPasskeysByUserID(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]passkeyDTO, len(passkeys))
		for i, p := range passkeys {
			out[i] = passkeyDTO{ID: p.ID, Label: p.Label, CreatedAt: p.CreatedAt}
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func deletePasskeyHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid passkey id")
			return
		}
		if err := db.DeletePasskey(r.Context(), d, id, u.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodePasskeyNotFound, "passkey not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

type beginPasskeyLoginResponse struct {
	SessionID int64       `json:"session_id"`
	Options   interface{} `json:"options"`
}

func beginPasskeyLoginHandler(d *sql.DB, wa *webauthn.WebAuthn, dep authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertion, waSession, err := wa.BeginDiscoverableLogin()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		sessionJSON, err := json.Marshal(waSession)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		_, anonTokenHash, err2 := auth.MintSessionToken()
		if err2 != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err2.Error())
			return
		}
		anonCSRF, err2 := auth.MintCSRFToken()
		if err2 != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err2.Error())
			return
		}
		now := time.Now()
		sid, err := db.InsertAnonymousSession(r.Context(), d,
			anonTokenHash, anonCSRF,
			now.Unix(), now.Unix(),
			now.Add(5*time.Minute).Unix(),
			now.Add(5*time.Minute).Unix())
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		if err := db.SetWebAuthnChallenge(r.Context(), d, sid, sessionJSON); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, beginPasskeyLoginResponse{SessionID: sid, Options: assertion})
	})
}

func finishPasskeyLoginHandler(d *sql.DB, wa *webauthn.WebAuthn, dep authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "cannot read body")
			return
		}

		// Extract session_id from the outer JSON wrapper.
		var outer struct {
			SessionID int64           `json:"session_id"`
			Assertion json.RawMessage `json:"assertion"`
		}
		if err := json.Unmarshal(bodyBytes, &outer); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		sess, err := db.GetSessionByID(r.Context(), d, outer.SessionID)
		if err != nil || sess.UserID != 0 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "invalid session")
			return
		}
		if sess.WebAuthnChallenge == nil {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "no challenge for session")
			return
		}
		if time.Now().Unix() > sess.AbsoluteExpiresAt {
			_ = db.DeleteSession(r.Context(), d, sess.ID)
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "challenge expired")
			return
		}

		var waSession webauthn.SessionData
		if err := json.Unmarshal(sess.WebAuthnChallenge, &waSession); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		// Parse the assertion using bytes.
		assertionData := outer.Assertion
		if assertionData == nil {
			assertionData = bodyBytes // fallback: client may send assertion directly
		}

		parsedAssertion, err := protocol.ParseCredentialRequestResponseBytes(assertionData)
		if err != nil {
			_ = db.DeleteSession(r.Context(), d, sess.ID)
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid assertion: "+err.Error())
			return
		}

		handler := webauthn.DiscoverableUserHandler(func(rawID, userHandle []byte) (webauthn.User, error) {
			pk, err := db.GetPasskeyByCredentialID(r.Context(), d, rawID)
			if err != nil {
				return nil, err
			}
			u, err := db.GetUserByID(r.Context(), d, pk.UserID)
			if err != nil {
				return nil, err
			}
			allPasskeys, _ := db.GetPasskeysByUserID(r.Context(), d, u.ID)
			return &webAuthnUser{user: u, passkeys: allPasskeys}, nil
		})

		waUserResult, credential, err := wa.ValidatePasskeyLogin(handler, waSession, parsedAssertion)
		if err != nil {
			_ = db.DeleteSession(r.Context(), d, sess.ID)
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "authentication failed")
			return
		}

		waU, ok := waUserResult.(*webAuthnUser)
		if !ok {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "invalid user type")
			return
		}

		pk, err := db.GetPasskeyByCredentialID(r.Context(), d, credential.ID)
		if err == nil {
			_ = db.UpdatePasskeySignCounter(r.Context(), d, pk.ID, int64(credential.Authenticator.SignCount))
		}

		cookieValue, tokenHash, err := auth.MintSessionToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		csrfToken, err := auth.MintCSRFToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.UpgradeAnonymousSession(r.Context(), d, sess.ID, waU.user.ID, tokenHash, csrfToken); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		setSessionCookie(w, cookieValue, dep.sessionAbsoluteTTL, dep.cookieSecure)
		writeJSON(w, http.StatusOK, loginResponse{User: buildUserDTO(r.Context(), d, waU.user), CSRFToken: csrfToken})
	})
}

