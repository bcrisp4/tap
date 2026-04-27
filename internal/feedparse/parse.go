package feedparse

import (
	"bytes"
	"fmt"
	"net/url"

	"github.com/mmcdole/gofeed"

	"github.com/bcrisp4/tap/internal/storage"
)

// FeedMeta holds normalised feed-level fields.
type FeedMeta struct {
	Title       string
	SiteURL     string
	Description string
}

// Result is what Parse returns: feed metadata + entries (with feed_id
// and user_id left zeroed for the caller to fill in).
type Result struct {
	Meta    FeedMeta
	Entries []*storage.Entry
}

// Parse normalises an RSS / Atom / JSON Feed body into Tap's shape.
// feedURL is used to (a) resolve relative entry links and (b) seed the
// hash for entries lacking GUID and link.
//
// Note on EntryHash and feed_id: Parse doesn't know the persistent
// feed_id (the row may not exist on first subscribe, or the caller
// could be re-parsing). The poller (Plan 07) recomputes the hash with
// the real feed_id when committing. The hash returned here is good
// enough for in-memory dedup within one parse call; the canonical hash
// is the one written to entries.hash.
func Parse(body []byte, feedURL string) (*Result, error) {
	fp := gofeed.NewParser()
	feed, err := fp.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("feedparse: %w", err)
	}

	base, _ := url.Parse(feedURL)

	out := &Result{
		Meta: FeedMeta{
			Title:       feed.Title,
			SiteURL:     resolveOrEmpty(base, feed.Link),
			Description: feed.Description,
		},
	}
	for _, item := range feed.Items {
		e := normaliseItem(item, base)
		// FeedID is zero here — the caller fills it in. We pass 0 to
		// EntryHash for now; the caller is expected to recompute the
		// hash with the real feed_id at insert time.
		guid := item.GUID
		link := stringOr(e.URL)
		title := e.Title
		pub := int64Or(e.PublishedAt)
		e.Hash = EntryHash(0, guid, link, title, pub)
		// Reading-time uses content if present, falling back to summary.
		text := stringOr(e.Content)
		if text == "" {
			text = stringOr(e.Summary)
		}
		e.ReadingTime = Minutes(text)
		out.Entries = append(out.Entries, e)
	}
	return out, nil
}

func normaliseItem(item *gofeed.Item, base *url.URL) *storage.Entry {
	e := &storage.Entry{
		Title: item.Title,
	}
	if u := resolveOrEmpty(base, item.Link); u != "" {
		e.URL = &u
	}
	if a := item.Author; a != nil && a.Name != "" {
		name := a.Name
		e.Author = &name
	}
	if s := item.Description; s != "" {
		e.Summary = &s
	}
	if c := item.Content; c != "" {
		e.Content = &c
	}
	if t := item.PublishedParsed; t != nil {
		ts := t.Unix()
		e.PublishedAt = &ts
	}
	return e
}

func resolveOrEmpty(base *url.URL, raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if base == nil || u.IsAbs() {
		return u.String()
	}
	return base.ResolveReference(u).String()
}

func stringOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func int64Or(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
