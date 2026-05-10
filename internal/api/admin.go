package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
)

type adminUserDTO struct {
	ID           int64   `json:"id"`
	Username     string  `json:"username"`
	Role         string  `json:"role"`
	CreatedAt    int64   `json:"created_at"`
	DisabledAt   *int64  `json:"disabled_at"`
	HasTOTP      bool    `json:"has_totp"`
	PasskeyCount int     `json:"passkey_count"`
}

func toAdminUserDTO(u db.User) adminUserDTO {
	dto := adminUserDTO{
		ID:        u.ID,
		Username:  u.Username,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
	if u.DisabledAt.Valid {
		v := u.DisabledAt.Int64
		dto.DisabledAt = &v
	}
	return dto
}

// listUsersHandler handles GET /api/v1/admin/users.
func listUsersHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		users, err := db.ListUsers(r.Context(), d)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]adminUserDTO, 0, len(users))
		for _, u := range users {
			dto := toAdminUserDTO(u)
			dto.HasTOTP, _, _ = db.GetUserTOTPStatus(r.Context(), d, u.ID)
			dto.PasskeyCount, _ = db.GetUserPasskeyCount(r.Context(), d, u.ID)
			out = append(out, dto)
		}
		writeJSON(w, http.StatusOK, out)
	})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// createUserHandler handles POST /api/v1/admin/users.
func createUserHandler(d *sql.DB, hashParams auth.Params) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if err := auth.ValidatePassword(body.Password); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodePasswordTooShort, "password too short")
			return
		}
		if body.Role != "admin" && body.Role != "user" {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "role must be admin or user")
			return
		}
		hash, err := auth.Hash(body.Password, hashParams)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		id, err := db.InsertUser(r.Context(), d, db.NewUser{
			Username:     body.Username,
			PasswordHash: hash,
			Role:         body.Role,
			CreatedAt:    time.Now().Unix(),
		})
		if err != nil {
			if errors.Is(err, db.ErrUserExists) {
				writeError(w, http.StatusConflict, ErrCodeUserAlreadyExists, "username already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		u, err := db.GetUserByID(r.Context(), d, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, toAdminUserDTO(u))
	})
}

type patchUserRequest struct {
	Role     *string `json:"role"`
	Disabled *bool   `json:"disabled"`
}

// patchUserHandler handles PATCH /api/v1/admin/users/{id}.
func patchUserHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid user id")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body patchUserRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		if body.Role != nil {
			if *body.Role != "admin" && *body.Role != "user" {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "role must be admin or user")
				return
			}
			if err := db.UpdateUserRole(r.Context(), d, id, *body.Role); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, ErrCodeNotFound, "user not found")
					return
				}
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
		}
		if body.Disabled != nil {
			if *body.Disabled {
				if err := db.DisableUser(r.Context(), d, id, time.Now().Unix()); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, ErrCodeNotFound, "user not found")
						return
					}
					writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
					return
				}
			} else {
				if err := db.EnableUser(r.Context(), d, id); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, ErrCodeNotFound, "user not found")
						return
					}
					writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
					return
				}
			}
		}

		u, err := db.GetUserByID(r.Context(), d, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toAdminUserDTO(u))
	})
}

// resetUserPasswordHandler handles POST /api/v1/admin/users/{id}/password-reset.
func resetUserPasswordHandler(d *sql.DB, hashParams auth.Params) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid user id")
			return
		}

		tmpPass, err := generateTemporaryPassword(16)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		hash, err := auth.Hash(tmpPass, hashParams)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.UpdatePasswordHash(r.Context(), d, id, hash); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.DeleteSessionsByUserID(r.Context(), d, id); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"temporary_password": tmpPass})
	})
}

// disableUserTOTPHandler handles POST /api/v1/admin/users/{id}/disable-totp.
func disableUserTOTPHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid user id")
			return
		}
		if err := db.DeleteTOTPSecret(r.Context(), d, id); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.DeleteRecoveryCodes(r.Context(), d, id); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// deleteUserHandler handles DELETE /api/v1/admin/users/{id}.
func deleteUserHandler(d *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid user id")
			return
		}
		if id == caller.ID {
			writeError(w, http.StatusBadRequest, ErrCodeCannotDeleteSelf, "cannot delete yourself")
			return
		}
		if err := db.DeleteUser(r.Context(), d, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

const tmpPassAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"

func generateTemporaryPassword(length int) (string, error) {
	result := make([]byte, length)
	max := big.NewInt(int64(len(tmpPassAlphabet)))
	for i := range result {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate temporary password: %w", err)
		}
		result[i] = tmpPassAlphabet[n.Int64()]
	}
	return string(result), nil
}
