# M2 Sanitisation Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add server-side HTML sanitisation that runs in the polling worker between feed parse and DB commit, so every entry's `content` column holds final-form HTML safe for the SPA to render directly.

**Architecture:** Two new internal packages — `internal/urlcleaner` (strips well-known tracking parameters from URLs) and `internal/sanitise` (bluemonday-backed allowlist policy + a `golang.org/x/net/html` post-pass that handles iframe-host allowlisting, pixel-tracker drop, and URL cleaning). The polling worker carries a `*sanitise.Policy` and runs `policy.Sanitise(content)` before constructing each `db.NewEntry`. No schema migration; the README upgrade note tells operators to delete `tap.db`.

**Tech Stack:** Go 1.25, `microcosm-cc/bluemonday` (allowlist HTML sanitiser, pure-Go), `golang.org/x/net/html` (HTML parser, already a transitive dep via gofeed). Tests use `testify/require` (already a project dep).

**Reference spec:** `docs/specs/2026-05-08-m2-sanitisation.md` — re-read this if anything below is unclear.

**Reference source for the parameter list:** `/home/ben.guest/Users/ben/vendor/miniflux/internal/reader/urlcleaner/urlcleaner.go`. **Do not copy verbatim** — adapt the lists into our package with the required attribution comment (Apache-2.0 from miniflux, our package is also permissively licensed under whatever Tap ships).

**Reference source for the iframe allowlist:** `/home/ben.guest/Users/ben/vendor/miniflux/internal/reader/sanitizer/sanitizer.go:121-135`.

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `go.mod`, `go.sum` | Modify | Add `github.com/microcosm-cc/bluemonday` dependency |
| `internal/urlcleaner/urlcleaner.go` | Create | `Clean(rawURL string) string` — strip tracking parameters from URL query string |
| `internal/urlcleaner/urlcleaner_test.go` | Create | Table-driven tests for `Clean` |
| `internal/sanitise/sanitise.go` | Create | `Policy` struct + `DefaultPolicy()` + `(*Policy).Sanitise(rawHTML string) string` |
| `internal/sanitise/sanitise_test.go` | Create | Sanitiser tests (UGC smoke, schemes, iframe allowlist, pixel tracker, URL cleaning, total-function contract) |
| `internal/poll/worker.go` | Modify | `WorkerOpts` gains `Policy *sanitise.Policy`; `Worker.Run` calls `Sanitise` before building `db.NewEntry` |
| `internal/poll/worker_test.go` | Modify | Update existing tests to pass `Policy`; add new sanitisation assertions |
| `internal/poll/scheduler.go` | Modify | `SchedulerOpts` gains `Policy *sanitise.Policy`; threaded through to `NewWorker` |
| `internal/poll/scheduler_test.go` | Modify | Update existing scheduler tests for the new SchedulerOpts field |
| `cmd/tap/main.go` | Modify | Construct `sanitise.DefaultPolicy()` and pass it via `SchedulerOpts` |
| `cmd/tap/main_test.go` | Modify | Hostile-fixture entry; assert post-poll body is sanitised |
| `README.md` | Modify | Replace M1 "deployment safety" section with M2 trust posture + upgrade note |

---

## Task 1: Add bluemonday dependency — MERGED INTO TASK 8

This task was a sequencing mistake. `go mod tidy` removes any dependency that no `.go` file imports, so adding bluemonday before any code uses it is a no-op (the entry vanishes on the next tidy). The dependency now lands as part of Task 8, where `internal/sanitise/sanitise.go` first imports it.

Skip this task — proceed directly to Task 2.

---

## Task 2: urlcleaner — first failing test for stripping utm_source

**Files:**
- Create: `internal/urlcleaner/urlcleaner.go`
- Create: `internal/urlcleaner/urlcleaner_test.go`

- [ ] **Step 1: Create the package skeleton**

Write `internal/urlcleaner/urlcleaner.go`:

```go
// Package urlcleaner strips well-known tracking parameters from URLs.
//
// Parameter list adapted from github.com/miniflux/v2 internal/reader/urlcleaner.
// Original copyright miniflux contributors, Apache-2.0.
package urlcleaner

// Clean returns rawURL with well-known tracking parameters removed from
// the query string. Returns rawURL unchanged on parse error or non-URL
// input — URL hygiene must never silently break an entry.
func Clean(rawURL string) string {
	return rawURL
}
```

- [ ] **Step 2: Write the first failing test**

Write `internal/urlcleaner/urlcleaner_test.go`:

```go
package urlcleaner

import "testing"

func TestClean_StripsUtmSource(t *testing.T) {
	t.Parallel()
	got := Clean("https://example.com/page?utm_source=newsletter&id=42")
	want := "https://example.com/page?id=42"
	if got != want {
		t.Errorf("Clean() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 3: Run the test and verify it fails**

Run: `go test ./internal/urlcleaner -run TestClean_StripsUtmSource -v`
Expected: FAIL — `Clean() = "https://example.com/page?utm_source=newsletter&id=42", want "https://example.com/page?id=42"`

- [ ] **Step 4: Commit the failing test (red)**

```bash
git add internal/urlcleaner/
git commit -m "Add urlcleaner package skeleton and failing test for utm_source"
```

---

## Task 3: urlcleaner — implement minimal Clean

**Files:**
- Modify: `internal/urlcleaner/urlcleaner.go`

- [ ] **Step 1: Replace the package body with a working implementation**

Overwrite `internal/urlcleaner/urlcleaner.go`:

```go
// Package urlcleaner strips well-known tracking parameters from URLs.
//
// Parameter list adapted from github.com/miniflux/v2 internal/reader/urlcleaner.
// Original copyright miniflux contributors, Apache-2.0.
package urlcleaner

import (
	"net/url"
	"strings"
)

// trackingParams is the set of full-name query parameters to drop.
var trackingParams = map[string]struct{}{
	"utm_source": {},
}

// trackingPrefixes is the set of query-parameter name prefixes to drop.
var trackingPrefixes = []string{}

// Clean returns rawURL with well-known tracking parameters removed from
// the query string. Returns rawURL unchanged on parse error or non-URL
// input — URL hygiene must never silently break an entry.
func Clean(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.RawQuery == "" {
		return rawURL
	}
	q := u.Query()
	changed := false
	for k := range q {
		if isTracking(k) {
			q.Del(k)
			changed = true
		}
	}
	if !changed {
		return rawURL
	}
	u.RawQuery = q.Encode()
	// u.String() may re-encode the path/query (escape normalisation,
	// %20 <-> +). The URL stays semantically equivalent; we only reach
	// this line when tracking params were actually stripped.
	return u.String()
}

func isTracking(name string) bool {
	if _, ok := trackingParams[strings.ToLower(name)]; ok {
		return true
	}
	for _, p := range trackingPrefixes {
		if strings.HasPrefix(strings.ToLower(name), p) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the test and verify it passes**

Run: `go test ./internal/urlcleaner -run TestClean_StripsUtmSource -v`
Expected: PASS

- [ ] **Step 3: Commit (green)**

```bash
git add internal/urlcleaner/urlcleaner.go
git commit -m "Implement urlcleaner.Clean for full-name tracking params"
```

---

## Task 4: urlcleaner — full inbound parameter list (table-driven)

**Files:**
- Modify: `internal/urlcleaner/urlcleaner_test.go`
- Modify: `internal/urlcleaner/urlcleaner.go`

- [ ] **Step 1: Write a table-driven test for the inbound parameter list**

Append to `internal/urlcleaner/urlcleaner_test.go`:

```go
func TestClean_StripsKnownInboundTrackers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"utm_source", "https://e.com/?utm_source=x&keep=1", "https://e.com/?keep=1"},
		{"utm_medium", "https://e.com/?utm_medium=email&keep=1", "https://e.com/?keep=1"},
		{"utm_campaign", "https://e.com/?utm_campaign=fall&keep=1", "https://e.com/?keep=1"},
		{"utm_term", "https://e.com/?utm_term=foo&keep=1", "https://e.com/?keep=1"},
		{"utm_content", "https://e.com/?utm_content=bar&keep=1", "https://e.com/?keep=1"},
		{"mc_eid", "https://e.com/?mc_eid=abc&keep=1", "https://e.com/?keep=1"},
		{"mc_cid", "https://e.com/?mc_cid=def&keep=1", "https://e.com/?keep=1"},
		{"mkt_tok", "https://e.com/?mkt_tok=ghi&keep=1", "https://e.com/?keep=1"},
		{"hsCtaTracking", "https://e.com/?hsCtaTracking=jkl&keep=1", "https://e.com/?keep=1"},
		{"_hsmi", "https://e.com/?_hsmi=mno&keep=1", "https://e.com/?keep=1"},
		{"_hsenc", "https://e.com/?_hsenc=pqr&keep=1", "https://e.com/?keep=1"},
		{"vero_id", "https://e.com/?vero_id=stu&keep=1", "https://e.com/?keep=1"},
		{"oly_anon_id", "https://e.com/?oly_anon_id=vwx&keep=1", "https://e.com/?keep=1"},
		{"wickedid", "https://e.com/?wickedid=yz&keep=1", "https://e.com/?keep=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Clean(tc.in); got != tc.want {
				t.Errorf("Clean(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify the new cases fail**

Run: `go test ./internal/urlcleaner -run TestClean_StripsKnownInboundTrackers -v`
Expected: most subtests FAIL (only `utm_source` passes from the existing implementation).

- [ ] **Step 3: Expand the trackingParams map**

In `internal/urlcleaner/urlcleaner.go`, replace the `trackingParams` declaration with:

```go
// trackingParams is the set of full-name query parameters to drop.
// Adapted from miniflux's urlcleaner — see package doc comment.
var trackingParams = map[string]struct{}{
	"utm_source":    {},
	"utm_medium":    {},
	"utm_campaign":  {},
	"utm_term":      {},
	"utm_content":   {},
	"mc_eid":        {},
	"mc_cid":        {},
	"mkt_tok":       {},
	"hsctatracking": {}, // case-insensitive match — store lowercase
	"_hsmi":         {},
	"_hsenc":        {},
	"vero_id":       {},
	"vero_conv":     {},
	"oly_anon_id":   {},
	"oly_enc_id":    {},
	"wickedid":      {},
}
```

- [ ] **Step 4: Run the test and verify all subtests pass**

Run: `go test ./internal/urlcleaner -run TestClean_StripsKnownInboundTrackers -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/urlcleaner/
git commit -m "Expand urlcleaner inbound tracking-parameter list"
```

---

## Task 5: urlcleaner — outbound click-trackers

**Files:**
- Modify: `internal/urlcleaner/urlcleaner_test.go`
- Modify: `internal/urlcleaner/urlcleaner.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/urlcleaner/urlcleaner_test.go`:

```go
func TestClean_StripsKnownOutboundTrackers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"fbclid", "https://e.com/?fbclid=a&keep=1", "https://e.com/?keep=1"},
		{"gclid", "https://e.com/?gclid=b&keep=1", "https://e.com/?keep=1"},
		{"dclid", "https://e.com/?dclid=c&keep=1", "https://e.com/?keep=1"},
		{"msclkid", "https://e.com/?msclkid=d&keep=1", "https://e.com/?keep=1"},
		{"yclid", "https://e.com/?yclid=e&keep=1", "https://e.com/?keep=1"},
		{"igshid", "https://e.com/?igshid=f&keep=1", "https://e.com/?keep=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Clean(tc.in); got != tc.want {
				t.Errorf("Clean(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/urlcleaner -run TestClean_StripsKnownOutboundTrackers -v`
Expected: all subtests FAIL.

- [ ] **Step 3: Add the outbound entries to trackingParams**

In `internal/urlcleaner/urlcleaner.go`, extend the `trackingParams` map by appending these entries inside the existing literal:

```go
"fbclid":  {},
"gclid":   {},
"dclid":   {},
"msclkid": {},
"yclid":   {},
"igshid":  {},
```

(Add them before the closing `}` of the map literal.)

- [ ] **Step 4: Run the test and verify all subtests pass**

Run: `go test ./internal/urlcleaner -run TestClean_StripsKnownOutboundTrackers -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/urlcleaner/
git commit -m "Add outbound click-tracker parameters to urlcleaner"
```

---

## Task 6: urlcleaner — prefix matching (utm_, mtm_, pk_)

**Files:**
- Modify: `internal/urlcleaner/urlcleaner_test.go`
- Modify: `internal/urlcleaner/urlcleaner.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/urlcleaner/urlcleaner_test.go`:

```go
func TestClean_StripsPrefixedTrackers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"utm_unknown", "https://e.com/?utm_zzz=x&keep=1", "https://e.com/?keep=1"},
		{"mtm_source", "https://e.com/?mtm_source=x&keep=1", "https://e.com/?keep=1"},
		{"mtm_unknown", "https://e.com/?mtm_anything=x&keep=1", "https://e.com/?keep=1"},
		{"pk_campaign", "https://e.com/?pk_campaign=x&keep=1", "https://e.com/?keep=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Clean(tc.in); got != tc.want {
				t.Errorf("Clean(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/urlcleaner -run TestClean_StripsPrefixedTrackers -v`
Expected: all subtests FAIL (these names aren't in the static map and no prefixes are configured).

- [ ] **Step 3: Populate trackingPrefixes**

In `internal/urlcleaner/urlcleaner.go`, replace the empty `trackingPrefixes` slice with:

```go
// trackingPrefixes is the set of query-parameter name prefixes to drop.
// Adapted from miniflux's urlcleaner — see package doc comment.
// Matched case-insensitively against the parameter name.
var trackingPrefixes = []string{
	"utm_",
	"mtm_",
	"pk_",
}
```

- [ ] **Step 4: Run the test and verify all subtests pass**

Run: `go test ./internal/urlcleaner -run TestClean_StripsPrefixedTrackers -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/urlcleaner/
git commit -m "Add prefix-based tracking-parameter matching to urlcleaner"
```

---

## Task 7: urlcleaner — defensive contract (parse errors, non-URLs, legitimate params survive, no trailing `?`)

**Files:**
- Modify: `internal/urlcleaner/urlcleaner_test.go`

- [ ] **Step 1: Write the contract tests**

Append to `internal/urlcleaner/urlcleaner_test.go`:

```go
func TestClean_LegitimateParamsSurvive(t *testing.T) {
	t.Parallel()
	in := "https://example.com/article?id=42&page=3&q=hello"
	if got := Clean(in); got != in {
		t.Errorf("Clean(%q) modified URL with no tracking params: got %q", in, got)
	}
}

func TestClean_UnchangedOnNoTrackingParams(t *testing.T) {
	t.Parallel()
	in := "https://example.com/article"
	if got := Clean(in); got != in {
		t.Errorf("Clean(%q) modified URL with no query string: got %q", in, got)
	}
}

func TestClean_NonURLReturnsUnchanged(t *testing.T) {
	t.Parallel()
	// Inputs urlcleaner cannot make sense of: empty string, junk, malformed
	// URL with no scheme. urlcleaner is not a security boundary — dangerous
	// schemes (javascript:, data:) are dropped by sanitise's URL allowlist
	// upstream, not here.
	for _, in := range []string{"", "not a url", "://malformed"} {
		if got := Clean(in); got != in {
			t.Errorf("Clean(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestClean_NoTrailingQuestionMarkAfterStrip(t *testing.T) {
	t.Parallel()
	in := "https://example.com/article?utm_source=foo"
	want := "https://example.com/article"
	if got := Clean(in); got != want {
		t.Errorf("Clean(%q) = %q, want %q (no trailing '?')", in, got, want)
	}
}
```

- [ ] **Step 2: Run the tests**

Run: `go test ./internal/urlcleaner -v`
Expected: all PASS. (Current implementation already satisfies these — `Clean` returns input unchanged when nothing changed and `u.String()` omits `?` when `RawQuery` is empty after `q.Encode()`.)

- [ ] **Step 3: If TestClean_NoTrailingQuestionMarkAfterStrip fails**

The likely cause is `q.Encode()` returning `""` and then `u.String()` not handling that. Inspect `internal/urlcleaner/urlcleaner.go` and ensure after the `q.Del` loop:

```go
u.RawQuery = q.Encode()
```

`q.Encode()` returns `""` if the query is empty, and `url.URL.String()` correctly omits the `?` when `RawQuery == ""`. If the test still fails, add an explicit guard:

```go
u.RawQuery = q.Encode()
return u.String()
```

(Should not need additional code; `url.URL.String()` handles the empty case.)

- [ ] **Step 4: Commit**

```bash
git add internal/urlcleaner/urlcleaner_test.go
git commit -m "Add urlcleaner defensive-contract tests"
```

---

## Task 8: sanitise — package skeleton + first test for script stripping

**Files:**
- Create: `internal/sanitise/sanitise.go`
- Create: `internal/sanitise/sanitise_test.go`
- Modify: `go.mod`, `go.sum` (bluemonday lands here, since this is the first task that imports it; Task 1 was a no-op)

- [ ] **Step 1: Add the bluemonday dependency**

From the repo root, run:

```bash
go get github.com/microcosm-cc/bluemonday@latest
```

Don't run `go mod tidy` yet — there's no importer in the tree until Step 2 lands. (`go get` writes the dep to `go.mod`; `go mod tidy` would immediately strip it.)

- [ ] **Step 2: Create the package skeleton**

Write `internal/sanitise/sanitise.go`:

```go
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
```

- [ ] **Step 3: Write the failing test**

Write `internal/sanitise/sanitise_test.go`:

```go
package sanitise

import (
	"strings"
	"testing"
)

func TestSanitise_StripsScript(t *testing.T) {
	t.Parallel()
	in := `<p>hi</p><script>alert('xss')</script>`
	got := DefaultPolicy().Sanitise(in)
	if strings.Contains(got, "<script>") || strings.Contains(got, "alert") {
		t.Errorf("script not stripped: got %q", got)
	}
	if !strings.Contains(got, "<p>hi</p>") {
		t.Errorf("legitimate <p> stripped: got %q", got)
	}
}
```

- [ ] **Step 4: Tidy and run the test**

Run:

```bash
go mod tidy
go test ./internal/sanitise -run TestSanitise_StripsScript -v
```

Expected: PASS — bluemonday's UGCPolicy strips `<script>` out of the box. `go mod tidy` now keeps bluemonday because `sanitise.go` imports it.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/sanitise/
git commit -m "Add sanitise package with bluemonday UGCPolicy baseline"
```

---

## Task 9: sanitise — URL scheme tightening (drop javascript:, data:)

**Files:**
- Modify: `internal/sanitise/sanitise_test.go`
- Modify: `internal/sanitise/sanitise.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/sanitise/sanitise_test.go`:

```go
func TestSanitise_DropsDangerousSchemes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
	}{
		{"javascript in href", `<a href="javascript:alert(1)">click</a>`},
		{"javascript in img src", `<img src="javascript:alert(1)">`},
		{"data in img src", `<img src="data:image/png;base64,AAAA">`},
		{"vbscript in href", `<a href="vbscript:msgbox(1)">click</a>`},
	}
	p := DefaultPolicy()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			for _, scheme := range []string{"javascript:", "data:", "vbscript:"} {
				if strings.Contains(got, scheme) {
					t.Errorf("scheme %q survived in %q output: %q", scheme, tc.name, got)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/sanitise -run TestSanitise_DropsDangerousSchemes -v`
Expected: at minimum the `javascript in href`, `javascript in img src`, and `vbscript in href` cases PASS (UGCPolicy already restricts these); the `data in img src` case may FAIL because UGCPolicy permits `data:` URLs in `<img>` by default.

- [ ] **Step 3: Tighten the scheme allowlist**

In `internal/sanitise/sanitise.go`, modify `New()` to set the allowed URL schemes explicitly:

```go
func New() *Policy {
	bm := bluemonday.UGCPolicy()
	// Tighten URL schemes: drop data:, vbscript:, etc. UGCPolicy already
	// forbids javascript: but we narrow to a deliberate three-scheme set.
	bm.AllowURLSchemes("http", "https", "mailto")
	return &Policy{bm: bm}
}
```

- [ ] **Step 4: Run the test and verify all subtests pass**

Run: `go test ./internal/sanitise -run TestSanitise_DropsDangerousSchemes -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sanitise/
git commit -m "Tighten sanitise URL schemes to http/https/mailto"
```

---

## Task 10: sanitise — iframe host allowlist (post-pass walker)

**Files:**
- Modify: `internal/sanitise/sanitise_test.go`
- Modify: `internal/sanitise/sanitise.go`

This task introduces the post-pass HTML walker. Subsequent tasks (pixel tracker, URL cleaning) will extend the same walker.

- [ ] **Step 1: Write the failing test**

Append to `internal/sanitise/sanitise_test.go`:

```go
func TestSanitise_IframeAllowlist(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()

	allowed := []struct {
		name string
		src  string
	}{
		{"youtube embed", "https://www.youtube.com/embed/dQw4w9WgXcQ"},
		{"youtube embed bare", "https://youtube.com/embed/dQw4w9WgXcQ"},
		{"youtube-nocookie", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"},
		{"vimeo player", "https://player.vimeo.com/video/76979871"},
		{"bandcamp", "https://bandcamp.com/EmbeddedPlayer/album=12345/"},
		{"dailymotion", "https://dailymotion.com/embed/video/x123"},
		{"twitch player", "https://player.twitch.tv/?channel=foo"},
		{"spotify open", "https://open.spotify.com/embed/track/abc"},
		{"soundcloud", "https://soundcloud.com/oembed?url=foo"},
		{"soundcloud w", "https://w.soundcloud.com/player/?url=foo"},
		{"bilibili", "https://player.bilibili.com/player.html?aid=1"},
		{"vk", "https://vk.com/video_ext.php?oid=1&id=2"},
		{"framatube", "https://framatube.org/videos/embed/abc"},
		{"embedly cdn", "https://cdn.embedly.com/widgets/media.html?src=foo"},
	}
	for _, tc := range allowed {
		t.Run("allowed/"+tc.name, func(t *testing.T) {
			t.Parallel()
			in := `<p>before</p><iframe src="` + tc.src + `"></iframe><p>after</p>`
			got := p.Sanitise(in)
			if !strings.Contains(got, "<iframe") {
				t.Errorf("allowed iframe stripped: in=%q out=%q", in, got)
			}
			if !strings.Contains(got, tc.src) {
				t.Errorf("allowed iframe src missing: in=%q out=%q", in, got)
			}
		})
	}

	denied := []struct {
		name string
		src  string
	}{
		{"unknown host", "https://evil.example/embed"},
		{"subdomain attack on youtube", "https://youtube.com.evil.example/embed/x"},
		{"prefix attack on youtube", "https://evil.example/youtube.com/embed/x"},
		{"non-www subdomain of youtube", "https://m.youtube.com/embed/x"},
	}
	for _, tc := range denied {
		t.Run("denied/"+tc.name, func(t *testing.T) {
			t.Parallel()
			in := `<p>before</p><iframe src="` + tc.src + `"></iframe><p>after</p>`
			got := p.Sanitise(in)
			if strings.Contains(got, "<iframe") {
				t.Errorf("disallowed iframe survived: in=%q out=%q", in, got)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/sanitise -run TestSanitise_IframeAllowlist -v`
Expected: every subtest FAILs — bluemonday's UGCPolicy strips ALL iframes by default, so allowed-host cases fail too.

- [ ] **Step 3: Allow iframes in bluemonday and add the post-pass walker**

Replace the contents of `internal/sanitise/sanitise.go` with:

```go
// Package sanitise provides server-side HTML cleaning for entry content.
// The output is final-form HTML safe to render directly; the SPA never
// runs a runtime sanitiser.
package sanitise

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// defaultIframeHosts is the set of hosts whose <iframe> embeds survive
// sanitisation. Adapted from miniflux's iframeAllowList.
//
// Notable omissions on purpose: m.youtube.com (mobile) and youtu.be
// (share-link host, not an embed URL). Per-user override is on the
// roadmap as a deferred item (post-M6) and will let users add hosts
// like these.
var defaultIframeHosts = map[string]struct{}{
	"bandcamp.com":         {},
	"cdn.embedly.com":      {},
	"dailymotion.com":      {},
	"framatube.org":        {},
	"open.spotify.com":     {},
	"player.bilibili.com":  {},
	"player.twitch.tv":     {},
	"player.vimeo.com":     {},
	"soundcloud.com":       {},
	"vk.com":               {},
	"w.soundcloud.com":     {},
	"youtube-nocookie.com": {},
	"youtube.com":          {},
}

// Policy is a configured sanitiser. Construct with DefaultPolicy or New.
type Policy struct {
	bm           *bluemonday.Policy
	iframeHosts  map[string]struct{}
}

// DefaultPolicy returns the policy used in production.
func DefaultPolicy() *Policy { return New() }

// New constructs a Policy. (Currently no options; M5 will extend.)
func New() *Policy {
	bm := bluemonday.UGCPolicy()
	bm.AllowURLSchemes("http", "https", "mailto")
	// Allow iframes through bluemonday; the post-pass enforces the
	// host allowlist (which bluemonday's regex matching can't express
	// cleanly because we want exact-after-www-strip semantics).
	// AllowElements is belt-and-braces: AllowAttrs.OnElements implicitly
	// allows the element in current bluemonday, but explicit is safer.
	bm.AllowElements("iframe")
	bm.AllowAttrs("src", "width", "height", "frameborder", "allowfullscreen", "allow").OnElements("iframe")
	return &Policy{
		bm:          bm,
		iframeHosts: defaultIframeHosts,
	}
}

// Sanitise returns final-form HTML safe to render directly.
// Total function — never errors, never panics. Worst case returns "".
func (p *Policy) Sanitise(rawHTML string) string {
	cleaned := p.bm.Sanitize(rawHTML)
	return p.postProcess(cleaned)
}

// postProcess walks the bluemonday output and applies the rules
// bluemonday can't express directly: iframe-host allowlisting,
// pixel-tracker drop, URL tracking-param cleaning.
func (p *Policy) postProcess(s string) string {
	if s == "" {
		return ""
	}
	// ParseFragment with body context so the result doesn't get wrapped
	// in <html><head><body>; we want a fragment in, a fragment out.
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		return s
	}
	for _, n := range nodes {
		p.walk(n)
	}
	var buf bytes.Buffer
	for _, n := range nodes {
		if err := html.Render(&buf, n); err != nil {
			return s
		}
	}
	return buf.String()
}

// walk mutates the node tree in place.
func (p *Policy) walk(n *html.Node) {
	// Iterate children manually so we can safely remove during traversal.
	c := n.FirstChild
	for c != nil {
		next := c.NextSibling
		if c.Type == html.ElementNode {
			switch c.Data {
			case "iframe":
				if !p.iframeHostAllowed(getAttr(c, "src")) {
					n.RemoveChild(c)
					c = next
					continue
				}
			}
		}
		p.walk(c)
		c = next
	}
}

func (p *Policy) iframeHostAllowed(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	_, ok := p.iframeHosts[host]
	return ok
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
```

- [ ] **Step 4: Run the test**

Run: `go test ./internal/sanitise -run TestSanitise_IframeAllowlist -v`
Expected: all subtests PASS.

- [ ] **Step 5: Run the full sanitise test file to make sure nothing earlier regressed**

Run: `go test ./internal/sanitise -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sanitise/
git commit -m "Add iframe host allowlist via post-pass HTML walker"
```

---

## Task 11: sanitise — pixel-tracker drop

**Files:**
- Modify: `internal/sanitise/sanitise_test.go`
- Modify: `internal/sanitise/sanitise.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/sanitise/sanitise_test.go`:

```go
func TestSanitise_PixelTracker(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()

	dropped := []struct {
		name string
		in   string
	}{
		{"1x1", `<p>a</p><img src="https://t.example/p" width="1" height="1"><p>b</p>`},
		{"0x0", `<p>a</p><img src="https://t.example/p" width="0" height="0"><p>b</p>`},
		{"1x0", `<p>a</p><img src="https://t.example/p" width="1" height="0"><p>b</p>`},
		{"0x1", `<p>a</p><img src="https://t.example/p" width="0" height="1"><p>b</p>`},
	}
	for _, tc := range dropped {
		t.Run("dropped/"+tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			if strings.Contains(got, "<img") {
				t.Errorf("pixel tracker survived: in=%q out=%q", tc.in, got)
			}
			if !strings.Contains(got, "<p>a</p>") || !strings.Contains(got, "<p>b</p>") {
				t.Errorf("surrounding content damaged: in=%q out=%q", tc.in, got)
			}
		})
	}

	kept := []struct {
		name string
		in   string
	}{
		{"1x2", `<img src="https://e.com/i" width="1" height="2">`},
		{"2x1", `<img src="https://e.com/i" width="2" height="1">`},
		{"5x5", `<img src="https://e.com/i" width="5" height="5">`},
		{"no dims", `<img src="https://e.com/i">`},
		{"only width", `<img src="https://e.com/i" width="1">`},
		{"only height", `<img src="https://e.com/i" height="1">`},
	}
	for _, tc := range kept {
		t.Run("kept/"+tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			if !strings.Contains(got, "<img") {
				t.Errorf("legitimate image dropped: in=%q out=%q", tc.in, got)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/sanitise -run TestSanitise_PixelTracker -v`
Expected: every `dropped/` subtest FAILs (pixel tracker survives) AND every `kept/` subtest may FAIL (UGCPolicy strips `width`/`height` attributes from `<img>` because they're not in its default allowlist, so `<img>` outputs lose those attributes — and the post-pass won't see them). The fix below addresses both.

- [ ] **Step 3: Allow width/height on <img> in bluemonday + extend the walker**

In `internal/sanitise/sanitise.go`:

1. Inside `New()`, after the `AllowAttrs("src", ...)` call for iframe, add:

```go
bm.AllowAttrs("width", "height").OnElements("img")
```

2. In `walk`, extend the `switch c.Data` block to handle `img`. Replace the existing `case "iframe":` block with:

```go
case "iframe":
    if !p.iframeHostAllowed(getAttr(c, "src")) {
        n.RemoveChild(c)
        c = next
        continue
    }
case "img":
    if isPixelTracker(c) {
        n.RemoveChild(c)
        c = next
        continue
    }
```

3. Add the `isPixelTracker` helper at the bottom of the file:

```go
// isPixelTracker matches <img> whose width AND height attributes are
// both "0" or "1". Mirrors miniflux's heuristic
// (internal/reader/sanitizer/sanitizer.go isPixelTracker).
//
// Known gaps (intentional — concept doc scopes us to "obvious" trackers):
// CSS-styled trackers (<img style="width:1px">), 2x2 pixels, and naked
// <img> with no dims that the browser sizes from a 1x1 source bitmap
// all survive. Expanding the heuristic risks false positives on
// legitimate content (icons, spacers).
func isPixelTracker(n *html.Node) bool {
	w := getAttr(n, "width")
	h := getAttr(n, "height")
	if w == "" || h == "" {
		return false
	}
	return (w == "0" || w == "1") && (h == "0" || h == "1")
}
```

- [ ] **Step 4: Run the test**

Run: `go test ./internal/sanitise -run TestSanitise_PixelTracker -v`
Expected: all subtests PASS.

- [ ] **Step 5: Run the full sanitise test file**

Run: `go test ./internal/sanitise -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sanitise/
git commit -m "Drop 1x1 and 0x0 <img> as pixel-tracker heuristic"
```

---

## Task 12: sanitise — URL cleaning integration (href and img src)

**Files:**
- Modify: `internal/sanitise/sanitise_test.go`
- Modify: `internal/sanitise/sanitise.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/sanitise/sanitise_test.go`:

```go
func TestSanitise_URLCleanerIntegration(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()

	cases := []struct {
		name        string
		in          string
		mustContain string
		mustNot     []string
	}{
		{
			name:        "anchor utm_source",
			in:          `<a href="https://e.com/x?utm_source=foo&id=1">link</a>`,
			mustContain: `href="https://e.com/x?id=1"`,
			mustNot:     []string{"utm_source"},
		},
		{
			name:        "img fbclid",
			in:          `<img src="https://e.com/i.png?fbclid=bar&v=1">`,
			mustContain: `src="https://e.com/i.png?v=1"`,
			mustNot:     []string{"fbclid"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			if !strings.Contains(got, tc.mustContain) {
				t.Errorf("missing expected fragment %q in output %q", tc.mustContain, got)
			}
			for _, ng := range tc.mustNot {
				if strings.Contains(got, ng) {
					t.Errorf("unwanted fragment %q in output %q", ng, got)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/sanitise -run TestSanitise_URLCleanerIntegration -v`
Expected: both subtests FAIL — URLs are not yet cleaned.

- [ ] **Step 3: Wire urlcleaner into the walker**

In `internal/sanitise/sanitise.go`:

1. Add the import (top of file):

```go
"github.com/bcrisp4/tap/internal/urlcleaner"
```

2. Extend the `walk` function's `case` block. The full updated `case` block reads:

```go
case "iframe":
    if !p.iframeHostAllowed(getAttr(c, "src")) {
        n.RemoveChild(c)
        c = next
        continue
    }
case "img":
    if isPixelTracker(c) {
        n.RemoveChild(c)
        c = next
        continue
    }
    cleanAttrURL(c, "src")
case "a":
    cleanAttrURL(c, "href")
```

3. Add the `cleanAttrURL` helper at the bottom of the file:

```go
// cleanAttrURL strips tracking parameters from the named attribute's
// URL value, in place. No-op if the attribute is missing.
//
// Scope: M2 covers <a href> and <img src> only. UGCPolicy may also
// permit <source src/srcset>, <video src/poster>, <audio src>, etc.;
// tracking parameters in those URLs are not yet cleaned. Add cases
// here if a real feed surfaces survivors.
func cleanAttrURL(n *html.Node, key string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = urlcleaner.Clean(a.Val)
			return
		}
	}
}
```

- [ ] **Step 4: Run the test**

Run: `go test ./internal/sanitise -run TestSanitise_URLCleanerIntegration -v`
Expected: both subtests PASS.

- [ ] **Step 5: Run the full sanitise test file**

Run: `go test ./internal/sanitise -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sanitise/
git commit -m "Strip tracking params from <a href> and <img src> in sanitiser"
```

---

## Task 13: sanitise — pre-sanitise size cap (1 MiB)

**Files:**
- Modify: `internal/sanitise/sanitise_test.go`
- Modify: `internal/sanitise/sanitise.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/sanitise/sanitise_test.go`:

```go
func TestSanitise_TruncatesOversizeInput(t *testing.T) {
	t.Parallel()
	// 2 MiB of "x" wrapped in a <p>; bluemonday should still produce
	// finite output and never see more than ~1 MiB of input.
	const wantCap = 1 << 20
	huge := "<p>" + strings.Repeat("x", 2*wantCap) + "</p>"
	got := DefaultPolicy().Sanitise(huge)
	// Slack of 256 covers any wrapping/balancing the HTML parser may
	// add when re-serialising the truncated fragment.
	if len(got) > wantCap+256 {
		t.Errorf("output size %d exceeds expected cap (%d + slack)", len(got), wantCap)
	}
}

func TestSanitise_TruncationSnapsToUTF8Boundary(t *testing.T) {
	t.Parallel()
	// Build input where byte position MaxInputBytes lands inside a
	// multi-byte UTF-8 codepoint. "€" is 3 bytes (E2 82 AC). Filling
	// up to MaxInputBytes-1 with ASCII then inserting "€" puts the
	// truncation cut mid-codepoint.
	const cap = 1 << 20
	body := strings.Repeat("a", cap-1) + "€" + strings.Repeat("b", 100)
	got := DefaultPolicy().Sanitise(body)
	// Result must be valid UTF-8 — no replacement chars from a partial
	// multi-byte sequence handed to the HTML parser.
	if !utf8.ValidString(got) {
		t.Errorf("output is not valid UTF-8")
	}
	if strings.Contains(got, "�") {
		t.Errorf("output contains U+FFFD replacement character (input was cut mid-codepoint)")
	}
}
```

- [ ] **Step 2: Add the utf8 import to the test file if not already present**

In `internal/sanitise/sanitise_test.go` imports block, add:

```go
"unicode/utf8"
```

- [ ] **Step 3: Run the tests**

Run: `go test ./internal/sanitise -run "TestSanitise_TruncatesOversizeInput|TestSanitise_TruncationSnapsToUTF8Boundary" -v`
Expected: both FAIL — no cap is enforced; output will be ~2 MiB and the UTF-8 test will see a replacement character once the cap is naively enforced.

- [ ] **Step 4: Add the cap with UTF-8 boundary snap-back**

In `internal/sanitise/sanitise.go`, add the `unicode/utf8` import and a constant near the top of the file (just below imports):

```go
// MaxInputBytes caps the raw-HTML input to Sanitise. Defence-in-depth on
// top of feed.Fetch's 10 MiB body cap; per-entry HTML beyond this is
// pathological. Truncation snaps back to a UTF-8 codepoint boundary so
// bluemonday never sees a partial multi-byte sequence.
const MaxInputBytes = 1 << 20 // 1 MiB
```

Modify `Sanitise` to apply the cap as the first thing it does:

```go
func (p *Policy) Sanitise(rawHTML string) string {
	if len(rawHTML) > MaxInputBytes {
		rawHTML = rawHTML[:MaxInputBytes]
		// Snap back to a UTF-8 boundary. UTF-8 codepoints are at most
		// 4 bytes; this loop runs at most 3 times.
		for len(rawHTML) > 0 && !utf8.ValidString(rawHTML) {
			rawHTML = rawHTML[:len(rawHTML)-1]
		}
	}
	cleaned := p.bm.Sanitize(rawHTML)
	return p.postProcess(cleaned)
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/sanitise -run "TestSanitise_TruncatesOversizeInput|TestSanitise_TruncationSnapsToUTF8Boundary" -v`
Expected: both PASS.

- [ ] **Step 6: Run the full sanitise test file**

Run: `go test ./internal/sanitise -v`
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/sanitise/
git commit -m "Cap sanitise input at 1 MiB with UTF-8 boundary snap-back"
```

---

## Task 14: sanitise — observability (DEBUG slog of what got dropped)

**Files:**
- Modify: `internal/sanitise/sanitise.go`
- Modify: `internal/sanitise/sanitise_test.go`

DEBUG-level structured logging so an operator can investigate "X feed looks broken" without attaching a debugger. Off by default (project log level is INFO).

- [ ] **Step 1: Write the failing test**

Append to `internal/sanitise/sanitise_test.go`:

```go
func TestSanitise_LogsDropStats(t *testing.T) {
	t.Parallel()
	// Capture slog output at DEBUG level via a buffer-backed handler.
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	in := `<p>ok</p>` +
		`<iframe src="https://evil.example/x"></iframe>` +
		`<img src="https://t.example/p" width="1" height="1">`
	_ = DefaultPolicy().Sanitise(in)

	out := buf.String()
	for _, want := range []string{`"sanitise"`, `"dropped_iframes":1`, `"dropped_pixel_trackers":1`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in slog output: %s", want, out)
		}
	}
}
```

(Add `"bytes"` and `"log/slog"` to the test file's import block.)

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./internal/sanitise -run TestSanitise_LogsDropStats -v`
Expected: FAIL — no logging exists.

- [ ] **Step 3: Add stat-tracking and DEBUG log in Sanitise**

In `internal/sanitise/sanitise.go`:

1. Add the import:

```go
"log/slog"
```

2. Add a stats struct and modify `walk` to take it. Replace the existing `walk` and `postProcess` methods, and `Sanitise`:

```go
type walkStats struct {
	droppedIframes       int
	droppedPixelTrackers int
}

func (p *Policy) Sanitise(rawHTML string) string {
	truncated := false
	if len(rawHTML) > MaxInputBytes {
		rawHTML = rawHTML[:MaxInputBytes]
		for len(rawHTML) > 0 && !utf8.ValidString(rawHTML) {
			rawHTML = rawHTML[:len(rawHTML)-1]
		}
		truncated = true
	}
	cleaned := p.bm.Sanitize(rawHTML)
	out, stats := p.postProcess(cleaned)
	if truncated || stats.droppedIframes > 0 || stats.droppedPixelTrackers > 0 {
		slog.Debug("sanitise",
			"input_bytes", len(rawHTML),
			"output_bytes", len(out),
			"truncated", truncated,
			"dropped_iframes", stats.droppedIframes,
			"dropped_pixel_trackers", stats.droppedPixelTrackers,
		)
	}
	return out
}

func (p *Policy) postProcess(s string) (string, walkStats) {
	var stats walkStats
	if s == "" {
		return "", stats
	}
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		return s, stats
	}
	for _, n := range nodes {
		p.walk(n, &stats)
	}
	var buf bytes.Buffer
	for _, n := range nodes {
		if err := html.Render(&buf, n); err != nil {
			return s, stats
		}
	}
	return buf.String(), stats
}

func (p *Policy) walk(n *html.Node, stats *walkStats) {
	c := n.FirstChild
	for c != nil {
		next := c.NextSibling
		if c.Type == html.ElementNode {
			switch c.Data {
			case "iframe":
				if !p.iframeHostAllowed(getAttr(c, "src")) {
					n.RemoveChild(c)
					stats.droppedIframes++
					c = next
					continue
				}
			case "img":
				if isPixelTracker(c) {
					n.RemoveChild(c)
					stats.droppedPixelTrackers++
					c = next
					continue
				}
				cleanAttrURL(c, "src")
			case "a":
				cleanAttrURL(c, "href")
			}
		}
		p.walk(c, stats)
		c = next
	}
}
```

- [ ] **Step 4: Run the new test**

Run: `go test ./internal/sanitise -run TestSanitise_LogsDropStats -v`
Expected: PASS.

- [ ] **Step 5: Run the full sanitise test file**

Run: `go test ./internal/sanitise -race -v`
Expected: all PASS — the existing tests don't depend on slog output and shouldn't regress.

- [ ] **Step 6: Commit**

```bash
git add internal/sanitise/
git commit -m "Add DEBUG-level slog stats for sanitiser drops"
```

---

## Task 15: sanitise — concurrent-safety test

**Files:**
- Modify: `internal/sanitise/sanitise_test.go`

`bluemonday.Policy.Sanitize` is documented as safe for concurrent use after construction; the scheduler shares one Policy across N=3 workers. Verify with `-race`.

- [ ] **Step 1: Add the test**

Append to `internal/sanitise/sanitise_test.go`:

```go
func TestSanitise_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()
	inputs := []string{
		`<p>hello</p><script>alert(1)</script>`,
		`<a href="https://e.com/x?utm_source=foo&id=1">link</a>`,
		`<img width="1" height="1" src="https://t.example/p">`,
		`<iframe src="https://www.youtube.com/embed/abc"></iframe>`,
		`<p>` + strings.Repeat("x", 10000) + `</p>`,
	}
	const goroutines = 50
	const perGoroutine = 100
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				_ = p.Sanitise(inputs[(i+j)%len(inputs)])
			}
		}(i)
	}
	wg.Wait()
}
```

(Add `"sync"` to the test file's import block.)

- [ ] **Step 2: Run with -race**

Run: `go test ./internal/sanitise -race -run TestSanitise_ConcurrentSafe -v`
Expected: PASS, no race warnings.

- [ ] **Step 3: Commit**

```bash
git add internal/sanitise/sanitise_test.go
git commit -m "Add concurrent-safety test for shared sanitise.Policy"
```

---

## Task 16: sanitise — total-function contract (no panics on adversarial input)

**Files:**
- Modify: `internal/sanitise/sanitise_test.go`

- [ ] **Step 1: Write the contract tests**

Append to `internal/sanitise/sanitise_test.go`:

```go
func TestSanitise_NeverPanics(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"",
		"<<<>>>",
		`<p>unclosed`,
		`<script>`,
		`<img src=x onerror=alert(1)>`,
		`<iframe`,
		`<!--<script>--><script>alert(1)</script>`,
		strings.Repeat("<div>", 2000) + strings.Repeat("</div>", 2000),
	}
	p := DefaultPolicy()
	for i, in := range inputs {
		// Defer-recover ensures panics surface as test failures.
		func(i int, in string) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("input %d panicked: %v (input=%q)", i, r, in)
				}
			}()
			_ = p.Sanitise(in)
		}(i, in)
	}
}

func TestSanitise_EmptyInputReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := DefaultPolicy().Sanitise(""); got != "" {
		t.Errorf("empty input produced non-empty output: %q", got)
	}
}
```

- [ ] **Step 2: Run the tests**

Run: `go test ./internal/sanitise -run "TestSanitise_NeverPanics|TestSanitise_EmptyInputReturnsEmpty" -v`
Expected: PASS.

- [ ] **Step 3: Run the full test suite with -race to be sure**

Run: `go test ./internal/sanitise -race -v`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/sanitise/sanitise_test.go
git commit -m "Add total-function contract tests for sanitise"
```

---

## Task 17: Worker — carry Policy and sanitise content before insert

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1a: Add the sanitise import**

In `internal/poll/worker_test.go`, add to the existing import block:

```go
"github.com/bcrisp4/tap/internal/sanitise"
```

- [ ] **Step 1b: Append the failing test**

Append to `internal/poll/worker_test.go`:

```go
const hostileAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Hostile</title>
  <id>urn:hostile</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>Bad</title>
    <id>urn:hostile:1</id>
    <link href="https://hostile.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;ok&lt;/p&gt;&lt;script&gt;alert(1)&lt;/script&gt;&lt;a href=&quot;https://e.com/?utm_source=foo&amp;id=1&quot;&gt;link&lt;/a&gt;</content>
  </entry>
</feed>`

func TestWorker_SanitisesContent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hostileAtom))
	}))
	defer srv.Close()

	d := newDB(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Cadence: 30 * time.Minute,
		Policy:  sanitise.DefaultPolicy(),
	})
	w.Run(context.Background(), db.DueSubscription{ID: subID, FeedURL: srv.URL})

	entries, _, _, err := db.ListEntries(context.Background(), d, db.ListEntriesParams{Limit: 100})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Fetch full entry (list response strips body).
	full, err := db.GetEntry(context.Background(), d, entries[0].ID)
	require.NoError(t, err)
	body := full.Content
	require.NotContains(t, body, "<script>", "script tag survived sanitise: %s", body)
	require.NotContains(t, body, "alert", "script body survived sanitise: %s", body)
	require.NotContains(t, body, "utm_source", "tracking param survived urlcleaner: %s", body)
	require.Contains(t, body, "<p>ok</p>", "legitimate paragraph stripped: %s", body)
}
```

(Delete the `import_addition_marker_for_sanitise // ...` line — it's just a placeholder to remind you to add the import. Update the imports block at the top of `worker_test.go` to include `"github.com/bcrisp4/tap/internal/sanitise"`.)

- [ ] **Step 2: Run the test and verify it fails to compile**

Run: `go test ./internal/poll -run TestWorker_SanitisesContent -v`
Expected: COMPILE ERROR — `WorkerOpts` has no field `Policy`. (This is the red.)

- [ ] **Step 3: Add Policy to WorkerOpts and use it in Run**

In `internal/poll/worker.go`:

1. Add the import:

```go
"github.com/bcrisp4/tap/internal/sanitise"
```

2. Replace the `WorkerOpts` and `Worker` types and `NewWorker` constructor:

```go
type WorkerOpts struct {
	Cadence time.Duration    // fixed retry/next-poll interval for M1
	Policy  *sanitise.Policy // sanitiser applied to every entry's HTML body. Required (panics on nil).
}

type Worker struct {
	db     *sql.DB
	client *http.Client
	opts   WorkerOpts
	policy *sanitise.Policy
}

// NewWorker requires a non-nil Policy. Defaulting it here would silently
// hide tests that forget to pass one — the public construction surface
// (Scheduler) supplies the production default.
func NewWorker(d *sql.DB, c *http.Client, o WorkerOpts) *Worker {
	if o.Policy == nil {
		panic("poll.NewWorker: Policy is required")
	}
	if o.Cadence <= 0 {
		o.Cadence = 30 * time.Minute
	}
	if c == nil {
		c = http.DefaultClient
	}
	return &Worker{db: d, client: c, opts: o, policy: o.Policy}
}
```

3. In `Run`, modify the loop that builds `newEntries` to sanitise the content. Locate the existing block (around lines 65–83) and replace it:

```go
newEntries := make([]db.NewEntry, 0, len(res.Feed.Items))
for _, item := range res.Feed.Items {
	pubAt := now
	if item.PublishedParsed != nil {
		pubAt = item.PublishedParsed.Unix()
	}
	content := item.Content
	if content == "" {
		content = item.Description
	}
	content = w.policy.Sanitise(content)
	newEntries = append(newEntries, db.NewEntry{
		Hash:        feed.EntryHash(sub.ID, item),
		Title:       item.Title,
		Author:      authorName(item),
		URL:         item.Link,
		Content:     content,
		PublishedAt: pubAt,
	})
}
```

- [ ] **Step 4: Update the existing worker tests to pass a Policy**

In `internal/poll/worker_test.go`, find both calls to `NewWorker` in `TestWorker_SuccessfulPoll` and `TestWorker_ErrorIncrementsCount`. Add `Policy: sanitise.DefaultPolicy(),` to each `WorkerOpts` literal:

```go
w := NewWorker(d, http.DefaultClient, WorkerOpts{
    Cadence: 30 * time.Minute,
    Policy:  sanitise.DefaultPolicy(),
})
```

(Both call sites need the same edit.)

- [ ] **Step 5: Run the full poll-package test suite**

Run: `go test ./internal/poll -race -v`
Expected: all PASS, including the new `TestWorker_SanitisesContent`.

- [ ] **Step 6: Commit**

```bash
git add internal/poll/worker.go internal/poll/worker_test.go
git commit -m "Wire sanitise.Policy into the polling worker"
```

---

## Task 18: Scheduler — thread Policy through SchedulerOpts

**Files:**
- Modify: `internal/poll/scheduler.go`
- Modify: `internal/poll/scheduler_test.go` (only if it constructs `SchedulerOpts` and needs updating — usually not)

- [ ] **Step 1: Add Policy to SchedulerOpts and pass it to NewWorker**

In `internal/poll/scheduler.go`:

1. Add the import:

```go
"github.com/bcrisp4/tap/internal/sanitise"
```

2. Replace `SchedulerOpts` to add the `Policy` field:

```go
type SchedulerOpts struct {
	TickInterval time.Duration    // default 60s
	Workers      int              // default 3
	Cadence      time.Duration    // default 30m
	Policy       *sanitise.Policy // default sanitise.DefaultPolicy()
}
```

3. In `NewScheduler`, default the Policy and pass it to `NewWorker`. Replace the relevant lines (the existing `if o.Cadence` defaulting block and the `worker:` line of the struct literal):

```go
if o.Cadence <= 0 {
    o.Cadence = 30 * time.Minute
}
if o.Policy == nil {
    o.Policy = sanitise.DefaultPolicy()
}
parentCtx, parentCancel := context.WithCancel(base)
s := &Scheduler{
    db:           d,
    client:       c,
    opts:         o,
    inflight:     NewInflight(),
    worker:       NewWorker(d, c, WorkerOpts{Cadence: o.Cadence, Policy: o.Policy}),
    jobs:         make(chan db.DueSubscription, o.Workers*2),
    tickDone:     make(chan struct{}),
    poke:         make(chan struct{}, 1),
    parentCtx:    parentCtx,
    parentCancel: parentCancel,
}
```

- [ ] **Step 2: Run the full poll-package test suite**

Run: `go test ./internal/poll -race -v`
Expected: all PASS. (No test changes required — the Policy defaults to `DefaultPolicy()` when callers omit it, which is the path existing tests take.)

- [ ] **Step 3: Commit**

```bash
git add internal/poll/scheduler.go
git commit -m "Thread sanitise.Policy through SchedulerOpts"
```

---

## Task 19: main.go — construct DefaultPolicy and pass it to NewScheduler

**Files:**
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Wire the policy in main**

In `cmd/tap/main.go`:

1. Add the import:

```go
"github.com/bcrisp4/tap/internal/sanitise"
```

2. Replace the `sched := poll.NewScheduler(...)` line (currently `sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{})`) with:

```go
sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
    Policy: sanitise.DefaultPolicy(),
})
```

- [ ] **Step 2: Build to confirm everything compiles**

Run: `go build ./cmd/tap`
Expected: builds cleanly.

- [ ] **Step 3: Run the full test suite**

Run: `make test`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/tap/main.go
git commit -m "Construct sanitise.DefaultPolicy and pass it to scheduler"
```

---

## Task 20: End-to-end test — hostile-fixture entry round-trips clean

**Files:**
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Replace the fixture entry in the e2e test with a hostile one**

In `cmd/tap/main_test.go`, locate the `const atom = ...` block. Replace it with:

```go
const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>End-to-end</title>
  <id>urn:e2e</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>Hello</title>
    <id>urn:e2e:1</id>
    <link href="https://e2e.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;hello&lt;/p&gt;&lt;script&gt;alert(1)&lt;/script&gt;&lt;a href=&quot;https://e.com/?utm_source=feed&amp;id=1&quot; onclick=&quot;evil()&quot;&gt;link&lt;/a&gt;</content>
  </entry>
</feed>`
```

- [ ] **Step 2: Construct the scheduler with a real Policy and assert sanitised round-trip**

Two changes to the test body:

a) Replace the scheduler construction (currently `sched := poll.NewScheduler(context.Background(), d, http.DefaultClient, poll.SchedulerOpts{Workers: 1, Cadence: time.Hour})`) with:

```go
sched := poll.NewScheduler(context.Background(), d, http.DefaultClient, poll.SchedulerOpts{
    Workers: 1,
    Cadence: time.Hour,
    Policy:  sanitise.DefaultPolicy(),
})
```

b) After the `GET /api/v1/entries` block, add a `GET /api/v1/entries/{id}` round-trip and assert sanitisation. Append at the end of the test (before the closing `}`):

```go
// Detail GET — body must be sanitised.
entryID := int64(resp.Data[0]["id"].(float64))
detailURL := "/api/v1/entries/" + strconv.FormatInt(entryID, 10)
rr3 := httptest.NewRecorder()
mux.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, detailURL, nil))
require.Equal(t, http.StatusOK, rr3.Code, rr3.Body.String())

var detail struct {
    Content string `json:"content"`
}
require.NoError(t, json.NewDecoder(rr3.Body).Decode(&detail))
require.NotContains(t, detail.Content, "<script>")
require.NotContains(t, detail.Content, "alert")
require.NotContains(t, detail.Content, "onclick")
require.NotContains(t, detail.Content, "utm_source")
require.Contains(t, detail.Content, "<p>hello</p>")
```

- [ ] **Step 3: Add required imports**

The new test code uses `strconv` and `sanitise`. Add to the import block at the top of `cmd/tap/main_test.go`:

```go
"strconv"

"github.com/bcrisp4/tap/internal/sanitise"
```

- [ ] **Step 4: Run the test**

Run: `make test`
Expected: all PASS, including the updated `TestEndToEnd_SubscribePollServeEntries`.

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/main_test.go
git commit -m "Extend e2e test to assert hostile-fixture content round-trips clean"
```

---

## Task 21: README — replace M1 deployment-safety section

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Replace the "M1 deployment safety" section**

In `README.md`, replace the entire `## M1 deployment safety` section (and its body) with:

```markdown
## Trust posture

Feed HTML is sanitised on the server before storage (M2): scripts, on-event
handlers, dangerous URL schemes, iframes outside a small allowlist, 1×1
tracking pixels, and well-known tracking parameters in `<a href>` and
`<img src>` URLs are all stripped. The SPA renders the stored HTML
directly without a runtime sanitiser.

The binary still defaults to `-addr 127.0.0.1:8080` as defence in depth
(concept §6.11). The container variant binds `0.0.0.0:8080` because
Docker port mapping requires it.

## Upgrading from M1

M1 databases are incompatible with M2 — the entries table holds raw HTML
that the M2 sanitiser was never run against. Before starting M2:

- **Binary deployment:** delete `tap.db` from your data directory and
  re-subscribe.
- **Container deployment:** delete the `/data` volume (or its `tap.db`
  file) and re-subscribe.
```

- [ ] **Step 2: Verify the README still parses cleanly**

Run: `cat README.md`
Expected: file is well-formed; the old "Do not reverse-proxy the M1 container..." paragraph is gone.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "Update README for M2 trust posture and M1 upgrade note"
```

---

## Task 22: Final smoke — full test suite, race detector, build artifact

- [ ] **Step 1: Full test suite with race detector**

Run: `make test`
Expected: every package passes; no race warnings. (This is the real verification gate.)

- [ ] **Step 2: Static binary build**

Run: `make build`
Expected: produces `bin/tap` cleanly. (Confirms the embed + Go build paths still work end-to-end.)

- [ ] **Step 3: Optional offline smoke against a local fixture**

Skip this step in CI. Run only locally if you want manual eyeballs on the running binary. Avoid hitting live feeds — flaky and not under our control.

```bash
rm -rf data/
./bin/tap -addr 127.0.0.1:8080 &
TAP_PID=$!

# Subscribe to a localhost fixture, not a real feed, so the test is hermetic.
# (Easiest: start a small `python3 -m http.server` from a directory with
# an atom.xml, or use any local feed file you've staged.)
# Adjust the URL accordingly:
curl -sS -X POST http://127.0.0.1:8080/api/v1/subscriptions \
  -H 'content-type: application/json' \
  -d '{"feed_url":"http://127.0.0.1:8000/atom.xml"}'
sleep 2
curl -sS 'http://127.0.0.1:8080/api/v1/entries?unread=1&limit=3' | head -c 1000

kill $TAP_PID
```

Expected: subscription created (HTTP 201), entries appear within seconds, returned content has no `<script>` tags.

- [ ] **Step 4: If make test and make build are green, M2 is done**

Report back to the user. Ready for review.

---

## Notes for the engineer

- **TDD discipline.** Per `docs/roadmap.md` §"Working cadence", every behaviour-bearing change is red → green → refactor. The plan is structured this way: write the test, see it fail, implement, see it pass, commit. Don't shortcut — a test that doesn't fail in red doesn't exercise real code.
- **Don't pre-commit code that isn't tested.** Each task's "commit" step is the natural checkpoint.
- **British spelling.** The package is `sanitise` (not `sanitize`); methods are `Sanitise` (not `Sanitize`). Concept doc uses British spelling throughout. bluemonday's API uses American (`Sanitize`) — that's fine, it's an upstream library.
- **Don't expand the iframe allowlist beyond the 13 hosts in `defaultIframeHosts`.** The spec is deliberate about matching miniflux's list verbatim. Per-user override is a deferred item.
- **Don't add a tracker host blocklist.** The spec is deliberate about not maintaining one. Pixel-size heuristic + URL parameter cleaning is the policy.
- **If a step doesn't fit a 2–5 minute window**, you've likely conflated two changes. Split, commit each, move on.
