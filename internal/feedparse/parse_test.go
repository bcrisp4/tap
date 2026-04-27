package feedparse_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/feedparse"
)

func loadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return body
}

func TestParse_Atom(t *testing.T) {
	body := loadTestdata(t, "jvns.atom.xml")
	got, err := feedparse.Parse(body, "https://jvns.ca/feed.atom")
	require.NoError(t, err)

	require.Equal(t, "jvns.ca", got.Meta.Title)
	require.Equal(t, "https://jvns.ca/", got.Meta.SiteURL)
	require.Len(t, got.Entries, 1)

	e := got.Entries[0]
	require.Equal(t, "Reasons why bugs might feel \"impossible\"", e.Title)
	require.Equal(t, "https://jvns.ca/blog/2026/04/26/bugs-impossible/", *e.URL)
	require.NotNil(t, e.PublishedAt)
	require.Greater(t, *e.PublishedAt, int64(0))
	require.NotEmpty(t, e.Hash, "hash must be set")
	require.Greater(t, e.ReadingTime, 0, "reading time must be > 0")
}

func TestParse_RSS(t *testing.T) {
	body := loadTestdata(t, "lobsters.rss.xml")
	got, err := feedparse.Parse(body, "https://lobste.rs/rss")
	require.NoError(t, err)

	require.Equal(t, "Lobsters", got.Meta.Title)
	require.Len(t, got.Entries, 1)
	require.Equal(t, "Show: I wrote a tiny SQLite-backed task queue in 200 lines of Go", got.Entries[0].Title)
}

func TestParse_JSONFeed(t *testing.T) {
	body := loadTestdata(t, "jsonfeed.json")
	got, err := feedparse.Parse(body, "https://example.com/feed.json")
	require.NoError(t, err)

	require.Equal(t, "Example JSON Feed", got.Meta.Title)
	require.Len(t, got.Entries, 1)
	require.Equal(t, "Hello JSON Feed", got.Entries[0].Title)
}

func TestParse_HashStableAcrossCalls(t *testing.T) {
	body := loadTestdata(t, "jvns.atom.xml")
	r1, err := feedparse.Parse(body, "https://jvns.ca/feed.atom")
	require.NoError(t, err)
	r2, err := feedparse.Parse(body, "https://jvns.ca/feed.atom")
	require.NoError(t, err)
	require.Equal(t, r1.Entries[0].Hash, r2.Entries[0].Hash, "hash must be deterministic")
}

func TestParse_InvalidFeedURLErrors(t *testing.T) {
	body := loadTestdata(t, "jvns.atom.xml")
	// Control bytes inside a URL trip url.Parse; the body is irrelevant —
	// we should reject before parsing it.
	_, err := feedparse.Parse(body, "https://example.com/\x7f")
	require.Error(t, err)
	require.Contains(t, err.Error(), "feedURL")
}

func TestParse_RelativeLinkResolved(t *testing.T) {
	atom := []byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
		<title>X</title><id>https://x.com/</id>
		<entry><id>e1</id><title>T</title>
		<link href="/posts/1"/></entry></feed>`)
	got, err := feedparse.Parse(atom, "https://x.com/feed.atom")
	require.NoError(t, err)
	require.Equal(t, "https://x.com/posts/1", *got.Entries[0].URL)
}
