package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInsertCategory_Roundtrip(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "alice")
	id, err := InsertCategory(context.Background(), d, NewCategory{
		UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)
	require.Positive(t, id)

	cats, err := ListCategories(context.Background(), d, userID)
	require.NoError(t, err)
	require.Len(t, cats, 1)
	require.Equal(t, "Tech", cats[0].Name)
	require.Equal(t, 0, cats[0].Unread)
}

func TestInsertCategory_DuplicateNameSameUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "bob")
	nc := NewCategory{UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix()}
	_, err := InsertCategory(context.Background(), d, nc)
	require.NoError(t, err)
	_, err = InsertCategory(context.Background(), d, nc)
	require.ErrorIs(t, err, ErrCategoryNameTaken)
}

func TestInsertCategory_SameNameDifferentUsers(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "carol")
	u2 := insertTestUser(t, d, "dave")
	now := time.Now().Unix()
	_, err := InsertCategory(context.Background(), d, NewCategory{UserID: u1, Name: "Tech", CreatedAt: now})
	require.NoError(t, err)
	_, err = InsertCategory(context.Background(), d, NewCategory{UserID: u2, Name: "Tech", CreatedAt: now})
	require.NoError(t, err, "same name for different users should be allowed")
}

func TestGetCategory_WrongUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "eve")
	u2 := insertTestUser(t, d, "frank")
	id, err := InsertCategory(context.Background(), d, NewCategory{UserID: u1, Name: "Tech", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	_, err = GetCategory(context.Background(), d, id, u2)
	require.ErrorIs(t, err, ErrCategoryNotFound)
}

func TestDeleteCategory_SetsSubscriptionCategoryToNull(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "grace")
	catID, err := InsertCategory(context.Background(), d, NewCategory{UserID: userID, Name: "Tech", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)

	// Insert a subscription assigned to this category.
	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID:  userID,
		Title:   "Test Feed",
		FeedURL: "https://example.com/feed",
		Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`UPDATE subscriptions SET category_id = ? WHERE id = ?`, catID, subID)
	require.NoError(t, err)

	// Delete the category.
	require.NoError(t, DeleteCategory(context.Background(), d, catID, userID))

	// Subscription still exists but category_id is now NULL.
	sub, err := GetSubscription(context.Background(), d, subID, userID)
	require.NoError(t, err)
	require.False(t, sub.CategoryID.Valid, "category_id should be NULL after category deletion")
}

func TestMarkCategoryRead_ScopedToUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "heidi")
	u2 := insertTestUser(t, d, "ivan")

	// User 1: category + subscription + entry.
	catID, err := InsertCategory(context.Background(), d, NewCategory{UserID: u1, Name: "Tech", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	subID, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: u1, Title: "Feed", FeedURL: "https://u1.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`UPDATE subscriptions SET category_id = ? WHERE id = ?`, catID, subID)
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h1', 'Entry1', '', 'https://u1.com/1', '<p>text</p>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), u1)
	require.NoError(t, err)

	// User 2: subscription + entry (no category).
	subID2, err := InsertSubscription(context.Background(), d, NewSubscription{
		UserID: u2, Title: "Feed2", FeedURL: "https://u2.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h2', 'Entry2', '', 'https://u2.com/1', '<p>text</p>', ?, ?, 0, 0, ?)`,
		subID2, time.Now().Unix(), time.Now().Unix(), u2)
	require.NoError(t, err)

	// Mark u1's category as read.
	require.NoError(t, MarkCategoryRead(context.Background(), d, catID, u1))

	// u1's entry should be read.
	var u1Read int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT read FROM entries WHERE subscription_id = ?`, subID).Scan(&u1Read))
	require.Equal(t, 1, u1Read)

	// u2's entry should still be unread.
	var u2Read int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT read FROM entries WHERE subscription_id = ?`, subID2).Scan(&u2Read))
	require.Equal(t, 0, u2Read)
}

func TestUpdateCategoryName(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "judy")
	id, err := InsertCategory(context.Background(), d, NewCategory{UserID: userID, Name: "Old", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)

	require.NoError(t, UpdateCategoryName(context.Background(), d, id, userID, "New"))
	cat, err := GetCategory(context.Background(), d, id, userID)
	require.NoError(t, err)
	require.Equal(t, "New", cat.Name)
}

func TestUpdateCategoryName_WrongUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "ken")
	u2 := insertTestUser(t, d, "leo")
	id, err := InsertCategory(context.Background(), d, NewCategory{UserID: u1, Name: "Tech", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	err = UpdateCategoryName(context.Background(), d, id, u2, "New")
	require.ErrorIs(t, err, ErrCategoryNotFound)
}
