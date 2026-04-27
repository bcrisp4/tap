package httpclient

import (
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
)

// brotliTransport wraps an http.RoundTripper so it negotiates brotli
// in addition to gzip.
//
// gzip is left to net/http: when no explicit Accept-Encoding is set,
// the stdlib transport adds "gzip" and decompresses transparently.
// We override that to "br, gzip" so the server can pick brotli,
// then if it does, decompress on our side.
type brotliTransport struct {
	wrapped http.RoundTripper
}

func newBrotliTransport(wrapped http.RoundTripper) http.RoundTripper {
	if wrapped == nil {
		wrapped = http.DefaultTransport
	}
	return &brotliTransport{wrapped: wrapped}
}

func (t *brotliTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Per the http.RoundTripper contract, RoundTrip must not mutate
	// the supplied request — clone before touching headers.
	// Setting Accept-Encoding manually disables stdlib's auto-gzip,
	// so we must include both.
	if req.Header.Get("Accept-Encoding") == "" {
		req = req.Clone(req.Context())
		req.Header.Set("Accept-Encoding", "br, gzip")
	}
	resp, err := t.wrapped.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if resp.Header.Get("Content-Encoding") == "br" {
		resp.Body = brotliReadCloser{r: brotli.NewReader(resp.Body), src: resp.Body}
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.ContentLength = -1
	}
	return resp, nil
}

type brotliReadCloser struct {
	r   io.Reader
	src io.Closer
}

func (b brotliReadCloser) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b brotliReadCloser) Close() error               { return b.src.Close() }
