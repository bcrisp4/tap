package api

import (
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/poller"
	"github.com/bcrisp4/tap/internal/version"
)

type systemHandlers struct {
	state     *poller.RunState
	startedAt time.Time
}

// runStateOut is the wire shape of the run_state field. Mirrors
// poller.RunStateSnapshot but with explicit JSON tags + an omitempty
// on RecentErrors so a clean process emits a tidy payload.
type runStateOut struct {
	ActivePolls  int      `json:"active_polls"`
	LastPollAt   int64    `json:"last_poll_at"`
	RecentErrors []string `json:"recent_errors,omitempty"`
}

type statusResponse struct {
	Version       string      `json:"version"`
	UptimeSeconds int64       `json:"uptime_seconds"`
	RunState      runStateOut `json:"run_state"`
}

func (h *systemHandlers) status(w http.ResponseWriter, _ *http.Request) {
	snap := h.state.Snapshot()
	errs := make([]string, len(snap.RecentErrors))
	for i, e := range snap.RecentErrors {
		errs[i] = e.Error
	}
	WriteOK(w, http.StatusOK, statusResponse{
		Version:       version.String(),
		UptimeSeconds: int64(time.Since(h.startedAt).Seconds()),
		RunState: runStateOut{
			ActivePolls:  snap.ActivePolls,
			LastPollAt:   snap.LastPollAt,
			RecentErrors: errs,
		},
	})
}
