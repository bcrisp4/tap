package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/bcrisp4/tap/internal/discover"
)

type discoverCandidateDTO struct {
	Title   string `json:"title"`
	FeedURL string `json:"feed_url"`
	SiteURL string `json:"site_url"`
	Type    string `json:"type"`
}

func registerDiscoverRoutes(m *http.ServeMux, client *http.Client) {
	m.HandleFunc("POST /api/v1/discover", func(w http.ResponseWriter, r *http.Request) {
		_, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		if u, err := url.Parse(body.URL); err != nil || !u.IsAbs() {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "url must be an absolute URL")
			return
		}

		results, err := discover.Discover(r.Context(), client, body.URL)
		if err != nil {
			if errors.Is(err, discover.ErrNoFeeds) {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"error":      map[string]string{"code": ErrCodeNoFeedsFound, "message": "no feeds found at that URL"},
					"candidates": []discoverCandidateDTO{},
				})
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
			return
		}

		candidates := make([]discoverCandidateDTO, 0, len(results))
		for _, r := range results {
			candidates = append(candidates, discoverCandidateDTO{
				Title:   r.Title,
				FeedURL: r.FeedURL,
				SiteURL: r.SiteURL,
				Type:    r.Type,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"candidates": candidates})
	})
}
