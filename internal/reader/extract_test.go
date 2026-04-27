package reader_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/reader"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(body)
}

func TestExtract_ReadabilityFallback(t *testing.T) {
	html := loadFixture(t, "article.html")
	got, err := reader.Extract(html, "https://example.test/blog/2026/04/26/synthetic/", "")
	require.NoError(t, err)
	require.NotEmpty(t, got)

	// Article body retained.
	require.Contains(t, got, "Readability heuristics")
	require.Contains(t, got, "go test ./internal/reader/...")
	// Chrome stripped.
	require.NotContains(t, got, "Subscribe")
	require.NotContains(t, got, "Tap project test fixture")
}

func TestExtract_ScraperRulesWin(t *testing.T) {
	html := `<html><body>
		<aside class="sidebar"><h2>Sidebar</h2></aside>
		<main><article class="post"><p>Body text</p></article></main>
	</body></html>`
	// Selector picks main > article.post; aside must not appear.
	got, err := reader.Extract(html, "https://x/y", "main article.post")
	require.NoError(t, err)
	require.Contains(t, got, "Body text")
	require.NotContains(t, got, "Sidebar")
}

func TestExtract_NoMatchReturnsInput(t *testing.T) {
	html := `<p>tiny stub</p>`
	got, err := reader.Extract(html, "https://x", "")
	require.NoError(t, err)
	require.True(t, strings.Contains(got, "tiny stub"),
		"falls back to input when neither rules nor Readability finds anything")
}
