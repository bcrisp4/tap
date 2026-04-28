package api

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/bcrisp4/tap/internal/storage"
)

type entryHandlers struct {
	store *storage.Store
}

func (h *entryHandlers) list(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseEntriesFilter(w, r)
	if !ok {
		return
	}
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
	WriteList(w, stripContent(entries), filter.Limit, filter.Offset, total)
}

// stripContent clones the slice and clears the Content field on each
// entry. Design.md §6 keeps list payloads small by excluding the full
// article body; the SPA fetches it via /entries/{id} when needed.
func stripContent(entries []*storage.Entry) []*storage.Entry {
	out := make([]*storage.Entry, len(entries))
	for i, e := range entries {
		c := *e
		c.Content = nil
		out[i] = &c
	}
	return out
}

// parseEntriesFilter pulls the query params documented in design.md §6
// into an EntriesFilter. Defaults: status=unread, sort=published_at,
// order=desc, limit=50, offset=0.
//
// The `order` param is overloaded: legacy values ("asc"/"desc") drive
// the sort direction (Order); the Plan 14 value "read_at" drives the
// column choice (OrderBy) so /history can opt into read_at DESC. Any
// other value yields a 400 bad_query so we never pass user input
// through to the SQL builder. Returns ok=false after writing an error
// response; callers must early-return.
func parseEntriesFilter(w http.ResponseWriter, r *http.Request) (storage.EntriesFilter, bool) {
	q := r.URL.Query()
	f := storage.EntriesFilter{
		Status: q.Get("status"),
		Sort:   q.Get("sort"),
	}
	switch v := q.Get("order"); v {
	case "", "desc", "asc":
		f.Order = v
	case "read_at":
		f.OrderBy = "read_at"
	default:
		WriteError(w, http.StatusBadRequest, "bad_query",
			"order must be asc, desc, or read_at")
		return storage.EntriesFilter{}, false
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
	return f, true
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
	id, ok := requirePathID(w, r, "entry")
	if !ok {
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
	id, ok := requirePathID(w, r, "entry")
	if !ok {
		return
	}
	var req entryStateReq
	if !decodeJSON(w, r, &req) {
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
	if !decodeJSON(w, r, &req) {
		return
	}
	scope := storage.BulkScope{FeedID: req.FeedID, CategoryID: req.CategoryID}
	if err := h.store.BulkMarkRead(r.Context(), userID, scope); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
