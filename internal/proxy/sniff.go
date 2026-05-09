package proxy

import (
	"encoding/binary"
	"net/http"
	"strings"
)

// allowedMIMETypes is the M3 image allowlist. SVG is excluded — see
// docs/roadmap.md "Deferred items" for the rationale and the two paths
// to enabling it later.
var allowedMIMETypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/gif":  {},
	"image/webp": {},
	"image/avif": {},
}

// detectAVIF returns "image/avif" if body begins with an ISO Base Media
// File Format ftyp box whose major brand or one of its compatible brands
// is "avif" or "avis". Go's http.DetectContentType has no AVIF support
// (it only knows video/mp4 from the same container family), so we
// implement the minimal check ourselves.
//
// ftyp box layout (each field is 4 bytes, big-endian):
//
//	[box_size][ftyp][major_brand][minor_version][compatible_brand...]
//
// We only require len(body) >= 12 (size + "ftyp" + major_brand) so that
// minimal magic-byte snips (which may be shorter than boxSize) still work.
// We walk brands up to min(boxSize, len(body)) to handle truncated reads.
func detectAVIF(body []byte) string {
	const minLen = 12 // size(4) + "ftyp"(4) + major_brand(4)
	if len(body) < minLen {
		return ""
	}
	boxSize := int(binary.BigEndian.Uint32(body[:4]))
	if boxSize < minLen || boxSize%4 != 0 {
		return ""
	}
	if string(body[4:8]) != "ftyp" {
		return ""
	}
	// Walk as far as we have bytes (body may be a truncated sniff buffer).
	limit := boxSize
	if limit > len(body) {
		limit = len(body)
	}
	// Walk brands: major_brand at offset 8, then compatible_brands from 16.
	for off := 8; off+4 <= limit; off += 4 {
		if off == 12 {
			// minor_version field — skip, not a brand.
			continue
		}
		brand := string(body[off : off+4])
		if brand == "avif" || brand == "avis" {
			return "image/avif"
		}
	}
	return ""
}

// sniffType returns the MIME type of body. It extends http.DetectContentType
// with AVIF detection, which Go's stdlib does not support.
func sniffType(body []byte) string {
	if ct := detectAVIF(body); ct != "" {
		return ct
	}
	return http.DetectContentType(body)
}

// validateImage sniffs the first up-to-512 bytes of body and confirms
// that the result is in the allowlist AND agrees with the origin's
// Content-Type header (charset / boundary parameters are ignored on the
// header side). Returns the sniffed canonical type on success.
func validateImage(body []byte, headerCT string) (string, bool) {
	sniffed := sniffType(body)
	if _, ok := allowedMIMETypes[sniffed]; !ok {
		return "", false
	}
	headerType := strings.TrimSpace(strings.SplitN(headerCT, ";", 2)[0])
	if !strings.EqualFold(headerType, sniffed) {
		return "", false
	}
	return sniffed, true
}
