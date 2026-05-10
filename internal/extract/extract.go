// Package extract fetches an article URL and returns its main body as
// HTML, either via Mozilla Readability heuristics or a per-feed CSS
// selector. The output is raw HTML — sanitisation and image-URL
// rewriting are the caller's job (the polling worker runs the result
// through processor.Process).
package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/andybalholm/cascadia"
	readability "codeberg.org/readeck/go-readability/v2"
	"golang.org/x/net/html"
)

// Extract fetches articleURL via client and returns the article body
// as HTML. selector empty → Readability mode; non-empty → CSS-selector
// mode. bodyCap caps the response body before parsing.
//
// Errors on: HTTP non-2xx, body cap exceeded, missing or non-HTML
// Content-Type, malformed selector, selector matched no node,
// Readability returned empty content.
//
// The fetch uses the caller-supplied client so the M4 SSRF guard,
// per-host limiter, and timeout apply uniformly with feed and proxy
// fetches.
func Extract(ctx context.Context, client *http.Client, articleURL, selector string, bodyCap int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, articleURL, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "text/html, application/xhtml+xml;q=0.9, */*;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !isHTMLContentType(ct) {
		return "", fmt.Errorf("non-HTML content-type %q", ct)
	}

	body, err := readCapped(resp.Body, bodyCap)
	if err != nil {
		return "", err
	}

	pageURL, perr := url.Parse(articleURL)
	if perr != nil {
		return "", fmt.Errorf("parse article URL: %w", perr)
	}

	if selector != "" {
		sel, serr := cascadia.Compile(selector)
		if serr != nil {
			return "", fmt.Errorf("compile selector: %w", serr)
		}
		doc, perr := html.Parse(bytes.NewReader(body))
		if perr != nil {
			return "", fmt.Errorf("parse html: %w", perr)
		}
		match := sel.MatchFirst(doc)
		if match == nil {
			return "", errors.New("selector matched no node")
		}
		var buf bytes.Buffer
		if err := html.Render(&buf, match); err != nil {
			return "", fmt.Errorf("render selector match: %w", err)
		}
		out := strings.TrimSpace(buf.String())
		if out == "" {
			return "", errors.New("selector match rendered empty")
		}
		return out, nil
	}

	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return "", fmt.Errorf("readability: %w", err)
	}
	if article.Node == nil {
		return "", errors.New("readability returned empty content")
	}
	var buf bytes.Buffer
	if err := article.RenderHTML(&buf); err != nil {
		return "", fmt.Errorf("render html: %w", err)
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return "", errors.New("readability returned empty content")
	}
	return out, nil
}

// readCapped reads up to cap+1 bytes; returns an error if the body
// exceeds cap. We read one extra byte so we can distinguish "exactly
// at the cap" from "the body was truncated."
func readCapped(r io.Reader, cap int64) ([]byte, error) {
	if cap <= 0 {
		cap = 5 << 20 // 5 MiB sane default if a caller passes 0
	}
	body, err := io.ReadAll(io.LimitReader(r, cap+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > cap {
		return nil, fmt.Errorf("body exceeds %d-byte cap", cap)
	}
	return body, nil
}

// isHTMLContentType returns true for text/html and application/xhtml+xml,
// optionally with parameters (charset etc.). Comparison is on the media
// type only; parameters are ignored.
func isHTMLContentType(ct string) bool {
	if ct == "" {
		return false
	}
	mt := strings.TrimSpace(strings.ToLower(strings.SplitN(ct, ";", 2)[0]))
	return mt == "text/html" || mt == "application/xhtml+xml"
}
