package feedparse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntryHash_StableForSameInputs(t *testing.T) {
	h1 := EntryHash(42, "https://example.com/post-1", "https://example.com/post-1", "Hello", 0)
	h2 := EntryHash(42, "https://example.com/post-1", "https://example.com/post-1", "Hello", 0)
	require.Equal(t, h1, h2)
	require.Len(t, h1, 64, "sha256 hex digest is 64 chars")
}

func TestEntryHash_DifferentFeedsNeverCollide(t *testing.T) {
	h1 := EntryHash(1, "g", "u", "T", 0)
	h2 := EntryHash(2, "g", "u", "T", 0)
	require.NotEqual(t, h1, h2)
}

func TestEntryHash_PrefersGUIDOverLink(t *testing.T) {
	withGUID := EntryHash(1, "stable-guid", "https://example.com/x", "T", 0)
	noGUID := EntryHash(1, "", "https://example.com/x", "T", 0)
	require.NotEqual(t, withGUID, noGUID)
}

func TestEntryHash_FallbackToSynthesisedKey(t *testing.T) {
	// Some feeds (Hacker News, certain JSON Feeds) provide neither GUID
	// nor link. We fall back to title+published_at.
	h := EntryHash(1, "", "", "Title only", 1700000000)
	require.NotEmpty(t, h)
}

func TestEntryHash_TitleChangeDoesNotChangeHashWhenGUIDStable(t *testing.T) {
	// Stability requirement: re-titled entries shouldn't reappear as new.
	h1 := EntryHash(1, "guid-stable", "https://x.com/a", "Old Title", 1700000000)
	h2 := EntryHash(1, "guid-stable", "https://x.com/a", "New Title", 1700000000)
	require.Equal(t, h1, h2)
}
