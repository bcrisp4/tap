package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/bcrisp4/tap/internal/db"
)

type searchResultDTO struct {
	ID             int64   `json:"id"`
	SubscriptionID int64   `json:"subscription_id"`
	Title          string  `json:"title"`
	URL            string  `json:"url"`
	Author         string  `json:"author,omitempty"`
	PublishedAt    int64   `json:"published_at"`
	Read           bool    `json:"read"`
	Saved          bool    `json:"saved"`
	Rank           float64 `json:"rank"`
}

func registerSearchRoutes(m *http.ServeMux, d *sql.DB) {
	m.HandleFunc("GET /api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		q := r.URL.Query().Get("q")
		if len(q) < 3 {
			writeError(w, http.StatusBadRequest, ErrCodeQueryTooShort, "query must be at least 3 characters")
			return
		}

		limit := 50
		if ls := r.URL.Query().Get("limit"); ls != "" {
			if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}

		results, err := db.SearchEntries(r.Context(), d, u.ID, q, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		out := make([]searchResultDTO, 0, len(results))
		for _, r := range results {
			out = append(out, searchResultDTO{
				ID:             r.ID,
				SubscriptionID: r.SubscriptionID,
				Title:          r.Title,
				URL:            r.URL,
				Author:         r.Author,
				PublishedAt:    r.PublishedAt,
				Read:           r.Read,
				Saved:          r.Saved,
				Rank:           r.Rank,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	})
}
