package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

type subscriptionDTO struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	FeedURL    string `json:"feed_url"`
	SiteURL    string `json:"site_url,omitempty"`
	NextPollAt int64  `json:"next_poll_at"`
	LastPollAt int64  `json:"last_poll_at,omitempty"`
	ErrorCount int    `json:"error_count"`
	LastError  string `json:"last_error,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

func toDTO(s db.Subscription) subscriptionDTO {
	d := subscriptionDTO{
		ID:         s.ID,
		Title:      s.Title,
		FeedURL:    s.FeedURL,
		NextPollAt: s.NextPollAt,
		ErrorCount: s.ErrorCount,
		CreatedAt:  s.CreatedAt,
	}
	if s.SiteURL.Valid {
		d.SiteURL = s.SiteURL.String
	}
	if s.LastPollAt.Valid {
		d.LastPollAt = s.LastPollAt.Int64
	}
	if s.LastError.Valid {
		d.LastError = s.LastError.String
	}
	return d
}

func registerSubscriptionRoutes(m *http.ServeMux, d *sql.DB, poke func()) {
	m.HandleFunc("GET /api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		subs, err := db.ListSubscriptions(r.Context(), d)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]subscriptionDTO, 0, len(subs))
		for _, s := range subs {
			out = append(out, toDTO(s))
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	})

	m.HandleFunc("POST /api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB cap on request body
		var body struct {
			FeedURL string `json:"feed_url"`
			Title   string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if _, err := url.ParseRequestURI(body.FeedURL); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "feed_url must be an absolute URL")
			return
		}
		title := body.Title
		if title == "" {
			title = body.FeedURL
		}
		id, err := db.InsertSubscription(r.Context(), d, db.NewSubscription{
			Title:    title,
			FeedURL:  body.FeedURL,
			NextPoll: 0,
			Created:  time.Now().Unix(),
		})
		if err != nil {
			writeError(w, http.StatusConflict, ErrCodeConflict, "subscription already exists")
			return
		}
		s, err := db.GetSubscription(r.Context(), d, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		// Poke the scheduler so the new feed polls within seconds, not
		// up-to-TickInterval. Best-effort: a missed poke just delays
		// the first poll to the next regular tick.
		if poke != nil {
			poke()
		}
		writeJSON(w, http.StatusCreated, toDTO(s))
	})

	m.HandleFunc("DELETE /api/v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		// Idempotent: deleting a non-existent ID is a 204, not a 404.
		// Matches the SPA's optimistic-delete model (the client may retry).
		if err := db.DeleteSubscription(r.Context(), d, id); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
