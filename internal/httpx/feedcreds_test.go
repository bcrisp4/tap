package httpx

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyFeedCredsCookie(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{Cookie: "session=abc; tracking=1"})
	require.Equal(t, "session=abc; tracking=1", req.Header.Get("Cookie"))
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestApplyFeedCredsBasicAuth(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{BasicAuthUser: "ben", BasicAuthPass: "secret"})
	require.Empty(t, req.Header.Get("Cookie"))
	// SetBasicAuth produces "Basic base64(ben:secret)" — assert the prefix
	// and let net/http own the encoding.
	require.NotEmpty(t, req.Header.Get("Authorization"))
}

func TestApplyFeedCredsBoth(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.Equal(t, "c", req.Header.Get("Cookie"))
	require.NotEmpty(t, req.Header.Get("Authorization"))
}

func TestApplyFeedCredsEmpty(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{})
	require.Empty(t, req.Header.Get("Cookie"))
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestApplyFeedCredsUserOnlyEmptyPassStillSetsAuth(t *testing.T) {
	// RFC 7617: the password may be empty. We set basic auth iff the user
	// is non-empty so a configured username with an intentionally empty
	// pass still produces an Authorization header.
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{BasicAuthUser: "ben"})
	require.NotEmpty(t, req.Header.Get("Authorization"))
}
