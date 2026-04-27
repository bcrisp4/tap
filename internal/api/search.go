package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/bcrisp4/tap/internal/storage"
)

type searchHandlers struct {
	store *storage.Store
}

func (h *searchHandlers) handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		WriteError(w, http.StatusBadRequest, "missing_query", "q is required")
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	results, err := h.store.SearchEntries(r.Context(), userID, query, limit, offset)
	if err != nil {
		if errors.Is(err, storage.ErrBadQuery) {
			WriteError(w, http.StatusBadRequest, "bad_query", err.Error())
			return
		}
		writeErr(w, err)
		return
	}

	// design.md §6: list payloads exclude content. Match the /entries
	// shape so the SPA can hand search hits to the same renderer.
	stripped := make([]*storage.Entry, len(results))
	for i, e := range results {
		c := *e
		c.Content = nil
		stripped[i] = &c
	}
	WriteList(w, stripped, limit, offset, len(stripped))
}
