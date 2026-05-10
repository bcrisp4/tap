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
