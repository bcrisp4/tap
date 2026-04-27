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

// Process runs extract → media rewrite → sanitize and returns the
// final body HTML, ready for storage in entries.content.
func (p *Pipeline) Process(entryHTML, articleURL, scraperRules string) (string, error) {
	extracted, err := Extract(entryHTML, articleURL, scraperRules)
	if err != nil {
		return "", err
	}
	rewritten, err := RewriteMedia(extracted, articleURL, p.cfg.Encode)
	if err != nil {
		return "", err
	}
	return Sanitize(rewritten, SanitizeOptions{
		ArticleURL:      articleURL,
		IframeAllowlist: p.cfg.IframeAllowlist,
	})
}
