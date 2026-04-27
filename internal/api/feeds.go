package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/storage"
)

// userID is the single hard-coded v1 user. design.md §1 caps Tap at
// one user; auth lands post-v1.
const userID int64 = 1

type feedHandlers struct {
	store  *storage.Store
	client *httpclient.Client
}

func (h *feedHandlers) list(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.store.ListFeeds(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	WriteList(w, feeds, len(feeds), 0, len(feeds))
}

type subscribeReq struct {
	FeedURL    string `json:"feed_url"`
	Title      string `json:"title"`
	CategoryID *int64 `json:"category_id,omitempty"`
}

type idResponse struct {
	ID int64 `json:"id"`
}

func (h *feedHandlers) subscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.FeedURL == "" {
		WriteError(w, http.StatusBadRequest, "missing_feed_url", "feed_url is required")
		return
	}
	title := req.Title
	if title == "" {
		title = req.FeedURL
	}
	now := time.Now().Unix()
	feed := &storage.Feed{
		UserID:       userID,
		CategoryID:   req.CategoryID,
		Title:        title,
		FeedURL:      req.FeedURL,
		PollInterval: 3600,
		NextPollAt:   &now,
	}
	id, err := h.store.CreateFeed(r.Context(), feed)
	if errors.Is(err, storage.ErrConflict) {
		WriteError(w, http.StatusConflict, "duplicate", "already subscribed to "+req.FeedURL)
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	WriteOK(w, http.StatusCreated, idResponse{ID: id})
}

func (h *feedHandlers) get(w http.ResponseWriter, r *http.Request) {
	id, ok := requirePathID(w, r, "feed")
	if !ok {
		return
	}
	feed, err := h.store.GetFeed(r.Context(), userID, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, feed)
}

type updateFeedReq struct {
	Title              *string `json:"title,omitempty"`
	CategoryID         *int64  `json:"category_id,omitempty"`
	FeedURL            *string `json:"feed_url,omitempty"`
	SiteURL            *string `json:"site_url,omitempty"`
	Crawler            *bool   `json:"crawler,omitempty"`
	ScraperRules       *string `json:"scraper_rules,omitempty"`
	Disabled           *bool   `json:"disabled,omitempty"`
	IgnoreEntryUpdates *bool   `json:"ignore_entry_updates,omitempty"`
	UserAgent          *string `json:"user_agent,omitempty"`
	Cookie             *string `json:"cookie,omitempty"`
	Username           *string `json:"username,omitempty"`
	Password           *string `json:"password,omitempty"`
	ProxyURL           *string `json:"proxy_url,omitempty"`
}

// applyUpdate copies non-nil fields from req onto feed in place.
func (req *updateFeedReq) applyUpdate(feed *storage.Feed) {
	if req.Title != nil {
		feed.Title = *req.Title
	}
	if req.CategoryID != nil {
		feed.CategoryID = req.CategoryID
	}
	if req.FeedURL != nil {
		feed.FeedURL = *req.FeedURL
	}
	if req.SiteURL != nil {
		feed.SiteURL = req.SiteURL
	}
	if req.Crawler != nil {
		feed.Crawler = *req.Crawler
	}
	if req.ScraperRules != nil {
		feed.ScraperRules = req.ScraperRules
	}
	if req.Disabled != nil {
		feed.Disabled = *req.Disabled
	}
	if req.IgnoreEntryUpdates != nil {
		feed.IgnoreEntryUpdates = *req.IgnoreEntryUpdates
	}
	if req.UserAgent != nil {
		feed.UserAgent = req.UserAgent
	}
	if req.Cookie != nil {
		feed.Cookie = req.Cookie
	}
	if req.Username != nil {
		feed.Username = req.Username
	}
	if req.Password != nil {
		feed.Password = req.Password
	}
	if req.ProxyURL != nil {
		feed.ProxyURL = req.ProxyURL
	}
}

func (h *feedHandlers) update(w http.ResponseWriter, r *http.Request) {
	id, ok := requirePathID(w, r, "feed")
	if !ok {
		return
	}
	feed, err := h.store.GetFeed(r.Context(), userID, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	var req updateFeedReq
	if !decodeJSON(w, r, &req) {
		return
	}
	req.applyUpdate(feed)
	if err := h.store.UpdateFeed(r.Context(), feed); err != nil {
		writeErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, feed)
}

func (h *feedHandlers) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := requirePathID(w, r, "feed")
	if !ok {
		return
	}
	if err := h.store.DeleteFeed(r.Context(), userID, id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// refresh sets next_poll_at = now so the dispatcher picks the feed up
// on its next tick. Returns 202 Accepted: the actual fetch happens
// asynchronously.
func (h *feedHandlers) refresh(w http.ResponseWriter, r *http.Request) {
	id, ok := requirePathID(w, r, "feed")
	if !ok {
		return
	}
	if err := h.store.SetNextPollAt(r.Context(), userID, id, time.Now().Unix()); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

type discoverReq struct {
	URL string `json:"url"`
}

type discoverCandidate struct {
	HRef  string `json:"href"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type,omitempty"`
}

type discoverResponse struct {
	Candidates []discoverCandidate `json:"candidates"`
}

func (h *feedHandlers) discover(w http.ResponseWriter, r *http.Request) {
	var req discoverReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.URL == "" {
		WriteError(w, http.StatusBadRequest, "missing_url", "url is required")
		return
	}
	resp, err := h.client.Get(r.Context(), req.URL, nil)
	if err != nil {
		WriteError(w, http.StatusBadGateway, "fetch_failed", err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		WriteError(w, http.StatusBadGateway, "read_failed", err.Error())
		return
	}
	WriteOK(w, http.StatusOK, discoverResponse{
		Candidates: extractFeedLinks(body, req.URL),
	})
}

// feedLinkTypes lists the @type values we treat as feed candidates on
// <link rel="alternate"> tags.
var feedLinkTypes = map[string]bool{
	"application/rss+xml":   true,
	"application/atom+xml":  true,
	"application/feed+json": true,
	"application/json":      true,
}

// extractFeedLinks walks <link rel="alternate"> nodes and returns the
// candidates with absolute URLs (relative hrefs are resolved against
// baseURL via net/url.ResolveReference).
func extractFeedLinks(htmlBody []byte, baseURL string) []discoverCandidate {
	base, _ := url.Parse(baseURL)
	out := []discoverCandidate{}
	z := html.NewTokenizer(bytes.NewReader(htmlBody))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return out
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		name, hasAttr := z.TagName()
		if string(name) != "link" || !hasAttr {
			continue
		}
		attrs := readAttrs(z)
		if attrs["rel"] != "alternate" {
			continue
		}
		if !feedLinkTypes[strings.ToLower(attrs["type"])] {
			continue
		}
		out = append(out, discoverCandidate{
			HRef:  resolveHRef(attrs["href"], base),
			Title: attrs["title"],
			Type:  attrs["type"],
		})
	}
}

func readAttrs(z *html.Tokenizer) map[string]string {
	attrs := map[string]string{}
	for {
		k, v, more := z.TagAttr()
		attrs[strings.ToLower(string(k))] = string(v)
		if !more {
			return attrs
		}
	}
}

// resolveHRef turns a relative href into an absolute URL using
// net/url.ResolveReference. Returns href verbatim on parse failure
// (absolute URLs round-trip cleanly).
func resolveHRef(href string, base *url.URL) string {
	if href == "" {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil || base == nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

// pathInt extracts an integer path segment by name from r.PathValue.
func pathInt(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// requirePathID is the common preamble used by every handler keyed on
// {id}: parse the path segment, or write 400 bad_id and return false.
func requirePathID(w http.ResponseWriter, r *http.Request, resource string) (int64, bool) {
	id, ok := pathInt(r, "id")
	if !ok {
		WriteError(w, http.StatusBadRequest, "bad_id", resource+" id must be integer")
	}
	return id, ok
}
