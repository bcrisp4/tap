package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/bcrisp4/tap/internal/storage"
)

type entryHandlers struct {
	store *storage.Store
}

func (h *entryHandlers) list(w http.ResponseWriter, r *http.Request) {
	filter := parseEntriesFilter(r)
	entries, err := h.store.ListEntries(r.Context(), userID, filter)
	if err != nil {
		writeErr(w, err)
		return
	}
	total, err := h.store.CountEntries(r.Context(), userID, filter)
	if err != nil {
		writeErr(w, err)
		return
	}
	// Strip content from list payloads (design.md §6: full content is
	// fetched via /entries/{id}). Cloning is cheap: Entry is a value
	// type with pointer fields we set to nil.
	stripped := make([]*storage.Entry, len(entries))
	for i, e := range entries {
		c := *e
		c.Content = nil
		stripped[i] = &c
	}
	WriteList(w, stripped, filter.Limit, filter.Offset, total)
}

// parseEntriesFilter pulls the query params documented in design.md §6
// into an EntriesFilter. Defaults: status=unread, sort=published_at,
// order=desc, limit=50, offset=0.
func parseEntriesFilter(r *http.Request) storage.EntriesFilter {
	q := r.URL.Query()
	f := storage.EntriesFilter{
		Status: q.Get("status"),
		Sort:   q.Get("sort"),
		Order:  q.Get("order"),
	}
	if f.Status == "" {
		f.Status = "unread"
	}
	if f.Status == "all" {
		f.Status = ""
	}
	if v := q.Get("saved"); v == "true" || v == "false" {
		b := v == "true"
		f.Saved = &b
	}
	if id, ok := queryInt(q, "feed_id"); ok {
		f.FeedID = &id
	}
	if id, ok := queryInt(q, "category_id"); ok {
		f.CategoryID = &id
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("offset"); v != "" {
		f.Offset, _ = strconv.Atoi(v)
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	return f
}

// queryInt parses an integer query parameter; ok is false on missing
// or malformed input so the caller can leave the filter pointer nil.
func queryInt(q url.Values, key string) (int64, bool) {
	v := q.Get(key)
	if v == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// entryWithEnclosures is the wire shape for /entries/{id}. design.md
// §6 promises content + enclosures here.
type entryWithEnclosures struct {
	*storage.Entry
	Enclosures []*storage.Enclosure `json:"enclosures"`
}

func (h *entryHandlers) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		WriteError(w, http.StatusBadRequest, "bad_id", "entry id must be integer")
		return
	}
	e, err := h.store.GetEntry(r.Context(), userID, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	encs, err := h.store.ListEnclosures(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if encs == nil {
		encs = []*storage.Enclosure{}
	}
	WriteOK(w, http.StatusOK, entryWithEnclosures{Entry: e, Enclosures: encs})
}

type entryStateReq struct {
	Read  *bool `json:"read,omitempty"`
	Saved *bool `json:"saved,omitempty"`
}

func (h *entryHandlers) put(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		WriteError(w, http.StatusBadRequest, "bad_id", "entry id must be integer")
		return
	}
	var req entryStateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if err := h.store.UpdateEntryState(r.Context(), userID, id, req.Read, req.Saved); err != nil {
		writeErr(w, err)
		return
	}
	updated, err := h.store.GetEntry(r.Context(), userID, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, updated)
}

type bulkReq struct {
	FeedID     *int64 `json:"feed_id,omitempty"`
	CategoryID *int64 `json:"category_id,omitempty"`
}

func (h *entryHandlers) bulkRead(w http.ResponseWriter, r *http.Request) {
	var req bulkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	scope := storage.BulkScope{FeedID: req.FeedID, CategoryID: req.CategoryID}
	if err := h.store.BulkMarkRead(r.Context(), userID, scope); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
