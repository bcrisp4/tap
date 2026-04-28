package poller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bcrisp4/tap/internal/feedparse"
	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/iconfetch"
	"github.com/bcrisp4/tap/internal/limiter"
	"github.com/bcrisp4/tap/internal/reader"
	"github.com/bcrisp4/tap/internal/storage"
)

// maxLastErrorBytes caps last_error to keep the column small. The
// underlying message often includes long URLs and stack traces.
const maxLastErrorBytes = 500

// pollUserID is the single-user id for v1; multi-user is out of scope.
const pollUserID = 1

// extractionConcurrency caps in-flight article fetches per feed-poll.
// The per-host limiter only serialises by host, so a feed whose
// entries link to N distinct hosts could otherwise spawn N concurrent
// fetches at once. Eight is enough to mask network latency without
// flooding the local outbound pool.
const extractionConcurrency = 8

// WorkerConfig wires the worker's deps. Pipeline is optional — when
// nil (or feed.Crawler == false), content extraction is skipped.
// RunState is optional; when set, the worker bumps it around each poll.
// IconFetcher is optional; when nil, favicon scraping is skipped (the
// existing tests don't need it and don't want to mock the network).
type WorkerConfig struct {
	Store       *storage.Store
	Client      *httpclient.Client
	Limiter     *limiter.HostLimiter
	Pipeline    *reader.Pipeline
	RunState    *RunState
	PollFactor  float64
	IconFetcher *iconfetch.Fetcher
}

// Worker handles one feed end-to-end. Construct once via NewWorker; the
// type is immutable and safe to share across goroutines (its
// dependencies are themselves concurrency-safe).
type Worker struct {
	cfg WorkerConfig
}

// NewWorker builds a Worker from the given config.
func NewWorker(cfg WorkerConfig) *Worker { return &Worker{cfg: cfg} }

// PollOne executes the full poll for feedID. Network/parse failures
// are recorded on the feed via Store.CommitPollFailure and *not*
// returned — that's the contract the dispatcher relies on so workers
// stay alive on bad feeds.
func (w *Worker) PollOne(ctx context.Context, feedID int64) error {
	if w.cfg.RunState != nil {
		w.cfg.RunState.PollStarted()
	}
	var pollErr error
	defer func() {
		if w.cfg.RunState != nil {
			w.cfg.RunState.PollFinished(pollErr)
		}
	}()

	feed, err := w.cfg.Store.GetFeed(ctx, pollUserID, feedID)
	if err != nil {
		pollErr = err
		return nil
	}

	host := hostOf(feed.FeedURL)
	if err := w.cfg.Limiter.Acquire(ctx, host); err != nil {
		pollErr = err
		return nil
	}
	defer w.cfg.Limiter.Release(host)

	resp, fetchErr := w.fetchFeed(ctx, feed)
	if fetchErr != nil {
		pollErr = fetchErr
		w.recordFailure(ctx, feed, fetchErr, 0)
		return nil
	}
	defer resp.Body.Close()

	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
	maxAge := parseMaxAge(resp.Header.Get("Cache-Control"), resp.Header.Get("Expires"))
	now := time.Now().Unix()

	// 304 path: refresh validators + advance next_poll_at, no inserts.
	if resp.StatusCode == http.StatusNotModified {
		out := NextPollAt(PollOutcome{
			Now: now, WeeklyEntries: feed.WeeklyEntryCount,
			RetryAfter: retryAfter,
			MaxAge:     maxAge,
		}, w.cfg.PollFactor)
		if err := w.cfg.Store.CommitPollNotModified(ctx, feed.ID,
			resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), out.NextPollAt); err != nil {
			pollErr = err
		}
		return nil
	}

	// Any non-2xx (e.g. 429 / 5xx) is a poll failure. Honour
	// Retry-After here too — design.md §5 rule 1 wins over the
	// exponential backoff from rule 2.
	if resp.StatusCode/100 != 2 {
		err := fmt.Errorf("origin returned %d", resp.StatusCode)
		pollErr = err
		w.recordFailure(ctx, feed, err, retryAfter)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		pollErr = err
		w.recordFailure(ctx, feed, err, retryAfter)
		return nil
	}

	parsed, err := feedparse.Parse(body, feed.FeedURL)
	if err != nil {
		pollErr = err
		w.recordFailure(ctx, feed, err, retryAfter)
		return nil
	}

	newEntries, err := w.filterAndExtract(ctx, feed, parsed.Entries)
	if err != nil {
		pollErr = err
		w.recordFailure(ctx, feed, err, retryAfter)
		return nil
	}

	// Compute next-poll using last-known weekly count plus the new
	// arrivals; CommitPollSuccess will recompute the canonical value
	// and persist it.
	out := NextPollAt(PollOutcome{
		Now:           now,
		WeeklyEntries: feed.WeeklyEntryCount + len(newEntries),
		RetryAfter:    retryAfter,
		MaxAge:        maxAge,
	}, w.cfg.PollFactor)

	if _, err := w.cfg.Store.CommitPollSuccess(ctx, feed.ID, newEntries,
		resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"),
		out.ErrorCount, out.NextPollAt); err != nil {
		pollErr = err
	}

	// Best-effort favicon scrape on first successful poll. Errors are
	// intentionally silent — a missing favicon must never affect the
	// poll status, and the next successful poll will retry naturally
	// (the IconID-nil guard is the only gate).
	w.maybeFetchIcon(ctx, feed, parsed.Meta.SiteURL)

	return nil
}

// maybeFetchIcon scrapes a favicon for feeds that don't yet have an
// icon attached. Runs after CommitPollSuccess on every successful
// poll — the cheap path (IconID != nil) bails immediately, so we
// only pay the network cost once per feed.
func (w *Worker) maybeFetchIcon(ctx context.Context, feed *storage.Feed, parsedSiteURL string) {
	if w.cfg.IconFetcher == nil {
		return
	}
	if feed.IconID != nil {
		return
	}
	siteURL := parsedSiteURL
	if siteURL == "" && feed.SiteURL != nil {
		siteURL = *feed.SiteURL
	}
	if siteURL == "" {
		// No site URL means we can't even build the /favicon.ico
		// fallback. Skip silently; we'll retry next poll if the
		// site URL ever lands.
		return
	}
	// Pull the site HTML so we can find <link rel="icon"> hints.
	// A failure here just means we go straight to the /favicon.ico
	// fallback — Discover handles that order.
	htmlBody := w.fetchSiteHTML(ctx, siteURL)
	candidates := iconfetch.CandidatesFor(htmlBody, siteURL)
	if len(candidates) == 0 {
		return
	}
	res, err := w.cfg.IconFetcher.Discover(ctx, candidates)
	if err != nil {
		// ErrNoIcon (or any sub-error) — silent; retry on next poll.
		return
	}
	hash := sha256Hex(res.Bytes)
	iconID, err := w.cfg.Store.InsertIcon(ctx, hash, res.MIME, res.Bytes)
	if err != nil {
		// Hash collision means another feed already cached the same
		// bytes — look up the existing row and reuse its id.
		existing, gerr := w.cfg.Store.GetIconByHash(ctx, hash)
		if gerr != nil || existing == nil {
			return
		}
		iconID = existing.ID
	}
	// The feed could have been deleted mid-poll (ErrNotFound), or hit
	// any other transient write error. Either way the next successful
	// poll will retry — the gate is feed.IconID == nil, which we
	// haven't actually mutated.
	_ = w.cfg.Store.SetFeedIcon(ctx, feed.ID, &iconID)
}

// fetchSiteHTML returns the response body for siteURL, or nil on any
// error / non-2xx. A nil return tells iconfetch.CandidatesFor it has
// no HTML to mine, which is fine — it'll just produce the
// /favicon.ico fallback.
func (w *Worker) fetchSiteHTML(ctx context.Context, siteURL string) []byte {
	resp, err := w.cfg.Client.Get(ctx, siteURL, nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	return body
}

// sha256Hex returns the lowercase hex-encoded sha256 of b. Used as
// the content-addressable key into the icons table.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fetchFeed performs the conditional GET against feed.FeedURL.
func (w *Worker) fetchFeed(ctx context.Context, f *storage.Feed) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.FeedURL, nil)
	if err != nil {
		return nil, err
	}
	if f.ETag != nil && *f.ETag != "" {
		req.Header.Set("If-None-Match", *f.ETag)
	}
	if f.LastModified != nil && *f.LastModified != "" {
		req.Header.Set("If-Modified-Since", *f.LastModified)
	}
	return w.cfg.Client.Do(req, optionsFromFeed(f))
}

// filterAndExtract drops entries already seen (by hash, with the real
// feed_id mixed in) and entries that match a tombstone, then runs
// content extraction for crawler=1 feeds. Reading-time is computed
// here so the storage layer doesn't need to know about it.
func (w *Worker) filterAndExtract(ctx context.Context, f *storage.Feed, parsed []*storage.Entry) ([]*storage.Entry, error) {
	var fresh []*storage.Entry
	for _, e := range parsed {
		guid := guidFromEntry(e)
		link := stringOr(e.URL)
		pub := int64Or(e.PublishedAt)
		e.Hash = feedparse.EntryHash(f.ID, guid, link, e.Title, pub)
		e.FeedID = f.ID
		e.UserID = f.UserID

		exists, err := w.cfg.Store.EntryExists(ctx, f.ID, e.Hash)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		has, err := w.cfg.Store.HasTombstone(ctx, f.ID, e.Hash)
		if err != nil {
			return nil, err
		}
		if has {
			continue
		}
		fresh = append(fresh, e)
	}

	if len(fresh) == 0 {
		return fresh, nil
	}

	if f.Crawler && w.cfg.Pipeline != nil {
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(extractionConcurrency)
		rules := stringOr(f.ScraperRules)
		for _, e := range fresh {
			if e.URL == nil || *e.URL == "" {
				continue
			}
			g.Go(func() error {
				content, ok := w.fetchAndExtract(gctx, f, *e.URL, rules)
				if !ok {
					e.ExtractionFailed = true
					if e.Summary != nil && (e.Content == nil || *e.Content == "") {
						sum := *e.Summary
						e.Content = &sum
					}
					return nil
				}
				e.Content = &content
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	// Universal pass: every entry's stored content goes through
	// RewriteAndSanitize so right-click→copy-image-URL yields a
	// /api/v1/proxy/<token> URL and the sanitizer drops anything
	// dangerous. Successful crawler entries already flowed through
	// Pipeline.Process (which includes this step), so we skip the
	// second pass for them to avoid double-encoding proxy URLs. Two
	// other shapes also need sanitising here:
	//   - non-crawler entries (Content came from the feed parser);
	//   - crawler entries whose extraction failed and fell back to
	//     the feed-supplied summary (which never went through Process).
	// The frontend renders content via `{@html entry.content}` so any
	// untrusted HTML reaching storage is a stored-XSS vector — this
	// pass is the security boundary, not a nice-to-have.
	if w.cfg.Pipeline != nil {
		for _, e := range fresh {
			if f.Crawler && !e.ExtractionFailed {
				continue
			}
			source := stringOr(e.Content)
			if source == "" {
				source = stringOr(e.Summary)
			}
			if source == "" {
				continue
			}
			articleURL := stringOr(e.URL)
			if articleURL == "" {
				articleURL = f.FeedURL
			}
			safe, err := w.cfg.Pipeline.RewriteAndSanitize(source, articleURL)
			if err != nil {
				// Sanitisation must never silently leak unsafe HTML to
				// storage. If even RewriteAndSanitize's degraded
				// fallback path fails (would be a bluemonday
				// programming bug), drop the content rather than
				// store the original.
				empty := ""
				e.Content = &empty
				continue
			}
			e.Content = &safe
		}
	}

	for _, e := range fresh {
		e.ReadingTime = readingTimeFor(e)
	}
	return fresh, nil
}

func (w *Worker) fetchAndExtract(ctx context.Context, f *storage.Feed, articleURL, rules string) (string, bool) {
	host := hostOf(articleURL)
	if err := w.cfg.Limiter.Acquire(ctx, host); err != nil {
		return "", false
	}
	defer w.cfg.Limiter.Release(host)

	resp, err := w.cfg.Client.Get(ctx, articleURL, optionsFromFeed(f))
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}
	out, err := w.cfg.Pipeline.Process(string(body), articleURL, rules)
	if err != nil {
		return "", false
	}
	return out, true
}

// recordFailure computes the backoff and asks the store to persist
// the failure. retryAfter is honoured per design.md §5 rule 1: when
// set, it overrides the exponential failure backoff.
func (w *Worker) recordFailure(ctx context.Context, f *storage.Feed, fetchErr error, retryAfter time.Duration) {
	now := time.Now().Unix()
	out := NextPollAt(PollOutcome{
		Now: now, Failed: true, ErrorCount: f.ErrorCount, RetryAfter: retryAfter,
	}, w.cfg.PollFactor)
	msg := ""
	if fetchErr != nil {
		msg = fetchErr.Error()
		if len(msg) > maxLastErrorBytes {
			msg = msg[:maxLastErrorBytes]
		}
	}
	// On a Retry-After response the adaptive formula resets ErrorCount
	// to zero, but the underlying request still didn't yield entries —
	// preserve the existing count by adding 1 in that case so the
	// exponential branch still applies on the next failure.
	errCount := out.ErrorCount
	if retryAfter > 0 {
		errCount = f.ErrorCount + 1
	}
	_ = w.cfg.Store.CommitPollFailure(ctx, f.ID, errCount, msg, out.NextPollAt)
}

// readingTimeFor picks the best text source (content > summary) and
// runs feedparse.Minutes against it.
func readingTimeFor(e *storage.Entry) int {
	text := stringOr(e.Content)
	if text == "" {
		text = stringOr(e.Summary)
	}
	return feedparse.Minutes(text)
}

// optionsFromFeed plucks the per-feed override columns into the
// httpclient.Options shape.
func optionsFromFeed(f *storage.Feed) *httpclient.Options {
	return &httpclient.Options{
		UserAgent:       stringOr(f.UserAgent),
		Cookie:          stringOr(f.Cookie),
		Username:        stringOr(f.Username),
		Password:        stringOr(f.Password),
		ProxyURL:        stringOr(f.ProxyURL),
		DisableHTTP2:    f.DisableHTTP2,
		AllowSelfSigned: f.AllowSelfSignedCerts,
	}
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "_unknown"
	}
	return strings.ToLower(u.Hostname())
}

func guidFromEntry(e *storage.Entry) string {
	if e.URL != nil && *e.URL != "" {
		return *e.URL
	}
	return e.Title
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func parseMaxAge(cc, expires string) time.Duration {
	if cc != "" {
		const k = "max-age="
		for _, p := range strings.Split(cc, ",") {
			p = strings.TrimSpace(strings.ToLower(p))
			if strings.HasPrefix(p, k) {
				if secs, err := strconv.Atoi(p[len(k):]); err == nil && secs > 0 {
					return time.Duration(secs) * time.Second
				}
			}
		}
	}
	if expires != "" {
		if t, err := http.ParseTime(expires); err == nil {
			if d := time.Until(t); d > 0 {
				return d
			}
		}
	}
	return 0
}

func stringOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func int64Or(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
