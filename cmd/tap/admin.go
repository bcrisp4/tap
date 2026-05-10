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
		fmt.Fprintln(stderr, "usage: tap admin <create|passwd|list|disable|disable-totp> ...")
		return adminExitGeneric
	}
	switch args[0] {
	case "create":
		return runAdminCreate(args[1:], stdin, stdout, stderr, hashParams)
	case "passwd":
		return runAdminPasswd(args[1:], stdin, stdout, stderr, hashParams)
	case "list":
		return runAdminList(args[1:], stdout, stderr)
	case "disable":
		return runAdminDisable(args[1:], stdout, stderr)
	case "disable-totp":
		return runAdminDisableTOTP(args[1:], stdout, stderr)
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
		Username: username, PasswordHash: hash, Role: *role, CreatedAt: time.Now().Unix(),
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
		// sql.ErrNoRows is the genuine not-found case; anything else is a
		// real DB error (locked, migration drift, ...) and should surface
		// rather than masquerade as "user not found".
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Fprintf(stderr, "user '%s' not found\n", username)
			return adminExitUserExistsOrGone
		}
		fmt.Fprintf(stderr, "lookup user: %v\n", err)
		return adminExitGeneric
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

// runAdminList implements `tap admin list`. Prints all users as a table.
func runAdminList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("admin list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
	if err := fs.Parse(args); err != nil {
		return adminExitGeneric
	}

	ctx := context.Background()
	d, err := openAdminDB(ctx, *dataDir)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return adminExitGeneric
	}
	defer d.Close()

	users, err := db.ListUsers(ctx, d)
	if err != nil {
		fmt.Fprintf(stderr, "list users: %v\n", err)
		return adminExitGeneric
	}

	fmt.Fprintf(stdout, "%-4s  %-20s  %-6s  %-20s  %s\n", "ID", "USERNAME", "ROLE", "CREATED", "DISABLED")
	for _, u := range users {
		created := time.Unix(u.CreatedAt, 0).UTC().Format(time.RFC3339)
		disabled := "no"
		if u.DisabledAt.Valid {
			disabled = "yes (" + time.Unix(u.DisabledAt.Int64, 0).UTC().Format(time.RFC3339) + ")"
		}
		fmt.Fprintf(stdout, "%-4d  %-20s  %-6s  %-20s  %s\n",
			u.ID, u.Username, u.Role, created, disabled)
	}
	return adminExitOK
}

// runAdminDisable implements `tap admin disable <username>`.
func runAdminDisable(args []string, stdout, stderr io.Writer) int {
	// Accept username as either the first positional arg (before flags) or the
	// last positional arg (after flags). Separate non-flag args from flag args.
	var positional []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			// If this flag takes a value and next arg doesn't start with '-', consume it.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") &&
				!strings.Contains(args[i], "=") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, args[i])
		}
	}

	fs := flag.NewFlagSet("admin disable", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
	if err := fs.Parse(flagArgs); err != nil {
		return adminExitGeneric
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "usage: tap admin disable <username>")
		return adminExitGeneric
	}
	username := positional[0]

	ctx := context.Background()
	d, err := openAdminDB(ctx, *dataDir)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return adminExitGeneric
	}
	defer d.Close()

	u, err := db.GetUserByUsername(ctx, d, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Fprintf(stderr, "user '%s' not found\n", username)
			return adminExitUserExistsOrGone
		}
		fmt.Fprintf(stderr, "lookup user: %v\n", err)
		return adminExitGeneric
	}

	if u.DisabledAt.Valid {
		fmt.Fprintf(stderr, "user '%s' is already disabled\n", username)
		return adminExitPasswordMismatch // exit 3 per spec
	}

	if err := db.DisableUser(ctx, d, u.ID, time.Now().Unix()); err != nil {
		fmt.Fprintf(stderr, "disable user: %v\n", err)
		return adminExitGeneric
	}
	if err := db.DeleteSessionsByUserID(ctx, d, u.ID); err != nil {
		fmt.Fprintf(stderr, "delete sessions: %v\n", err)
		return adminExitGeneric
	}
	fmt.Fprintf(stdout, "disabled user '%s'\n", username)
	return adminExitOK
}

// bootstrapAdmin creates an admin user from the supplied credentials. Used
// by the env-var first-launch shortcut after migrations. Trims the username
// and rejects empty-after-trim (TAP_ADMIN_USERNAME=" " would otherwise
// produce an unloggable account: login trims the input and would fail to
// match an empty stored username). Validates the password so a too-short
// bootstrap password aborts loudly rather than producing an unusable
// account. The caller already emits an INFO log on success, so this helper
// stays quiet.
func bootstrapAdmin(ctx context.Context, d *sql.DB, username, password string, hashParams auth.Params) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("bootstrap username is empty after trimming whitespace")
	}
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := auth.Hash(password, hashParams)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	if _, err := db.InsertUser(ctx, d, db.NewUser{
		Username: username, PasswordHash: hash, Role: "admin", CreatedAt: time.Now().Unix(),
	}); err != nil {
		return fmt.Errorf("insert bootstrap admin: %w", err)
	}
	return nil
}

// runAdminDisableTOTP implements `tap admin disable-totp <username>`.
// Removes the user's TOTP secret and recovery codes. Subsequent logins will
// not prompt for a second factor.
func runAdminDisableTOTP(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("admin disable-totp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
	if err := fs.Parse(args); err != nil {
		return adminExitGeneric
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: tap admin disable-totp <username>")
		return adminExitGeneric
	}
	username := fs.Arg(0)

	ctx := context.Background()
	d, err := openAdminDB(ctx, *dataDir)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return adminExitGeneric
	}
	defer d.Close()

	u, err := db.GetUserByUsername(ctx, d, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Fprintf(stderr, "user '%s' not found\n", username)
			return adminExitUserExistsOrGone
		}
		fmt.Fprintf(stderr, "lookup user: %v\n", err)
		return adminExitGeneric
	}

	if err := db.DeleteTOTPSecret(ctx, d, u.ID); err != nil {
		fmt.Fprintf(stderr, "delete totp secret: %v\n", err)
		return adminExitGeneric
	}
	if err := db.DeleteRecoveryCodes(ctx, d, u.ID); err != nil {
		fmt.Fprintf(stderr, "delete recovery codes: %v\n", err)
		return adminExitGeneric
	}
	fmt.Fprintf(stdout, "2FA disabled for '%s'\n", username)
	return adminExitOK
}
