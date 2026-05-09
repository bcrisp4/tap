package feed

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/require"
)

func TestEntryHash_PrefersGUID(t *testing.T) {
	t.Parallel()
	pub := time.Unix(1700000000, 0)
	item := &gofeed.Item{GUID: "stable-guid", Link: "https://example.com/a", Title: "A", PublishedParsed: &pub}
	h := EntryHash(42, item)
	require.NotEmpty(t, h)

	// Same feed + same GUID = same hash, regardless of other fields.
	item.Link = "different"
	item.Title = "different"
	require.Equal(t, h, EntryHash(42, item))

	// Different feed = different hash even with the same GUID.
	require.NotEqual(t, h, EntryHash(99, item))
}

func TestEntryHash_FallsBackToURL(t *testing.T) {
	t.Parallel()
	item := &gofeed.Item{Link: "https://example.com/a", Title: "A"}
	h := EntryHash(1, item)
	require.NotEmpty(t, h)

	item.Title = "different title"
	require.Equal(t, h, EntryHash(1, item), "URL fallback ignores title")
}

func TestEntryHash_FallsBackToTitlePlusDate(t *testing.T) {
	t.Parallel()
	pub := time.Unix(1700000000, 0)
	item := &gofeed.Item{Title: "A", PublishedParsed: &pub}
	h := EntryHash(1, item)
	require.NotEmpty(t, h)

	pub2 := time.Unix(1800000000, 0)
	item2 := &gofeed.Item{Title: "A", PublishedParsed: &pub2}
	require.NotEqual(t, h, EntryHash(1, item2), "different dates produce different hashes")
}
