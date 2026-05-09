package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/mmcdole/gofeed"
)

type FetchOpts struct {
	PriorETag         string
	PriorLastModified string
	UserAgent         string
}

type FetchResult struct {
	Status       int
	ETag         string
	LastModified string
	Feed         *gofeed.Feed // nil on 304
}

// Fetch issues a conditional GET against feedURL and parses the response.
// Returns 304 with Feed=nil on Not Modified. Errors include any non-2xx/304 response.
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
	ua := opts.UserAgent
	if ua == "" {
		ua = "tap/0.1 (+https://github.com/bcrisp4/tap)"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/json, application/xml;q=0.9, */*;q=0.5")

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

	switch {
	case resp.StatusCode == http.StatusNotModified:
		return res, nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Cap how much of the response we'll feed to the parser. A hostile
		// or misconfigured origin returning a 1 GB "feed" must not OOM us.
		const maxBody = 10 << 20 // 10 MiB
		f, err := gofeed.NewParser().Parse(io.LimitReader(resp.Body, maxBody))
		if err != nil {
			return FetchResult{}, fmt.Errorf("parse feed: %w", err)
		}
		res.Feed = f
		return res, nil
	default:
		return FetchResult{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}
