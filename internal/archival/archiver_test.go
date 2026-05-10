package archival

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestArchiver_StartStop_NoSweep(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	a := NewArchiver(d, ArchiverOpts{
		Interval: 24 * time.Hour,
		CacheDir: t.TempDir(),
	})
	a.Start()
	a.Stop()
}

func TestArchiver_Stop_WaitsForInProgressSweep(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	gate := make(chan struct{})
	var started atomic.Bool

	a := NewArchiver(d, ArchiverOpts{
		Interval:  1 * time.Millisecond,
		CacheDir:  t.TempDir(),
		sweepHook: func() { started.Store(true); <-gate },
	})
	a.Start()

	require.Eventually(t, func() bool { return started.Load() }, 2*time.Second, 5*time.Millisecond)

	stopDone := make(chan struct{})
	go func() { a.Stop(); close(stopDone) }()

	select {
	case <-stopDone:
		t.Fatal("Stop() returned before sweep finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(gate)
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return after sweep unblocked")
	}
}

func TestArchiver_SweepFires(t *testing.T) {
	t.Parallel()
	d := openTestDB(t)
	var count atomic.Int32
	a := NewArchiver(d, ArchiverOpts{
		Interval:  1 * time.Millisecond,
		CacheDir:  t.TempDir(),
		sweepHook: func() { count.Add(1) },
	})
	a.Start()
	require.Eventually(t, func() bool { return count.Load() >= 1 }, 2*time.Second, 5*time.Millisecond)
	a.Stop()
	require.GreaterOrEqual(t, count.Load(), int32(1))
}
