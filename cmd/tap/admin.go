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

// runAdminCreate is implemented in Task G2.
func runAdminCreate(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	_ = args
	_ = stdin
	_ = stdout
	_ = hashParams
	fmt.Fprintln(stderr, "tap admin create: not implemented yet")
	_ = flag.NewFlagSet("admin create", flag.ContinueOnError) // import-keeper
	return adminExitGeneric
}

// runAdminPasswd is implemented in Task G3.
func runAdminPasswd(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	_ = args
	_ = stdin
	_ = stdout
	_ = hashParams
	fmt.Fprintln(stderr, "tap admin passwd: not implemented yet")
	return adminExitGeneric
}

// timeNow lets tests override the clock if needed. Production: time.Now.
var timeNow = time.Now

// timeNowUnix is broken out so tests can inject a fixed clock by overriding
// timeNow. Used by runAdminCreate and (later) bootstrapAdmin.
func timeNowUnix() int64 { return timeNow().Unix() }
