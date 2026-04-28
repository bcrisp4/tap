package reader

// PipelineConfig assembles the dependencies the pipeline needs.
type PipelineConfig struct {
	// Encode produces the proxy URL for a given absolute source URL.
	// Plan 06 supplies the real implementation; tests use a stub.
	Encode ProxyEncoder
	// IframeAllowlist overrides the spec's default host allowlist.
	// Empty means use DefaultIframeHosts().
	IframeAllowlist []string
}

// Pipeline runs the content pipeline (extract → media-rewrite →
// sanitize) on article HTML. One per worker is fine — the type is
// stateless.
type Pipeline struct {
	cfg PipelineConfig
}

// NewPipeline builds a Pipeline. Cheap; one per worker is fine.
//
// Defaults:
//   - IframeAllowlist falls back to DefaultIframeHosts() when empty.
//   - Encode falls back to identity (no rewriting), so tests that
//     don't care about media URLs don't have to supply a stub.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	if len(cfg.IframeAllowlist) == 0 {
		cfg.IframeAllowlist = DefaultIframeHosts()
	}
	if cfg.Encode == nil {
		cfg.Encode = func(u string) string { return u }
	}
	return &Pipeline{cfg: cfg}
}

// Extract runs the optional extraction layer (CSS rules → go-readability
// fallback) over a fetched article HTML. Plan 15 splits this from the
// universal media-rewrite + sanitize so callers that already have body
// HTML (e.g. feed-supplied summaries) can skip extraction and still
// benefit from the proxy + sanitiser.
func (p *Pipeline) Extract(entryHTML, articleURL, scraperRules string) (string, error) {
	return Extract(entryHTML, articleURL, scraperRules)
}

// RewriteAndSanitize rewrites <img>/<source> URLs through the proxy
// encoder and sanitizes the result via the bluemonday policy. Universal:
// every entry's content (extracted or feed-supplied) flows through this
// step before storage so right-click→copy-image-URL always yields a
// /api/v1/proxy/<token> URL.
func (p *Pipeline) RewriteAndSanitize(entryHTML, articleURL string) (string, error) {
	rewritten, err := RewriteMedia(entryHTML, articleURL, p.cfg.Encode)
	if err != nil {
		return "", err
	}
	return Sanitize(rewritten, SanitizeOptions{
		ArticleURL:      articleURL,
		IframeAllowlist: p.cfg.IframeAllowlist,
	})
}

// Process runs Extract → RewriteAndSanitize and returns the final body
// HTML, ready for storage in entries.content. Convenience wrapper for
// callers that want the full crawler-mode pipeline.
func (p *Pipeline) Process(entryHTML, articleURL, scraperRules string) (string, error) {
	extracted, err := p.Extract(entryHTML, articleURL, scraperRules)
	if err != nil {
		return "", err
	}
	return p.RewriteAndSanitize(extracted, articleURL)
}
