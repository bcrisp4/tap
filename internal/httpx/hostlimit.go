package httpx

import (
	"io"
	"net/http"
	"sync"
)

// hostLimiter is a RoundTripper that caps concurrent in-flight requests per
// hostname using a buffered chan as a counting semaphore. The map of chans
// is lazy — a host's chan is allocated on its first request — and guarded
// by a single mutex held only while reading/writing the map (not while
// waiting on a slot). The slot is released on response Body.Close() so
// HTTP/1.1 keep-alive connections aren't reused before the prior caller
// has finished reading.
type hostLimiter struct {
	inner http.RoundTripper
	n     int
	mu    sync.Mutex
	sem   map[string]chan struct{}
}

func newHostLimiter(inner http.RoundTripper, n int) *hostLimiter {
	return &hostLimiter{inner: inner, n: n, sem: map[string]chan struct{}{}}
}

func (h *hostLimiter) acquireChan(host string) chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sem[host]
	if !ok {
		s = make(chan struct{}, h.n)
		h.sem[host] = s
	}
	return s
}

func (h *hostLimiter) RoundTrip(req *http.Request) (*http.Response, error) {
	if h.n <= 0 {
		return h.inner.RoundTrip(req)
	}
	// Normalise so case and trailing-dot variants share the same semaphore —
	// without this, "Example.COM" / "example.com" / "example.com." would
	// each get their own slot and defeat the cap.
	sem := h.acquireChan(normaliseHost(req.URL.Hostname()))

	select {
	case sem <- struct{}{}:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}

	resp, err := h.inner.RoundTrip(req)
	if err != nil {
		<-sem
		return nil, err
	}
	resp.Body = &releasingBody{
		ReadCloser: resp.Body,
		release:    func() { <-sem },
	}
	return resp, nil
}

type releasingBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *releasingBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}
