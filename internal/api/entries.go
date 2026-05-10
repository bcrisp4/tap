package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bcrisp4/tap/internal/db"
)

type entryListItemDTO struct {
	ID             int64  `json:"id"`
	SubscriptionID int64  `json:"subscription_id"`
	Title          string `json:"title"`
	Author         string `json:"author,omitempty"`
	URL            string `json:"url"`
	PublishedAt    int64  `json:"published_at"`
	FetchedAt      int64  `json:"fetched_at"`
	Read           bool   `json:"read"`
	Saved          bool   `json:"saved"`
	ExtractFailed  bool   `json:"extract_failed"`
}

type entryDetailDTO struct {
	entryListItemDTO
	Content string `json:"content"`
}

func toListItem(e db.Entry) entryListItemDTO {
	d := entryListItemDTO{
		ID:             e.ID,
		SubscriptionID: e.SubscriptionID,
		Title:          e.Title,
		URL:            e.URL,
		PublishedAt:    e.PublishedAt,
		FetchedAt:      e.FetchedAt,
		Read:           e.Read,
		Saved:          e.Saved,
		ExtractFailed:  e.ExtractFailed,
	}
	if e.Author.Valid {
		d.Author = e.Author.String
	}
	return d
}

func registerEntryRoutes(m *http.ServeMux, d *sql.DB) {
	m.HandleFunc("GET /api/v1/entries", func(w http.ResponseWriter, r *http.Request) {
		p := db.ListEntriesParams{
			UnreadOnly: r.URL.Query().Get("unread") == "1",
		}
		if v := r.URL.Query().Get("feed"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid feed id")
				return
			}
			p.SubscriptionID = id
		}
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid limit")
				return
			}
			p.Limit = n
		}
		if v := r.URL.Query().Get("cursor"); v != "" {
			// Cursor format: "<published_at>_<id>". Opaque to callers — they
			// just round-trip whatever next_cursor came back from the prior page.
			parts := strings.SplitN(v, "_", 2)
			if len(parts) != 2 {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid cursor")
				return
			}
			cp, err1 := strconv.ParseInt(parts[0], 10, 64)
			ci, err2 := strconv.ParseInt(parts[1], 10, 64)
			if err1 != nil || err2 != nil {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid cursor")
				return
			}
			p.CursorPublishedAt = cp
			p.CursorID = ci
		}

		entries, nextPub, nextID, err := db.ListEntries(r.Context(), d, p)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]entryListItemDTO, 0, len(entries))
		for _, e := range entries {
			out = append(out, toListItem(e))
		}
		resp := map[string]any{"data": out}
		if nextPub > 0 {
			resp["next_cursor"] = fmt.Sprintf("%d_%d", nextPub, nextID)
		}
		writeJSON(w, http.StatusOK, resp)
	})

	m.HandleFunc("GET /api/v1/entries/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		e, err := db.GetEntry(r.Context(), d, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "entry not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entryDetailDTO{entryListItemDTO: toListItem(e), Content: e.Content})
	})

	m.HandleFunc("PATCH /api/v1/entries/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			Read  *bool `json:"read"`
			Saved *bool `json:"saved"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if err := db.UpdateEntry(r.Context(), d, id, db.EntryUpdate{Read: body.Read, Saved: body.Saved}); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		e, err := db.GetEntry(r.Context(), d, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "entry not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toListItem(e))
	})
}
