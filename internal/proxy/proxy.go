package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"golang.org/x/sync/singleflight"

	"github.com/bcrisp4/tap/internal/httpclient"
)

// Config wires the proxy's dependencies.
type Config struct {
	// Secret is the HMAC secret used by EncodeToken / DecodeToken.
	Secret string
	// Client is the shared outbound HTTP client (Plan 03), responsible
	// for SSRF protection, per-host limits, timeouts, and redirect
	// handling.
	Client *httpclient.Client
	// Cache is the on-disk body+meta store.
	Cache *Cache
	// MaxBodyBytes caps a single origin response body. Currently
	// reserved — the underlying httpclient already enforces its own
	// MaxBodyBytes.
	MaxBodyBytes int64
	// MaxCacheBytes is the disk size cap. Eviction runs inline at the
	// end of every cache miss when > 0.
	MaxCacheBytes int64
	// CacheControlMaxAge is the browser-facing Cache-Control max-age in
	// seconds. Defaults to 30 days when 0 (per design.md §5).
	CacheControlMaxAge int
}

// Proxy serves /api/v1/proxy/<token>.
type Proxy struct {
	cfg     Config
	flights singleflight.Group
}

// New builds a Proxy. Cheap; one per process.
func New(cfg Config) *Proxy {
	if cfg.CacheControlMaxAge == 0 {
		cfg.CacheControlMaxAge = 30 * 24 * 3600
	}
	return &Proxy{cfg: cfg}
}

// allowedMIMEs is the spec's MIME-type allowlist (design.md §5).
var allowedMIMEs = map[string]struct{}{
	"image/jpeg":    {},
	"image/png":     {},
	"image/gif":     {},
	"image/webp":    {},
	"image/avif":    {},
	"image/svg+xml": {},
}

// errMIMENotAllowed signals a 415 response.
var errMIMENotAllowed = errors.New("proxy: mime not on allowlist")

// ServeHTTP handles a single proxy request.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy/")
	if tok == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	srcURL, err := DecodeToken(tok, p.cfg.Secret)
	if err != nil {
		http.Error(w, "bad token", http.StatusBadRequest)
		return
	}

	entry, ok, err := p.cfg.Cache.Get(srcURL)
	if err != nil {
		http.Error(w, "cache error", http.StatusInternalServerError)
		return
	}
	if ok {
		p.serveEntry(w, r, entry)
		return
	}

	// Coalesce concurrent misses for the same URL — one origin fetch
	// per URL even under high concurrency.
	v, err, _ := p.flights.Do(srcURL, func() (any, error) {
		return p.fetchAndCache(r.Context(), srcURL)
	})
	if err != nil {
		switch {
		case errors.Is(err, errMIMENotAllowed):
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		default:
			http.Error(w, "origin error", http.StatusNotFound)
		}
		return
	}
	p.serveEntry(w, r, v.(Entry))
}

// fetchAndCache fetches srcURL via the shared httpclient, validates the
// response, writes it to the cache, and returns the resulting Entry.
func (p *Proxy) fetchAndCache(ctx context.Context, srcURL string) (Entry, error) {
	resp, err := p.cfg.Client.Get(ctx, srcURL, nil)
	if err != nil {
		return Entry{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Entry{}, fmt.Errorf("origin returned %d", resp.StatusCode)
	}

	// mime.ParseMediaType strips parameters (charset, boundary, …) and
	// lowercases the type, both of which the allowlist needs.
	ct, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return Entry{}, errMIMENotAllowed
	}
	if _, ok := allowedMIMEs[ct]; !ok {
		return Entry{}, errMIMENotAllowed
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Entry{}, err
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		etag = etagOf(body)
	}
	if err := p.cfg.Cache.Put(srcURL, body, ct, etag); err != nil {
		return Entry{}, err
	}
	if p.cfg.MaxCacheBytes > 0 {
		_ = p.cfg.Cache.Evict(p.cfg.MaxCacheBytes)
	}
	return Entry{Body: body, ContentType: ct, ETag: etag}, nil
}

// serveEntry writes the cached entry to the response, honoring
// If-None-Match (304) and setting Cache-Control + ETag.
func (p *Proxy) serveEntry(w http.ResponseWriter, r *http.Request, e Entry) {
	if e.ETag != "" && r.Header.Get("If-None-Match") == e.ETag {
		w.Header().Set("ETag", e.ETag)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", e.ContentType)
	if e.ETag != "" {
		w.Header().Set("ETag", e.ETag)
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", p.cfg.CacheControlMaxAge))
	_, _ = w.Write(e.Body)
}

// Encoder returns a function suitable for reader.ProxyEncoder. It
// produces absolute Tap-relative URLs, e.g. "/api/v1/proxy/<token>".
func (p *Proxy) Encoder() func(string) string {
	secret := p.cfg.Secret
	return func(srcURL string) string {
		return "/api/v1/proxy/" + EncodeToken(srcURL, secret)
	}
}
