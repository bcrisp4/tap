package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bcrisp4/tap/internal/storage"
)

// iconHandlers serves cached favicon bytes by their content-addressable
// sha256 hash. Bytes are stored once, served forever — the URL is
// effectively immutable, so the response is cached aggressively.
type iconHandlers struct {
	store *storage.Store
}

// get streams the cached icon bytes for the requested hash. Responds
// 404 when the hash isn't known. Sets a content-addressable ETag and
// a year-long immutable Cache-Control so browsers + intermediaries
// can keep favicons indefinitely.
func (h *iconHandlers) get(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.PathValue("hash"))
	if hash == "" {
		WriteError(w, http.StatusBadRequest, "bad_hash", "icon hash is required")
		return
	}

	// ETag short-circuit: if the client already has this exact hash
	// cached, skip the body. The hash is the entire content
	// fingerprint, so an If-None-Match match is a guaranteed hit.
	if match := r.Header.Get("If-None-Match"); match != "" && etagMatches(match, hash) {
		w.Header().Set("ETag", quoteETag(hash))
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	icon, err := h.store.GetIconByHash(r.Context(), hash)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "not_found", "icon not found")
			return
		}
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", icon.MIMEType)
	w.Header().Set("ETag", quoteETag(hash))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(icon.Content)
}

// quoteETag wraps the hash in the ETag header's required double
// quotes. The hash is always hex (sha256), so it's safe inside an
// unescaped quoted string.
func quoteETag(hash string) string { return `"` + hash + `"` }

// etagMatches reports whether the If-None-Match header's value
// includes our content-addressable ETag. Handles the common cases
// (single quoted ETag, comma-separated list, and the bare wildcard).
func etagMatches(header, hash string) bool {
	header = strings.TrimSpace(header)
	if header == "*" {
		return true
	}
	target := quoteETag(hash)
	for _, part := range strings.Split(header, ",") {
		// Strip the optional W/ weak-etag prefix; favicon responses
		// are always strong, so a weak match is still a hit.
		part = strings.TrimPrefix(strings.TrimSpace(part), "W/")
		if part == target {
			return true
		}
	}
	return false
}
