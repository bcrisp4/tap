package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/ring"
)

type statusResponse struct {
	Version       string        `json:"version"`
	UptimeSeconds int64         `json:"uptime_seconds"`
	DB            string        `json:"db"`
	PollsActive   int64         `json:"polls_active"`
	PollsTotal    int64         `json:"polls_total"`
	LastPollAt    *int64        `json:"last_poll_at"`
	RecentErrors  []statusEvent `json:"recent_errors"`
}

type statusEvent struct {
	Time  string         `json:"time"`
	Level string         `json:"level"`
	Event string         `json:"event"`
	Attrs map[string]any `json:"attrs"`
}

type statusDeps struct {
	db          *sql.DB
	buf         *ring.Buffer
	startTime   time.Time
	version     string
	pollsActive func() int64
	pollsTotal  func() int64
	lastPollAt  func() *int64
}

func statusHandler(deps statusDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok || u.Role != "admin" {
			writeError(w, http.StatusForbidden, ErrCodeForbidden, "forbidden")
			return
		}

		dbStatus := "ok"
		if deps.db != nil {
			if err := deps.db.PingContext(r.Context()); err != nil {
				dbStatus = "degraded"
			}
		}

		var active, total int64
		var lastAt *int64
		if deps.pollsActive != nil {
			active = deps.pollsActive()
		}
		if deps.pollsTotal != nil {
			total = deps.pollsTotal()
		}
		if deps.lastPollAt != nil {
			lastAt = deps.lastPollAt()
		}

		var events []ring.Event
		if deps.buf != nil {
			events = deps.buf.Recent(20)
		}
		recent := make([]statusEvent, len(events))
		for i, e := range events {
			recent[i] = statusEvent{
				Time:  e.Time.UTC().Format(time.RFC3339),
				Level: e.Level,
				Event: e.Event,
				Attrs: e.Attrs,
			}
		}

		resp := statusResponse{
			Version:       deps.version,
			UptimeSeconds: int64(time.Since(deps.startTime).Seconds()),
			DB:            dbStatus,
			PollsActive:   active,
			PollsTotal:    total,
			LastPollAt:    lastAt,
			RecentErrors:  recent,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}
