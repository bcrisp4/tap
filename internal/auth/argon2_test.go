package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testParams keeps the test fast — production uses DefaultParams.
var testParams = Params{Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 8, KeyLen: 16}

func TestHashVerifyRoundTrip(t *testing.T) {
	t.Parallel()
	encoded, err := Hash("correct horse battery staple", testParams)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encoded, "$argon2id$v=19$"),
		"encoded hash must use the standard PHC format, got: %s", encoded)

	ok, err := Verify(encoded, "correct horse battery staple")
	require.NoError(t, err)
	require.True(t, ok, "Verify should accept the same password")
}

func TestVerifyRejectsWrongPassword(t *testing.T) {
	t.Parallel()
	encoded, err := Hash("hunter2", testParams)
	require.NoError(t, err)

	ok, err := Verify(encoded, "Hunter2")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerifyRejectsMalformedEncoding(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"plaintext",
		"$argon2id$v=19$m=8192,t=1,p=1$abc",          // missing the hash component
		"$argon2id$v=99$m=8192,t=1,p=1$YWFh$YmJi",    // wrong version
		"$argon2i$v=19$m=8192,t=1,p=1$YWFh$YmJi",     // wrong family
		"$argon2id$v=19abc$m=8192,t=1,p=1$YWFh$YmJi", // trailing garbage in version
		"$argon2id$v=19$m=8192,xxxx,p=1$YWFh$YmJi",   // malformed param prefix
		"$argon2id$v=19$m=8192,t=1$YWFh$YmJi",        // wrong number of params
	}
	for _, e := range cases {
		ok, err := Verify(e, "anything")
		require.False(t, ok, "encoded=%q", e)
		require.Error(t, err, "encoded=%q", e)
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", ErrPasswordTooShort},
		{"seven chars", "1234567", ErrPasswordTooShort},
		{"eight chars", "12345678", nil},
		{"long", strings.Repeat("a", 200), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.in)
			if tc.want == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}

// TestVerifyAcceptsHandcraftedPHCEncoding locks the on-disk format. If a
// future Hash refactor changes the encoding shape, the parser will reject
// this hand-crafted PHC string and this test will fail loudly. Back the
// refactor out — it would invalidate every existing user's password.
func TestVerifyAcceptsHandcraftedPHCEncoding(t *testing.T) {
	t.Parallel()
	// Random salt + random hash bytes; the password doesn't match, but Verify
	// must parse the encoding cleanly and return (false, nil) — not an error.
	const handcrafted = "$argon2id$v=19$m=8192,t=1,p=1$YWFhYWFhYWE$YmJiYmJiYmJiYmJiYmJiYg"
	ok, err := Verify(handcrafted, "this-will-not-match")
	require.NoError(t, err, "parser must accept the standard PHC encoding")
	require.False(t, ok, "but the password is wrong, so Verify should return false")
}
