package poller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordDispatchPanic_RecordsAsProcessWide(t *testing.T) {
	state := NewRunState(8)
	recordDispatchPanic(state, 42, "boom")

	snap := state.Snapshot()
	require.Len(t, snap.RecentErrors, 1)
	e := snap.RecentErrors[0]

	require.EqualValues(t, 0, e.FeedID, "dispatcher panics are process-wide; feed_id must be 0")
	require.Empty(t, e.FeedTitle, "dispatcher panics are process-wide; feed_title must be empty")
	require.Contains(t, e.Error, "poller panic on feed 42", "feed ID must be preserved in the message for forensics")
	require.Contains(t, e.Error, "boom", "panic value must be in the message")
	require.NotZero(t, e.At)
}

func TestRecordDispatchPanic_NilStateIsNoOp(t *testing.T) {
	require.NotPanics(t, func() { recordDispatchPanic(nil, 42, "boom") })
}
