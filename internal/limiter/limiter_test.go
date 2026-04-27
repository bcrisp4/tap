package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHostLimiter_SerializesPerHost(t *testing.T) {
	l := NewHostLimiter(1)
	var inFlight, peak int32

	work := func(host string) {
		ctx := context.Background()
		require.NoError(t, l.Acquire(ctx, host))
		defer l.Release(host)
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if cur <= p || atomic.CompareAndSwapInt32(&peak, p, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
	}

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); work("example.com") }()
	}
	wg.Wait()
	require.EqualValues(t, 1, peak, "weight=1 → never more than one in flight per host")
}

func TestHostLimiter_DifferentHostsConcurrent(t *testing.T) {
	l := NewHostLimiter(1)
	ctx := context.Background()

	require.NoError(t, l.Acquire(ctx, "a.com"))
	defer l.Release("a.com")

	// b.com must not be blocked by a.com.
	done := make(chan struct{})
	go func() {
		require.NoError(t, l.Acquire(ctx, "b.com"))
		l.Release("b.com")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("b.com acquire blocked by a.com")
	}
}
