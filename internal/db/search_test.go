package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSearchEntries_InsertAndFind(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "search1")
	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Feed", FeedURL: "https://s1.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h1', 'Golang Tutorial', 'alice', 'https://s1.com/1', '<p>Learn Go programming</p>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), userID)
	require.NoError(t, err)

	results, err := SearchEntries(context.Background(), d, userID, "Golang", 50)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Golang Tutorial", results[0].Title)
}

func TestSearchEntries_DeleteEntry_NotSearchable(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "search2")
	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Feed", FeedURL: "https://s2.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	res, err := d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h2', 'UniqueSearchableTerm', 'bob', 'https://s2.com/1', '<p>foo</p>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), userID)
	require.NoError(t, err)
	entryID, _ := res.LastInsertId()

	results, err := SearchEntries(context.Background(), d, userID, "UniqueSearchableTerm", 50)
	require.NoError(t, err)
	require.Len(t, results, 1)

	_, err = d.ExecContext(context.Background(), `DELETE FROM entries WHERE id = ?`, entryID)
	require.NoError(t, err)

	results, err = SearchEntries(context.Background(), d, userID, "UniqueSearchableTerm", 50)
	require.NoError(t, err)
	require.Empty(t, results, "deleted entry must not appear in search")
}

func TestSearchEntries_UpdateTitle_ReflectsNewTitle(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "search3")
	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Feed", FeedURL: "https://s3.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	res, err := d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h3', 'OldTitle123', 'carol', 'https://s3.com/1', '<p>content</p>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), userID)
	require.NoError(t, err)
	entryID, _ := res.LastInsertId()

	_, err = d.ExecContext(context.Background(), `UPDATE entries SET title = 'NewTitle456' WHERE id = ?`, entryID)
	require.NoError(t, err)

	results, err := SearchEntries(context.Background(), d, userID, "NewTitle456", 50)
	require.NoError(t, err)
	require.Len(t, results, 1)

	results, err = SearchEntries(context.Background(), d, userID, "OldTitle123", 50)
	require.NoError(t, err)
	require.Empty(t, results, "old title must not appear after update")
}

func TestSearchEntries_CrossUserIsolation(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "search_u1")
	u2 := insertTestUser(t, d, "search_u2")

	subID1, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: u1, Title: "FeedA", FeedURL: "https://ua.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'hx', 'PrivateTermXYZ', 'alice', 'https://ua.com/1', '<p>secret</p>', ?, ?, 0, 0, ?)`,
		subID1, time.Now().Unix(), time.Now().Unix(), u1)
	require.NoError(t, err)

	results, err := SearchEntries(context.Background(), d, u2, "PrivateTermXYZ", 50)
	require.NoError(t, err)
	require.Empty(t, results, "cross-user search must return no results")
}

func TestSearchEntries_HTMLTagsNotSearchable(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "search4")
	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Feed", FeedURL: "https://s4.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h4', 'Title', 'author', 'https://s4.com/1', '<article>plaintext here</article>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), userID)
	require.NoError(t, err)

	// The tag name "article" is stripped; only "plaintext here" is indexed.
	// Searching for the tag name alone must return 0 results.
	results, err := SearchEntries(context.Background(), d, userID, "article", 50)
	require.NoError(t, err)
	require.Empty(t, results, "HTML tag name must not be searchable after tap_strip_html")

	// The text content must still be searchable.
	results, err = SearchEntries(context.Background(), d, userID, "plaintext", 50)
	require.NoError(t, err)
	require.Len(t, results, 1, "text content should be searchable")
}

func TestSearchEntries_AuthorSearchable(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "search5")
	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Feed", FeedURL: "https://s5.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h5', 'Some Title', 'AuthorUniqueZZZ', 'https://s5.com/1', '<p>content</p>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), userID)
	require.NoError(t, err)

	results, err := SearchEntries(context.Background(), d, userID, "AuthorUniqueZZZ", 50)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "AuthorUniqueZZZ", results[0].Author)
}

func TestSearchEntries_LimitRespected(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "search6")
	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: userID, Title: "Feed", FeedURL: "https://s6.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	for i := range 5 {
		_, err = d.ExecContext(context.Background(),
			`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
			 VALUES (?, ?, 'LimitTest entry', 'auth', ?, '<p>content</p>', ?, ?, 0, 0, ?)`,
			subID, fmt.Sprintf("hp%d", i), fmt.Sprintf("https://s6.com/%d", i),
			time.Now().Unix(), time.Now().Unix(), userID)
		require.NoError(t, err)
	}

	results, err := SearchEntries(context.Background(), d, userID, "LimitTest", 3)
	require.NoError(t, err)
	require.Len(t, results, 3)
}
