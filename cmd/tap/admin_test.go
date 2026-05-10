package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

// testHashParams keeps argon2id cheap so admin CLI tests don't pay the
// 19 MiB / 2-iteration production cost on every run.
var testHashParams = auth.Params{Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 8, KeyLen: 16}

func TestAdminCreateHappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdin := strings.NewReader("ben\nsupersecret\nsupersecret\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exit := runAdminCreate(
		[]string{"--data", dir},
		stdin, stdout, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit, "stderr=%q", stderr.String())
	require.Contains(t, stdout.String(), "created admin user 'ben'")

	// Verify the user landed.
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	defer d.Close()

	u, err := db.GetUserByUsername(context.Background(), d, "ben")
	require.NoError(t, err)
	require.Equal(t, "admin", u.Role)

	ok, err := auth.Verify(u.PasswordHash, "supersecret")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestAdminCreateDuplicateUsername(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// First create.
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\nsupersecret\nsupersecret\n"),
		&bytes.Buffer{}, &bytes.Buffer{},
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit)

	// Second create with same username.
	stderr := &bytes.Buffer{}
	exit = runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\nanothersecret\nanothersecret\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitUserExistsOrGone, exit)
	require.Contains(t, stderr.String(), "user 'ben' already exists")
}

func TestAdminCreatePasswordMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stderr := &bytes.Buffer{}
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\npassword1\npassword2\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitPasswordMismatch, exit)
	require.Contains(t, stderr.String(), "passwords do not match")
}

func TestAdminCreateEmptyUsername(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stderr := &bytes.Buffer{}
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("\nsupersecret\nsupersecret\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitGeneric, exit)
	require.Contains(t, stderr.String(), "username is required")
}

func TestAdminPasswdHappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Seed a user.
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\noldsecret\noldsecret\n"),
		&bytes.Buffer{}, &bytes.Buffer{},
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit)

	// Insert a session for ben so we can verify the reset purges it.
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	u, err := db.GetUserByUsername(context.Background(), d, "ben")
	require.NoError(t, err)
	_, err = db.InsertSession(context.Background(), d, db.NewSession{
		UserID: u.ID, TokenHash: "h", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)
	d.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exit = runAdminPasswd(
		[]string{"--data", dir, "ben"},
		strings.NewReader("newsecret\nnewsecret\n"),
		stdout, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit, "stderr=%q", stderr.String())
	require.Contains(t, stdout.String(), "password reset for 'ben'")

	// Verify hash + session deletion.
	d, err = db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	defer d.Close()

	u, err = db.GetUserByUsername(context.Background(), d, "ben")
	require.NoError(t, err)
	ok, err := auth.Verify(u.PasswordHash, "newsecret")
	require.NoError(t, err)
	require.True(t, ok)

	_, err = db.GetSessionByTokenHash(context.Background(), d, "h")
	require.Error(t, err, "session should be deleted")
}

func TestAdminPasswdMissingUser(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stderr := &bytes.Buffer{}
	exit := runAdminPasswd(
		[]string{"--data", dir, "nobody"},
		strings.NewReader("newsecret\nnewsecret\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitUserExistsOrGone, exit)
	require.Contains(t, stderr.String(), "user 'nobody' not found")
}

func TestAdminPasswdMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\noldsecret\noldsecret\n"),
		&bytes.Buffer{}, &bytes.Buffer{},
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit)

	stderr := &bytes.Buffer{}
	exit = runAdminPasswd(
		[]string{"--data", dir, "ben"},
		strings.NewReader("a-good-password\nb-different-pw\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitPasswordMismatch, exit)
	require.Contains(t, stderr.String(), "passwords do not match")
}

func TestAdminList_Empty(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runAdmin([]string{"list", "--data", t.TempDir()},
		strings.NewReader(""), &stdout, &stderr, auth.DefaultParams)
	require.Equal(t, adminExitOK, code)
	require.Contains(t, stdout.String(), "ID")
}

func TestAdminList_WithUsers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out, errBuf bytes.Buffer
	runAdmin([]string{"create", "--data", dir},
		strings.NewReader("alice\npassword123\npassword123\n"),
		&out, &errBuf, testHashParams)

	var stdout bytes.Buffer
	code := runAdmin([]string{"list", "--data", dir},
		strings.NewReader(""), &stdout, &bytes.Buffer{}, auth.DefaultParams)
	require.Equal(t, adminExitOK, code)
	require.Contains(t, stdout.String(), "alice")
}

func TestAdminDisable_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out bytes.Buffer
	runAdmin([]string{"create", "--data", dir},
		strings.NewReader("bob\npassword123\npassword123\n"),
		&out, &bytes.Buffer{}, testHashParams)

	var stdout, stderr bytes.Buffer
	code := runAdmin([]string{"disable", "--data", dir, "bob"},
		strings.NewReader(""), &stdout, &stderr, auth.DefaultParams)
	require.Equal(t, adminExitOK, code)
	require.Contains(t, stdout.String(), "disabled user 'bob'")
}

func TestAdminDisable_NotFound(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runAdmin([]string{"disable", "--data", t.TempDir(), "nobody"},
		strings.NewReader(""), &stdout, &stderr, auth.DefaultParams)
	require.Equal(t, adminExitUserExistsOrGone, code)
	require.Contains(t, stderr.String(), "not found")
}

func TestAdminDisable_AlreadyDisabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out bytes.Buffer
	runAdmin([]string{"create", "--data", dir},
		strings.NewReader("carl\npassword123\npassword123\n"),
		&out, &bytes.Buffer{}, testHashParams)
	// Disable once.
	runAdmin([]string{"disable", "--data", dir, "carl"},
		strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, auth.DefaultParams)
	// Disable again.
	code := runAdmin([]string{"disable", "--data", dir, "carl"},
		strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, auth.DefaultParams)
	require.Equal(t, adminExitPasswordMismatch, code) // exit 3 = already disabled
}

func TestHealthcheck_NoServer(t *testing.T) {
	t.Parallel()
	// Point at a port that nothing is listening on.
	code := runHealthcheck([]string{"--addr", "127.0.0.1:19999", "--timeout", "1s"})
	require.Equal(t, 1, code)
}

func TestAdminListUsers_DBFunctions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(ctx, d))

	users, err := db.ListUsers(ctx, d)
	require.NoError(t, err)
	require.Empty(t, users)
}
