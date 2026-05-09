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
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
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

	if resp.StatusCode >= 500 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return FetchedResource{}, &fetchError{status: http.StatusBadGateway, msg: fmt.Sprintf("origin %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 400 {
		// Drain a few bytes to free the connection and propagate the status.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return FetchedResource{}, &fetchError{status: resp.StatusCode, msg: fmt.Sprintf("origin %d", resp.StatusCode)}
	}

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
