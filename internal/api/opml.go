package api

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/storage"
)

// maxOPMLBodyBytes caps inbound OPML payloads. A typical subscription
// list is well under 100KB; 4MiB leaves headroom for power users with
// thousands of feeds while keeping a runaway client from exhausting
// memory.
const maxOPMLBodyBytes int64 = 4 << 20

// opmlOutline is one <outline> node. Container outlines (categories)
// have Children; feed outlines have XMLURL.
type opmlOutline struct {
	XMLName  xml.Name      `xml:"outline"`
	Type     string        `xml:"type,attr,omitempty"`
	Text     string        `xml:"text,attr"`
	Title    string        `xml:"title,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Children []opmlOutline `xml:"outline"`
}

// opmlDoc is the top-level OPML 2.0 document.
type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    struct {
		Title string `xml:"title"`
	} `xml:"head"`
	Body struct {
		Outlines []opmlOutline `xml:"outline"`
	} `xml:"body"`
}

type opmlHandlers struct {
	store *storage.Store
}

// importHandler accepts an OPML 2.0 document and walks the outline
// tree. Container outlines (no xmlUrl) become categories; feed outlines
// inherit the surrounding category. Duplicate feed URLs (UNIQUE in
// the schema) are silently skipped via storage.ErrConflict so a
// re-import is idempotent; any other CreateFeed error aborts the
// import with 500 so failures aren't masked by a partial 201.
func (h *opmlHandlers) importHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxOPMLBodyBytes))
	if err != nil {
		WriteError(w, http.StatusRequestEntityTooLarge, "body_too_large", err.Error())
		return
	}
	var doc opmlDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_opml", err.Error())
		return
	}

	imported, err := h.importOutlines(r, doc.Body.Outlines)
	if err != nil {
		writeErr(w, err)
		return
	}

	WriteOK(w, http.StatusCreated, struct {
		Imported int `json:"imported"`
	}{Imported: imported})
}

// importOutlines walks the outline forest and returns the count of
// successfully-inserted feeds. Duplicate URLs (ErrConflict) are
// silently skipped; every other error short-circuits the walk.
func (h *opmlHandlers) importOutlines(r *http.Request, outlines []opmlOutline) (int, error) {
	imported := 0
	now := time.Now().Unix()

	var walk func(o opmlOutline, categoryID *int64) error
	walk = func(o opmlOutline, categoryID *int64) error {
		if o.XMLURL != "" {
			_, err := h.store.CreateFeed(r.Context(), opmlFeedToStorage(o, categoryID, now))
			if errors.Is(err, storage.ErrConflict) {
				return nil
			}
			if err != nil {
				return err
			}
			imported++
			return nil
		}
		// Container outline → category. Empty-text containers
		// (sometimes seen in exporters that lump uncategorised feeds
		// under an unnamed group) propagate the parent category id.
		newCatID := categoryID
		if o.Text != "" {
			id, err := h.store.CreateCategory(r.Context(), userID, o.Text)
			if err != nil {
				return err
			}
			newCatID = &id
		}
		for _, child := range o.Children {
			if err := walk(child, newCatID); err != nil {
				return err
			}
		}
		return nil
	}
	for _, o := range outlines {
		if err := walk(o, nil); err != nil {
			return imported, err
		}
	}
	return imported, nil
}

// opmlFeedToStorage converts a feed-shaped <outline> into a
// storage.Feed, picking the title from text → title → xmlUrl in that
// order.
func opmlFeedToStorage(o opmlOutline, categoryID *int64, scheduledAt int64) *storage.Feed {
	title := o.Text
	if title == "" {
		title = o.Title
	}
	if title == "" {
		title = o.XMLURL
	}
	feed := &storage.Feed{
		UserID:       userID,
		CategoryID:   categoryID,
		Title:        title,
		FeedURL:      o.XMLURL,
		PollInterval: 3600,
		NextPollAt:   &scheduledAt,
	}
	if o.HTMLURL != "" {
		htmlURL := o.HTMLURL
		feed.SiteURL = &htmlURL
	}
	return feed
}

// exportHandler emits an OPML 2.0 document with all feeds for the
// single user, grouped under their category outlines (uncategorised
// feeds go at the top level).
func (h *opmlHandlers) exportHandler(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.store.ListFeeds(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	cats, err := h.store.ListCategories(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}

	doc := opmlDoc{Version: "2.0"}
	doc.Head.Title = "Tap subscriptions"
	doc.Body.Outlines = buildExportOutlines(feeds, cats)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="tap-subscriptions.opml"`)
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(doc)
}

// buildExportOutlines groups feeds into per-category container
// outlines, with uncategorised feeds at the top level. Categories
// without feeds emerge as empty containers — that's fine: they round-
// trip cleanly on import.
func buildExportOutlines(feeds []*storage.Feed, cats []*storage.Category) []opmlOutline {
	groups := map[int64][]opmlOutline{}
	uncategorised := []opmlOutline{}
	for _, f := range feeds {
		o := opmlOutline{Type: "rss", Text: f.Title, XMLURL: f.FeedURL}
		if f.SiteURL != nil {
			o.HTMLURL = *f.SiteURL
		}
		if f.CategoryID != nil {
			groups[*f.CategoryID] = append(groups[*f.CategoryID], o)
		} else {
			uncategorised = append(uncategorised, o)
		}
	}

	out := make([]opmlOutline, 0, len(cats)+len(uncategorised))
	for _, c := range cats {
		out = append(out, opmlOutline{Text: c.Name, Children: groups[c.ID]})
	}
	out = append(out, uncategorised...)
	return out
}
