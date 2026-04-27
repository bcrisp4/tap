// Package feedparse turns raw feed bytes into Tap's normalised entry
// shape, deduplicated by EntryHash and annotated with reading time.
package feedparse

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// EntryHash returns the deterministic 64-char hex SHA-256 used as
// `entries.hash`. Inputs:
//   - feedID: the row's parent feed
//   - guid:   the entry's `<guid>` / atom:id / json id (preferred key)
//   - link:   the entry's URL (used when guid is empty)
//   - title:  the entry title (only used in the fallback)
//   - publishedAt: unix seconds (only used in the fallback)
//
// When guid is set, title and publishedAt are NOT mixed in — that's
// the stability property: re-titling an existing entry must not
// produce a new hash.
func EntryHash(feedID int64, guid, link, title string, publishedAt int64) string {
	h := sha256.New()
	h.Write([]byte(strconv.FormatInt(feedID, 10)))
	h.Write([]byte{0})
	switch {
	case guid != "":
		h.Write([]byte(guid))
	case link != "":
		h.Write([]byte(link))
	default:
		h.Write([]byte(title))
		h.Write([]byte{0})
		h.Write([]byte(strconv.FormatInt(publishedAt, 10)))
	}
	return hex.EncodeToString(h.Sum(nil))
}
