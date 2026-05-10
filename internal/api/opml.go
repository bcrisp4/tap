package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

const opmlBodyLimit = 10 << 20 // 10 MiB

func registerOPMLRoutes(m *http.ServeMux, d *sql.DB) {
	m.HandleFunc("GET /api/v1/opml", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		data, err := db.ExportOPML(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="subscriptions.opml"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	m.HandleFunc("POST /api/v1/opml", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		// 10 MiB cap — OPML files are much larger than typical JSON bodies.
		lr := io.LimitReader(r.Body, opmlBodyLimit+1)
		data, err := io.ReadAll(lr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "failed to read body")
			return
		}
		if int64(len(data)) > opmlBodyLimit {
			writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
			return
		}

		imported, skipped, errs, err := db.ImportOPML(r.Context(), d, u.ID, data, time.Now().Unix())
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "failed to parse OPML: "+err.Error())
			return
		}

		if errs == nil {
			errs = []string{}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"imported": imported,
			"skipped":  skipped,
			"errors":   errs,
		})
	})
}
