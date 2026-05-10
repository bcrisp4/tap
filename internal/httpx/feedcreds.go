package httpx

import "net/http"

// FeedCreds carries a subscription's per-feed credentials. Empty values mean
// "do not send"; the helper below is layered onto outbound requests at the
// caller site (the polling worker and the article extractor). The shared
// HTTP client itself is unchanged — SSRF, per-host concurrency cap, and
// timeout still apply to the resulting request.
type FeedCreds struct {
	Cookie        string
	BasicAuthUser string
	BasicAuthPass string
}

// ApplyFeedCreds layers per-feed Cookie and Authorization headers onto req.
// An empty Cookie leaves any pre-existing Cookie header alone (the caller
// is responsible for not setting one if they don't want one). Basic auth
// is applied iff BasicAuthUser is non-empty — RFC 7617 permits an empty
// password and we don't second-guess the operator's intent.
func ApplyFeedCreds(req *http.Request, creds FeedCreds) {
	if creds.Cookie != "" {
		req.Header.Set("Cookie", creds.Cookie)
	}
	if creds.BasicAuthUser != "" {
		req.SetBasicAuth(creds.BasicAuthUser, creds.BasicAuthPass)
	}
}
