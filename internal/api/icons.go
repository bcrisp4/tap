package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/bcrisp4/tap/internal/storage"
)

// sha256HexRe matches a 64-char lowercase hex string — the only
// shape an icons.hash value ever takes (sha256 hex, written by the
// poller via crypto/sha256). Validating up front avoids leaking
// arbitrary user input into the ETag header and skips a needless
// storage round-trip on obviously malformed paths.
var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

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
	if !sha256HexRe.MatchString(hash) {
		WriteError(w, http.StatusBadRequest, "bad_hash", "icon hash must be 64 lowercase hex chars")
		return
	}

	// Confirm the icon exists before short-circuiting on If-None-Match.
	// RFC 7232 §3.2: a 304 means "the resource exists and your cached
	// copy is fresh"; returning 304 for an unknown hash would falsely
	// promise the body. Looking the row up first also lets us reject
	// `If-None-Match: *` against a missing resource with 404.
	icon, err := h.store.GetIconByHash(r.Context(), hash)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "not_found", "icon not found")
			return
		}
		writeErr(w, err)
		return
	}

	if match := r.Header.Get("If-None-Match"); match != "" && etagMatches(match, hash) {
		w.Header().Set("ETag", quoteETag(hash))
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.WriteHeader(http.StatusNotModified)
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
