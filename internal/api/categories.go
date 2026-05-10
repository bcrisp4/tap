package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

type categoryDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Unread    int    `json:"unread"`
	CreatedAt int64  `json:"created_at"`
}

func toCategoryDTO(c db.Category) categoryDTO {
	return categoryDTO{
		ID:        c.ID,
		Name:      c.Name,
		Unread:    c.Unread,
		CreatedAt: c.CreatedAt,
	}
}

func registerCategoryRoutes(m *http.ServeMux, d *sql.DB) {
	m.HandleFunc("GET /api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		cats, err := db.ListCategories(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]categoryDTO, 0, len(cats))
		for _, c := range cats {
			out = append(out, toCategoryDTO(c))
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	})

	m.HandleFunc("POST /api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if body.Name == "" {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "name is required")
			return
		}
		id, err := db.InsertCategory(r.Context(), d, db.NewCategory{
			UserID:    u.ID,
			Name:      body.Name,
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			if errors.Is(err, db.ErrCategoryNameTaken) {
				writeError(w, http.StatusBadRequest, ErrCodeCategoryNameTaken, "category name already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		cat, err := db.GetCategory(r.Context(), d, id, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, toCategoryDTO(cat))
	})

	m.HandleFunc("PATCH /api/v1/categories/{id}", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if body.Name == "" {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "name is required")
			return
		}
		if err := db.UpdateCategoryName(r.Context(), d, id, u.ID, body.Name); err != nil {
			if errors.Is(err, db.ErrCategoryNotFound) {
				writeError(w, http.StatusNotFound, ErrCodeCategoryNotFound, "category not found")
				return
			}
			if errors.Is(err, db.ErrCategoryNameTaken) {
				writeError(w, http.StatusBadRequest, ErrCodeCategoryNameTaken, "category name already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		cat, err := db.GetCategory(r.Context(), d, id, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toCategoryDTO(cat))
	})

	m.HandleFunc("DELETE /api/v1/categories/{id}", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		if err := db.DeleteCategory(r.Context(), d, id, u.ID); err != nil {
			if errors.Is(err, db.ErrCategoryNotFound) {
				writeError(w, http.StatusNotFound, ErrCodeCategoryNotFound, "category not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	m.HandleFunc("POST /api/v1/categories/{id}/mark-read", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		if err := db.MarkCategoryRead(r.Context(), d, id, u.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
