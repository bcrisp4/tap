package httpx

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRT counts concurrent in-flight calls and (optionally) blocks on a chan
// so the test can orchestrate ordering.
type fakeRT struct {
	inflight    atomic.Int32
	maxObserved atomic.Int32
	block       chan struct{} // nil means don't block
}

func (f *fakeRT) RoundTrip(req *http.Request) (*http.Response, error) {
	n := f.inflight.Add(1)
	for {
		m := f.maxObserved.Load()
		if n <= m || f.maxObserved.CompareAndSwap(m, n) {
			break
		}
	}
	defer f.inflight.Add(-1)
	if f.block != nil {
		select {
		case <-f.block:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

func mustReq(t *testing.T, ctx context.Context, urlStr string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return req
}

func TestHostLimiter_SerialisesSameHost(t *testing.T) {
	inner := &fakeRT{block: make(chan struct{})}
	h := newHostLimiter(inner, 1)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := mustReq(t, context.Background(), "http://example.com/")
			resp, err := h.RoundTrip(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
	}

	time.Sleep(20 * time.Millisecond)
	if got := inner.inflight.Load(); got > 1 {
		t.Errorf("inflight = %d; want 1 max with cap=1", got)
	}
	close(inner.block)
	wg.Wait()
	if got := inner.maxObserved.Load(); got != 1 {
		t.Errorf("maxObserved = %d; want exactly 1", got)
	}
}

func TestHostLimiter_DifferentHostsParallel(t *testing.T) {
	inner := &fakeRT{block: make(chan struct{})}
	h := newHostLimiter(inner, 1)

	var wg sync.WaitGroup
	for _, host := range []string{"a.example", "b.example", "c.example"} {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			req := mustReq(t, context.Background(), "http://"+host+"/")
			resp, err := h.RoundTrip(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}(host)
	}
	time.Sleep(20 * time.Millisecond)
	if got := inner.inflight.Load(); got != 3 {
		t.Errorf("inflight = %d; want 3 (different hosts)", got)
	}
	close(inner.block)
	wg.Wait()
}

func TestHostLimiter_ContextCancelDuringWait(t *testing.T) {
	inner := &fakeRT{block: make(chan struct{})}
	h := newHostLimiter(inner, 1)

	holder := make(chan struct{})
	go func() {
		req := mustReq(t, context.Background(), "http://example.com/")
		resp, _ := h.RoundTrip(req)
		if resp != nil {
			<-holder
			_ = resp.Body.Close()
		}
	}()
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		req := mustReq(t, ctx, "http://example.com/")
		_, err := h.RoundTrip(req)
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected ctx error, got nil")
		}
	case <-time.After(time.Second):
		t.Fatal("RoundTrip did not return after ctx cancel")
	}
	close(inner.block)
	close(holder)
}

func TestHostLimiter_ReleaseOnBodyClose(t *testing.T) {
	inner := &fakeRT{}
	h := newHostLimiter(inner, 1)

	req1 := mustReq(t, context.Background(), "http://example.com/")
	resp1, err := h.RoundTrip(req1)
	if err != nil {
		t.Fatal(err)
	}

	req2 := mustReq(t, context.Background(), "http://example.com/")
	done := make(chan struct{})
	go func() {
		resp2, _ := h.RoundTrip(req2)
		if resp2 != nil {
			_ = resp2.Body.Close()
		}
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("second request returned before first body closed")
	case <-time.After(20 * time.Millisecond):
	}

	_ = resp1.Body.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second request did not proceed after first body closed")
	}
}

func TestHostLimiter_DoubleCloseIdempotent(t *testing.T) {
	inner := &fakeRT{}
	h := newHostLimiter(inner, 1)
	req := mustReq(t, context.Background(), "http://example.com/")
	resp, err := h.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	_ = resp.Body.Close() // must not panic, must not over-release
	done := make(chan struct{})
	go func() {
		req2 := mustReq(t, context.Background(), "http://example.com/")
		if r, _ := h.RoundTrip(req2); r != nil {
			_ = r.Body.Close()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("third request blocked — slot was double-released")
	}
}

// TestHostLimiter_SameHostCasingAndTrailingDot verifies the limiter
// normalises hostnames before keying the semaphore map. Without
// normalisation, "Example.COM" / "example.com" / "example.com." would
// each get their own slot, defeating the per-host cap.
func TestHostLimiter_SameHostCasingAndTrailingDot(t *testing.T) {
	inner := &fakeRT{block: make(chan struct{})}
	h := newHostLimiter(inner, 1)

	var wg sync.WaitGroup
	for _, host := range []string{"example.com", "Example.COM", "example.com."} {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			req := mustReq(t, context.Background(), "http://"+host+"/")
			resp, err := h.RoundTrip(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}(host)
	}

	time.Sleep(20 * time.Millisecond)
	if got := inner.inflight.Load(); got > 1 {
		t.Errorf("inflight = %d; want 1 max — same host with different casing/trailing dot must share slot", got)
	}
	close(inner.block)
	wg.Wait()
	if got := inner.maxObserved.Load(); got != 1 {
		t.Errorf("maxObserved = %d; want exactly 1 (same host)", got)
	}
}
