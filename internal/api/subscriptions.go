package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/bcrisp4/tap/internal/db"
)

type subscriptionDTO struct {
	ID              int64   `json:"id"`
	Title           string  `json:"title"`
	FeedURL         string  `json:"feed_url"`
	SiteURL         string  `json:"site_url,omitempty"`
	NextPollAt      int64   `json:"next_poll_at"`
	LastPollAt      int64   `json:"last_poll_at,omitempty"`
	ErrorCount      int     `json:"error_count"`
	LastError       string  `json:"last_error,omitempty"`
	CreatedAt       int64   `json:"created_at"`
	Extract         bool    `json:"extract"`
	ExtractSelector string  `json:"extract_selector"`
	HasCookie       bool    `json:"has_cookie"`
	HasBasicAuth    bool    `json:"has_basic_auth"`
	CategoryID      *int64  `json:"category_id"`
}

func toDTO(s db.Subscription) subscriptionDTO {
	d := subscriptionDTO{
		ID:              s.ID,
		Title:           s.Title,
		FeedURL:         s.FeedURL,
		NextPollAt:      s.NextPollAt,
		ErrorCount:      s.ErrorCount,
		CreatedAt:       s.CreatedAt,
		Extract:         s.Extract,
		ExtractSelector: s.ExtractSelector,
		HasCookie:       s.Cookie != "",
		HasBasicAuth:    s.BasicAuthUser != "",
	}
	if s.SiteURL.Valid {
		d.SiteURL = s.SiteURL.String
	}
	if s.LastPollAt.Valid {
		d.LastPollAt = s.LastPollAt.Int64
	}
	if s.LastError.Valid {
		d.LastError = s.LastError.String
	}
	if s.CategoryID.Valid {
		v := s.CategoryID.Int64
		d.CategoryID = &v
	}
	return d
}

// validateCategoryOwnership checks that categoryID belongs to the given user.
// Returns (true, nil) when valid, (false, nil) when the category doesn't exist,
// and (false, err) on a DB error.
func validateCategoryOwnership(r *http.Request, d *sql.DB, categoryID, userID int64) (bool, error) {
	_, err := db.GetCategory(r.Context(), d, categoryID, userID)
	if err != nil {
		if errors.Is(err, db.ErrCategoryNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func registerSubscriptionRoutes(m *http.ServeMux, d *sql.DB, poke func()) {
	m.HandleFunc("GET /api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		subs, err := db.ListSubscriptions(r.Context(), d, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		out := make([]subscriptionDTO, 0, len(subs))
		for _, s := range subs {
			out = append(out, toDTO(s))
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	})

	m.HandleFunc("POST /api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB cap on request body
		var body struct {
			FeedURL       string `json:"feed_url"`
			Title         string `json:"title"`
			Extract       bool   `json:"extract"`
			Cookie        string `json:"cookie"`
			BasicAuthUser string `json:"basic_auth_user"`
			BasicAuthPass string `json:"basic_auth_pass"`
			CategoryID    *int64 `json:"category_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeRequestTooLarge, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		feedURL, err := url.Parse(body.FeedURL)
		if err != nil || !feedURL.IsAbs() {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "feed_url must be an absolute URL")
			return
		}
		if feedURL.Scheme != "http" && feedURL.Scheme != "https" {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "feed_url must use http or https")
			return
		}
		if body.CategoryID != nil {
			ok, err := validateCategoryOwnership(r, d, *body.CategoryID, u.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			if !ok {
				writeError(w, http.StatusBadRequest, ErrCodeCategoryNotFound, "category not found")
				return
			}
		}
		title := body.Title
		if title == "" {
			title = body.FeedURL
		}
		id, err := db.InsertSubscription(r.Context(), d, db.NewSubscription{
			UserID:        u.ID,
			Title:         title,
			FeedURL:       body.FeedURL,
			NextPoll:      0,
			Created:       time.Now().Unix(),
			Extract:       body.Extract,
			Cookie:        body.Cookie,
			BasicAuthUser: body.BasicAuthUser,
			BasicAuthPass: body.BasicAuthPass,
			CategoryID:    body.CategoryID,
		})
		if err != nil {
			if errors.Is(err, db.ErrSubscriptionExists) {
				writeError(w, http.StatusConflict, ErrCodeConflict, "subscription already exists")
				return
			}
			slog.Error("insert subscription failed", "feed_url", body.FeedURL, "err", err)
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "could not create subscription")
			return
		}
		s, err := db.GetSubscription(r.Context(), d, id, u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		// Poke the scheduler so the new feed polls within seconds, not
		// up-to-TickInterval. Best-effort: a missed poke just delays
		// the first poll to the next regular tick.
		if poke != nil {
			poke()
		}
		writeJSON(w, http.StatusCreated, toDTO(s))
	})

	m.HandleFunc("PATCH /api/v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		// Decode into a raw map so we can detect explicit null vs omitted category_id.
		var rawMap map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&rawMap); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeRequestTooLarge, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		var body struct {
			Extract         *bool   `json:"extract"`
			ExtractSelector *string `json:"extract_selector"`
			Cookie          *string `json:"cookie"`
			BasicAuthUser   *string `json:"basic_auth_user"`
			BasicAuthPass   *string `json:"basic_auth_pass"`
		}
		// Re-decode typed fields from map values; return 400 on type mismatch.
		for k, v := range rawMap {
			var unmarshalErr error
			switch k {
			case "extract":
				unmarshalErr = json.Unmarshal(v, &body.Extract)
			case "extract_selector":
				unmarshalErr = json.Unmarshal(v, &body.ExtractSelector)
			case "cookie":
				unmarshalErr = json.Unmarshal(v, &body.Cookie)
			case "basic_auth_user":
				unmarshalErr = json.Unmarshal(v, &body.BasicAuthUser)
			case "basic_auth_pass":
				unmarshalErr = json.Unmarshal(v, &body.BasicAuthPass)
			}
			if unmarshalErr != nil {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid value for field "+k)
				return
			}
		}

		// Pre-read the row so omitted PATCH fields keep their current values.
		s, err := db.GetSubscription(r.Context(), d, id, u.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		extract := s.Extract
		selector := s.ExtractSelector
		cookie := s.Cookie
		basicUser := s.BasicAuthUser
		basicPass := s.BasicAuthPass

		if body.Extract != nil {
			extract = *body.Extract
		}
		if body.ExtractSelector != nil {
			candidate := *body.ExtractSelector
			if candidate != "" {
				if _, cerr := cascadia.Compile(candidate); cerr != nil {
					writeError(w, http.StatusBadRequest, ErrCodeExtractSelectorInvalid,
						"extract_selector did not compile: "+cerr.Error())
					return
				}
			}
			selector = candidate
		}
		if body.Cookie != nil {
			cookie = *body.Cookie
		}
		if body.BasicAuthUser != nil {
			basicUser = *body.BasicAuthUser
		}
		if body.BasicAuthPass != nil {
			basicPass = *body.BasicAuthPass
		}

		if err := db.UpdateSubscriptionPatch(r.Context(), d, id, u.ID,
			extract, selector, cookie, basicUser, basicPass); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		// Handle refresh_now: when true, reset next_poll_at = 0 and poke the scheduler.
		if raw, ok := rawMap["refresh_now"]; ok {
			var refreshNow bool
			if err := json.Unmarshal(raw, &refreshNow); err != nil {
				writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "refresh_now must be a boolean")
				return
			}
			if refreshNow {
				if err := db.UpdateSubscriptionRefreshNow(r.Context(), d, id, u.ID); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
						return
					}
					writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
					return
				}
				if poke != nil {
					poke()
				}
				// Re-read the row so the DTO reflects the new next_poll_at.
				s, err = db.GetSubscription(r.Context(), d, id, u.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
					return
				}
			}
		}

		// Handle category_id: if present in the raw map, update it (null = uncategorise).
		if raw, ok := rawMap["category_id"]; ok {
			var catID *int64
			if string(raw) != "null" {
				var cid int64
				if err := json.Unmarshal(raw, &cid); err != nil {
					writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "category_id must be an integer or null")
					return
				}
				// Validate the category belongs to this user.
				owned, err := validateCategoryOwnership(r, d, cid, u.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
					return
				}
				if !owned {
					writeError(w, http.StatusBadRequest, ErrCodeCategoryNotFound, "category not found")
					return
				}
				catID = &cid
			}
			if err := db.UpdateSubscriptionCategory(r.Context(), d, id, u.ID, catID); err != nil && !errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
			// Refresh the subscription to get updated category_id.
			s, err = db.GetSubscription(r.Context(), d, id, u.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
				return
			}
		}

		s.Extract = extract
		s.ExtractSelector = selector
		s.Cookie = cookie
		s.BasicAuthUser = basicUser
		s.BasicAuthPass = basicPass
		writeJSON(w, http.StatusOK, toDTO(s))
	})

	m.HandleFunc("GET /api/v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		s, err := db.GetSubscription(r.Context(), d, id, u.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toDTO(s))
	})

	m.HandleFunc("DELETE /api/v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		if err := db.DeleteSubscription(r.Context(), d, id, u.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	m.HandleFunc("POST /api/v1/subscriptions/{id}/mark-read", func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
			return
		}
		// Ownership check — return 404 on a wrong-user attempt to avoid leaking the
		// existence of another user's subscription IDs.
		if _, err := db.GetSubscription(r.Context(), d, id, u.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
				return
			}
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.MarkSubscriptionRead(r.Context(), d, id, u.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
