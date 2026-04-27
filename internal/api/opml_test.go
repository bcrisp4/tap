package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

const opmlSample = `<?xml version="1.0"?>
<opml version="2.0">
  <head><title>Subscriptions</title></head>
  <body>
    <outline text="Tech">
      <outline type="rss" text="jvns.ca"   xmlUrl="https://jvns.ca/atom.xml"   htmlUrl="https://jvns.ca/"/>
      <outline type="rss" text="Dan Luu"   xmlUrl="https://danluu.com/atom.xml" htmlUrl="https://danluu.com/"/>
    </outline>
    <outline type="rss" text="lobste.rs" xmlUrl="https://lobste.rs/rss"/>
  </body>
</opml>`

func TestOPML_Import(t *testing.T) {
	f := newAPIFixture(t)
	w := f.doRaw(t, "POST", "/api/v1/opml/import", "application/xml",
		strings.NewReader(opmlSample))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"imported":3`)

	feeds, _ := f.store.ListFeeds(context.Background(), 1)
	require.Len(t, feeds, 3)

	cats, _ := f.store.ListCategories(context.Background(), 1)
	require.Len(t, cats, 1)
	require.Equal(t, "Tech", cats[0].Name)

	// The two Tech feeds should be tagged with the Tech category id.
	techCount := 0
	for _, fd := range feeds {
		if fd.CategoryID != nil && *fd.CategoryID == cats[0].ID {
			techCount++
		}
	}
	require.Equal(t, 2, techCount)
}

func TestOPML_Import_BadXML(t *testing.T) {
	f := newAPIFixture(t)
	w := f.doRaw(t, "POST", "/api/v1/opml/import", "application/xml",
		strings.NewReader("not valid xml <<<"))
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code":"bad_opml"`)
}

func TestOPML_Export(t *testing.T) {
	f := newAPIFixture(t)
	catID, err := f.store.CreateCategory(context.Background(), 1, "Tech")
	require.NoError(t, err)
	siteURL := "https://jvns.ca/"
	_, err = f.store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, CategoryID: &catID, Title: "jvns.ca",
		FeedURL: "https://jvns.ca/atom.xml", SiteURL: &siteURL, PollInterval: 3600,
	})
	require.NoError(t, err)

	w := f.do(t, "GET", "/api/v1/opml/export", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "xml")
	body := w.Body.String()
	require.Contains(t, body, "<opml")
	require.Contains(t, body, `xmlUrl="https://jvns.ca/atom.xml"`)
	require.Contains(t, body, `htmlUrl="https://jvns.ca/"`)
	require.Contains(t, body, `text="Tech"`)
	require.Contains(t, body, `text="jvns.ca"`)
}

func TestOPML_Export_EmptyOK(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "GET", "/api/v1/opml/export", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `<opml version="2.0"`)
}
