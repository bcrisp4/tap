package proxy

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCache_PutThenGet(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)

	url := "https://cdn.example.com/x.png"
	body := []byte{0x89, 'P', 'N', 'G'}
	require.NoError(t, c.Put(url, body, "image/png", `"v1"`))

	got, ok, err := c.Get(url)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "image/png", got.ContentType)
	require.Equal(t, `"v1"`, got.ETag)
	require.Equal(t, body, got.Body)
}

func TestCache_GetMiss(t *testing.T) {
	c := NewCache(t.TempDir())
	_, ok, err := c.Get("https://nope")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestCache_LayoutShardedByHashPrefix(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)
	require.NoError(t, c.Put("https://x/y", []byte("hi"), "image/png", ""))

	// One subdirectory matching the SHA-256 prefix.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.True(t, entries[0].IsDir())
	require.Len(t, entries[0].Name(), 2, "shard dir is 2 hex chars")
}

func TestCache_EvictByOldestMtime(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)

	// Three 1KB entries.
	for i := 0; i < 3; i++ {
		url := "https://x/" + string(rune('a'+i))
		require.NoError(t, c.Put(url, make([]byte, 1024), "image/png", ""))
		// Stagger mtimes so the oldest is unambiguous.
		time.Sleep(20 * time.Millisecond)
	}

	// Cap to 2KB → expect oldest entry gone.
	require.NoError(t, c.Evict(2*1024))

	_, ok, _ := c.Get("https://x/a")
	require.False(t, ok, "oldest must have been evicted")
	_, ok, _ = c.Get("https://x/c")
	require.True(t, ok, "newest must remain")
}

func TestCache_PutOverwrites(t *testing.T) {
	c := NewCache(t.TempDir())
	url := "https://x"
	require.NoError(t, c.Put(url, []byte("v1"), "image/png", ""))
	require.NoError(t, c.Put(url, []byte("v2"), "image/png", ""))
	got, _, _ := c.Get(url)
	require.Equal(t, []byte("v2"), got.Body)
}

func TestCache_PathSplittingNoCollisions(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)
	require.NoError(t, c.Put("https://x/a", []byte("a"), "image/png", ""))
	require.NoError(t, c.Put("https://x/b", []byte("b"), "image/png", ""))

	got1, _, _ := c.Get("https://x/a")
	got2, _, _ := c.Get("https://x/b")
	require.NotEqual(t, got1.Body, got2.Body)
	require.False(t, strings.Contains(string(got1.Body), "b"))
}
