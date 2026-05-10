package api

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func newTestAPIDB(t *testing.T) *sql.DB {
	t.Helper()
	return newTestDB(t) // reuse the helper from middleware_test.go
}

func insertAPITestUser(t *testing.T, d *sql.DB, username string) int64 {
	t.Helper()
	id, err := db.InsertUser(context.Background(), d, db.NewUser{
		Username: username, PasswordHash: "x", Role: "admin", CreatedAt: 0,
	})
	require.NoError(t, err)
	return id
}

func itoa(id int64) string {
	return fmt.Sprintf("%d", id)
}
