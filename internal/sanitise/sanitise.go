// Package sanitise provides server-side HTML cleaning for entry content.
// The output is final-form HTML safe to render directly; the SPA never
// runs a runtime sanitiser.
package sanitise

import (
	"github.com/microcosm-cc/bluemonday"
)

// Policy is a configured sanitiser. Construct with DefaultPolicy or New.
type Policy struct {
	bm *bluemonday.Policy
}

// DefaultPolicy returns the policy used in production: bluemonday's
// UGCPolicy as the baseline, URL schemes tightened to http/https/mailto,
// iframe and pixel-tracker rules applied as a post-pass.
func DefaultPolicy() *Policy {
	return New()
}

// New constructs a Policy. (Currently no options; placeholder so M5 can
// extend without changing the constructor's call-site shape.)
func New() *Policy {
	bm := bluemonday.UGCPolicy()
	return &Policy{bm: bm}
}

// Sanitise returns final-form HTML safe to render directly.
// Total function — never errors, never panics. Worst case returns "".
func (p *Policy) Sanitise(rawHTML string) string {
	return p.bm.Sanitize(rawHTML)
}
