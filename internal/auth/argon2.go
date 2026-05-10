// Package auth owns password hashing (argon2id, PHC-encoded) and the
// random-token primitives used by the session and CSRF mechanisms.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the tunable cost knobs of argon2id. Encoded into every hash so
// Verify can run with the same params the hash was produced under, and so a
// future params bump (paired with re-hash-on-verify in M12) can identify
// which hashes are still on weaker params.
type Params struct {
	Time    uint32
	Memory  uint32 // KiB
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

// DefaultParams: OWASP 2026 recommendation for argon2id. Pinned as a constant;
// a future milestone that bumps these is also responsible for the
// re-hash-on-verify path (M12 line in docs/roadmap.md).
var DefaultParams = Params{
	Time:    2,
	Memory:  64 * 1024, // KiB → 64 MiB
	Threads: 1,
	SaltLen: 16,
	KeyLen:  32,
}

// Hash returns the PHC-encoded argon2id hash of password under the given
// params. The encoding is the standard
//
//	$argon2id$v=19$m=<mem>,t=<time>,p=<threads>$<salt-b64>$<hash-b64>
//
// where the b64 forms are unpadded base64 (RawStdEncoding).
func Hash(password string, p Params) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2: read salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Threads,
		enc.EncodeToString(salt), enc.EncodeToString(key),
	), nil
}

// Verify parses encoded as a PHC-format argon2id hash and reports whether it
// corresponds to password. Returns (false, nil) on a normal mismatch and
// (false, err) on a malformed encoding.
func Verify(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// Expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errors.New("argon2: malformed hash")
	}
	// Strict parse: fmt.Sscanf would silently accept trailing garbage
	// (e.g. "v=19abc" or "m=8192,xxxx,p=1"). Use explicit prefix-strip +
	// strconv.ParseUint so anything off-format is rejected loudly.
	versionStr, ok := strings.CutPrefix(parts[2], "v=")
	if !ok {
		return false, fmt.Errorf("argon2: malformed version %q", parts[2])
	}
	version, err := strconv.ParseUint(versionStr, 10, 32)
	if err != nil {
		return false, fmt.Errorf("argon2: parse version: %w", err)
	}
	if uint32(version) != argon2.Version {
		return false, fmt.Errorf("argon2: unsupported version %d", version)
	}

	paramFields := strings.Split(parts[3], ",")
	if len(paramFields) != 3 {
		return false, fmt.Errorf("argon2: malformed params %q", parts[3])
	}
	var p Params
	for i, prefix := range []string{"m=", "t=", "p="} {
		val, ok := strings.CutPrefix(paramFields[i], prefix)
		if !ok {
			return false, fmt.Errorf("argon2: malformed params %q", parts[3])
		}
		n, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return false, fmt.Errorf("argon2: parse param %s: %w", prefix, err)
		}
		switch i {
		case 0:
			p.Memory = uint32(n)
		case 1:
			p.Time = uint32(n)
		case 2:
			p.Threads = uint8(n)
		}
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("argon2: decode salt: %w", err)
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("argon2: decode hash: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// ErrPasswordTooShort is returned by ValidatePassword for inputs shorter
// than MinPasswordLength runes.
var ErrPasswordTooShort = errors.New("password too short")

// MinPasswordLength is the minimum length policy for new passwords.
// Length over class diversity per modern guidance — no complexity rules.
const MinPasswordLength = 8

// ValidatePassword returns ErrPasswordTooShort if s is shorter than
// MinPasswordLength runes. Caller is responsible for trimming whitespace
// if appropriate (the API and CLI accept passwords verbatim — leading
// or trailing spaces become part of the password).
func ValidatePassword(s string) error {
	if len([]rune(s)) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}
