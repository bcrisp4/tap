package poller

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunState_PollLifecycle(t *testing.T) {
	rs := NewRunState(8)
	rs.PollStarted()
	require.Equal(t, 1, rs.Snapshot().ActivePolls)
	rs.PollFinished(0, "", nil)
	snap := rs.Snapshot()
	require.Equal(t, 0, snap.ActivePolls)
	require.NotZero(t, snap.LastPollAt)
}

func TestRunState_RingBufferDropsOldest(t *testing.T) {
	rs := NewRunState(2)
	rs.PollStarted()
	rs.PollFinished(1, "Feed A", errors.New("first"))
	rs.PollStarted()
	rs.PollFinished(2, "Feed B", errors.New("second"))
	rs.PollStarted()
	rs.PollFinished(3, "Feed C", errors.New("third"))

	snap := rs.Snapshot()
	require.Len(t, snap.RecentErrors, 2)
	require.Equal(t, "second", snap.RecentErrors[0].Error)
	require.Equal(t, "third", snap.RecentErrors[1].Error)
}

func TestRunState_SnapshotIsCopy(t *testing.T) {
	rs := NewRunState(4)
	rs.PollStarted()
	rs.PollFinished(1, "Feed A", errors.New("e"))
	s1 := rs.Snapshot()
	s1.RecentErrors[0].Error = "MUTATED"
	s2 := rs.Snapshot()
	require.Equal(t, "e", s2.RecentErrors[0].Error, "Snapshot must return a defensive copy")
}

func TestRunState_RecordsFeedMetadata(t *testing.T) {
	rs := NewRunState(8)
	rs.PollStarted()
	rs.PollFinished(17, "Hacker News", errors.New("context deadline exceeded"))

	got := rs.Snapshot().RecentErrors
	require.Len(t, got, 1)
	e := got[0]
	require.Equal(t, int64(17), e.FeedID)
	require.Equal(t, "Hacker News", e.FeedTitle)
	require.Equal(t, "context deadline exceeded", e.Error)
	require.NotZero(t, e.At)
	require.LessOrEqual(t, e.At, time.Now().Unix())
}

func TestRunState_RecordErrorTakesFeedMetadata(t *testing.T) {
	rs := NewRunState(4)
	rs.RecordError(42, "Archival", errors.New("sweep failed"))

	got := rs.Snapshot().RecentErrors
	require.Len(t, got, 1)
	require.Equal(t, int64(42), got[0].FeedID)
	require.Equal(t, "Archival", got[0].FeedTitle)
	require.Equal(t, "sweep failed", got[0].Error)
	require.NotZero(t, got[0].At)
}
