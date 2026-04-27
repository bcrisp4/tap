package poller

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunState_PollLifecycle(t *testing.T) {
	rs := NewRunState(8)
	rs.PollStarted()
	require.Equal(t, 1, rs.Snapshot().ActivePolls)
	rs.PollFinished(nil)
	snap := rs.Snapshot()
	require.Equal(t, 0, snap.ActivePolls)
	require.NotZero(t, snap.LastPollAt)
}

func TestRunState_RingBufferDropsOldest(t *testing.T) {
	rs := NewRunState(2)
	rs.PollStarted()
	rs.PollFinished(errors.New("first"))
	rs.PollStarted()
	rs.PollFinished(errors.New("second"))
	rs.PollStarted()
	rs.PollFinished(errors.New("third"))

	snap := rs.Snapshot()
	require.Len(t, snap.RecentErrors, 2)
	require.Equal(t, "second", snap.RecentErrors[0])
	require.Equal(t, "third", snap.RecentErrors[1])
}

func TestRunState_SnapshotIsCopy(t *testing.T) {
	rs := NewRunState(4)
	rs.PollStarted()
	rs.PollFinished(errors.New("e"))
	s1 := rs.Snapshot()
	s1.RecentErrors[0] = "MUTATED"
	s2 := rs.Snapshot()
	require.Equal(t, "e", s2.RecentErrors[0], "Snapshot must return a defensive copy")
}
