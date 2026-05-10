package db

import (
	"context"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExportOPML_Shape(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "opml_export")
	now := time.Now().Unix()

	catID, err := InsertCategory(context.Background(), d, NewCategory{UserID: userID, Name: "Tech", CreatedAt: now})
	require.NoError(t, err)

	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Go Blog", FeedURL: "https://go.dev/blog/feed.atom",
		SiteURL: "https://go.dev", Created: now,
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(), `UPDATE subscriptions SET category_id = ? WHERE id = ?`, catID, subID)
	require.NoError(t, err)

	_, err = InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Uncategorised", FeedURL: "https://example.com/feed", Created: now,
	})
	require.NoError(t, err)

	data, err := ExportOPML(context.Background(), d, userID)
	require.NoError(t, err)

	// Must be valid XML.
	var doc opmlDoc
	require.NoError(t, xml.Unmarshal(data, &doc))
	require.Equal(t, "2.0", doc.Version)

	// Find the category folder.
	var techFolder *opmlOutline
	var uncatOutlines []opmlOutline
	for i := range doc.Body.Outlines {
		o := doc.Body.Outlines[i]
		if strings.ToLower(o.Type) == "folder" && o.Text == "Tech" {
			techFolder = &doc.Body.Outlines[i]
		} else if o.XMLURL != "" {
			uncatOutlines = append(uncatOutlines, o)
		}
	}
	require.NotNil(t, techFolder, "Tech folder should appear in OPML")
	require.Len(t, techFolder.Children, 1)
	require.Equal(t, "https://go.dev/blog/feed.atom", techFolder.Children[0].XMLURL)

	require.Len(t, uncatOutlines, 1)
	require.Equal(t, "https://example.com/feed", uncatOutlines[0].XMLURL)
}

func TestImportOPML_Flat(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "opml_flat")
	now := time.Now().Unix()

	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Test</title></head>
<body>
  <outline type="rss" text="Feed1" title="Feed1" xmlUrl="https://flat1.com/feed" htmlUrl="https://flat1.com"/>
  <outline type="rss" text="Feed2" title="Feed2" xmlUrl="https://flat2.com/feed"/>
</body></opml>`

	imported, skipped, errs, err := ImportOPML(context.Background(), d, userID, []byte(opmlData), now)
	require.NoError(t, err)
	require.Equal(t, 2, imported)
	require.Equal(t, 0, skipped)
	require.Empty(t, errs)
}

func TestImportOPML_NestedFolder_CreatesCategory(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "opml_nested")
	now := time.Now().Unix()

	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Test</title></head>
<body>
  <outline type="folder" text="Tech">
    <outline type="rss" text="Go Blog" xmlUrl="https://go.dev/feed.atom"/>
    <outline type="rss" text="Rust Blog" xmlUrl="https://blog.rust-lang.org/feed.xml"/>
  </outline>
</body></opml>`

	imported, skipped, errs, err := ImportOPML(context.Background(), d, userID, []byte(opmlData), now)
	require.NoError(t, err)
	require.Equal(t, 2, imported)
	require.Equal(t, 0, skipped)
	require.Empty(t, errs)

	cats, err := ListCategories(context.Background(), d, userID)
	require.NoError(t, err)
	require.Len(t, cats, 1)
	require.Equal(t, "Tech", cats[0].Name)

	subs, err := ListSubscriptionsByCategory(context.Background(), d, cats[0].ID, userID)
	require.NoError(t, err)
	require.Len(t, subs, 2)
}

func TestImportOPML_DeeperNesting_FlattenedToParent(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "opml_deep")
	now := time.Now().Unix()

	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Test</title></head>
<body>
  <outline type="folder" text="Tech">
    <outline type="folder" text="Languages">
      <outline type="rss" text="Nested Feed" xmlUrl="https://nested.com/feed"/>
    </outline>
  </outline>
</body></opml>`

	imported, skipped, errs, err := ImportOPML(context.Background(), d, userID, []byte(opmlData), now)
	require.NoError(t, err)
	require.Equal(t, 1, imported)
	require.Equal(t, 0, skipped)
	require.Empty(t, errs)

	cats, err := ListCategories(context.Background(), d, userID)
	require.NoError(t, err)
	require.Len(t, cats, 1)
	require.Equal(t, "Tech", cats[0].Name)
}

func TestImportOPML_DuplicateFeedURL_Skipped(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "opml_dup")
	now := time.Now().Unix()

	_, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Existing", FeedURL: "https://dup.com/feed", Created: now,
	})
	require.NoError(t, err)

	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Test</title></head>
<body>
  <outline type="rss" text="Dup Feed" xmlUrl="https://dup.com/feed"/>
  <outline type="rss" text="New Feed" xmlUrl="https://new.com/feed"/>
</body></opml>`

	imported, skipped, errs, err := ImportOPML(context.Background(), d, userID, []byte(opmlData), now)
	require.NoError(t, err)
	require.Equal(t, 1, imported)
	require.Equal(t, 1, skipped)
	require.Empty(t, errs)
}

func TestImportOPML_InvalidURL_InErrors(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "opml_invalid")
	now := time.Now().Unix()

	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Test</title></head>
<body>
  <outline type="rss" text="Bad URL" xmlUrl="not-a-url"/>
  <outline type="rss" text="Good Feed" xmlUrl="https://good.com/feed"/>
</body></opml>`

	imported, skipped, errs, err := ImportOPML(context.Background(), d, userID, []byte(opmlData), now)
	require.NoError(t, err)
	require.Equal(t, 1, imported)
	require.Equal(t, 0, skipped)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0], "invalid URL")
}

func TestImportOPML_Idempotent(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "opml_idempotent")
	now := time.Now().Unix()

	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Test</title></head>
<body>
  <outline type="rss" text="Feed1" xmlUrl="https://idm1.com/feed"/>
  <outline type="rss" text="Feed2" xmlUrl="https://idm2.com/feed"/>
</body></opml>`

	imp1, skip1, errs1, err := ImportOPML(context.Background(), d, userID, []byte(opmlData), now)
	require.NoError(t, err)
	require.Equal(t, 2, imp1)
	require.Equal(t, 0, skip1)
	require.Empty(t, errs1)

	imp2, skip2, errs2, err := ImportOPML(context.Background(), d, userID, []byte(opmlData), now)
	require.NoError(t, err)
	require.Equal(t, 0, imp2)
	require.Equal(t, 2, skip2)
	require.Empty(t, errs2)
}

func TestImportOPML_ParseError_Rollback(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "opml_rollback")
	now := time.Now().Unix()

	_, _, _, err := ImportOPML(context.Background(), d, userID, []byte("not xml at all"), now)
	require.Error(t, err, "invalid OPML must return error")

	subs, err := ListSubscriptions(context.Background(), d, userID)
	require.NoError(t, err)
	require.Empty(t, subs, "no subscriptions should exist after parse error")
}
