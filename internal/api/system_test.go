package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
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
			ActivePolls  int      `json:"active_polls"`
			LastPollAt   int64    `json:"last_poll_at"`
			RecentErrors []string `json:"recent_errors,omitempty"`
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
