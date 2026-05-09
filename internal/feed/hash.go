// Package feed wraps the gofeed parser and exposes the entry-hash contract.
package feed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/mmcdole/gofeed"
)

// EntryHash computes the per-entry deduplication key. The feed identifier
// is mixed in so two feeds with overlapping GUIDs do not collide.
//
// Resolution order: feed-provided GUID, then entry URL, then SHA-256(title || published_at).
func EntryHash(subID int64, item *gofeed.Item) string {
	if item.GUID != "" {
		return digest(fmt.Sprintf("%d|guid|%s", subID, item.GUID))
	}
	if item.Link != "" {
		return digest(fmt.Sprintf("%d|url|%s", subID, item.Link))
	}
	pub := ""
	if item.PublishedParsed != nil {
		pub = item.PublishedParsed.UTC().Format("20060102T150405Z")
	}
	return digest(fmt.Sprintf("%d|td|%s|%s", subID, item.Title, pub))
}

func digest(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
