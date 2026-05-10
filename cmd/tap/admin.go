package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"golang.org/x/term"
)

// adminExit* report exit codes from the `tap admin` subcommands. Constants
// kept here so tests can assert against them without duplicating literals.
const (
	adminExitOK               = 0
	adminExitGeneric          = 1
	adminExitUserExistsOrGone = 2
	adminExitPasswordMismatch = 3
)

// runAdmin dispatches `tap admin <subcommand> ...`. Returns an exit code so
// tests can call it directly without spawning a subprocess.
func runAdmin(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: tap admin <create|passwd> ...")
		return adminExitGeneric
	}
	switch args[0] {
	case "create":
		return runAdminCreate(args[1:], stdin, stdout, stderr, hashParams)
	case "passwd":
		return runAdminPasswd(args[1:], stdin, stdout, stderr, hashParams)
	default:
		fmt.Fprintf(stderr, "unknown admin subcommand %q\n", args[0])
		return adminExitGeneric
	}
}

// openAdminDB opens the DB at <dataDir>/tap.db, applies migrations, and
// returns a *sql.DB ready for admin work.
func openAdminDB(ctx context.Context, dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	d, err := db.Open(ctx, filepath.Join(dataDir, "tap.db"))
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, d); err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}

// readPassword reads from stdin without echoing. If stdin is a terminal, it
// uses term.ReadPassword. Otherwise (test injection via *bytes.Reader,
// strings.Reader, or a piped file) it falls back to a line read.
func readPassword(stdin io.Reader, prompt string, stdout io.Writer) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(stdout, prompt)
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stdout)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// Non-terminal (test or pipe): read a line.
	fmt.Fprint(stdout, prompt)
	return readUntilNewline(stdin)
}

// readLine reads a single echoed line from stdin (used for usernames).
func readLine(stdin io.Reader, prompt string, stdout io.Writer) (string, error) {
	fmt.Fprint(stdout, prompt)
	s, err := readUntilNewline(stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// readUntilNewline reads bytes from r until a '\n' or EOF, and returns the
// accumulated string with any trailing '\r' trimmed (no other trimming —
// callers decide whether to strip whitespace on echoed input).
func readUntilNewline(r io.Reader) (string, error) {
	var sb strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n > 0 && buf[0] == '\n' {
			break
		}
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
	}
	return strings.TrimRight(sb.String(), "\r"), nil
}

// runAdminCreate is the implementation of `tap admin create`. Reads
// username + password (twice, no echo) from stdin, hashes via auth.Hash,
// and inserts a new user row.
func runAdminCreate(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
	role := fs.String("role", "admin", "role for the new user (admin or user)")
	if err := fs.Parse(args); err != nil {
		return adminExitGeneric
	}

	if *role != "admin" && *role != "user" {
		fmt.Fprintf(stderr, "role must be admin or user, got %q\n", *role)
		return adminExitGeneric
	}

	username, err := readLine(stdin, "username: ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read username: %v\n", err)
		return adminExitGeneric
	}
	if username == "" {
		fmt.Fprintln(stderr, "username is required")
		return adminExitGeneric
	}

	pass1, err := readPassword(stdin, "password: ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read password: %v\n", err)
		return adminExitGeneric
	}
	if err := auth.ValidatePassword(pass1); err != nil {
		fmt.Fprintf(stderr, "password too short (min %d): %v\n", auth.MinPasswordLength, err)
		return adminExitGeneric
	}
	pass2, err := readPassword(stdin, "confirm:  ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read password: %v\n", err)
		return adminExitGeneric
	}
	if pass1 != pass2 {
		fmt.Fprintln(stderr, "passwords do not match")
		return adminExitPasswordMismatch
	}

	ctx := context.Background()
	d, err := openAdminDB(ctx, *dataDir)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return adminExitGeneric
	}
	defer d.Close()

	hash, err := auth.Hash(pass1, hashParams)
	if err != nil {
		fmt.Fprintf(stderr, "hash password: %v\n", err)
		return adminExitGeneric
	}
	id, err := db.InsertUser(ctx, d, db.NewUser{
		Username: username, PasswordHash: hash, Role: *role, CreatedAt: timeNowUnix(),
	})
	if err != nil {
		if errors.Is(err, db.ErrUserExists) {
			fmt.Fprintf(stderr, "user '%s' already exists\n", username)
			return adminExitUserExistsOrGone
		}
		fmt.Fprintf(stderr, "create user: %v\n", err)
		return adminExitGeneric
	}
	fmt.Fprintf(stdout, "created %s user '%s' (id=%d)\n", *role, username, id)
	return adminExitOK
}

// runAdminPasswd is the implementation of `tap admin passwd <username>`.
// Resets a user's password and revokes every active session for them
// (admin reset implies the user is locked out and must log in again).
func runAdminPasswd(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	fs := flag.NewFlagSet("admin passwd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
	if err := fs.Parse(args); err != nil {
		return adminExitGeneric
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: tap admin passwd <username>")
		return adminExitGeneric
	}
	username := fs.Arg(0)
	if username == "" {
		fmt.Fprintln(stderr, "usage: tap admin passwd <username>")
		return adminExitGeneric
	}

	ctx := context.Background()
	d, err := openAdminDB(ctx, *dataDir)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return adminExitGeneric
	}
	defer d.Close()

	u, err := db.GetUserByUsername(ctx, d, username)
	if err != nil {
		fmt.Fprintf(stderr, "user '%s' not found\n", username)
		return adminExitUserExistsOrGone
	}

	pass1, err := readPassword(stdin, "new password: ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read password: %v\n", err)
		return adminExitGeneric
	}
	if err := auth.ValidatePassword(pass1); err != nil {
		fmt.Fprintf(stderr, "password too short (min %d)\n", auth.MinPasswordLength)
		return adminExitGeneric
	}
	pass2, err := readPassword(stdin, "confirm:      ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read password: %v\n", err)
		return adminExitGeneric
	}
	if pass1 != pass2 {
		fmt.Fprintln(stderr, "passwords do not match")
		return adminExitPasswordMismatch
	}

	hash, err := auth.Hash(pass1, hashParams)
	if err != nil {
		fmt.Fprintf(stderr, "hash password: %v\n", err)
		return adminExitGeneric
	}
	if err := db.UpdatePasswordHash(ctx, d, u.ID, hash); err != nil {
		fmt.Fprintf(stderr, "update password: %v\n", err)
		return adminExitGeneric
	}
	// Admin reset always invalidates ALL sessions for the user — they are
	// expected to log in again with the new credentials.
	if err := db.DeleteSessionsByUserID(ctx, d, u.ID); err != nil {
		fmt.Fprintf(stderr, "delete sessions: %v\n", err)
		return adminExitGeneric
	}
	fmt.Fprintf(stdout, "password reset for '%s'\n", username)
	return adminExitOK
}

// timeNow lets tests override the clock if needed. Production: time.Now.
var timeNow = time.Now

// timeNowUnix is broken out so tests can inject a fixed clock by overriding
// timeNow. Used by runAdminCreate and (later) bootstrapAdmin.
func timeNowUnix() int64 { return timeNow().Unix() }

// bootstrapAdmin creates an admin user from the supplied credentials. Used
// by the env-var first-launch shortcut after migrations. Validates the
// password (so a too-short bootstrap password aborts loudly rather than
// producing an unusable account) and inserts the user atomically.
//
// logs is reserved for future use (an audit-style log line). The current
// caller already emits an INFO log on success at the call site, so this
// helper stays quiet.
func bootstrapAdmin(ctx context.Context, d *sql.DB, username, password string, hashParams auth.Params, logs io.Writer) error {
	_ = logs // reserved for future audit logging
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := auth.Hash(password, hashParams)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	if _, err := db.InsertUser(ctx, d, db.NewUser{
		Username: username, PasswordHash: hash, Role: "admin", CreatedAt: timeNowUnix(),
	}); err != nil {
		return fmt.Errorf("insert bootstrap admin: %w", err)
	}
	return nil
}
