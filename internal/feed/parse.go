package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/cadence"
	"github.com/bcrisp4/tap/internal/httpx"
	"github.com/mmcdole/gofeed"
)

type FetchOpts struct {
	PriorETag         string
	PriorLastModified string
	Creds             httpx.FeedCreds
}

type FetchResult struct {
	Status       int
	ETag         string
	LastModified string
	RetryAfter   time.Time     // zero if absent
	CacheMaxAge  time.Duration // 0 if absent
	Feed         *gofeed.Feed  // nil on 304 or error
}

// Fetch issues a conditional GET against feedURL and parses the response.
// Returns 304 with Feed=nil on Not Modified. Errors include any non-2xx/304 response.
//
// FetchResult is populated from response headers (Status, ETag, Last-Modified,
// RetryAfter, CacheMaxAge) even when Fetch returns a non-nil error, so the
// caller can floor the next-poll time against an origin's Retry-After on a 503.
// FetchResult is the zero value only when no response was received at all
// (e.g. transport error, malformed request).
func Fetch(ctx context.Context, client *http.Client, feedURL string, opts FetchOpts) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("new request: %w", err)
	}
	if opts.PriorETag != "" {
		req.Header.Set("If-None-Match", opts.PriorETag)
	}
	if opts.PriorLastModified != "" {
		req.Header.Set("If-Modified-Since", opts.PriorLastModified)
	}
	req.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/json, application/xml;q=0.9, */*;q=0.5")
	httpx.ApplyFeedCreds(req, opts.Creds)

	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	res := FetchResult{
		Status:       resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
	if h := resp.Header.Get("Retry-After"); h != "" {
		if t, ok := cadence.ParseRetryAfter(h, time.Now()); ok {
			res.RetryAfter = t
		}
	}
	if h := resp.Header.Get("Cache-Control"); h != "" {
		if d, ok := cadence.ParseCacheMaxAge(h); ok {
			res.CacheMaxAge = d
		}
	}

	switch {
	case resp.StatusCode == http.StatusNotModified:
		return res, nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Cap how much of the response we'll feed to the parser. A hostile
		// or misconfigured origin returning a 1 GB "feed" must not OOM us.
		const maxBody = 10 << 20 // 10 MiB
		f, err := gofeed.NewParser().Parse(io.LimitReader(resp.Body, maxBody))
		if err != nil {
			return res, fmt.Errorf("parse feed: %w", err)
		}
		res.Feed = f
		return res, nil
	default:
		return res, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}
