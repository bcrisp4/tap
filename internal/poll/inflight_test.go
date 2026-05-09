package poll

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInflight_AcquireAndRelease(t *testing.T) {
	t.Parallel()
	tr := NewInflight()

	require.True(t, tr.TryAcquire(1))
	require.False(t, tr.TryAcquire(1), "duplicate acquire should fail")
	require.True(t, tr.TryAcquire(2), "different id should succeed")

	tr.Release(1)
	require.True(t, tr.TryAcquire(1), "release should let it be re-acquired")
}

func TestInflight_Concurrent(t *testing.T) {
	t.Parallel()
	tr := NewInflight()
	var wg sync.WaitGroup
	wins := make(chan int, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tr.TryAcquire(7) {
				wins <- 1
			}
		}()
	}
	wg.Wait()
	close(wins)

	count := 0
	for range wins {
		count++
	}
	require.Equal(t, 1, count, "exactly one goroutine should win the acquire race")
}
