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

func TestInsertCategory_AssignsNextPosition(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "alice-nextpos")
	u2 := insertTestUser(t, d, "bob-nextpos")
	mk := func(uid int64, name string) Category {
		id, err := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: name, CreatedAt: time.Now().Unix()})
		require.NoError(t, err)
		c, err := GetCategory(context.Background(), d, id, uid)
		require.NoError(t, err)
		return c
	}
	a1 := mk(u1, "A")
	a2 := mk(u1, "B")
	a3 := mk(u1, "C")
	b1 := mk(u2, "X") // different user starts at 0 independently
	require.Equal(t, int64(0), a1.Position)
	require.Equal(t, int64(1), a2.Position)
	require.Equal(t, int64(2), a3.Position)
	require.Equal(t, int64(0), b1.Position)
}

func TestListCategories_OrdersByPosition(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertTestUser(t, d, "alice-pos")
	// Insert three categories; we set positions manually.
	for i, name := range []string{"Charlie", "Alpha", "Bravo"} {
		id, err := InsertCategory(context.Background(), d, NewCategory{UserID: userID, Name: name, CreatedAt: time.Now().Unix()})
		require.NoError(t, err)
		_, err = d.ExecContext(context.Background(),
			`UPDATE categories SET position = ? WHERE id = ?`, int64(2-i), id)
		require.NoError(t, err)
	}
	cats, err := ListCategories(context.Background(), d, userID)
	require.NoError(t, err)
	require.Len(t, cats, 3)
	require.Equal(t, "Bravo", cats[0].Name)   // position 0
	require.Equal(t, "Alpha", cats[1].Name)   // position 1
	require.Equal(t, "Charlie", cats[2].Name) // position 2
	require.Equal(t, int64(0), cats[0].Position)
	require.Equal(t, int64(1), cats[1].Position)
	require.Equal(t, int64(2), cats[2].Position)
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

func TestReorderCategories_RewritesPositions(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "reorder-alice")
	ids := make([]int64, 3)
	for i, n := range []string{"A", "B", "C"} {
		id, err := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: n, CreatedAt: time.Now().Unix()})
		require.NoError(t, err)
		ids[i] = id
	}
	// Reverse order: C, B, A.
	require.NoError(t, ReorderCategories(context.Background(), d, uid, []int64{ids[2], ids[1], ids[0]}))
	cats, err := ListCategories(context.Background(), d, uid)
	require.NoError(t, err)
	require.Equal(t, "C", cats[0].Name)
	require.Equal(t, "B", cats[1].Name)
	require.Equal(t, "A", cats[2].Name)
}

func TestReorderCategories_RejectsMissingIDs(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "reorder-bob")
	a, err := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: "A", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	_, err = InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: "B", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	err = ReorderCategories(context.Background(), d, uid, []int64{a}) // missing B
	require.ErrorIs(t, err, ErrCategoryReorderMismatch)
}

func TestReorderCategories_RejectsExtraOrCrossUserIDs(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertTestUser(t, d, "reorder-u1")
	u2 := insertTestUser(t, d, "reorder-u2")
	a, err := InsertCategory(context.Background(), d, NewCategory{UserID: u1, Name: "A", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	b, err := InsertCategory(context.Background(), d, NewCategory{UserID: u2, Name: "B", CreatedAt: time.Now().Unix()})
	require.NoError(t, err)
	err = ReorderCategories(context.Background(), d, u1, []int64{a, b}) // b belongs to u2
	require.ErrorIs(t, err, ErrCategoryReorderMismatch)
}

func TestReorderCategories_Atomic(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := insertTestUser(t, d, "reorder-carol")
	a, _ := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: "A", CreatedAt: time.Now().Unix()})
	b, _ := InsertCategory(context.Background(), d, NewCategory{UserID: uid, Name: "B", CreatedAt: time.Now().Unix()})
	// Duplicate ID in the order list — must be rejected, and positions must remain unchanged.
	err := ReorderCategories(context.Background(), d, uid, []int64{a, a})
	require.ErrorIs(t, err, ErrCategoryReorderMismatch)
	cats, err := ListCategories(context.Background(), d, uid)
	require.NoError(t, err)
	require.Equal(t, "A", cats[0].Name)
	require.Equal(t, int64(0), cats[0].Position)
	require.Equal(t, "B", cats[1].Name)
	require.Equal(t, int64(1), cats[1].Position)
	_ = b
}
