package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/poller"
)

func TestSystem_Status(t *testing.T) {
	f := newAPIFixture(t)
	f.state.PollStarted()
	f.state.PollFinished(0, "", nil)

	w := f.do(t, "GET", "/api/v1/system/status", "")
	require.Equal(t, http.StatusOK, w.Code)

	var got struct {
		Version       string `json:"version"`
		UptimeSeconds int64  `json:"uptime_seconds"`
		RunState      struct {
			ActivePolls  int                  `json:"active_polls"`
			LastPollAt   int64                `json:"last_poll_at"`
			RecentErrors []poller.PollerError `json:"recent_errors,omitempty"`
		} `json:"run_state"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.NotEmpty(t, got.Version)
	require.GreaterOrEqual(t, got.UptimeSeconds, int64(0))
	require.Equal(t, 0, got.RunState.ActivePolls)
	require.Greater(t, got.RunState.LastPollAt, int64(0))
}

func TestSystem_Status_IncludesRecentErrors(t *testing.T) {
	f := newAPIFixture(t)
	f.state.PollStarted()
	f.state.PollFinished(0, "", errors.New("boom"))

	w := f.do(t, "GET", "/api/v1/system/status", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"recent_errors"`)
	require.Contains(t, w.Body.String(), "boom")
}

func TestSystem_Status_RecentErrorsShape(t *testing.T) {
	f := newAPIFixture(t)
	f.state.PollStarted()
	f.state.PollFinished(17, "Hacker News", errors.New("context deadline exceeded"))

	w := f.do(t, "GET", "/api/v1/system/status", "")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		RunState struct {
			RecentErrors []poller.PollerError `json:"recent_errors"`
		} `json:"run_state"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.RunState.RecentErrors, 1)
	require.Equal(t, int64(17), body.RunState.RecentErrors[0].FeedID)
	require.Equal(t, "Hacker News", body.RunState.RecentErrors[0].FeedTitle)
	require.Equal(t, "context deadline exceeded", body.RunState.RecentErrors[0].Error)
	require.NotZero(t, body.RunState.RecentErrors[0].At)
}
