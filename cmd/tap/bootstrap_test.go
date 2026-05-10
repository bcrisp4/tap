package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

// TestBootstrapAdminCreatesUser exercises the env-var first-launch path: a
// happy-path call inserts an admin row whose hash verifies against the
// supplied password.
func TestBootstrapAdminCreatesUser(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	defer d.Close()

	require.NoError(t, bootstrapAdmin(context.Background(), d, "ben", "supersecret", testHashParams))

	u, err := db.GetUserByUsername(context.Background(), d, "ben")
	require.NoError(t, err)
	require.Equal(t, "admin", u.Role)

	ok, err := auth.Verify(u.PasswordHash, "supersecret")
	require.NoError(t, err)
	require.True(t, ok)
}

// TestBootstrapAdminRejectsTooShortPassword ensures validation runs BEFORE
// any DB write — so an operator who supplies a 5-char password ends up with
// a clean failure rather than an unusable DB row.
func TestBootstrapAdminRejectsTooShortPassword(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	defer d.Close()

	err = bootstrapAdmin(context.Background(), d, "ben", "short", testHashParams)
	require.ErrorIs(t, err, auth.ErrPasswordTooShort)

	n, err := db.CountUsers(context.Background(), d)
	require.NoError(t, err)
	require.Equal(t, 0, n, "no user should be created on validation failure")
}

// TestBootstrapAdminRejectsWhitespaceUsername guards against the footgun
// where TAP_ADMIN_USERNAME=" " would pass main.go's user!="" check and then
// produce an unloggable account: login trims the input and would never
// match an all-whitespace stored username. Empty-after-trim must abort.
func TestBootstrapAdminRejectsWhitespaceUsername(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	defer d.Close()

	cases := []string{"", "   ", "\t\n"}
	for _, u := range cases {
		err := bootstrapAdmin(context.Background(), d, u, "validpassword", testHashParams)
		require.Error(t, err, "username=%q should be rejected", u)
	}

	n, err := db.CountUsers(context.Background(), d)
	require.NoError(t, err)
	require.Equal(t, 0, n, "no user should be created when username is empty after trim")
}
