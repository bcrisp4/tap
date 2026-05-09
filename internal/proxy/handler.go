package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

// cacheControlImmutable is the Cache-Control value emitted for every successful
// proxy response. Source URLs are content-addressable (per concept §6.7), so
// an extreme max-age + immutable is correct — the same proxy URL will never
// return different bytes.
const cacheControlImmutable = "public, max-age=31536000, immutable"

// drainBytes is how much of an error-response body we read before discarding,
// so the connection can be reused. Tiny on purpose.
const drainBytes = 1024

// Handler serves GET /api/v1/proxy/{token}. It verifies the token, consults
// the cache, and on miss fetches the origin URL through the supplied client.
type Handler struct {
	signer  *Signer
	cache   *Cache
	client  *http.Client
	bodyCap int64
}

// NewHandler constructs the proxy HTTP handler. bodyCap is the per-response
// byte limit applied via http.MaxBytesReader.
func NewHandler(signer *Signer, cache *Cache, client *http.Client, bodyCap int64) http.Handler {
	if signer == nil || cache == nil {
		panic("proxy.NewHandler: signer and cache are required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Handler{signer: signer, cache: cache, client: client, bodyCap: bodyCap}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tok := r.PathValue("token")
	rawURL, ok := h.signer.Verify(tok)
	if !ok {
		http.NotFound(w, r)
		return
	}

	hash := urlHashOf(rawURL)
	got, err := h.cache.Get(r.Context(), hash, func(ctx context.Context) (FetchedResource, error) {
		return h.fetchOrigin(ctx, rawURL)
	})
	if err != nil {
		var fe *fetchError
		if errors.As(err, &fe) {
			http.Error(w, http.StatusText(fe.status), fe.status)
			return
		}
		slog.WarnContext(r.Context(), "proxy fetch failed", "url", rawURL, "err", err)
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", got.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(got.Bytes)))
	w.Header().Set("Cache-Control", cacheControlImmutable)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(got.Bytes)
}

// fetchError carries an HTTP status to surface back to the client without
// being treated as an internal cache write failure.
type fetchError struct {
	status int
	msg    string
}

func (e *fetchError) Error() string { return e.msg }

func (h *Handler) fetchOrigin(ctx context.Context, rawURL string) (FetchedResource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return FetchedResource{}, &fetchError{status: http.StatusBadGateway, msg: err.Error()}
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return FetchedResource{}, &fetchError{status: http.StatusBadGateway, msg: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Drain a few bytes to free the connection. 5xx → 502 to the client;
		// 4xx → status passes through unchanged (404 → 404, 410 → 410).
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainBytes))
		proxyStatus := resp.StatusCode
		if proxyStatus >= 500 {
			proxyStatus = http.StatusBadGateway
		}
		return FetchedResource{}, &fetchError{status: proxyStatus, msg: fmt.Sprintf("origin %d", resp.StatusCode)}
	}

	// MaxBytesReader: nil w is correct here — we're capping a *response* body
	// (origin → us), not a request body (client → us). The ResponseWriter
	// argument is only used to set 413 on request bodies; nil disables that.
	body, err := io.ReadAll(http.MaxBytesReader(nil, resp.Body, h.bodyCap))
	if err != nil {
		return FetchedResource{}, &fetchError{status: http.StatusBadGateway, msg: err.Error()}
	}

	sniffed, ok := validateImage(body, resp.Header.Get("Content-Type"))
	if !ok {
		return FetchedResource{}, &fetchError{status: http.StatusUnsupportedMediaType, msg: "MIME validation failed"}
	}

	return FetchedResource{
		Bytes:       body,
		ContentType: sniffed,
		ETag:        resp.Header.Get("ETag"),
	}, nil
}

func urlHashOf(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}
